package linkding

import (
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/lisieckir/lynk/internal/auth"
	"github.com/lisieckir/lynk/internal/bookmark"
)

const bookmarksPath = "/api/bookmarks/"

// API routes Linkding-compatible requests to owner-scoped handlers. It is
// always wrapped in auth.RequireAuth by NewHandler.
type API struct {
	db         *sql.DB
	service    *bookmark.Service
	serviceErr error
	logger     *slog.Logger
}

// NewHandler returns a handler for all /api/ routes. /health remains outside
// this handler and therefore unauthenticated.
func NewHandler(db *sql.DB, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	service, err := bookmark.NewService(db)
	api := &API{db: db, service: service, serviceErr: err, logger: logger}
	if db == nil {
		// There is no authentication store to consult; fail closed without
		// panicking on a nil *sql.DB.
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeInternalError(w, logger, "api", errors.New("linkding: nil database"))
		})
	}
	return auth.RequireAuth(db, api)
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a.serviceErr != nil || a.service == nil {
		writeInternalError(w, a.logger, r.URL.Path, errors.New("linkding: bookmark service unavailable"))
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	path := r.URL.Path
	switch path {
	case "/api/user/profile/":
		if a.rejectMethod(w, r, http.MethodGet) {
			return
		}
		a.getProfile(w, r, userID)
		return
	case "/api/tags/":
		if a.rejectMethod(w, r, http.MethodGet) {
			return
		}
		a.listTags(w, r, userID)
		return
	case bookmarksPath:
		switch r.Method {
		case http.MethodGet:
			a.listBookmarks(w, r, userID, false)
		case http.MethodPost:
			a.createBookmark(w, r, userID)
		default:
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	case bookmarksPath + "archived/":
		if a.rejectMethod(w, r, http.MethodGet) {
			return
		}
		a.listBookmarks(w, r, userID, true)
		return
	case bookmarksPath + "check/":
		if a.rejectMethod(w, r, http.MethodGet) {
			return
		}
		a.checkBookmark(w, r, userID)
		return
	}

	// Detail routes are the only dynamic routes. Anything without the exact
	// trailing slash, a non-numeric id or an extra segment is a plain 404:
	// no implicit redirect is issued for API clients.
	if strings.HasPrefix(path, bookmarksPath) && strings.HasSuffix(path, "/") {
		rest := strings.TrimSuffix(strings.TrimPrefix(path, bookmarksPath), "/")
		idStr := rest
		action := ""
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			idStr = rest[:i]
			action = rest[i+1:]
			if action == "" || strings.Contains(action, "/") {
				writeNotFound(w)
				return
			}
		}
		if idStr != "" && isDecimal(idStr) {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil || id <= 0 {
				writeNotFound(w)
				return
			}
			switch action {
			case "":
				switch r.Method {
				case http.MethodGet:
					a.getBookmark(w, r, userID, id)
				case http.MethodPut:
					a.updateBookmark(w, r, userID, id, true)
				case http.MethodPatch:
					a.updateBookmark(w, r, userID, id, false)
				case http.MethodDelete:
					a.deleteBookmark(w, r, userID, id)
				default:
					writeMethodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete)
				}
			case "archive":
				if a.rejectMethod(w, r, http.MethodPost) {
					return
				}
				a.archiveBookmark(w, r, userID, id)
			case "unarchive":
				if a.rejectMethod(w, r, http.MethodPost) {
					return
				}
				a.unarchiveBookmark(w, r, userID, id)
			default:
				writeNotFound(w)
			}
			return
		}
	}
	writeNotFound(w)
}

func (a *API) rejectMethod(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	for _, method := range allowed {
		if r.Method == method {
			return false
		}
	}
	writeMethodNotAllowed(w, allowed...)
	return true
}

func isDecimal(value string) bool {
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
