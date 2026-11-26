package metadata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrBlockedURL is returned when the URL is not allowed by SSRF policy.
	ErrBlockedURL = errors.New("metadata: blocked url")
)

// Result holds extracted metadata fields.
type Result struct {
	Title           string
	Description     string
	OGTitle         string
	OGDescription   string
	FaviconURL      string
	PreviewImageURL string
}

// Config controls fetching behavior.
type Config struct {
	Timeout        time.Duration
	ConnectTimeout time.Duration
	MaxRedirects   int
	MaxBytes       int64
}

func defaultConfig() Config {
	return Config{
		Timeout:        10 * time.Second,
		ConnectTimeout: 5 * time.Second,
		MaxRedirects:   3,
		MaxBytes:       2 * 1024 * 1024, // 2MB
	}
}

// Fetcher fetches and parses metadata with SSRF protections.
type Fetcher struct {
	cfg    Config
	client *http.Client
}

func NewFetcher(cfg Config) *Fetcher {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultConfig().Timeout
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = defaultConfig().ConnectTimeout
	}
	if cfg.MaxRedirects <= 0 {
		cfg.MaxRedirects = defaultConfig().MaxRedirects
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = defaultConfig().MaxBytes
	}
	f := &Fetcher{cfg: cfg}
	f.client = &http.Client{
		Timeout: cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("too many redirects")
			}
			if err := validateURL(req.URL); err != nil {
				return err
			}
			if err := validateHost(req.Context(), req.URL.Hostname()); err != nil {
				return err
			}
			return nil
		},
	}
	f.client.Transport = &http.Transport{
		DialContext: f.safeDialContext(),
	}
	return f
}

func validateURL(u *url.URL) error {
	if u == nil {
		return ErrBlockedURL
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: scheme not allowed", ErrBlockedURL)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrBlockedURL)
	}
	h := strings.ToLower(host)
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || h == "metadata.google.internal" || h == "metadata" {
		return fmt.Errorf("%w: localhost/metadata host", ErrBlockedURL)
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	return false
}

func validateHost(ctx context.Context, host string) error {
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrBlockedURL)
	}
	h := strings.ToLower(host)
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || h == "metadata.google.internal" || h == "metadata" {
		return fmt.Errorf("%w: localhost/metadata host", ErrBlockedURL)
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w: ip blocked", ErrBlockedURL)
		}
		return nil
	}
	resolver := net.DefaultResolver
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return fmt.Errorf("%w: no addresses", ErrBlockedURL)
	}
	for _, a := range addrs {
		if isBlockedIP(a.IP) {
			return fmt.Errorf("%w: resolved ip blocked", ErrBlockedURL)
		}
	}
	return nil
}

func (f *Fetcher) safeDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: f.cfg.ConnectTimeout}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if err := validateHost(ctx, host); err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}
		resolver := net.DefaultResolver
		addrs, err := resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, a := range addrs {
			if isBlockedIP(a.IP) {
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(a.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("%w: no valid addresses to dial", ErrBlockedURL)
	}
}

// Fetch retrieves and parses metadata from the given URL.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (Result, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return Result{}, err
	}
	if err := validateURL(u); err != nil {
		return Result{}, err
	}
	if err := validateHost(ctx, u.Hostname()); err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", "Lynk-MetadataFetcher/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	resp, err := f.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(strings.ToLower(ct), "text/html") && !strings.Contains(strings.ToLower(ct), "application/xhtml") {
		// Not HTML, skip parsing
		return Result{}, nil
	}
	limited := io.LimitReader(resp.Body, f.cfg.MaxBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return Result{}, err
	}
	res := parseHTML(body)
	// Resolve relative URLs
	base := resp.Request.URL
	if res.FaviconURL != "" {
		if abs, err := base.Parse(res.FaviconURL); err == nil {
			res.FaviconURL = abs.String()
		}
	}
	if res.PreviewImageURL != "" {
		if abs, err := base.Parse(res.PreviewImageURL); err == nil {
			res.PreviewImageURL = abs.String()
		}
	}
	return res, nil
}
