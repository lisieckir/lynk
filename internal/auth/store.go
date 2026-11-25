package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNoUser is returned when no user exists in the database.
	ErrNoUser = errors.New("no user found")
	// ErrTokenNotFound is returned when no non-revoked token matches the given prefix.
	ErrTokenNotFound = errors.New("token not found")
	// ErrTokenPrefixAmbiguous is returned when multiple tokens match the given prefix.
	ErrTokenPrefixAmbiguous = errors.New("token prefix is ambiguous")
)

// User represents a minimal user record.
type User struct {
	ID       int64
	Username string
}

// TokenInfo is returned after creating a token.
type TokenInfo struct {
	ID        int64
	UserID    int64
	Token     string // raw token, returned once
	TokenName string
	Prefix    string
	CreatedAt time.Time
}

// CreateToken creates a token for the given username. If no users exist, it creates the first user
// with that username (bootstrap). If users exist, the username must match an existing user.
func CreateToken(ctx context.Context, db *sql.DB, username, tokenName string) (*TokenInfo, error) {
	if username == "" {
		username = "admin"
	}
	if tokenName == "" {
		tokenName = "default"
	}
	var userID int64
	// Check if any user exists
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if count == 0 {
		// Create first user (bootstrap)
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := db.ExecContext(ctx,
			"INSERT INTO users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)",
			username, "!", now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
		userID, err = res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("user id: %w", err)
		}
		_, err = db.ExecContext(ctx,
			`INSERT INTO user_profiles (user_id, theme, bookmark_date_display, bookmark_link_target, web_archive_integration,
				tag_search, enable_sharing, enable_public_sharing, enable_favicons, display_url, permanent_notes, search_preferences)
			 VALUES (?, '', '', '', '', 0, 0, 0, 1, 0, '', '{}')`,
			userID,
		)
		if err != nil {
			return nil, fmt.Errorf("create user profile: %w", err)
		}
	} else {
		// Find existing user by username
		err := db.QueryRowContext(ctx, "SELECT id FROM users WHERE username = ?", username).Scan(&userID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("user %q not found", username)
			}
			return nil, fmt.Errorf("lookup user: %w", err)
		}
	}

	rawToken, err := GenerateToken()
	if err != nil {
		return nil, err
	}
	digest := HashToken(rawToken)
	prefix := TokenPrefix(rawToken)
	now := time.Now().UTC().Format(time.RFC3339)

	res, err := db.ExecContext(ctx,
		"INSERT INTO api_tokens (user_id, token_digest, token_prefix, name, created_at) VALUES (?, ?, ?, ?, ?)",
		userID, digest, prefix, tokenName, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create api token: %w", err)
	}
	tokenID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("token id: %w", err)
	}

	return &TokenInfo{
		ID:        tokenID,
		UserID:    userID,
		Token:     rawToken,
		TokenName: tokenName,
		Prefix:    prefix,
	}, nil
}

// LookupByToken finds a non-revoked token by raw token string and returns the associated user.
// It updates last_used_at on successful lookup.
func LookupByToken(ctx context.Context, db *sql.DB, rawToken string) (*User, error) {
	if err := ValidateTokenFormat(rawToken); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}
	digest := HashToken(rawToken)

	var userID int64
	var username string
	err := db.QueryRowContext(ctx,
		`SELECT t.user_id, u.username
		 FROM api_tokens t
		 JOIN users u ON u.id = t.user_id
		 WHERE t.token_digest = ? AND t.revoked_at IS NULL
		 LIMIT 1`,
		digest,
	).Scan(&userID, &username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: token not found or revoked", ErrUnauthorized)
		}
		return nil, fmt.Errorf("lookup token: %w", err)
	}
	// Update last_used_at (best effort, don't fail the lookup)
	_, _ = db.ExecContext(ctx, "UPDATE api_tokens SET last_used_at = ? WHERE user_id = ? AND token_digest = ?", time.Now().UTC().Format(time.RFC3339), userID, digest)

	return &User{ID: userID, Username: username}, nil
}

// RevokeByPrefix revokes a non-revoked token identified by its prefix.
// Returns ErrTokenNotFound if none, ErrTokenPrefixAmbiguous if multiple match.
// This is safe: it does not reveal token details beyond the operation result.
func RevokeByPrefix(ctx context.Context, db *sql.DB, prefix string) (retErr error) {
	if prefix == "" {
		return ErrTokenNotFound
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id FROM api_tokens WHERE token_prefix = ? AND revoked_at IS NULL`,
		prefix,
	)
	if err != nil {
		return fmt.Errorf("revoke lookup: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("revoke close rows: %w", cerr)
		}
	}()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("revoke scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("revoke rows: %w", err)
	}
	if len(ids) == 0 {
		return ErrTokenNotFound
	}
	if len(ids) > 1 {
		return ErrTokenPrefixAmbiguous
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at = ? WHERE id = ?`, now, ids[0]); err != nil {
		return fmt.Errorf("revoke update: %w", err)
	}
	return nil
}
