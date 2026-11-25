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

// CreateFirstUserAndToken creates the first user (if none exists) and creates a token for them.
// If users already exist, it returns an error asking to specify which user (for now, we require no users exist).
func CreateFirstUserAndToken(ctx context.Context, db *sql.DB, username, tokenName string) (*TokenInfo, error) {
	if username == "" {
		username = "admin"
	}
	if tokenName == "" {
		tokenName = "default"
	}
	// Check if any user exists
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	var userID int64
	if count == 0 {
		// Create first user
		now := time.Now().UTC().Format(time.RFC3339)
		// password_hash is required but not used for token auth bootstrap
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
		// Create default user profile
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
		return nil, errors.New("users already exist; token creation for existing users not implemented in this task")
	}

	// Generate token
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
		return nil, err
	}
	digest := HashToken(rawToken)

	var userID int64
	var username string
	var revokedAt sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT t.user_id, u.username, t.revoked_at
		 FROM api_tokens t
		 JOIN users u ON u.id = t.user_id
		 WHERE t.token_digest = ? AND t.revoked_at IS NULL
		 LIMIT 1`,
		digest,
	).Scan(&userID, &username, &revokedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("token not found or revoked")
		}
		return nil, fmt.Errorf("lookup token: %w", err)
	}
	// Update last_used_at (best effort, don't fail the lookup)
	_, _ = db.ExecContext(ctx, "UPDATE api_tokens SET last_used_at = ? WHERE user_id = ? AND token_digest = ?", time.Now().UTC().Format(time.RFC3339), userID, digest)

	return &User{ID: userID, Username: username}, nil
}
