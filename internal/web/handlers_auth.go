package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/lisieckir/lynk/internal/auth"
)

func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookieName)
		if err != nil || c.Value == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		sess, ok := h.sessions.Get(c.Value)
		if !ok {
			ClearSessionCookie(w, h.secure)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		wu, ok := h.validateAndLoadUser(r.Context(), sess)
		if !ok {
			h.sessions.Delete(c.Value)
			ClearSessionCookie(w, h.secure)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := withWebUser(withSession(r.Context(), sess), wu)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (h *Handler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return h.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		wu, ok := webUserFromContext(r.Context())
		if !ok || wu.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

func (h *Handler) validateAndLoadUser(ctx context.Context, sess *Session) (*WebUser, bool) {
	var disabled int
	var epoch int64
	var username, role string
	err := h.db.QueryRowContext(ctx, "SELECT username, role, disabled, session_epoch FROM users WHERE id = ?", sess.UserID).Scan(&username, &role, &disabled, &epoch)
	if err != nil {
		return nil, false
	}
	if disabled != 0 {
		return nil, false
	}
	if sess.Epoch != epoch {
		return nil, false
	}
	return &WebUser{ID: sess.UserID, Username: username, Role: role}, true
}

func (h *Handler) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		if _, ok := h.sessions.Get(c.Value); ok {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}
	// Check if registration is open (no users)
	var count int
	_ = h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count)
	loginCSRF, err := generateRandomHex(csrfTokenBytes)
	if err != nil {
		h.logger.Error("generate login csrf", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	SetLoginCSRFCookie(w, loginCSRF, h.secure)
	data := map[string]any{
		"CSRFToken":   loginCSRF,
		"Error":       r.URL.Query().Get("err"),
		"CanRegister": count == 0,
	}
	_ = h.tmpl.ExecuteTemplate(w, "login", data)
}

func (h *Handler) handleRegisterGet(w http.ResponseWriter, r *http.Request) {
	var count int
	if err := h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		http.Redirect(w, r, "/login?err=registration+closed", http.StatusSeeOther)
		return
	}
	regCSRF, err := generateRandomHex(csrfTokenBytes)
	if err != nil {
		h.logger.Error("generate register csrf", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	SetRegisterCSRFCookie(w, regCSRF, h.secure)
	data := map[string]any{
		"CSRFToken": regCSRF,
		"Error":     r.URL.Query().Get("err"),
	}
	_ = h.tmpl.ExecuteTemplate(w, "register", data)
}

func (h *Handler) handleRegisterPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/register?err=invalid+form", http.StatusSeeOther)
		return
	}
	formCSRF := r.FormValue("csrf_token")
	if !ValidateRegisterCSRF(r, formCSRF) {
		http.Redirect(w, r, "/register?err=invalid+csrf", http.StatusSeeOther)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if err := auth.ValidateUsername(username); err != nil {
		http.Redirect(w, r, "/register?err=invalid+username+or+password", http.StatusSeeOther)
		return
	}
	if err := auth.ValidatePassword(password); err != nil {
		http.Redirect(w, r, "/register?err=invalid+username+or+password", http.StatusSeeOther)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		http.Redirect(w, r, "/register?err=invalid+username+or+password", http.StatusSeeOther)
		return
	}
	// Atomic first-user registration
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Redirect(w, r, "/register?err=registration+failed", http.StatusSeeOther)
		return
	}
	defer func() { _ = tx.Rollback() }()
	var count int
	if err := tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		http.Redirect(w, r, "/register?err=registration+failed", http.StatusSeeOther)
		return
	}
	if count > 0 {
		_ = tx.Rollback()
		http.Redirect(w, r, "/login?err=registration+closed", http.StatusSeeOther)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := tx.ExecContext(r.Context(),
		"INSERT INTO users (username, password_hash, role, disabled, created_at, updated_at) VALUES (?, ?, 'admin', 0, ?, ?)",
		username, hash, now, now,
	)
	if err != nil {
		http.Redirect(w, r, "/register?err=registration+failed", http.StatusSeeOther)
		return
	}
	uid, _ := res.LastInsertId()
	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO user_profiles (user_id, theme, bookmark_date_display, bookmark_link_target, web_archive_integration,
			tag_search, enable_sharing, enable_public_sharing, enable_favicons, display_url, permanent_notes, search_preferences)
		 VALUES (?, '', '', '', '', 0, 0, 0, 1, 0, '', '{}')`,
		uid,
	)
	if err != nil {
		http.Redirect(w, r, "/register?err=registration+failed", http.StatusSeeOther)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Redirect(w, r, "/register?err=registration+failed", http.StatusSeeOther)
		return
	}
	// Auto-login after register
	sid, _, err := h.sessions.Create(uid, 0)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	SetSessionCookie(w, sid, h.secure, int(h.sessions.ttl.Seconds()))
	ClearRegisterCSRFCookie(w, h.secure)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	ip := clientIP(r, h.trustProxyHeaders)
	if h.loginLimiter != nil && !h.loginLimiter.allow(ip) {
		http.Redirect(w, r, "/login?err=too+many+attempts", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?err=invalid+form", http.StatusSeeOther)
		return
	}
	formCSRF := r.FormValue("csrf_token")
	if !ValidateLoginCSRF(r, formCSRF) {
		http.Redirect(w, r, "/login?err=invalid+csrf", http.StatusSeeOther)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	token := strings.TrimSpace(r.FormValue("token"))

	// Legacy API token login path (explicit)
	if token != "" && password == "" && username == "" {
		user, err := auth.LookupByToken(r.Context(), h.db, token)
		if err != nil {
			http.Redirect(w, r, "/login?err=invalid+credentials", http.StatusSeeOther)
			return
		}
		if user.Disabled {
			http.Redirect(w, r, "/login?err=invalid+credentials", http.StatusSeeOther)
			return
		}
		if h.loginLimiter != nil {
			h.loginLimiter.reset(ip)
		}
		sid, _, err := h.sessions.Create(user.ID, user.SessionEpoch)
		if err != nil {
			h.logger.Error("create session", "error", err)
			http.Redirect(w, r, "/login?err=internal+error", http.StatusSeeOther)
			return
		}
		SetSessionCookie(w, sid, h.secure, int(h.sessions.ttl.Seconds()))
		ClearLoginCSRFCookie(w, h.secure)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Username/password login
	if username == "" || password == "" {
		http.Redirect(w, r, "/login?err=username+and+password+required", http.StatusSeeOther)
		return
	}
	user, err := auth.AuthenticatePassword(r.Context(), h.db, username, password)
	if err != nil {
		http.Redirect(w, r, "/login?err=invalid+credentials", http.StatusSeeOther)
		return
	}
	if h.loginLimiter != nil {
		h.loginLimiter.reset(ip)
	}
	sid, _, err := h.sessions.Create(user.ID, user.SessionEpoch)
	if err != nil {
		h.logger.Error("create session", "error", err)
		http.Redirect(w, r, "/login?err=internal+error", http.StatusSeeOther)
		return
	}
	SetSessionCookie(w, sid, h.secure, int(h.sessions.ttl.Seconds()))
	ClearLoginCSRFCookie(w, h.secure)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) handleLogoutPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?err=invalid+form", http.StatusSeeOther)
		return
	}
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !ValidateSessionCSRF(sess, r.FormValue("csrf_token")) {
		http.Redirect(w, r, "/?err=invalid+csrf", http.StatusSeeOther)
		return
	}
	c, err := r.Cookie(SessionCookieName)
	if err == nil && c.Value != "" {
		h.sessions.Delete(c.Value)
	}
	ClearSessionCookie(w, h.secure)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
