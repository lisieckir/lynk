package auth

import (
	"context"
	"database/sql"
	"net/http"
)

// Authenticate extracts the token from the Authorization header and looks it up in the database.
// Returns the authenticated user or an error.
func Authenticate(ctx context.Context, db *sql.DB, r *http.Request) (*User, error) {
	parsed, err := ParseAuthorization(r.Header.Get("Authorization"))
	if err != nil {
		return nil, err
	}
	return LookupByToken(ctx, db, parsed.Token)
}
