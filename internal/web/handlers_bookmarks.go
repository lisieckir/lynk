package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lisieckir/lynk/internal/bookmark"
)

func (h *Handler) handleBookmarksList(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	archived := r.URL.Query().Get("archived") == "1" || strings.ToLower(r.URL.Query().Get("archived")) == "true"
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	page, err := h.service.List(r.Context(), sess.UserID, archived, bookmark.PageOptions{
		Limit:  limit,
		Offset: offset,
		Query:  q,
	})
	if err != nil {
		h.logger.Error("list bookmarks", "error", err)
		http.Error(w, "failed to load bookmarks", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Bookmarks": page.Items,
		"Count":     page.Count,
		"Limit":     page.Limit,
		"Offset":    page.Offset,
		"Query":     q,
		"Archived":  archived,
		"CSRFToken": sess.CSRFToken,
		"Error":     r.URL.Query().Get("err"),
		"Success":   r.URL.Query().Get("ok"),
	}
	_ = h.tmpl.ExecuteTemplate(w, "bookmarks", data)
}

func (h *Handler) handleBookmarkNewGet(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	data := map[string]any{
		"CSRFToken": sess.CSRFToken,
		"Error":     r.URL.Query().Get("err"),
		"Bookmark":  nil,
		"IsEdit":    false,
	}
	_ = h.tmpl.ExecuteTemplate(w, "bookmark_form", data)
}

func (h *Handler) handleBookmarkCreatePost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/bookmarks/new?err=invalid+form", http.StatusSeeOther)
		return
	}
	if !ValidateSessionCSRF(sess, r.FormValue("csrf_token")) {
		http.Redirect(w, r, "/bookmarks/new?err=invalid+csrf", http.StatusSeeOther)
		return
	}
	urlStr := strings.TrimSpace(r.FormValue("url"))
	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	tagsStr := strings.TrimSpace(r.FormValue("tags"))
	unread := r.FormValue("unread") == "1" || r.FormValue("unread") == "on"
	shared := r.FormValue("shared") == "1" || r.FormValue("shared") == "on"

	if urlStr == "" {
		http.Redirect(w, r, "/bookmarks/new?err=url+required", http.StatusSeeOther)
		return
	}
	tags := parseTagsInput(tagsStr)
	// Detect duplicate to avoid silently "created" after merge
	if existing, gerr := h.service.GetByURL(r.Context(), sess.UserID, urlStr); gerr == nil && existing.ID > 0 {
		http.Redirect(w, r, fmt.Sprintf("/bookmarks/%d/edit?err=%s", existing.ID, url.QueryEscape("URL already exists; edit the existing bookmark")), http.StatusSeeOther)
		return
	}
	_, err := h.service.Create(r.Context(), sess.UserID, bookmark.CreateInput{
		URL:         urlStr,
		Title:       title,
		Description: description,
		Notes:       notes,
		Tags:        tags,
		Unread:      unread,
		Shared:      shared,
	})
	if err != nil {
		h.logger.Error("create bookmark", "error", err)
		msg := url.QueryEscape("failed to create bookmark")
		http.Redirect(w, r, "/bookmarks/new?err="+msg, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?ok=created", http.StatusSeeOther)
}

func parseTagsInput(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var tags []string
	seen := map[string]bool{}
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		lower := strings.ToLower(t)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		tags = append(tags, t)
	}
	return tags
}

func (h *Handler) handleBookmarkEditGet(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id, ok := parseBookmarkIDFromPath(r.URL.Path, "/bookmarks/", "/edit")
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	bm, err := h.service.Get(r.Context(), sess.UserID, id)
	if err != nil {
		http.Redirect(w, r, "/?err=not+found", http.StatusSeeOther)
		return
	}
	data := map[string]any{
		"CSRFToken": sess.CSRFToken,
		"Error":     r.URL.Query().Get("err"),
		"Bookmark":  bm,
		"IsEdit":    true,
		"TagsStr":   strings.Join(bm.Tags, ", "),
	}
	_ = h.tmpl.ExecuteTemplate(w, "bookmark_form", data)
}

func parseBookmarkIDFromPath(path, prefix, suffix string) (int64, bool) {
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return 0, false
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	id, err := strconv.ParseInt(mid, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func parseBookmarkIDFromPath2(path, prefix string) (int64, string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return 0, "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, "", false
	}
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}
	return id, action, true
}

func (h *Handler) handleBookmarkGetSubpath(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/edit") {
		h.handleBookmarkEditGet(w, r)
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) handleBookmarkByIDPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id, action, ok := parseBookmarkIDFromPath2(r.URL.Path, "/bookmarks/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?err=invalid+form", http.StatusSeeOther)
		return
	}
	if !ValidateSessionCSRF(sess, r.FormValue("csrf_token")) {
		http.Redirect(w, r, "/?err=invalid+csrf", http.StatusSeeOther)
		return
	}
	switch action {
	case "edit":
		h.doBookmarkUpdate(w, r, sess, id)
	case "delete":
		h.doBookmarkDelete(w, r, sess, id)
	case "archive":
		h.doBookmarkArchive(w, r, sess, id, true)
	case "unarchive":
		h.doBookmarkArchive(w, r, sess, id, false)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) doBookmarkUpdate(w http.ResponseWriter, r *http.Request, sess *Session, id int64) {
	urlStr := strings.TrimSpace(r.FormValue("url"))
	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	tagsStr := strings.TrimSpace(r.FormValue("tags"))
	unread := r.FormValue("unread") == "1" || r.FormValue("unread") == "on"
	shared := r.FormValue("shared") == "1" || r.FormValue("shared") == "on"
	if urlStr == "" {
		http.Redirect(w, r, fmt.Sprintf("/bookmarks/%d/edit?err=url+required", id), http.StatusSeeOther)
		return
	}
	tags := parseTagsInput(tagsStr)
	_, err := h.service.Update(r.Context(), sess.UserID, id, bookmark.UpdateInput{
		URL:         &urlStr,
		Title:       &title,
		Description: &description,
		Notes:       &notes,
		Tags:        &tags,
		Unread:      &unread,
		Shared:      &shared,
	})
	if err != nil {
		h.logger.Error("update bookmark", "error", err)
		http.Redirect(w, r, fmt.Sprintf("/bookmarks/%d/edit?err=update+failed", id), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?ok=updated", http.StatusSeeOther)
}

func (h *Handler) doBookmarkDelete(w http.ResponseWriter, r *http.Request, sess *Session, id int64) {
	if err := h.service.Delete(r.Context(), sess.UserID, id); err != nil {
		h.logger.Error("delete bookmark", "error", err)
		http.Redirect(w, r, "/?err=delete+failed", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?ok=deleted", http.StatusSeeOther)
}

func (h *Handler) doBookmarkArchive(w http.ResponseWriter, r *http.Request, sess *Session, id int64, archive bool) {
	var err error
	if archive {
		err = h.service.Archive(r.Context(), sess.UserID, id)
	} else {
		err = h.service.Unarchive(r.Context(), sess.UserID, id)
	}
	if err != nil {
		h.logger.Error("archive/unarchive bookmark", "error", err, "archive", archive)
		http.Redirect(w, r, "/?err=action+failed", http.StatusSeeOther)
		return
	}
	if archive {
		http.Redirect(w, r, "/?ok=archived", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/?ok=unarchived", http.StatusSeeOther)
	}
}
