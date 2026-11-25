// Package user reads the minimal user-owned profile data needed by the
// Linkding compatibility API. It deliberately exposes no HTTP or DTO types.
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrProfileNotFound is returned when a user has no profile row.
var ErrProfileNotFound = errors.New("user profile not found")

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
