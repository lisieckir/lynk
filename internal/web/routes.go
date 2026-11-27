package web

import "net/http"

func securityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; img-src 'self' data: https:; form-action 'self'; base-uri 'self'; frame-ancestors 'none'")
		next(w, r)
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	// Static
	if h.static != nil {
		mux.Handle("GET /static/", h.static)
	}
	// Public
	mux.HandleFunc("GET /login", securityHeaders(h.handleLoginGet))
	mux.HandleFunc("POST /login", securityHeaders(h.handleLoginPost))
	mux.HandleFunc("GET /register", securityHeaders(h.handleRegisterGet))
	mux.HandleFunc("POST /register", securityHeaders(h.handleRegisterPost))
	// Authenticated
	mux.HandleFunc("GET /{$}", securityHeaders(h.requireAuth(h.handleBookmarksList)))
	mux.HandleFunc("GET /bookmarks", securityHeaders(h.requireAuth(h.handleBookmarksList)))
	mux.HandleFunc("GET /bookmarks/new", securityHeaders(h.requireAuth(h.handleBookmarkNewGet)))
	mux.HandleFunc("POST /bookmarks", securityHeaders(h.requireAuth(h.handleBookmarkCreatePost)))
	mux.HandleFunc("POST /logout", securityHeaders(h.requireAuth(h.handleLogoutPost)))
	// Tokens (user)
	mux.HandleFunc("GET /settings/tokens", securityHeaders(h.requireAuth(h.handleTokensGet)))
	mux.HandleFunc("POST /settings/tokens", securityHeaders(h.requireAuth(h.handleTokensCreatePost)))
	mux.HandleFunc("POST /settings/tokens/revoke", securityHeaders(h.requireAuth(h.handleTokensRevokePost)))
	// Admin
	mux.HandleFunc("GET /admin/users", securityHeaders(h.requireAdmin(h.handleAdminUsersGet)))
	mux.HandleFunc("POST /admin/users", securityHeaders(h.requireAdmin(h.handleAdminUsersPost)))
	mux.HandleFunc("POST /admin/users/action", securityHeaders(h.requireAdmin(h.handleAdminUsersActionPost)))
	mux.HandleFunc("GET /admin/users/", securityHeaders(h.requireAdmin(h.handleAdminUserTokensGet)))
	mux.HandleFunc("POST /admin/users/", securityHeaders(h.requireAdmin(h.handleAdminUserTokensRevokePost)))
	// Bookmark actions
	mux.HandleFunc("GET /bookmarks/", securityHeaders(h.requireAuth(h.handleBookmarkGetSubpath)))
	mux.HandleFunc("POST /bookmarks/", securityHeaders(h.requireAuth(h.handleBookmarkByIDPost)))
}
