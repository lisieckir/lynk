package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
)

type contextKey string

const userContextKey contextKey = "authUser"

// WithUser stores the authenticated user in the context.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// UserFromContext returns the authenticated user from the context, if present.
func UserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userContextKey).(*User)
	return u, ok
}

// UserIDFromContext returns the authenticated user ID from the context, if present.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	u, ok := UserFromContext(ctx)
	if !ok || u == nil {
		return 0, false
	}
	return u.ID, true
}

// RequireAuth is HTTP middleware that authenticates requests using the Authorization header.
// On failure it responds with a stable JSON 401 body and does not log secrets.
func RequireAuth(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := Authenticate(r.Context(), db, r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		ctx := WithUser(r.Context(), u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
