package linkding

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

const jsonContentType = "application/json; charset=utf-8"

// unauthorizedDTO matches the stable body produced by internal/auth so every
// 401 looks identical whether it comes from middleware or a handler.
type unauthorizedDTO struct {
	Error string `json:"error"`
}

type detailDTO struct {
	Detail string `json:"detail"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeUnauthorized(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, unauthorizedDTO{Error: "unauthorized"})
}

func writeNotFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, detailDTO{Detail: "Not found."})
}

func writeMethodNotAllowed(w http.ResponseWriter, allow ...string) {
	w.Header().Set("Allow", joinMethods(allow...))
	writeJSON(w, http.StatusMethodNotAllowed, detailDTO{Detail: "Method not allowed."})
}

func writeInvalidJSON(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, detailDTO{Detail: "Invalid JSON."})
}

// writeValidation produces the field-keyed object the extension displays on
// 400 responses. Values are always arrays of strings like DRF.
func writeValidation(w http.ResponseWriter, field, message string) {
	writeJSON(w, http.StatusBadRequest, map[string][]string{field: {message}})
}

func writeInternalError(w http.ResponseWriter, logger *slog.Logger, route string, err error) {
	logger.Error("linkding api request failed", "route", route, "error", err)
	writeJSON(w, http.StatusInternalServerError, detailDTO{Detail: "Internal server error."})
}

func joinMethods(methods ...string) string {
	out := ""
	for _, method := range methods {
		if out == "" {
			out = method
			continue
		}
		out += ", " + method
	}
	return out
}
