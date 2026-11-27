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

// User represents an authenticated user record.
type User struct {
	ID       int64
	Username string
	Role     string
	Disabled bool
}

// UserSummary is user info without sensitive fields.
type UserSummary struct {
	ID        int64
	Username  string
	Role      string
	Disabled  bool
	CreatedAt string
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

// TokenMeta is token metadata returned to users/admins (no raw/digest).
type TokenMeta struct {
	ID         int64
	Name       string
	Prefix     string
	CreatedAt  string
	RevokedAt  *string
	LastUsedAt *string
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
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if count == 0 {
		// Create first user (bootstrap) — admin, with unusable password hash
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := db.ExecContext(ctx,
			"INSERT INTO users (username, password_hash, role, disabled, created_at, updated_at) VALUES (?, ?, 'admin', 0, ?, ?)",
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
		err := db.QueryRowContext(ctx, "SELECT id FROM users WHERE username = ?", username).Scan(&userID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("user %q not found", username)
			}
			return nil, fmt.Errorf("lookup user: %w", err)
		}
	}
	return CreateTokenForUser(ctx, db, userID, tokenName)
}

// CreateTokenForUser creates a token for an existing user by userID.
func CreateTokenForUser(ctx context.Context, db *sql.DB, userID int64, tokenName string) (*TokenInfo, error) {
	if tokenName == "" {
		tokenName = "default"
	}
	// Verify user exists and not disabled
	var disabled int
	err := db.QueryRowContext(ctx, "SELECT disabled FROM users WHERE id = ?", userID).Scan(&disabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoUser
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if disabled != 0 {
		return nil, errors.New("user is disabled")
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
		CreatedAt: time.Time{}, // not critical to parse
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
	var username, role string
	var disabled int
	err := db.QueryRowContext(ctx,
		`SELECT t.user_id, u.username, u.role, u.disabled
		 FROM api_tokens t
		 JOIN users u ON u.id = t.user_id
		 WHERE t.token_digest = ? AND t.revoked_at IS NULL
		 LIMIT 1`,
		digest,
	).Scan(&userID, &username, &role, &disabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: token not found or revoked", ErrUnauthorized)
		}
		return nil, fmt.Errorf("lookup token: %w", err)
	}
	if disabled != 0 {
		return nil, fmt.Errorf("%w: token not found or revoked", ErrUnauthorized)
	}
	// Update last_used_at (best effort, don't fail the lookup)
	_, _ = db.ExecContext(ctx, "UPDATE api_tokens SET last_used_at = ? WHERE user_id = ? AND token_digest = ?", time.Now().UTC().Format(time.RFC3339), userID, digest)

	return &User{ID: userID, Username: username, Role: role, Disabled: false}, nil
}

// AuthenticatePassword authenticates a user by username and password.
// Returns generic errors for invalid credentials or disabled users.
func AuthenticatePassword(ctx context.Context, db *sql.DB, username, password string) (*User, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := ValidatePassword(password); err != nil {
		return nil, ErrInvalidCredentials
	}
	var uid int64
	var pwHash, role string
	var disabled int
	err := db.QueryRowContext(ctx, "SELECT id, password_hash, role, disabled FROM users WHERE username = ?", username).Scan(&uid, &pwHash, &role, &disabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if disabled != 0 {
		return nil, ErrInvalidCredentials
	}
	if pwHash == "!" || pwHash == "" {
		return nil, ErrInvalidCredentials
	}
	if err := CheckPassword(pwHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}
	return &User{ID: uid, Username: username, Role: role, Disabled: false}, nil
}

// CreateUser creates a new user with password (admin action). Role is always 'user' unless specified otherwise by caller logic; but requirement says "create users" in admin panel — typically creates regular users. Let us set role='user'.
func CreateUser(ctx context.Context, db *sql.DB, username, password string) (*UserSummary, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		"INSERT INTO users (username, password_hash, role, disabled, created_at, updated_at) VALUES (?, ?, 'user', 0, ?, ?)",
		username, hash, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	uid, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("user id: %w", err)
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO user_profiles (user_id, theme, bookmark_date_display, bookmark_link_target, web_archive_integration,
			tag_search, enable_sharing, enable_public_sharing, enable_favicons, display_url, permanent_notes, search_preferences)
		 VALUES (?, '', '', '', '', 0, 0, 0, 1, 0, '', '{}')`,
		uid,
	)
	if err != nil {
		return nil, fmt.Errorf("create user profile: %w", err)
	}
	return &UserSummary{ID: uid, Username: username, Role: "user", Disabled: false, CreatedAt: now}, nil
}

// ListUsers returns all users (no password hashes).
func ListUsers(ctx context.Context, db *sql.DB) (out []UserSummary, retErr error) {
	rows, err := db.QueryContext(ctx, "SELECT id, username, role, disabled, created_at FROM users ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close rows: %w", cerr)
		}
	}()
	for rows.Next() {
		var u UserSummary
		var disabled int
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &disabled, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.Disabled = disabled != 0
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

// GetUserByID returns user summary by id.
func GetUserByID(ctx context.Context, db *sql.DB, id int64) (*UserSummary, error) {
	var u UserSummary
	var disabled int
	err := db.QueryRowContext(ctx, "SELECT id, username, role, disabled, created_at FROM users WHERE id = ?", id).Scan(&u.ID, &u.Username, &u.Role, &disabled, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoUser
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	u.Disabled = disabled != 0
	return &u, nil
}

// SetUserDisabled sets disabled state. When disabling, revokes all active tokens and sets session_valid_after.
func SetUserDisabled(ctx context.Context, db *sql.DB, userID int64, disabled bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if disabled {
		// Disable: set disabled, disabled_at, session_valid_after; revoke all tokens
		_, err := db.ExecContext(ctx, "UPDATE users SET disabled = 1, disabled_at = ?, session_valid_after = ?, updated_at = ? WHERE id = ?", now, now, now, userID)
		if err != nil {
			return fmt.Errorf("disable user: %w", err)
		}
		_, err = db.ExecContext(ctx, "UPDATE api_tokens SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL", now, userID)
		if err != nil {
			return fmt.Errorf("revoke tokens: %w", err)
		}
		return nil
	}
	// Enable: clear disabled
	_, err := db.ExecContext(ctx, "UPDATE users SET disabled = 0, disabled_at = NULL, updated_at = ? WHERE id = ?", now, userID)
	if err != nil {
		return fmt.Errorf("enable user: %w", err)
	}
	return nil
}

// ResetPassword resets a user's password (admin action).
func ResetPassword(ctx context.Context, db *sql.DB, userID int64, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.ExecContext(ctx, "UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?", hash, now, userID)
	if err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNoUser
	}
	return nil
}

// ListTokensForUser lists token metadata for a user (no secrets).
func ListTokensForUser(ctx context.Context, db *sql.DB, userID int64) (out []TokenMeta, retErr error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, token_prefix, created_at, revoked_at, last_used_at FROM api_tokens WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close rows: %w", cerr)
		}
	}()
	for rows.Next() {
		var t TokenMeta
		var revokedAt, lastUsedAt sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Prefix, &t.CreatedAt, &revokedAt, &lastUsedAt); err != nil {
			return nil, fmt.Errorf("scan token: %w", err)
		}
		if revokedAt.Valid {
			t.RevokedAt = &revokedAt.String
		}
		if lastUsedAt.Valid {
			t.LastUsedAt = &lastUsedAt.String
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

// RevokeTokenByID revokes a token by ID if it belongs to ownerUserID (or if isAdmin true, allows any).
func RevokeTokenByID(ctx context.Context, db *sql.DB, tokenID int64, ownerUserID int64, isAdmin bool) error {
	var uid int64
	err := db.QueryRowContext(ctx, "SELECT user_id FROM api_tokens WHERE id = ?", tokenID).Scan(&uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("lookup token: %w", err)
	}
	if !isAdmin && uid != ownerUserID {
		return ErrTokenNotFound // don't leak existence
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.ExecContext(ctx, "UPDATE api_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL", now, tokenID)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return nil
}

// RevokeByPrefix revokes a non-revoked token identified by its prefix.
// Returns ErrTokenNotFound if none, ErrTokenPrefixAmbiguous if multiple match.
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
