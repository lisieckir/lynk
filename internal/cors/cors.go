// Package cors implements Lynk's small, fail-closed browser CORS policy.
//
// Lynk authenticates API clients with an Authorization token, not browser
// cookies. The policy therefore never enables credentialed requests and never
// emits a wildcard origin: an Origin header is echoed only when it exactly
// matches an explicitly configured origin. Requests from any other origin are
// left untouched for non-browser clients and simply receive no Access-Control-*
// headers, which makes the browser discard the response. Preflight requests are
// answered without authentication, but only for configured origins, methods
// and headers.
package cors

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	// DefaultMaxAgeSeconds is the preflight cache lifetime when no explicit
	// value is configured.
	DefaultMaxAgeSeconds = 600
	// MaxMaxAgeSeconds caps the preflight cache lifetime at 24 hours so a
	// policy change cannot be pinned in browsers indefinitely.
	MaxMaxAgeSeconds = 86400
)

// DefaultMethods and DefaultHeaders are deliberately minimal. In particular,
// Authorization is not a default: a web client can only preflight token
// requests when the operator explicitly lists that header.
const (
	DefaultMethods = "GET, POST"
	DefaultHeaders = "Content-Type"
)

var knownMethods = []string{
	http.MethodGet, http.MethodHead, http.MethodPost,
	http.MethodPut, http.MethodPatch, http.MethodDelete,
}

// Options is the operator-supplied CORS policy. Every list is comma separated;
// an empty AllowedOrigins list means that no browser origin is granted CORS
// access. The zero value is therefore safe.
type Options struct {
	AllowedOrigins string
	AllowedMethods string
	AllowedHeaders string
	MaxAgeSeconds  int
}

// DefaultOptions returns the safe defaults: no allowed origins, GET/POST,
// Content-Type and a ten-minute preflight cache.
func DefaultOptions() Options {
	return Options{
		AllowedOrigins: "",
		AllowedMethods: DefaultMethods,
		AllowedHeaders: DefaultHeaders,
		MaxAgeSeconds:  DefaultMaxAgeSeconds,
	}
}

// Policy is a validated, immutable allow-list. Build it with New.
type Policy struct {
	origins    map[string]struct{}
	methods    map[string]struct{}
	headers    map[string]struct{}
	methodList string
	headerList string
	maxAge     int
	active     bool
}

// New validates opts and builds an immutable policy. An empty origin list is
// valid and produces an inactive policy that grants no CORS access.
func New(opts Options) (*Policy, error) {
	origins, err := parseOrigins(opts.AllowedOrigins)
	if err != nil {
		return nil, fmt.Errorf("allowed origins: %w", err)
	}
	methods, err := parseMethods(opts.AllowedMethods)
	if err != nil {
		return nil, fmt.Errorf("allowed methods: %w", err)
	}
	headers, err := parseHeaders(opts.AllowedHeaders)
	if err != nil {
		return nil, fmt.Errorf("allowed headers: %w", err)
	}
	if opts.MaxAgeSeconds < 0 || opts.MaxAgeSeconds > MaxMaxAgeSeconds {
		return nil, fmt.Errorf("max age: must be between 0 and %d seconds", MaxMaxAgeSeconds)
	}

	p := &Policy{
		origins:    setOf(origins),
		methods:    setOf(methods),
		headers:    setOf(headers),
		methodList: strings.Join(methods, ", "),
		headerList: strings.Join(headers, ", "),
		maxAge:     opts.MaxAgeSeconds,
		active:     len(origins) > 0,
	}
	return p, nil
}

// Handler wraps next with the CORS policy. A nil policy or an inactive policy
// is a no-op, preserving the original API behavior.
func (p *Policy) Handler(next http.Handler) http.Handler {
	if p == nil || !p.active {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		addVary(w, "Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := p.origins[origin]; !ok {
			// A preflight from an unexpected origin is rejected outright.
			// Other requests continue unauthenticated/authorized as usual but
			// receive no Access-Control-* headers, so a browser cannot read
			// the response and non-browser clients are not broken.
			if isPreflight(r) {
				forbidden(w)
			} else {
				next.ServeHTTP(w, r)
			}
			return
		}
		if isPreflight(r) {
			p.preflight(w, r, origin)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		next.ServeHTTP(w, r)
	})
}

