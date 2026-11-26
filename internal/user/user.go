// Package user reads the minimal user-owned profile data needed by the
// Linkding compatibility API. It deliberately exposes no HTTP or DTO types.
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrProfileNotFound is returned when a user has no profile row.
	ErrProfileNotFound = errors.New("user profile not found")
	// ErrNotFound is returned when no user matches an explicit identifier.
	ErrNotFound = errors.New("user not found")
)

// Identity is the minimal user record needed by owner-scoped CLI commands.
// It deliberately carries no credentials or token material.
type Identity struct {
	ID       int64
	Username string
}

// LookupByUsername resolves an explicitly supplied username to a user identity.
// It never creates users and never falls back to a default account.
func LookupByUsername(ctx context.Context, db *sql.DB, username string) (Identity, error) {
	if db == nil {
		return Identity{}, errors.New("user: nil database")
	}
	if strings.TrimSpace(username) == "" {
		return Identity{}, ErrNotFound
	}
	var identity Identity
	err := db.QueryRowContext(ctx, `SELECT id, username FROM users WHERE username = ?`, username).Scan(&identity.ID, &identity.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, ErrNotFound
	}
	if err != nil {
		return Identity{}, fmt.Errorf("lookup user by username: %w", err)
	}
	return identity, nil
}

// LookupByID resolves an explicitly supplied user id. It is the counterpart of
// LookupByUsername for operators that know the numeric owner.
func LookupByID(ctx context.Context, db *sql.DB, id int64) (Identity, error) {
	if db == nil {
		return Identity{}, errors.New("user: nil database")
	}
	if id <= 0 {
		return Identity{}, ErrNotFound
	}
	var identity Identity
	err := db.QueryRowContext(ctx, `SELECT id, username FROM users WHERE id = ?`, id).Scan(&identity.ID, &identity.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, ErrNotFound
	}
	if err != nil {
		return Identity{}, fmt.Errorf("lookup user by id: %w", err)
	}
	return identity, nil
}

// Profile is the internal profile state. Field names follow the database
// schema; the Linkding wire types are produced by the API layer.
type Profile struct {
	Theme                 string
	BookmarkDateDisplay   string
	BookmarkLinkTarget    string
	WebArchiveIntegration string
	TagSearch             int64
	EnableSharing         bool
	EnablePublicSharing   bool
	EnableFavicons        bool
	DisplayURL            bool
	PermanentNotes        string
	SearchPreferences     string
}

// GetProfile returns the profile owned by userID. Only owner-scoped columns
// are selected, so this never crosses user boundaries.
func GetProfile(ctx context.Context, db *sql.DB, userID int64) (Profile, error) {
	if db == nil {
		return Profile{}, errors.New("user: nil database")
	}
	var (
		p              Profile
		sharing        int64
		publicSharing  int64
		enableFavicons int64
		displayURL     int64
		webArchive     string
	)
	err := db.QueryRowContext(ctx, `
		SELECT theme, bookmark_date_display, bookmark_link_target,
		       web_archive_integration, tag_search, enable_sharing,
		       enable_public_sharing, enable_favicons, display_url,
		       permanent_notes, search_preferences
		FROM user_profiles WHERE user_id = ?`, userID).Scan(
		&p.Theme, &p.BookmarkDateDisplay, &p.BookmarkLinkTarget,
		&webArchive, &p.TagSearch, &sharing,
		&publicSharing, &enableFavicons, &displayURL,
		&p.PermanentNotes, &p.SearchPreferences,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrProfileNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("read user profile: %w", err)
	}
	p.WebArchiveIntegration = webArchive
	p.EnableSharing = sharing != 0
	p.EnablePublicSharing = publicSharing != 0
	p.EnableFavicons = enableFavicons != 0
	p.DisplayURL = displayURL != 0
	return p, nil
}
