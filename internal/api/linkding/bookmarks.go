package linkding

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/lisieckir/lynk/internal/bookmark"
)

// maxRequestBodyBytes bounds JSON bodies before decoding so a hostile client
// cannot make the API allocate without limit.
const maxRequestBodyBytes = 1 << 20

// createBookmarkRequest only contains fields Linkding clients send on save.
// Unknown fields (including generated ones) are ignored, never trusted.
type createBookmarkRequest struct {
	URL         *string   `json:"url"`
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	Notes       *string   `json:"notes"`
	TagNames    *[]string `json:"tag_names"`
	Unread      *bool     `json:"unread"`
	Shared      *bool     `json:"shared"`
}

// updateBookmarkRequest mirrors the writable fields of the Linkding bookmark
// serializer for PUT/PATCH. Pointers distinguish omitted fields from
// explicit zero values, so PATCH stays partial and PUT stays "url required".
type updateBookmarkRequest struct {
	URL         *string   `json:"url"`
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	Notes       *string   `json:"notes"`
	TagNames    *[]string `json:"tag_names"`
	IsArchived  *bool     `json:"is_archived"`
	Unread      *bool     `json:"unread"`
	Shared      *bool     `json:"shared"`
}

// pageOptionsFromRequest translates the Linkding limit/offset parameters into
// service options. Unparsable or non-positive limits fall back to the page
// default (DefaultLimit, Linkding's PAGE_SIZE) and unusable offsets read as
// zero. DRF itself accepts limit=0 as an empty page whose next link points at
// itself; Lynk treats it as invalid so that degenerate page cannot be
// requested.
func pageOptionsFromRequest(r *http.Request) bookmark.PageOptions {
	q := r.URL.Query()
	return bookmark.PageOptions{
		Limit:  parseLimitParam(q.Get("limit")),
		Offset: parseOffsetParam(q.Get("offset")),
		Query:  q.Get("q"),
	}
}

