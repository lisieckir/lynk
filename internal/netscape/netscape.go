// Package netscape implements bounded, standard-library-only Netscape bookmark
// HTML import and export.
//
// Format decisions (also documented in docs/development.md):
//
//   - Every <DT><A HREF=...> is one bookmark. ADD_DATE/LAST_MODIFIED are Unix
//     seconds and are mapped to date_added/date_modified. TAGS is a
//     comma-separated list. <DD> text becomes the bookmark description.
//   - Netscape has no first-class folder column. Each <H3> ancestor of a bookmark
//     becomes one path component stored as a single tag with the reserved
//     "folder:" prefix (components joined with "/"); export rebuilds the <H3>
//     hierarchy from that tag. Browser root headings ("Bookmarks bar", etc.)
//     are ordinary components: the mapping is literal and lossless for the
//     common case rather than guessing which folders are special.
//   - Netscape has no archived/unread fields either. The reserved tags
//     "lynk:archived" and "lynk:unread" carry those flags. Import consumes them
//     and export re-emits them, so a full export/import round trip preserves
//     both flags. Tags are matched exactly, so an ordinary "work" tag is never
//     reinterpreted.
//   - Import is strict and atomic by default: the whole file is parsed and
//     validated before one database transaction writes any row. A malformed
//     file or an invalid URL therefore leaves the database untouched. The
//     optional ContinueOnError mode skips only per-record validation failures
//     (counted as Failed) and still commits the valid records in one
//     transaction; database errors always abort the transaction.
//   - Duplicate URLs follow bookmark.Service.Create semantics: a URL that
//     already exists for the chosen owner is merged into the stored row, not
//     inserted twice. Ownership is always supplied by the caller; nothing in
//     the HTML can select or change it. Import never fetches URLs and never
//     enqueues metadata work.
package netscape

import (
	"errors"
	"fmt"
	"time"
)

// Limits bound the work an import can perform. Zero fields select the defaults;
// values above the hard caps are rejected rather than silently clamped.
type Limits struct {
	MaxBytes     int64
	MaxBookmarks int
}

const (
	// DefaultMaxBytes is 8 MiB, enough for large human-maintained bookmark files
	// while keeping parser memory predictable.
	DefaultMaxBytes int64 = 8 << 20
	// DefaultMaxBookmarks bounds the number of created/updated rows per import.
	DefaultMaxBookmarks = 10000

	// HardMaxBytes and HardMaxBookmarks cap operator overrides, including CLI
	// flags, so a misconfigured command cannot bypass the memory bound.
	HardMaxBytes     int64 = 64 << 20
	HardMaxBookmarks       = 100000

	MaxTitleBytes        = 16 << 10
	MaxDescriptionBytes  = 64 << 10
	MaxTagsPerBookmark   = 64
	MaxTagBytes          = 255
	MaxFolderDepth       = 16
	MaxFolderTagBytes    = 1024
	MaxExportRecords     = 10000
	HardMaxExportRecords = 100000
)

var (
	// ErrLimitExceeded reports a byte/count bound being crossed.
	ErrLimitExceeded = errors.New("netscape: limit exceeded")
	// ErrMalformed reports deterministic structural HTML errors.
	ErrMalformed = errors.New("netscape: malformed HTML")
	// ErrInvalidURL reports a record whose HREF is not a supported bookmark URL.
	ErrInvalidURL = errors.New("netscape: invalid bookmark URL")
	// ErrInvalidOptions reports bad caller-supplied options.
	ErrInvalidOptions = errors.New("netscape: invalid options")
)

// ParseError adds the 1-based input line to a parse failure. Error text is
// deterministic for a given input: no filesystem paths or clock values appear.
type ParseError struct {
	Line int
	Err  error
}

func (e *ParseError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("netscape: line %d: %v", e.Line, e.Err)
}

func (e *ParseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Record is one parsed bookmark. Dates are nil when ADD_DATE/LAST_MODIFIED was
// absent or unusable; Tags already includes the synthetic "folder:" tag.
type Record struct {
	URL          string
	Title        string
	Description  string
	Tags         []string
	DateAdded    *time.Time
	DateModified *time.Time
}

// Summary is the deterministic outcome of an import. Skipped is reserved for
// records intentionally omitted by a policy; duplicates are merged rather than
// skipped, and strict mode fails before writing, so both are zero on the
// default path.
type Summary struct {
	Parsed  int
	Created int
	Merged  int
	Skipped int
	Failed  int
}

// ImportOptions configures Import. The zero value is the safe strict default.
type ImportOptions struct {
	Limits          Limits
	ContinueOnError bool
}

// ArchiveFilter selects which owner rows an export includes. Netscape has no
// archive flag, so archived bookmarks are opt-in.
type ArchiveFilter string

const (
	FilterActive   ArchiveFilter = "active"
	FilterArchived ArchiveFilter = "archived"
	FilterAll      ArchiveFilter = "all"
)

// ExportOptions configures Export. The zero value exports active bookmarks.
type ExportOptions struct {
	Filter     ArchiveFilter
	MaxRecords int
	// Written, when non-nil, receives the number of exported bookmarks on success.
	Written *int
}

func (l Limits) withDefaults() (Limits, error) {
	if l.MaxBytes == 0 {
		l.MaxBytes = DefaultMaxBytes
	}
	if l.MaxBookmarks == 0 {
		l.MaxBookmarks = DefaultMaxBookmarks
	}
	if l.MaxBytes < 0 || l.MaxBytes > HardMaxBytes {
		return Limits{}, fmt.Errorf("%w: max bytes must be between 1 and %d", ErrInvalidOptions, HardMaxBytes)
	}
	if l.MaxBookmarks < 0 || l.MaxBookmarks > HardMaxBookmarks {
		return Limits{}, fmt.Errorf("%w: max bookmarks must be between 1 and %d", ErrInvalidOptions, HardMaxBookmarks)
	}
	return l, nil
}
