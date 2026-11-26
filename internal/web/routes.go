package web

import (
	"net/http"
)

func (h *Handler) Register(mux *http.ServeMux) {
	// Static
	if h.static != nil {
		mux.Handle("GET /static/", h.static)
	}
	// Public
	mux.HandleFunc("GET /login", h.handleLoginGet)
	mux.HandleFunc("POST /login", h.handleLoginPost)
	// Authenticated
	mux.HandleFunc("GET /{$}", h.requireAuth(h.handleBookmarksList))
	mux.HandleFunc("GET /bookmarks", h.requireAuth(h.handleBookmarksList))
	mux.HandleFunc("GET /bookmarks/new", h.requireAuth(h.handleBookmarkNewGet))
	mux.HandleFunc("POST /bookmarks", h.requireAuth(h.handleBookmarkCreatePost))
	mux.HandleFunc("POST /logout", h.requireAuth(h.handleLogoutPost))
	// Bookmark actions
	mux.HandleFunc("GET /bookmarks/", h.requireAuth(h.handleBookmarkGetSubpath))
	mux.HandleFunc("POST /bookmarks/", h.requireAuth(h.handleBookmarkByIDPost))
}