func (p *Policy) preflight(w http.ResponseWriter, r *http.Request, origin string) {
	method := strings.ToUpper(strings.TrimSpace(r.Header.Get("Access-Control-Request-Method")))
	if _, ok := p.methods[method]; !ok {
		forbidden(w)
		return
	}
	for _, requested := range splitList(r.Header.Get("Access-Control-Request-Headers")) {
		if _, ok := p.headers[http.CanonicalHeaderKey(strings.TrimSpace(requested))]; !ok {
			forbidden(w)
			return
		}
	}

	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Access-Control-Allow-Methods", p.methodList)
	if p.headerList != "" {
		h.Set("Access-Control-Allow-Headers", p.headerList)
	}
	if p.maxAge > 0 {
		h.Set("Access-Control-Max-Age", strconv.Itoa(p.maxAge))
	}
	addVary(w, "Access-Control-Request-Method", "Access-Control-Request-Headers")
	w.WriteHeader(http.StatusNoContent)
}

func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}

func forbidden(w http.ResponseWriter) {
	// No body and no echo of the Origin/token: an attacker must not learn
	// which origins are configured from an error response.
	w.WriteHeader(http.StatusForbidden)
}

func addVary(w http.ResponseWriter, names ...string) {
	h := w.Header()
	seen := make(map[string]struct{})
	for _, value := range h.Values("Vary") {
		for _, part := range strings.Split(value, ",") {
			seen[http.CanonicalHeaderKey(strings.TrimSpace(part))] = struct{}{}
		}
	}
	for _, name := range names {
		key := http.CanonicalHeaderKey(name)
		if _, ok := seen[key]; ok {
			continue
		}
		h.Add("Vary", name)
		seen[key] = struct{}{}
	}
}

func parseOrigins(raw string) ([]string, error) {
	items := splitList(raw)
	if len(items) == 0 {
		return nil, nil
	}
	origins := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		origin, err := normalizeOrigin(item)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	sort.Strings(origins)
	return origins, nil
}

// normalizeOrigin accepts only a serialized browser origin: an absolute
// http(s) scheme and host without user info, path, query or fragment. A
// wildcard is never accepted because it cannot be combined with a safe
// credential policy and would turn the allow-list into a public API.
func normalizeOrigin(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("origin must not be empty")
	}
	if strings.ContainsAny(raw, " \t\r\n") {
		return "", fmt.Errorf("origin %q contains whitespace", raw)
	}
	if strings.Contains(raw, "*") {
		return "", fmt.Errorf("origin %q must be explicit; wildcard is not allowed", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid origin %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("origin %q must use http or https", raw)
	}
	if u.Opaque != "" || u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("origin %q must have an absolute host", raw)
	}
	if u.User != nil {
		return "", fmt.Errorf("origin %q must not contain credentials", raw)
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("origin %q must not contain a path", raw)
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", fmt.Errorf("origin %q must not contain a query or fragment", raw)
	}
	return u.Scheme + "://" + strings.ToLower(u.Host), nil
}

func parseMethods(raw string) ([]string, error) {
	items := splitList(raw)
	if len(items) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(items))
	for _, method := range knownMethods {
		seen[method] = struct{}{}
	}
	result := make([]string, 0, len(items))
	requested := make(map[string]struct{}, len(items))
	for _, item := range items {
		method := strings.ToUpper(strings.TrimSpace(item))
		if strings.Contains(method, "*") {
			return nil, fmt.Errorf("method %q must be explicit; wildcard is not allowed", item)
		}
		if _, ok := seen[method]; !ok {
			return nil, fmt.Errorf("unsupported method %q", item)
		}
		requested[method] = struct{}{}
	}
	// Emit methods in the fixed API order, not the configuration order.
	for _, method := range knownMethods {
		if _, ok := requested[method]; ok {
			result = append(result, method)
		}
	}
	return result, nil
}

func parseHeaders(raw string) ([]string, error) {
	items := splitList(raw)
	if len(items) == 0 {
		return nil, nil
	}
	headers := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if strings.Contains(name, "*") {
			return nil, fmt.Errorf("header %q must be explicit; wildcard is not allowed", item)
		}
		if !validHeaderName(name) {
			return nil, fmt.Errorf("invalid header name %q", item)
		}
		canonical := http.CanonicalHeaderKey(name)
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		headers = append(headers, canonical)
	}
	sort.Strings(headers)
	return headers, nil
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.ContainsRune("!#$%&'+-.^_`|~", c):
		default:
			return false
		}
	}
	return true
}

func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var items []string
	for _, item := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func setOf(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
