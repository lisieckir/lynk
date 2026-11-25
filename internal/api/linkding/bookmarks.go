package linkding

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
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

func (a *API) listBookmarks(w http.ResponseWriter, r *http.Request, userID int64) {
	items, err := a.service.List(r.Context(), userID, false)
	if err != nil {
		writeInternalError(w, a.logger, "GET "+bookmarksPath, err)
		return
	}
	results := make([]bookmarkDTO, 0, len(items))
	for _, item := range items {
		results = append(results, newBookmarkDTO(item))
	}
	writeJSON(w, http.StatusOK, bookmarkPageDTO{Count: len(results), Results: results})
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
	if r.Body == nil {
		writeInvalidJSON(w)
		return req, false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	if err := decoder.Decode(&req); err != nil {
		writeDecodeError(w, err)
		return req, false
	}
	// DRF rejects trailing JSON values; a single object is the stable contract.
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			writeDecodeError(w, err)
		} else {
			writeInvalidJSON(w)
		}
		return req, false
	}
	return req, true
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
