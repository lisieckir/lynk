package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/lisieckir/lynk/internal/auth"
)

func (h *Handler) handleAdminUserTokensGet(w http.ResponseWriter, r *http.Request) {
	// Path: /admin/users/{id}/tokens
	path := strings.TrimPrefix(r.URL.Path, "/admin/users/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "tokens" {
		http.NotFound(w, r)
		return
	}
	uid, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || uid <= 0 {
		http.NotFound(w, r)
		return
	}
	sess, _ := sessionFromContext(r.Context())
	tokens, err := auth.ListTokensForUser(r.Context(), h.db, uid)
	if err != nil {
		h.logger.Error("admin list tokens", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	user, err := auth.GetUserByID(r.Context(), h.db, uid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data := map[string]any{
		"CSRFToken": sess.CSRFToken,
		"User":      user,
		"Tokens":    tokens,
		"Error":     r.URL.Query().Get("err"),
	}
	_ = h.tmpl.ExecuteTemplate(w, "admin_user_tokens", data)
}

func (h *Handler) handleAdminUserTokensRevokePost(w http.ResponseWriter, r *http.Request) {
	// Path: /admin/users/{id}/tokens/revoke
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	path := strings.TrimPrefix(r.URL.Path, "/admin/users/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[1] != "tokens" || parts[2] != "revoke" {
		http.NotFound(w, r)
		return
	}
	uid, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || uid <= 0 {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/users/"+parts[0]+"/tokens?err=invalid+form", http.StatusSeeOther)
		return
	}
	sess, ok := sessionFromContext(r.Context())
	if !ok || !ValidateSessionCSRF(sess, r.FormValue("csrf_token")) {
		http.Redirect(w, r, "/admin/users/"+parts[0]+"/tokens?err=invalid+csrf", http.StatusSeeOther)
		return
	}
	tidStr := r.FormValue("token_id")
	tid, err := strconv.ParseInt(tidStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/admin/users/"+parts[0]+"/tokens?err=invalid+token", http.StatusSeeOther)
		return
	}
	// Verify token belongs to this user (IDOR protection)
	var tokenUID int64
	err = h.db.QueryRowContext(r.Context(), "SELECT user_id FROM api_tokens WHERE id = ?", tid).Scan(&tokenUID)
	if err != nil || tokenUID != uid {
		http.Redirect(w, r, "/admin/users/"+parts[0]+"/tokens?err=revoke+failed", http.StatusSeeOther)
		return
	}
	if err := auth.RevokeTokenByID(r.Context(), h.db, tid, 0, true); err != nil {
		http.Redirect(w, r, "/admin/users/"+parts[0]+"/tokens?err=revoke+failed", http.StatusSeeOther)
		return
	}
	h.logger.Info("security_event", "event", "admin_token_revoked", "target_user_id", uid, "token_id", tid)
	http.Redirect(w, r, "/admin/users/"+parts[0]+"/tokens", http.StatusSeeOther)
}