func parseLimitParam(raw string) int {
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

func parseOffsetParam(raw string) int {
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// pageLinks mirrors DRF LimitOffsetPagination: next/previous are
// request-derived URLs carrying the current limit and the adjacent offset, or
// null at the ends of the result set.
func pageLinks(r *http.Request, count, limit, offset int) (next, previous *string) {
	// Guard against int overflow when offset or limit are huge: the next
	// page exists only when offset + limit < count. Rewrite the comparison
	// without addition so a bogus next URL cannot be produced.
	if limit > 0 && offset >= 0 && offset < count && count-offset > limit {
		u := pageURL(r, limit, offset+limit)
		next = &u
	}
	if offset > 0 {
		prev := offset - limit
		if prev < 0 {
			prev = 0
		}
		u := pageURL(r, limit, prev)
		previous = &u
	}
	return next, previous
}

func pageURL(r *http.Request, limit, offset int) string {
	u := *r.URL
	q := u.Query()
	q.Set("limit", strconv.Itoa(limit))
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	} else {
		q.Del("offset")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (a *API) listBookmarks(w http.ResponseWriter, r *http.Request, userID int64, archived bool) {
	opts := pageOptionsFromRequest(r)
	page, err := a.service.List(r.Context(), userID, archived, opts)
	if err != nil {
		writeInternalError(w, a.logger, "GET "+bookmarksPath, err)
		return
	}
	results := make([]bookmarkDTO, 0, len(page.Items))
	for _, item := range page.Items {
		results = append(results, newBookmarkDTO(item))
	}
	next, previous := pageLinks(r, page.Count, page.Limit, page.Offset)
	writeJSON(w, http.StatusOK, bookmarkPageDTO{
		Count:    page.Count,
		Next:     next,
		Previous: previous,
		Results:  results,
	})
}

// updateBookmark serves PUT and PATCH. PUT requires the URL (Linkding's
// serializer marks it required); PATCH is partial by definition.
func (a *API) updateBookmark(w http.ResponseWriter, r *http.Request, userID, id int64, requireURL bool) {
	var req updateBookmarkRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if requireURL && req.URL == nil {
		writeValidation(w, "url", "This field is required.")
		return
	}
	if req.TagNames != nil {
		for _, tag := range *req.TagNames {
			if strings.TrimSpace(tag) == "" {
				writeValidation(w, "tag_names", "Tags must not be empty.")
				return
			}
		}
	}
	in := bookmark.UpdateInput{
		URL:         req.URL,
		Title:       req.Title,
		Description: req.Description,
		Notes:       req.Notes,
		Tags:        req.TagNames,
		IsArchived:  req.IsArchived,
		Unread:      req.Unread,
		Shared:      req.Shared,
	}
	var updated bookmark.Bookmark
	var err error
	if requireURL {
		updated, err = a.service.Update(r.Context(), userID, id, in)
	} else {
		updated, err = a.service.Patch(r.Context(), userID, id, in)
	}
	switch {
	case errors.Is(err, bookmark.ErrNotFound):
		writeNotFound(w)
	case errors.Is(err, bookmark.ErrURLConflict):
		writeValidation(w, "url", "A bookmark with this URL already exists.")
	case errors.Is(err, bookmark.ErrInvalidBookmark):
		writeValidation(w, "url", "Enter a valid URL.")
	case err != nil:
		writeInternalError(w, a.logger, r.Method+" "+r.URL.Path, err)
	default:
		writeJSON(w, http.StatusOK, newBookmarkDTO(updated))
	}
}

func (a *API) archiveBookmark(w http.ResponseWriter, r *http.Request, userID, id int64) {
	a.setBookmarkArchived(w, r, userID, id, true)
}

func (a *API) unarchiveBookmark(w http.ResponseWriter, r *http.Request, userID, id int64) {
	a.setBookmarkArchived(w, r, userID, id, false)
}

func (a *API) setBookmarkArchived(w http.ResponseWriter, r *http.Request, userID, id int64, archived bool) {
	var err error
	if archived {
		err = a.service.Archive(r.Context(), userID, id)
	} else {
		err = a.service.Unarchive(r.Context(), userID, id)
	}
	switch {
	case errors.Is(err, bookmark.ErrNotFound):
		writeNotFound(w)
	case err != nil:
		writeInternalError(w, a.logger, r.Method+" "+r.URL.Path, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) createBookmark(w http.ResponseWriter, r *http.Request, userID int64) {
	req, ok := decodeCreateRequest(w, r)
	if !ok {
		return
	}
	if req.URL == nil || strings.TrimSpace(*req.URL) == "" {
		writeValidation(w, "url", "This field is required.")
		return
	}
	url := strings.TrimSpace(*req.URL)
	if err := bookmark.ValidateURL(url); err != nil {
		writeValidation(w, "url", "Enter a valid URL.")
		return
	}

	var tags []string
	if req.TagNames != nil {
		tags = *req.TagNames
		for _, tag := range tags {
			if strings.TrimSpace(tag) == "" {
				writeValidation(w, "tag_names", "Tags must not be empty.")
				return
			}
		}
	}
	in := bookmark.CreateInput{URL: url, Tags: tags}
	if req.Title != nil {
		in.Title = *req.Title
	}
	if req.Description != nil {
		in.Description = *req.Description
	}
	if req.Notes != nil {
		in.Notes = *req.Notes
	}
	if req.Unread != nil {
		in.Unread = *req.Unread
	}
	if req.Shared != nil {
		in.Shared = *req.Shared
	}

	created, err := a.service.Create(r.Context(), userID, in)
	switch {
	case errors.Is(err, bookmark.ErrInvalidBookmark):
		writeValidation(w, "url", "Enter a valid URL.")
	case err != nil:
		writeInternalError(w, a.logger, "POST "+bookmarksPath, err)
	default:
		writeJSON(w, http.StatusCreated, newBookmarkDTO(created))
	}
}

func (a *API) getBookmark(w http.ResponseWriter, r *http.Request, userID, id int64) {
	item, err := a.service.Get(r.Context(), userID, id)
	switch {
	case errors.Is(err, bookmark.ErrNotFound):
		writeNotFound(w)
	case err != nil:
		writeInternalError(w, a.logger, "GET "+r.URL.Path, err)
	default:
		writeJSON(w, http.StatusOK, newBookmarkDTO(item))
	}
}

func (a *API) deleteBookmark(w http.ResponseWriter, r *http.Request, userID, id int64) {
	err := a.service.Delete(r.Context(), userID, id)
	switch {
	case errors.Is(err, bookmark.ErrNotFound):
		writeNotFound(w)
	case err != nil:
		writeInternalError(w, a.logger, "DELETE "+r.URL.Path, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) checkBookmark(w http.ResponseWriter, r *http.Request, userID int64) {
	rawURL := r.URL.Query().Get("url")
	if strings.TrimSpace(rawURL) == "" {
		writeValidation(w, "url", "This field is required.")
		return
	}
	if err := bookmark.ValidateURL(rawURL); err != nil {
		writeValidation(w, "url", "Enter a valid URL.")
		return
	}

	var found *bookmarkDTO
	item, err := a.service.GetByURL(r.Context(), userID, rawURL)
	switch {
	case err == nil:
		dto := newBookmarkDTO(item)
		found = &dto
	case errors.Is(err, bookmark.ErrNotFound), errors.Is(err, bookmark.ErrInvalidBookmark):
		// An unknown URL is a normal check result, not an error.
	default:
		writeInternalError(w, a.logger, "GET "+bookmarksPath+"check/", err)
		return
	}

	writeJSON(w, http.StatusOK, checkDTO{
		Bookmark: found,
		Metadata: metadataDTO{URL: rawURL},
		AutoTags: []string{},
	})
}

func decodeCreateRequest(w http.ResponseWriter, r *http.Request) (createBookmarkRequest, bool) {
	var req createBookmarkRequest
	if !decodeJSONBody(w, r, &req) {
		return req, false
	}
	return req, true
}

// decodeJSONBody decodes a single JSON object from the request body, bounding
// it before decoding so a hostile client cannot make the API allocate without
// limit. DRF rejects trailing JSON values; a single object is the stable
// contract, so a second value is a 400.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil {
		writeInvalidJSON(w)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	if err := decoder.Decode(v); err != nil {
		writeDecodeError(w, err)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			writeDecodeError(w, err)
		} else {
			writeInvalidJSON(w)
		}
		return false
	}
	return true
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = typeErr.Struct
		}
		// encoding/json reports array element failures as "tag_names.0";
		// clients expect the DRF-style top-level field key.
		field, _, _ = strings.Cut(field, ".")
		if field != "" {
			writeValidation(w, field, "Expected a valid value.")
			return
		}
	}
	writeInvalidJSON(w)
}
