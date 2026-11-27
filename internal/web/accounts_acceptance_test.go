package web_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/api/linkding"
	"github.com/lisieckir/lynk/internal/auth"
	"github.com/lisieckir/lynk/internal/database"
	"github.com/lisieckir/lynk/internal/web"
)

// This file turns the P2-012 acceptance criteria into committed HTTP-level
// tests against a real migrated SQLite database and the real web/API handlers.
// Every client is a browser-like http.Client with a cookie jar and redirects
// disabled, so cookie paths, CSRF double submits and redirects are observable.

type webResult struct {
	code    int
	loc     string
	body    string
	cookies []*http.Cookie
}

var (
	hiddenInputRE  = regexp.MustCompile(`name="([^"]+)"\s+value="([^"]*)"`)
	oneTimeTokenRE = regexp.MustCompile(`<pre[^>]*>\s*([0-9a-f]{40})\s*</pre>`)
	hex40RE        = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

func browser() (*http.Client, *cookiejar.Jar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, err
	}
	return &http.Client{
		Jar:     jar,
		Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, jar, nil
}

func newBrowser(t *testing.T) (*http.Client, *cookiejar.Jar) {
	t.Helper()
	client, jar, err := browser()
	if err != nil {
		t.Fatalf("create browser: %v", err)
	}
	return client, jar
}

func send(client *http.Client, method, target string, form url.Values, extra ...*http.Cookie) (webResult, error) {
	var reader io.Reader
	if form != nil {
		reader = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, target, reader)
	if err != nil {
		return webResult{}, err
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range extra {
		req.AddCookie(c)
	}
	resp, err := client.Do(req)
	if err != nil {
		return webResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return webResult{}, err
	}
	return webResult{
		code:    resp.StatusCode,
		loc:     resp.Header.Get("Location"),
		body:    string(body),
		cookies: resp.Cookies(),
	}, nil
}

func doWeb(t *testing.T, client *http.Client, method, target string, form url.Values, extra ...*http.Cookie) webResult {
	t.Helper()
	res, err := send(client, method, target, form, extra...)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	return res
}

func findHidden(body, name string) (string, bool) {
	for _, match := range hiddenInputRE.FindAllStringSubmatch(body, -1) {
		if match[1] == name {
			return match[2], true
		}
	}
	return "", false
}

func hiddenValue(t *testing.T, body, name string) string {
	t.Helper()
	value, ok := findHidden(body, name)
	if !ok || value == "" {
		t.Fatalf("hidden input %q missing from body: %s", name, body)
	}
	return value
}

func mustParseURL(target string) *url.URL {
	u, err := url.Parse(target)
	if err != nil {
		panic(fmt.Sprintf("parse %q: %v", target, err))
	}
	return u
}

func cookieValue(t *testing.T, jar *cookiejar.Jar, target, name string) string {
	t.Helper()
	u := mustParseURL(target)
	for _, c := range jar.Cookies(u) {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func newEmptyServer(t *testing.T, cfg web.Config) (*httptest.Server, *sql.DB) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(context.Background(), database.Config{Path: dbPath})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	srv := httptest.NewServer(newMux(t, db, cfg))
	t.Cleanup(srv.Close)
	return srv, db
}

func newMux(t *testing.T, db *sql.DB, cfg web.Config) http.Handler {
	t.Helper()
	h, err := web.NewHandler(db, nil, cfg)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return mux
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return n
}

func registerFirstAdmin(t *testing.T, srv *httptest.Server, db *sql.DB) (*http.Client, *cookiejar.Jar, int64) {
	t.Helper()
	client, jar := newBrowser(t)
	res := doWeb(t, client, http.MethodGet, srv.URL+"/register", nil)
	if res.code != http.StatusOK {
		t.Fatalf("GET /register = %d, body=%s", res.code, res.body)
	}
	csrf := hiddenValue(t, res.body, "csrf_token")
	res = doWeb(t, client, http.MethodPost, srv.URL+"/register", url.Values{
		"csrf_token": {csrf},
		"username":   {"smoke-admin"},
		"password":   {"SmokePassw0rd!"},
	})
	if res.code != http.StatusSeeOther || res.loc != "/" {
		t.Fatalf("POST /register = %d %q, body=%s", res.code, res.loc, res.body)
	}
	if cookieValue(t, jar, srv.URL+"/", web.SessionCookieName) == "" {
		t.Fatal("registration did not set a session cookie")
	}
	var id int64
	if err := db.QueryRow("SELECT id FROM users WHERE username = 'smoke-admin'").Scan(&id); err != nil {
		t.Fatalf("look up admin id: %v", err)
	}
	return client, jar, id
}

func loginPassword(t *testing.T, srv *httptest.Server, username, password string) (*http.Client, *cookiejar.Jar) {
	t.Helper()
	client, jar := newBrowser(t)
	res := doWeb(t, client, http.MethodGet, srv.URL+"/login", nil)
	if res.code != http.StatusOK {
		t.Fatalf("GET /login = %d, body=%s", res.code, res.body)
	}
	csrf := hiddenValue(t, res.body, "csrf_token")
	res = doWeb(t, client, http.MethodPost, srv.URL+"/login", url.Values{
		"csrf_token": {csrf},
		"username":   {username},
		"password":   {password},
	})
	if res.code != http.StatusSeeOther || res.loc != "/" {
		t.Fatalf("POST /login = %d %q, body=%s", res.code, res.loc, res.body)
	}
	if cookieValue(t, jar, srv.URL+"/", web.SessionCookieName) == "" {
		t.Fatal("password login did not set a session cookie")
	}
	return client, jar
}

func adminCSRF(t *testing.T, client *http.Client, srv *httptest.Server) string {
	t.Helper()
	res := doWeb(t, client, http.MethodGet, srv.URL+"/admin/users", nil)
	if res.code != http.StatusOK {
		t.Fatalf("GET /admin/users = %d, body=%s", res.code, res.body)
	}
	return hiddenValue(t, res.body, "csrf_token")
}

func adminAction(t *testing.T, client *http.Client, srv *httptest.Server, csrf, action string, uid int64, newPassword string) webResult {
	t.Helper()
	form := url.Values{
		"csrf_token": {csrf},
		"action":     {action},
		"user_id":    {strconv.FormatInt(uid, 10)},
	}
	if newPassword != "" {
		form.Set("new_password", newPassword)
	}
	res := doWeb(t, client, http.MethodPost, srv.URL+"/admin/users/action", form)
	if res.code != http.StatusSeeOther {
		t.Fatalf("admin action %q = %d, body=%s", action, res.code, res.body)
	}
	return res
}

func createTokenViaUI(t *testing.T, srv *httptest.Server, client *http.Client) (string, webResult) {
	t.Helper()
	res := doWeb(t, client, http.MethodGet, srv.URL+"/settings/tokens", nil)
	if res.code != http.StatusOK {
		t.Fatalf("GET /settings/tokens = %d, body=%s", res.code, res.body)
	}
	csrf := hiddenValue(t, res.body, "csrf_token")
	created := doWeb(t, client, http.MethodPost, srv.URL+"/settings/tokens", url.Values{
		"csrf_token": {csrf},
		"name":       {"ui-token"},
	})
	if created.code != http.StatusSeeOther || created.loc != "/settings/tokens" {
		t.Fatalf("POST /settings/tokens = %d %q, body=%s", created.code, created.loc, created.body)
	}
	shown := doWeb(t, client, http.MethodGet, srv.URL+"/settings/tokens", nil)
	if shown.code != http.StatusOK {
		t.Fatalf("GET /settings/tokens after create = %d, body=%s", shown.code, shown.body)
	}
	matches := oneTimeTokenRE.FindAllStringSubmatch(shown.body, -1)
	if len(matches) != 1 || len(matches[0]) != 2 {
		t.Fatalf("one-time token not shown exactly once: %s", shown.body)
	}
	return matches[0][1], created
}

func TestRegisterCSRFCookieBrowserFlowAndClosure(t *testing.T) {
	srv, db := newEmptyServer(t, web.Config{})
	client, jar := newBrowser(t)
	registerURL := srv.URL + "/register"

	res := doWeb(t, client, http.MethodGet, registerURL, nil)
	if res.code != http.StatusOK {
		t.Fatalf("GET /register = %d, body=%s", res.code, res.body)
	}
	var csrfCookie *http.Cookie
	for _, c := range res.cookies {
		if c.Name == web.RegisterCSRFCookieName {
			csrfCookie = c
		}
	}
	if csrfCookie == nil {
		t.Fatalf("no register CSRF cookie; cookies=%v", res.cookies)
	}
	if csrfCookie.Path != "/register" {
		t.Errorf("register CSRF cookie path = %q, want /register", csrfCookie.Path)
	}
	if !csrfCookie.HttpOnly || csrfCookie.Secure || csrfCookie.SameSite != http.SameSiteLaxMode || csrfCookie.MaxAge != 600 {
		t.Errorf("register CSRF cookie flags = httponly:%v secure:%v samesite:%v maxage:%d",
			csrfCookie.HttpOnly, csrfCookie.Secure, csrfCookie.SameSite, csrfCookie.MaxAge)
	}
	// A real browser must scope the cookie to /register: it is not sent to /.
	if got := cookieValue(t, jar, srv.URL+"/", web.RegisterCSRFCookieName); got != "" {
		t.Errorf("register CSRF cookie leaked to /: %q", got)
	}
	if got := cookieValue(t, jar, registerURL, web.RegisterCSRFCookieName); got != csrfCookie.Value {
		t.Errorf("register CSRF cookie at /register = %q, want %q", got, csrfCookie.Value)
	}

	csrf := hiddenValue(t, res.body, "csrf_token")
	if csrf != csrfCookie.Value {
		t.Fatalf("form CSRF %q does not match cookie %q", csrf, csrfCookie.Value)
	}
	res = doWeb(t, client, http.MethodPost, registerURL, url.Values{
		"csrf_token": {csrf},
		"username":   {"first-admin"},
		"password":   {"FirstPassw0rd!"},
	})
	if res.code != http.StatusSeeOther || res.loc != "/" {
		t.Fatalf("POST /register = %d %q, body=%s", res.code, res.loc, res.body)
	}
	if cookieValue(t, jar, srv.URL+"/", web.SessionCookieName) == "" {
		t.Fatal("registration did not auto-login")
	}
	if got := countRows(t, db, "SELECT count(*) FROM users"); got != 1 {
		t.Fatalf("users after registration = %d, want 1", got)
	}

	// Registration is closed for both GET and POST after the first account.
	res = doWeb(t, client, http.MethodGet, registerURL, nil)
	if res.code != http.StatusSeeOther || res.loc != "/login?err=registration+closed" {
		t.Fatalf("GET /register after first user = %d %q", res.code, res.loc)
	}
	res = doWeb(t, client, http.MethodPost, registerURL, url.Values{
		"csrf_token": {csrf},
		"username":   {"second-admin"},
		"password":   {"SecondPassw0rd!"},
	}, &http.Cookie{Name: web.RegisterCSRFCookieName, Value: csrf})
	if res.code != http.StatusSeeOther || res.loc != "/login?err=registration+closed" {
		t.Fatalf("POST /register after first user = %d %q, body=%s", res.code, res.loc, res.body)
	}
	if got := countRows(t, db, "SELECT count(*) FROM users"); got != 1 {
		t.Fatalf("users after closed registration = %d, want 1", got)
	}
	var username, role string
	if err := db.QueryRow("SELECT username, role FROM users").Scan(&username, &role); err != nil {
		t.Fatalf("read first user: %v", err)
	}
	if username != "first-admin" || role != "admin" {
		t.Errorf("first user = (%q, %q), want (first-admin, admin)", username, role)
	}
}

func TestConcurrentFirstSignupCreatesExactlyOneAdmin(t *testing.T) {
	srv, db := newEmptyServer(t, web.Config{})
	const attempts = 6
	type outcome struct {
		err error
		loc string
	}
	results := make(chan outcome, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			client, jar, err := browser()
			if err != nil {
				results <- outcome{err: err}
				return
			}
			target := srv.URL + "/register"
			res, err := send(client, http.MethodGet, target, nil)
			if err != nil {
				results <- outcome{err: err}
				return
			}
			if res.code != http.StatusOK {
				results <- outcome{err: fmt.Errorf("attempt %d GET /register = %d", n, res.code)}
				return
			}
			csrf, ok := findHidden(res.body, "csrf_token")
			csrfInJar := false
			for _, c := range jar.Cookies(mustParseURL(target)) {
				if c.Name == web.RegisterCSRFCookieName && c.Value == csrf {
					csrfInJar = true
				}
			}
			if !ok || !csrfInJar {
				results <- outcome{err: fmt.Errorf("attempt %d missing CSRF", n)}
				return
			}
			res, err = send(client, http.MethodPost, target, url.Values{
				"csrf_token": {csrf},
				"username":   {fmt.Sprintf("racer-%d", n)},
				"password":   {"ConcurrentPass123"},
			})
			if err != nil {
				results <- outcome{err: err}
				return
			}
			results <- outcome{loc: res.loc}
		}(i)
	}
	wg.Wait()
	close(results)

	successes := 0
	for result := range results {
		if result.err != nil {
			t.Errorf("concurrent signup: %v", result.err)
			continue
		}
		if result.loc == "/" {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful signups = %d, want exactly 1", successes)
	}
	if got := countRows(t, db, "SELECT count(*) FROM users"); got != 1 {
		t.Fatalf("users after concurrent signup = %d, want 1", got)
	}
	if got := countRows(t, db, "SELECT count(*) FROM users WHERE role = 'admin'"); got != 1 {
		t.Fatalf("admins after concurrent signup = %d, want 1", got)
	}
	if got := countRows(t, db, "SELECT count(*) FROM user_profiles"); got != 1 {
		t.Fatalf("profiles after concurrent signup = %d, want 1", got)
	}
}

func TestTokenCreateLocationNeverContainsRawTokenAndOneTimeDisplay(t *testing.T) {
	srv, db := newEmptyServer(t, web.Config{})
	client, _, _ := registerFirstAdmin(t, srv, db)

	raw, created := createTokenViaUI(t, srv, client)
	if !hex40RE.MatchString(raw) {
		t.Fatalf("displayed token %q is not 40 hex characters", raw)
	}
	if strings.Contains(created.loc, raw) || strings.Contains(created.loc, "token=") || hex40RE.MatchString(created.loc) {
		t.Errorf("create Location = %q, must never carry a token", created.loc)
	}
	if strings.Contains(created.body, raw) {
		t.Error("create redirect body leaked the raw token")
	}
	var prefix string
	if err := db.QueryRow("SELECT token_prefix FROM api_tokens").Scan(&prefix); err != nil {
		t.Fatalf("read stored prefix: %v", err)
	}
	if prefix != raw[:8] {
		t.Errorf("stored prefix = %q, want prefix of displayed token %q", prefix, raw)
	}
	var digestCount int
	if err := db.QueryRow("SELECT count(*) FROM api_tokens WHERE token_digest = ?", auth.HashToken(raw)).Scan(&digestCount); err != nil {
		t.Fatalf("read digest: %v", err)
	}
	if digestCount != 1 {
		t.Errorf("matching digests = %d, want 1; raw token must not be stored", digestCount)
	}

	// First GET consumed the one-time token; every later refresh must omit it.
	for i := 0; i < 2; i++ {
		res := doWeb(t, client, http.MethodGet, srv.URL+"/settings/tokens", nil)
		if res.code != http.StatusOK {
			t.Fatalf("refresh %d = %d", i, res.code)
		}
		if strings.Contains(res.body, raw) || strings.Contains(res.body, "New token created") {
			t.Fatalf("refresh %d re-showed the one-time token", i)
		}
		if !strings.Contains(res.body, prefix) {
			t.Errorf("refresh %d lost the non-secret prefix %q", i, prefix)
		}
	}
}

func TestCreatedTokenAuthorizationAPIRequest(t *testing.T) {
	srv, db := newEmptyServer(t, web.Config{})
	client, _, adminID := registerFirstAdmin(t, srv, db)
	raw, _ := createTokenViaUI(t, srv, client)

	api := linkding.NewHandler(db, nil)
	request := func(authHeader string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/user/profile/", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		return rec
	}
	rec := request("Token " + raw)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile with created token = %d, body=%s", rec.Code, rec.Body.String())
	}
	var profile map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if _, ok := profile["theme"]; !ok {
		t.Errorf("profile = %v, want a linkding profile shape", profile)
	}
	if strings.Contains(rec.Body.String(), raw) {
		t.Error("profile response leaked the raw token")
	}
	if rec := request(""); rec.Code != http.StatusUnauthorized {
		t.Errorf("profile without Authorization = %d, want 401", rec.Code)
	}
	if rec := request("Token " + strings.Repeat("0", 40)); rec.Code != http.StatusUnauthorized {
		t.Errorf("profile with unknown token = %d, want 401", rec.Code)
	}
	var owner int64
	if err := db.QueryRow("SELECT user_id FROM api_tokens WHERE token_digest = ?", auth.HashToken(raw)).Scan(&owner); err != nil {
		t.Fatalf("look up token owner: %v", err)
	}
	if owner != adminID {
		t.Errorf("token owner = %d, want admin %d", owner, adminID)
	}
}

func TestDisableEnableInSameSecondInvalidatesOldSession(t *testing.T) {
	srv, db := newEmptyServer(t, web.Config{})
	adminClient, _, _ := registerFirstAdmin(t, srv, db)
	target, err := auth.CreateUser(context.Background(), db, "toggle-user", "TogglePass123")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	targetClient, jar := loginPassword(t, srv, target.Username, "TogglePass123")
	oldSession := cookieValue(t, jar, srv.URL+"/", web.SessionCookieName)
	if oldSession == "" {
		t.Fatal("no pre-disable session")
	}
	var epochBefore int64
	if err := db.QueryRow("SELECT session_epoch FROM users WHERE id = ?", target.ID).Scan(&epochBefore); err != nil {
		t.Fatalf("read epoch: %v", err)
	}

	csrf := adminCSRF(t, adminClient, srv)
	if res := adminAction(t, adminClient, srv, csrf, "disable", target.ID, ""); !strings.Contains(res.loc, "ok=done") {
		t.Fatalf("disable = %q", res.loc)
	}
	var disabledStamp string
	if err := db.QueryRow("SELECT session_valid_after FROM users WHERE id = ?", target.ID).Scan(&disabledStamp); err != nil {
		t.Fatalf("read disabled_at: %v", err)
	}
	if res := adminAction(t, adminClient, srv, csrf, "enable", target.ID, ""); !strings.Contains(res.loc, "ok=done") {
		t.Fatalf("enable = %q", res.loc)
	}
	var epochAfter int64
	var enabledStamp string
	if err := db.QueryRow("SELECT session_epoch, updated_at FROM users WHERE id = ?", target.ID).Scan(&epochAfter, &enabledStamp); err != nil {
		t.Fatalf("read epoch after enable: %v", err)
	}
	if epochAfter != epochBefore+2 {
		t.Errorf("epoch after disable+enable = %d, want %d", epochAfter, epochBefore+2)
	}
	disabledAt, err1 := time.Parse(time.RFC3339, disabledStamp)
	enabledAt, err2 := time.Parse(time.RFC3339, enabledStamp)
	if err1 != nil || err2 != nil {
		t.Fatalf("parse timestamps %q / %q: %v / %v", disabledStamp, enabledStamp, err1, err2)
	}
	if gap := enabledAt.Sub(disabledAt); gap < 0 || gap > time.Second {
		t.Errorf("disable/enable timestamps %q and %q are not in the same second", disabledStamp, enabledStamp)
	}

	// The session created before disable must not survive the enable.
	staleClient, _ := newBrowser(t)
	res := doWeb(t, staleClient, http.MethodGet, srv.URL+"/", nil,
		&http.Cookie{Name: web.SessionCookieName, Value: oldSession})
	if res.code != http.StatusSeeOther || res.loc != "/login" {
		t.Fatalf("old session after disable+enable = %d %q, want 303 /login", res.code, res.loc)
	}
	// The account itself is usable again.
	_, freshJar := loginPassword(t, srv, target.Username, "TogglePass123")
	if got := cookieValue(t, freshJar, srv.URL+"/", web.SessionCookieName); got == "" || got == oldSession {
		t.Errorf("new session after enable = %q, want a fresh session", got)
	}
	_ = targetClient // keep the pre-disable client alive until after the assertion
}

func TestPasswordResetInvalidatesOldSession(t *testing.T) {
	srv, db := newEmptyServer(t, web.Config{})
	adminClient, _, _ := registerFirstAdmin(t, srv, db)
	target, err := auth.CreateUser(context.Background(), db, "reset-user", "OldPassw0rd123")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_, jar := loginPassword(t, srv, target.Username, "OldPassw0rd123")
	oldSession := cookieValue(t, jar, srv.URL+"/", web.SessionCookieName)
	var epochBefore int64
	if err := db.QueryRow("SELECT session_epoch FROM users WHERE id = ?", target.ID).Scan(&epochBefore); err != nil {
		t.Fatalf("read epoch: %v", err)
	}

	csrf := adminCSRF(t, adminClient, srv)
	if res := adminAction(t, adminClient, srv, csrf, "reset_password", target.ID, "NewPassw0rd456"); !strings.Contains(res.loc, "ok=done") {
		t.Fatalf("reset password = %q", res.loc)
	}
	var epochAfter int64
	if err := db.QueryRow("SELECT session_epoch FROM users WHERE id = ?", target.ID).Scan(&epochAfter); err != nil {
		t.Fatalf("read epoch after reset: %v", err)
	}
	if epochAfter != epochBefore+1 {
		t.Errorf("epoch after reset = %d, want %d", epochAfter, epochBefore+1)
	}

	staleClient, _ := newBrowser(t)
	res := doWeb(t, staleClient, http.MethodGet, srv.URL+"/", nil,
		&http.Cookie{Name: web.SessionCookieName, Value: oldSession})
	if res.code != http.StatusSeeOther || res.loc != "/login" {
		t.Fatalf("old session after password reset = %d %q, want 303 /login", res.code, res.loc)
	}
	if _, err := auth.AuthenticatePassword(context.Background(), db, target.Username, "OldPassw0rd123"); err == nil {
		t.Error("old password still authenticates after reset")
	}
	_, freshJar := loginPassword(t, srv, target.Username, "NewPassw0rd456")
	if got := cookieValue(t, freshJar, srv.URL+"/", web.SessionCookieName); got == "" || got == oldSession {
		t.Errorf("new session after reset = %q, want a fresh session", got)
	}
}

func TestAdminTokenUIAuthCSRFAndIDOR(t *testing.T) {
	srv, db := newEmptyServer(t, web.Config{})
	adminClient, _, _ := registerFirstAdmin(t, srv, db)
	ctx := context.Background()
	alice, err := auth.CreateUser(ctx, db, "token-alice", "AlicePassw0rd1")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := auth.CreateUser(ctx, db, "token-bob", "BobPassw0rd12")
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}
	aliceToken, err := auth.CreateTokenForUser(ctx, db, alice.ID, "alice-token")
	if err != nil {
		t.Fatalf("create alice token: %v", err)
	}
	bobToken, err := auth.CreateTokenForUser(ctx, db, bob.ID, "bob-token")
	if err != nil {
		t.Fatalf("create bob token: %v", err)
	}
	tokenURL := fmt.Sprintf("%s/admin/users/%d/tokens", srv.URL, alice.ID)
	revokeURL := tokenURL + "/revoke"
	aliceIDStr := strconv.FormatInt(aliceToken.ID, 10)
	bobIDStr := strconv.FormatInt(bobToken.ID, 10)
	active := func(token string) {
		t.Helper()
		if _, err := auth.LookupByToken(ctx, db, token); err != nil {
			t.Errorf("token %q... should still be active: %v", token[:8], err)
		}
	}

	// Anonymous users are redirected to login before any token data is read.
	anonClient, _ := newBrowser(t)
	res := doWeb(t, anonClient, http.MethodGet, tokenURL, nil)
	if res.code != http.StatusSeeOther || res.loc != "/login" {
		t.Fatalf("anonymous admin token page = %d %q, want 303 /login", res.code, res.loc)
	}
	if strings.Contains(res.body, aliceToken.Prefix) {
		t.Error("anonymous response leaked a token prefix")
	}

	// An ordinary user gets 403 and no token metadata at all.
	bobClient, _ := loginPassword(t, srv, bob.Username, "BobPassw0rd12")
	res = doWeb(t, bobClient, http.MethodGet, tokenURL, nil)
	if res.code != http.StatusForbidden {
		t.Fatalf("ordinary user admin token page = %d, want 403; body=%s", res.code, res.body)
	}
	if strings.Contains(res.body, aliceToken.Prefix) || strings.Contains(res.body, "tokens/revoke") {
		t.Error("ordinary user response leaked admin token UI")
	}

	// Admin sees metadata only: prefix, never the raw token.
	res = doWeb(t, adminClient, http.MethodGet, tokenURL, nil)
	if res.code != http.StatusOK {
		t.Fatalf("admin token page = %d, body=%s", res.code, res.body)
	}
	if !strings.Contains(res.body, aliceToken.Prefix) {
		t.Error("admin page did not list alice's token prefix")
	}
	if strings.Contains(res.body, aliceToken.Token) || strings.Contains(res.body, bobToken.Token) {
		t.Error("admin page leaked a raw token")
	}
	if strings.Contains(res.body, bobToken.Prefix) {
		t.Error("admin page listed another user's token")
	}
	csrf := hiddenValue(t, res.body, "csrf_token")

	// POST without CSRF is rejected and leaves the token active.
	res = doWeb(t, adminClient, http.MethodPost, revokeURL, url.Values{"token_id": {aliceIDStr}})
	if res.code != http.StatusSeeOther || !strings.Contains(res.loc, "err=invalid+csrf") {
		t.Fatalf("revoke without CSRF = %d %q", res.code, res.loc)
	}
	active(aliceToken.Token)

	// IDOR: bob's token ID under alice's path must not be revoked.
	res = doWeb(t, adminClient, http.MethodPost, revokeURL, url.Values{
		"csrf_token": {csrf},
		"token_id":   {bobIDStr},
	})
	if res.code != http.StatusSeeOther || !strings.Contains(res.loc, "err=revoke+failed") {
		t.Fatalf("IDOR revoke = %d %q", res.code, res.loc)
	}
	active(aliceToken.Token)
	active(bobToken.Token)

	// Correct owner/path/CSRF combination revokes only alice's token.
	res = doWeb(t, adminClient, http.MethodPost, revokeURL, url.Values{
		"csrf_token": {csrf},
		"token_id":   {aliceIDStr},
	})
	if res.code != http.StatusSeeOther || res.loc != strings.TrimPrefix(tokenURL, srv.URL) {
		t.Fatalf("valid revoke = %d %q", res.code, res.loc)
	}
	if _, err := auth.LookupByToken(ctx, db, aliceToken.Token); err == nil {
		t.Error("alice's token still authenticates after revoke")
	}
	active(bobToken.Token)
}

func TestRegistrationAndLoginRateLimits(t *testing.T) {
	const maxAttempts = 2
	window := time.Minute

	t.Run("login", func(t *testing.T) {
		srv, db := newEmptyServer(t, web.Config{LoginRateLimitMax: maxAttempts, LoginRateLimitWindow: window})
		if _, err := auth.CreateUser(context.Background(), db, "limit-user", "RightPassw0rd1"); err != nil {
			t.Fatalf("create user: %v", err)
		}
		client, jar := newBrowser(t)
		res := doWeb(t, client, http.MethodGet, srv.URL+"/login", nil)
		csrf := hiddenValue(t, res.body, "csrf_token")
		for i := 0; i < maxAttempts; i++ {
			failed := doWeb(t, client, http.MethodPost, srv.URL+"/login", url.Values{
				"csrf_token": {csrf},
				"username":   {"limit-user"},
				"password":   {"WrongPassw0rd1"},
			})
			if !strings.Contains(failed.loc, "err=invalid+credentials") {
				t.Fatalf("attempt %d = %q, want invalid credentials", i, failed.loc)
			}
		}
		blocked := doWeb(t, client, http.MethodPost, srv.URL+"/login", url.Values{
			"csrf_token": {csrf},
			"username":   {"limit-user"},
			"password":   {"RightPassw0rd1"},
		})
		if blocked.code != http.StatusSeeOther || !strings.Contains(blocked.loc, "err=too+many+attempts") {
			t.Fatalf("correct password after limit = %d %q", blocked.code, blocked.loc)
		}
		if got := cookieValue(t, jar, srv.URL+"/", web.SessionCookieName); got != "" {
			t.Errorf("rate-limited login set a session cookie: %q", got)
		}
	})

	t.Run("registration", func(t *testing.T) {
		srv, db := newEmptyServer(t, web.Config{LoginRateLimitMax: maxAttempts, LoginRateLimitWindow: window})
		client, jar := newBrowser(t)
		res := doWeb(t, client, http.MethodGet, srv.URL+"/register", nil)
		csrf := hiddenValue(t, res.body, "csrf_token")
		wrongCSRF := strings.Repeat("0", len(csrf))
		for i := 0; i < maxAttempts; i++ {
			failed := doWeb(t, client, http.MethodPost, srv.URL+"/register", url.Values{
				"csrf_token": {wrongCSRF},
				"username":   {"limit-admin"},
				"password":   {"RightPassw0rd1"},
			})
			if !strings.Contains(failed.loc, "err=invalid+csrf") {
				t.Fatalf("attempt %d = %q, want invalid csrf", i, failed.loc)
			}
		}
		blocked := doWeb(t, client, http.MethodPost, srv.URL+"/register", url.Values{
			"csrf_token": {csrf},
			"username":   {"limit-admin"},
			"password":   {"RightPassw0rd1"},
		})
		if blocked.code != http.StatusSeeOther || !strings.Contains(blocked.loc, "err=too+many+attempts") {
			t.Fatalf("valid registration after limit = %d %q", blocked.code, blocked.loc)
		}
		if got := countRows(t, db, "SELECT count(*) FROM users"); got != 0 {
			t.Errorf("users after rate-limited registration = %d, want 0", got)
		}
		if got := cookieValue(t, jar, srv.URL+"/", web.SessionCookieName); got != "" {
			t.Errorf("rate-limited registration set a session cookie: %q", got)
		}
	})
}

func TestBookmarkPreservedAfterDisable(t *testing.T) {
	srv, db := newEmptyServer(t, web.Config{})
	adminClient, _, _ := registerFirstAdmin(t, srv, db)
	owner, err := auth.CreateUser(context.Background(), db, "bookmark-owner", "OwnerPassw0rd1")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := auth.CreateTokenForUser(context.Background(), db, owner.ID, "owner-token")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	api := linkding.NewHandler(db, nil)
	createReq := httptest.NewRequest(http.MethodPost, "/api/bookmarks/?disable_scraping",
		strings.NewReader(`{"url":"https://preserve.example/page","title":"Preserved bookmark"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Token "+token.Token)
	createRec := httptest.NewRecorder()
	api.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create bookmark = %d, body=%s", createRec.Code, createRec.Body.String())
	}
	profileReq := func(raw string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/bookmarks/", nil)
		req.Header.Set("Authorization", "Token "+raw)
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		return rec
	}
	if rec := profileReq(token.Token); rec.Code != http.StatusOK {
		t.Fatalf("list with original token = %d", rec.Code)
	}

	csrf := adminCSRF(t, adminClient, srv)
	if res := adminAction(t, adminClient, srv, csrf, "disable", owner.ID, ""); !strings.Contains(res.loc, "ok=done") {
		t.Fatalf("disable = %q", res.loc)
	}
	if rec := profileReq(token.Token); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked token after disable = %d, want 401", rec.Code)
	}
	var title string
	if err := db.QueryRow("SELECT title FROM bookmarks WHERE user_id = ?", owner.ID).Scan(&title); err != nil {
		t.Fatalf("bookmark was removed on disable: %v", err)
	}
	if title != "Preserved bookmark" {
		t.Errorf("stored title = %q, want Preserved bookmark", title)
	}

	if res := adminAction(t, adminClient, srv, csrf, "enable", owner.ID, ""); !strings.Contains(res.loc, "ok=done") {
		t.Fatalf("enable = %q", res.loc)
	}
	freshToken, err := auth.CreateTokenForUser(context.Background(), db, owner.ID, "fresh-token")
	if err != nil {
		t.Fatalf("create fresh token: %v", err)
	}
	rec := profileReq(freshToken.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("list after re-enable = %d, body=%s", rec.Code, rec.Body.String())
	}
	var page struct {
		Count   int `json:"count"`
		Results []struct {
			Title string `json:"title"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode bookmark page: %v", err)
	}
	if page.Count != 1 || len(page.Results) != 1 || page.Results[0].Title != "Preserved bookmark" {
		t.Errorf("bookmarks after re-enable = %+v, want the original bookmark", page)
	}
}
