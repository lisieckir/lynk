package web

import (
	"net/http"
	"strconv"

	"github.com/lisieckir/lynk/internal/auth"
)

func (h *Handler) handleTokensGet(w http.ResponseWriter, r *http.Request) {
	wu, _ := webUserFromContext(r.Context())
	sess, _ := sessionFromContext(r.Context())
	tokens, err := auth.ListTokensForUser(r.Context(), h.db, wu.ID)
	if err != nil {
		h.logger.Error("list tokens", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"CSRFToken": sess.CSRFToken,
		"Tokens":    tokens,
		"NewToken":  r.URL.Query().Get("newtoken"),
		"Error":     r.URL.Query().Get("err"),
	}
	_ = h.tmpl.ExecuteTemplate(w, "tokens", data)
}

func (h *Handler) handleTokensCreatePost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/settings/tokens?err=invalid+form", http.StatusSeeOther)
		return
	}
	sess, ok := sessionFromContext(r.Context())
	if !ok || !ValidateSessionCSRF(sess, r.FormValue("csrf_token")) {
		http.Redirect(w, r, "/settings/tokens?err=invalid+csrf", http.StatusSeeOther)
		return
	}
	wu, _ := webUserFromContext(r.Context())
	name := r.FormValue("name")
	if name == "" {
		name = "default"
	}
	info, err := auth.CreateTokenForUser(r.Context(), h.db, wu.ID, name)
	if err != nil {
		http.Redirect(w, r, "/settings/tokens?err=create+failed", http.StatusSeeOther)
		return
	}
	// Show token once via query param (simple approach; could be improved with flash)
	http.Redirect(w, r, "/settings/tokens?newtoken="+info.Token, http.StatusSeeOther)
}

func (h *Handler) handleTokensRevokePost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/settings/tokens?err=invalid+form", http.StatusSeeOther)
		return
	}
	sess, ok := sessionFromContext(r.Context())
	if !ok || !ValidateSessionCSRF(sess, r.FormValue("csrf_token")) {
		http.Redirect(w, r, "/settings/tokens?err=invalid+csrf", http.StatusSeeOther)
		return
	}
	wu, _ := webUserFromContext(r.Context())
	idStr := r.FormValue("token_id")
	tid, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/settings/tokens?err=invalid+id", http.StatusSeeOther)
		return
	}
	if err := auth.RevokeTokenByID(r.Context(), h.db, tid, wu.ID, false); err != nil {
		http.Redirect(w, r, "/settings/tokens?err=revoke+failed", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings/tokens", http.StatusSeeOther)
}
