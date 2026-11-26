package web

import (
	"net/http"
	"strings"

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
		ctx := withSession(r.Context(), sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (h *Handler) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		if _, ok := h.sessions.Get(c.Value); ok {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}
	loginCSRF, err := generateRandomHex(csrfTokenBytes)
	if err != nil {
		h.logger.Error("generate login csrf", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	SetLoginCSRFCookie(w, loginCSRF, h.secure)
	data := map[string]any{
		"CSRFToken": loginCSRF,
		"Error":     r.URL.Query().Get("err"),
	}
	_ = h.tmpl.ExecuteTemplate(w, "login", data)
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
	token := strings.TrimSpace(r.FormValue("token"))
	if token == "" {
		http.Redirect(w, r, "/login?err=token+required", http.StatusSeeOther)
		return
	}
	user, err := auth.LookupByToken(r.Context(), h.db, token)
	if err != nil {
		http.Redirect(w, r, "/login?err=invalid+token", http.StatusSeeOther)
		return
	}
	if h.loginLimiter != nil {
		h.loginLimiter.reset(ip)
	}
	sid, _, err := h.sessions.Create(user.ID)
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
