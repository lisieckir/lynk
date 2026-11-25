// Package linkding implements the minimal Linkding-compatible HTTP surface.
// The types in this file are API-only DTOs: internal database and domain fields
// never appear in serialized responses.
package linkding

import (
	"encoding/json"
	"time"

	"github.com/lisieckir/lynk/internal/bookmark"
	"github.com/lisieckir/lynk/internal/user"
)

// profileVersion is the compatibility value reported to the extension until
// build metadata is wired into the API layer.
const profileVersion = "dev"

// bookmarkDTO mirrors the stable Linkding BookmarkSerializer field set.
type bookmarkDTO struct {
	ID                    int64    `json:"id"`
	URL                   string   `json:"url"`
	Title                 string   `json:"title"`
	Description           string   `json:"description"`
	Notes                 string   `json:"notes"`
	WebArchiveSnapshotURL *string  `json:"web_archive_snapshot_url"`
	FaviconURL            *string  `json:"favicon_url"`
	PreviewImageURL       *string  `json:"preview_image_url"`
	IsArchived            bool     `json:"is_archived"`
	Unread                bool     `json:"unread"`
	Shared                bool     `json:"shared"`
	TagNames              []string `json:"tag_names"`
	DateAdded             string   `json:"date_added"`
	DateModified          string   `json:"date_modified"`
	WebsiteTitle          any      `json:"website_title"`
	WebsiteDescription    any      `json:"website_description"`
}

type tagDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	DateAdded string `json:"date_added"`
}

// profileDTO maps internal profile flags to Linkding's wire types. In
// particular Linkding emits tag_search as a string and permanent_notes as a
// boolean, while Lynk stores an integer and a text value respectively.
type profileDTO struct {
	Theme                 string         `json:"theme"`
	BookmarkDateDisplay   string         `json:"bookmark_date_display"`
	BookmarkLinkTarget    string         `json:"bookmark_link_target"`
	WebArchiveIntegration string         `json:"web_archive_integration"`
	TagSearch             string         `json:"tag_search"`
	EnableSharing         bool           `json:"enable_sharing"`
	EnablePublicSharing   bool           `json:"enable_public_sharing"`
	EnableFavicons        bool           `json:"enable_favicons"`
	DisplayURL            bool           `json:"display_url"`
	PermanentNotes        bool           `json:"permanent_notes"`
	SearchPreferences     map[string]any `json:"search_preferences"`
	Version               string         `json:"version"`
}

type metadataDTO struct {
	URL          string `json:"url"`
	Title        any    `json:"title"`
	Description  any    `json:"description"`
	PreviewImage any    `json:"preview_image"`
}

type checkDTO struct {
	Bookmark *bookmarkDTO `json:"bookmark"`
	Metadata metadataDTO  `json:"metadata"`
	AutoTags []string     `json:"auto_tags"`
}

// bookmarkPageDTO keeps Linkding's LimitOffsetPagination envelope: count is
// the total number of matching rows and next/previous are request-derived
// URLs that are null at the ends of the result set.
type bookmarkPageDTO struct {
	Count    int           `json:"count"`
	Next     *string       `json:"next"`
	Previous *string       `json:"previous"`
	Results  []bookmarkDTO `json:"results"`
}

type tagPageDTO struct {
	Count    int      `json:"count"`
	Next     *string  `json:"next"`
	Previous *string  `json:"previous"`
	Results  []tagDTO `json:"results"`
}

func newBookmarkDTO(b bookmark.Bookmark) bookmarkDTO {
	tags := b.Tags
	if tags == nil {
		tags = []string{}
	}
	return bookmarkDTO{
		ID:           b.ID,
		URL:          b.URL,
		Title:        b.Title,
		Description:  b.Description,
		Notes:        b.Notes,
		IsArchived:   b.IsArchived,
		Unread:       b.Unread,
		Shared:       b.Shared,
		TagNames:     tags,
		DateAdded:    wireStamp(b.DateAdded),
		DateModified: wireStamp(b.DateModified),
	}
}

func newTagDTO(t bookmark.Tag) tagDTO {
	return tagDTO{ID: t.ID, Name: t.Name, DateAdded: wireStamp(t.DateAdded)}
}

func newProfileDTO(p user.Profile) profileDTO {
	tagSearch := "strict"
	if p.TagSearch != 0 {
		tagSearch = "lax"
	}
	preferences := map[string]any{}
	if err := json.Unmarshal([]byte(p.SearchPreferences), &preferences); err != nil || preferences == nil {
		preferences = map[string]any{}
	}
	return profileDTO{
		Theme:                 p.Theme,
		BookmarkDateDisplay:   p.BookmarkDateDisplay,
		BookmarkLinkTarget:    p.BookmarkLinkTarget,
		WebArchiveIntegration: p.WebArchiveIntegration,
		TagSearch:             tagSearch,
		EnableSharing:         p.EnableSharing,
		EnablePublicSharing:   p.EnablePublicSharing,
		EnableFavicons:        p.EnableFavicons,
		DisplayURL:            p.DisplayURL,
		PermanentNotes:        p.PermanentNotes != "",
		SearchPreferences:     preferences,
		Version:               profileVersion,
	}
}

func wireStamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
