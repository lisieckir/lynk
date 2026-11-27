package web

import (
	"net/http"
	"strconv"

	"github.com/lisieckir/lynk/internal/auth"
)

func (h *Handler) handleAdminUsersGet(w http.ResponseWriter, r *http.Request) {
	sess, _ := sessionFromContext(r.Context())
	users, err := auth.ListUsers(r.Context(), h.db)
	if err != nil {
		h.logger.Error("list users", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"CSRFToken": sess.CSRFToken,
		"Users":     users,
		"Error":     r.URL.Query().Get("err"),
		"Success":   r.URL.Query().Get("ok"),
	}
	_ = h.tmpl.ExecuteTemplate(w, "admin_users", data)
}

func (h *Handler) handleAdminUsersPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/users?err=invalid+form", http.StatusSeeOther)
		return
	}
	sess, ok := sessionFromContext(r.Context())
	if !ok || !ValidateSessionCSRF(sess, r.FormValue("csrf_token")) {
		http.Redirect(w, r, "/admin/users?err=invalid+csrf", http.StatusSeeOther)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	newUser, err := auth.CreateUser(r.Context(), h.db, username, password)
	if err != nil {
		http.Redirect(w, r, "/admin/users?err=create+failed", http.StatusSeeOther)
		return
	}
	h.logger.Info("security_event", "event", "admin_user_created", "user_id", newUser.ID, "username", newUser.Username)
	http.Redirect(w, r, "/admin/users?ok=created", http.StatusSeeOther)
}

func (h *Handler) handleAdminUsersActionPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/users?err=invalid+form", http.StatusSeeOther)
		return
	}
	sess, ok := sessionFromContext(r.Context())
	if !ok || !ValidateSessionCSRF(sess, r.FormValue("csrf_token")) {
		http.Redirect(w, r, "/admin/users?err=invalid+csrf", http.StatusSeeOther)
		return
	}
	action := r.FormValue("action")
	idStr := r.FormValue("user_id")
	uid, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Redirect(w, r, "/admin/users?err=invalid+user", http.StatusSeeOther)
		return
	}
	switch action {
	case "disable":
		if err := auth.SetUserDisabled(r.Context(), h.db, uid, true); err != nil {
			http.Redirect(w, r, "/admin/users?err=action+failed", http.StatusSeeOther)
			return
		}
		h.logger.Info("security_event", "event", "user_disabled", "target_user_id", uid)
	case "enable":
		if err := auth.SetUserDisabled(r.Context(), h.db, uid, false); err != nil {
			http.Redirect(w, r, "/admin/users?err=action+failed", http.StatusSeeOther)
			return
		}
		h.logger.Info("security_event", "event", "user_enabled", "target_user_id", uid)
	case "reset_password":
		pw := r.FormValue("new_password")
		if err := auth.ResetPassword(r.Context(), h.db, uid, pw); err != nil {
			http.Redirect(w, r, "/admin/users?err=reset+failed", http.StatusSeeOther)
			return
		}
		h.logger.Info("security_event", "event", "password_reset", "target_user_id", uid)
	default:
		http.Redirect(w, r, "/admin/users?err=unknown+action", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/users?ok=done", http.StatusSeeOther)
}
