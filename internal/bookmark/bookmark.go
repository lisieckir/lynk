// Package bookmark implements bookmark ownership and persistence rules.
package bookmark

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	ErrNotFound        = errors.New("bookmark not found")
	ErrURLConflict     = errors.New("bookmark URL already exists")
	ErrInvalidBookmark = errors.New("invalid bookmark")
)

// Tag is a user's reusable tag.
type Tag struct {
	ID        int64
	UserID    int64
	Name      string
	DateAdded time.Time
}

// User identifies the owner of a bookmark. Authentication is outside this package.
type User struct {
	ID       int64
	Username string
}

// Bookmark is an internal domain object; it is not an API DTO.
type Bookmark struct {
	ID                    int64
	UserID                int64
	URL                   string
	Title                 string
	Description           string
	Notes                 string
	Tags                  []string
	IsArchived            bool
	Unread                bool
	Shared                bool
	WebArchiveSnapshotURL *string
	FaviconURL            *string
	PreviewImageURL       *string
	DateAdded             time.Time
	DateModified          time.Time
}

// CreateInput contains the writable fields for create/duplicate-merge.
// DateAdded and DateModified are pointers because Linkding's create uses them
// both as "set this value" and as "unset means server clock"; a nil pointer
// selects the current time. On the duplicate-merge path Linkding only copies
// title, description, notes, unread and shared into the stored row, so
// IsArchived, DateAdded and DateModified are applied on insert only and the
// merge keeps the stored values (date_modified is bumped to now by merge).
type CreateInput struct {
	URL, Title, Description, Notes string
	Tags                           []string
	IsArchived                     bool
	Unread, Shared                 bool
	DateAdded, DateModified        *time.Time
}

// UpdateInput uses pointers to distinguish omitted fields from explicit zero values.
type UpdateInput struct {
	URL          *string
	Title        *string
	Description  *string
	Notes        *string
	Tags         *[]string
	IsArchived   *bool
	Unread       *bool
	Shared       *bool
	DateAdded    *time.Time
	DateModified *time.Time
}

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func NewService(db *sql.DB) (*Service, error) {
	if db == nil {
		return nil, errors.New("bookmark: nil database")
	}
	return &Service{db: db, now: time.Now}, nil
}

func (s *Service) Create(ctx context.Context, userID int64, in CreateInput) (Bookmark, error) {
	if userID <= 0 || strings.TrimSpace(in.URL) == "" {
		return Bookmark{}, ErrInvalidBookmark
	}
	in.URL = strings.TrimSpace(in.URL)
	if err := ValidateURL(in.URL); err != nil {
		return Bookmark{}, fmt.Errorf("%w: %v", ErrInvalidBookmark, err)
	}
	normalized, err := NormalizeURL(in.URL)
	if err != nil {
		return Bookmark{}, fmt.Errorf("%w: %v", ErrInvalidBookmark, err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Bookmark{}, err
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := getByNormalized(ctx, tx, userID, normalized)
	now := s.now().UTC()
	switch {
	case err == nil:
		// Linkding's duplicate POST merges the submitted values into the
		// current row: _merge_bookmark_data copies only title, description,
		// notes, unread and shared, so the stored archive state and date_added
		// survive an extension edit while date_modified is bumped to now.
		_, err = tx.ExecContext(ctx, `UPDATE bookmarks SET url=?, title=?, description=?, notes=?, unread=?, shared=?, date_modified=? WHERE id=? AND user_id=?`,
			in.URL, in.Title, in.Description, in.Notes, in.Unread, in.Shared, stamp(now), existing.ID, userID)
		if err != nil {
			return Bookmark{}, err
		}
		if err = replaceTags(ctx, tx, userID, existing.ID, in.Tags, now); err != nil {
			return Bookmark{}, err
		}
		existing, err = getByID(ctx, tx, userID, existing.ID)
	case errors.Is(err, sql.ErrNoRows):
		// Linkding only fills date_added/date_modified when the client did not
		// send them, so clients can sync existing dates through the REST API.
		dateAdded, dateModified := now, now
		if in.DateAdded != nil {
			dateAdded = in.DateAdded.UTC()
		}
		if in.DateModified != nil {
			dateModified = in.DateModified.UTC()
		}
		result, execErr := tx.ExecContext(ctx, `INSERT INTO bookmarks(user_id,url,url_normalized,title,description,notes,is_archived,unread,shared,date_added,date_modified)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`, userID, in.URL, normalized, in.Title, in.Description, in.Notes, in.IsArchived, in.Unread, in.Shared, stamp(dateAdded), stamp(dateModified))
		if execErr != nil {
			return Bookmark{}, execErr
		}
		id, execErr := result.LastInsertId()
		if execErr != nil {
			return Bookmark{}, execErr
		}
		if err = replaceTags(ctx, tx, userID, id, in.Tags, now); err != nil {
			return Bookmark{}, err
		}
		existing, err = getByID(ctx, tx, userID, id)
	default:
		return Bookmark{}, err
	}
	if err != nil {
		return Bookmark{}, err
	}
	if err := tx.Commit(); err != nil {
		return Bookmark{}, err
	}
	return existing, nil
}

// Update performs PUT semantics: URL is required and collisions with another
// owned bookmark are errors. Patch permits URL omission.
func (s *Service) Update(ctx context.Context, userID, id int64, in UpdateInput) (Bookmark, error) {
	return s.update(ctx, userID, id, in, true)
}

func (s *Service) Patch(ctx context.Context, userID, id int64, in UpdateInput) (Bookmark, error) {
	return s.update(ctx, userID, id, in, false)
}

func (s *Service) update(ctx context.Context, userID, id int64, in UpdateInput, requireURL bool) (Bookmark, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Bookmark{}, err
	}
	defer func() { _ = tx.Rollback() }()
	current, err := getByID(ctx, tx, userID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Bookmark{}, ErrNotFound
	}
	if err != nil {
		return Bookmark{}, err
	}
	if requireURL && in.URL == nil {
		return Bookmark{}, fmt.Errorf("%w: URL required", ErrInvalidBookmark)
	}
	if in.URL != nil {
		u := strings.TrimSpace(*in.URL)
		if err := ValidateURL(u); err != nil {
			return Bookmark{}, fmt.Errorf("%w: %v", ErrInvalidBookmark, err)
		}
		normalized, normErr := NormalizeURL(u)
		if normErr != nil {
			return Bookmark{}, fmt.Errorf("%w: %v", ErrInvalidBookmark, normErr)
		}
		var collision int64
		err = tx.QueryRowContext(ctx, `SELECT id FROM bookmarks WHERE user_id=? AND url_normalized=? AND id<>?`, userID, normalized, id).Scan(&collision)
		if err == nil {
			return Bookmark{}, ErrURLConflict
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return Bookmark{}, err
		}
		current.URL = u
		if _, err = tx.ExecContext(ctx, `UPDATE bookmarks SET url=?,url_normalized=? WHERE id=? AND user_id=?`, u, normalized, id, userID); err != nil {
			return Bookmark{}, err
		}
	}
	if in.Title != nil {
		current.Title = *in.Title
	}
	if in.Description != nil {
		current.Description = *in.Description
	}
	if in.Notes != nil {
		current.Notes = *in.Notes
	}
	if in.IsArchived != nil {
		current.IsArchived = *in.IsArchived
	}
	if in.Unread != nil {
		current.Unread = *in.Unread
	}
	if in.Shared != nil {
		current.Shared = *in.Shared
	}
	now := s.now().UTC()
	// date_added only changes when the client sends it. date_modified defaults
	// to the current time — Linkding's update service always stamps it — and a
	// value the client sends explicitly is persisted as-is; see
	// docs/api-compatibility.md for that documented difference.
	dateAdded, dateModified := current.DateAdded, now
	if in.DateAdded != nil {
		dateAdded = in.DateAdded.UTC()
	}
	if in.DateModified != nil {
		dateModified = in.DateModified.UTC()
	}
	_, err = tx.ExecContext(ctx, `UPDATE bookmarks SET title=?,description=?,notes=?,is_archived=?,unread=?,shared=?,date_added=?,date_modified=? WHERE id=? AND user_id=?`,
		current.Title, current.Description, current.Notes, current.IsArchived, current.Unread, current.Shared, stamp(dateAdded), stamp(dateModified), id, userID)
	if err != nil {
		return Bookmark{}, err
	}
	if in.Tags != nil {
		if err := replaceTags(ctx, tx, userID, id, *in.Tags, now); err != nil {
			return Bookmark{}, err
		}
	}
	result, err := getByID(ctx, tx, userID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Bookmark{}, ErrNotFound
	}
	if err != nil {
		return Bookmark{}, err
	}
	if err := tx.Commit(); err != nil {
		return Bookmark{}, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, userID, id int64) (Bookmark, error) {
	b, err := getByID(ctx, s.db, userID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Bookmark{}, ErrNotFound
	}
	return b, err
}

// GetByURL returns the bookmark owned by userID whose normalized URL matches
// rawURL. Linkding's check endpoint uses the same normalized lookup as create,
// so a caller cannot discover another user's row.
func (s *Service) GetByURL(ctx context.Context, userID int64, rawURL string) (Bookmark, error) {
	if userID <= 0 || strings.TrimSpace(rawURL) == "" {
		return Bookmark{}, ErrInvalidBookmark
	}
	normalized, err := NormalizeURL(rawURL)
	if err != nil {
		return Bookmark{}, fmt.Errorf("%w: %v", ErrInvalidBookmark, err)
	}
	b, err := getByNormalized(ctx, s.db, userID, normalized)
	if errors.Is(err, sql.ErrNoRows) {
		return Bookmark{}, ErrNotFound
	}
	return b, err
}

// TagPage is one window of tags plus the total number of tags the user owns.
type TagPage struct {
	Count  int
	Items  []Tag
	Limit  int
	Offset int
}

// ListTags returns one page of tags owned by userID in a stable name order,
// bounded like the bookmark list: limit <= 0 selects DefaultLimit and a
// negative offset reads as zero. It is the read side used by /api/tags/ and
// never includes tags from other users.
func (s *Service) ListTags(ctx context.Context, userID int64, limit, offset int) (TagPage, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if offset < 0 {
		offset = 0
	}
	var count int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM tags WHERE user_id = ?`, userID).Scan(&count); err != nil {
		return TagPage{}, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, date_added FROM tags WHERE user_id = ?
		 ORDER BY name COLLATE BINARY, id LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return TagPage{}, err
	}
	var tags []Tag
	for rows.Next() {
		var t Tag
		var added string
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &added); err != nil {
			_ = rows.Close()
			return TagPage{}, err
		}
		t.DateAdded, err = time.Parse(time.RFC3339Nano, added)
		if err != nil {
			_ = rows.Close()
			return TagPage{}, err
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return TagPage{}, err
	}
	if err := rows.Close(); err != nil {
		return TagPage{}, err
	}
	if tags == nil {
		tags = []Tag{}
	}
	return TagPage{Count: count, Items: tags, Limit: limit, Offset: offset}, nil
}

// DefaultLimit matches Linkding's REST_FRAMEWORK PAGE_SIZE setting. It is
// applied when a bookmark or tag list request omits or sends an invalid
// limit, which is also how Linkding paginates GET /api/tags/.
const DefaultLimit = 100

// PageOptions bounds one list or search page.
type PageOptions struct {
	// Limit is the requested page size; values <= 0 select DefaultLimit.
	Limit int
	// Offset is the requested start index; negative values become zero.
	Offset int
	// Query is an FTS5 search term; when empty the page is the full list.
	Query string
}

// Page is one window of results plus the total number of matching rows.
type Page struct {
	Count  int
	Items  []Bookmark
	Limit  int
	Offset int
}

// List returns one page of bookmarks owned by userID, newest first. When
// opts.Query is non-empty the page holds FTS5 matches over the bookmark url,
// title, description, notes and tags instead of the whole list; the row count
// and the date_added-descending order are the same for both modes.
func (s *Service) List(ctx context.Context, userID int64, archived bool, opts PageOptions) (Page, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	match := ftsQuery(opts.Query)
	if opts.Query != "" && match == "" {
		// A query without word characters has no FTS terms; like Linkding's
		// icontains search, it matches nothing.
		return Page{Count: 0, Items: []Bookmark{}, Limit: limit, Offset: offset}, nil
	}

	conditions := []string{"user_id = ?", "is_archived = ?"}
	args := []any{userID, archived}
	if match != "" {
		conditions = append(conditions, `EXISTS(SELECT 1 FROM bookmark_search
			WHERE rowid = bookmarks.id AND bookmark_search MATCH ?)`)
		args = append(args, match)
	}
	where := strings.Join(conditions, " AND ")

	count, err := s.countMatches(ctx, where, args)
	if err != nil {
		return Page{}, err
	}
	listArgs := make([]any, 0, len(args)+2)
	listArgs = append(listArgs, args...)
	listArgs = append(listArgs, limit, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,user_id,url,title,description,notes,is_archived,unread,shared,web_archive_snapshot_url,favicon_url,preview_image_url,date_added,date_modified
		 FROM bookmarks WHERE `+where+`
		 ORDER BY date_added DESC, id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return Page{}, err
	}
	items, err := s.scanListRows(ctx, rows)
	if err != nil {
		return Page{}, err
	}
	return Page{Count: count, Items: items, Limit: limit, Offset: offset}, nil
}

func (s *Service) countMatches(ctx context.Context, where string, args []any) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM bookmarks WHERE `+where, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Service) scanListRows(ctx context.Context, rows *sql.Rows) ([]Bookmark, error) {
	defer func() { _ = rows.Close() }()
	var items []Bookmark
	for rows.Next() {
		item, err := scanBookmark(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		tags, err := loadTags(ctx, s.db, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Tags = tags
	}
	if items == nil {
		items = []Bookmark{}
	}
	return items, nil
}

// ftsQuery builds an FTS5 MATCH expression from arbitrary user input. Each
// run of letters and digits is quoted as a phrase, so quotes, operators,
// columns and other FTS5 metacharacters inside the query stay inert: the user
// text is never concatenated into the expression unquoted. Tokens are ANDed as
// phrases. The final token carries an FTS5 prefix marker so partial words —
// the extension's omnibox searches while the user is still typing — narrow
// the result set instead of matching nothing.
func ftsQuery(q string) string {
	var tokens []string
	var token strings.Builder
	flush := func() {
		if token.Len() > 0 {
			tokens = append(tokens, token.String())
			token.Reset()
		}
	}
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			token.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	if len(tokens) == 0 {
		return ""
	}
	quoted := make([]string, len(tokens))
	for i, t := range tokens {
		quoted[i] = `"` + t + `"`
	}
	quoted[len(quoted)-1] += "*"
	return strings.Join(quoted, " ")
}

func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM bookmarks WHERE user_id=? AND id=?`, userID, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Archive(ctx context.Context, userID, id int64) error {
	return s.setArchived(ctx, userID, id, true)
}
func (s *Service) Unarchive(ctx context.Context, userID, id int64) error {
	return s.setArchived(ctx, userID, id, false)
}

func (s *Service) setArchived(ctx context.Context, userID, id int64, archived bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE bookmarks SET is_archived=?,date_modified=? WHERE user_id=? AND id=?`, archived, stamp(s.now().UTC()), userID, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// NormalizeURL matches Linkding's normalize_url: it lowercases scheme and
// hostname, strips trailing path slashes, sorts query pairs, and preserves
// user information, fragments, parameters, and explicitly supplied ports.
func NormalizeURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Host != "" {
		host := strings.ToLower(u.Hostname())
		port := u.Port()
		authority := host
		if strings.Contains(host, ":") {
			authority = "[" + host + "]"
		}
		// Linkding retains any explicitly supplied port, including a default
		// port. URL.User.String preserves the parsed userinfo for reconstruction.
		if port != "" && port != "0" {
			authority += ":" + port
		}
		u.Host = authority
	}
	if u.User != nil {
		username := u.User.Username()
		password, hasPassword := u.User.Password()
		switch {
		case username == "":
			// Linkding only reconstructs userinfo when parsed.username is truthy.
			u.User = nil
		case !hasPassword || password == "":
			// Its normalizer appends the password separator only for a
			// non-empty parsed password.
			u.User = url.User(username)
		}
	}
	path := strings.TrimRight(u.EscapedPath(), "/")
	u.Path, err = url.PathUnescape(path)
	if err != nil {
		return "", err
	}
	u.RawPath = path
	if u.RawQuery != "" {
		pairs, err := parseQueryPairs(u.RawQuery)
		if err != nil {
			return "", err
		}
		sort.SliceStable(pairs, func(i, j int) bool {
			if pairs[i][0] != pairs[j][0] {
				return pairs[i][0] < pairs[j][0]
			}
			return pairs[i][1] < pairs[j][1]
		})
		encoded := make([]string, 0, len(pairs))
		for _, pair := range pairs {
			encoded = append(encoded, quoteCompat(pair[0])+"="+quoteCompat(pair[1]))
		}
		u.RawQuery = strings.Join(encoded, "&")
	} else {
		u.ForceQuery = false
	}
	return u.String(), nil
}

// ValidateURL applies Django's URLValidator default scheme allowlist, used by
// Linkding's BookmarkURLValidator: HTTP, HTTPS, FTP, and FTPS.
func ValidateURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "ftp", "ftps":
	default:
		return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	if u.Hostname() == "" {
		return errors.New("URL must include a hostname")
	}
	if _, err := strconv.Atoi(u.Port()); err != nil && u.Port() != "" {
		return errors.New("invalid URL port")
	}
	return nil
}

func parseQueryPairs(query string) ([][]string, error) {
	var pairs [][]string
	for _, field := range strings.Split(query, "&") {
		if field == "" {
			continue
		}
		key, value, _ := strings.Cut(field, "=")
		pairs = append(pairs, []string{queryUnescapeCompat(key), queryUnescapeCompat(value)})
	}
	return pairs, nil
}

// queryUnescapeCompat matches urllib.parse.unquote_plus's tolerant handling:
// malformed percent signs remain literal and are percent-encoded on output.
func queryUnescapeCompat(value string) string {
	const hex = "0123456789abcdef"
	decoded := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		switch {
		case value[i] == '+':
			decoded = append(decoded, ' ')
		case value[i] == '%' && i+2 < len(value):
			hi := strings.IndexByte(hex, lowerASCII(value[i+1]))
			lo := strings.IndexByte(hex, lowerASCII(value[i+2]))
			if hi >= 0 && lo >= 0 {
				decoded = append(decoded, byte(hi<<4|lo))
				i += 2
				continue
			}
			decoded = append(decoded, value[i])
		default:
			decoded = append(decoded, value[i])
		}
	}
	return strings.ToValidUTF8(string(decoded), "\uFFFD")
}

func lowerASCII(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

// quoteCompat matches urllib.parse.quote with its default safe="/" behavior.
func quoteCompat(value string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for _, c := range []byte(value) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			strings.ContainsRune("_-.~/", rune(c)) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		}
	}
	return b.String()
}

type scanner interface{ Scan(...any) error }
type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func getByNormalized(ctx context.Context, q queryer, userID int64, normalized string) (Bookmark, error) {
	return scanWithTags(ctx, q, q.QueryRowContext(ctx, `SELECT id,user_id,url,title,description,notes,is_archived,unread,shared,web_archive_snapshot_url,favicon_url,preview_image_url,date_added,date_modified FROM bookmarks WHERE user_id=? AND url_normalized=?`, userID, normalized))
}
func getByID(ctx context.Context, q queryer, userID, id int64) (Bookmark, error) {
	return scanWithTags(ctx, q, q.QueryRowContext(ctx, `SELECT id,user_id,url,title,description,notes,is_archived,unread,shared,web_archive_snapshot_url,favicon_url,preview_image_url,date_added,date_modified FROM bookmarks WHERE user_id=? AND id=?`, userID, id))
}
func scanWithTags(ctx context.Context, q queryer, row *sql.Row) (Bookmark, error) {
	b, err := scanBookmark(row)
	if err != nil {
		return Bookmark{}, err
	}
	b.Tags, err = loadTags(ctx, q, b.ID)
	return b, err
}
func scanBookmark(s scanner) (Bookmark, error) {
	var b Bookmark
	var archived, unread, shared bool
	var webArch, fav, prev sql.NullString
	var added, modified string
	err := s.Scan(&b.ID, &b.UserID, &b.URL, &b.Title, &b.Description, &b.Notes, &archived, &unread, &shared, &webArch, &fav, &prev, &added, &modified)
	if err != nil {
		return Bookmark{}, err
	}
	b.IsArchived, b.Unread, b.Shared = archived, unread, shared
	if webArch.Valid && webArch.String != "" {
		b.WebArchiveSnapshotURL = &webArch.String
	}
	if fav.Valid && fav.String != "" {
		b.FaviconURL = &fav.String
	}
	if prev.Valid && prev.String != "" {
		b.PreviewImageURL = &prev.String
	}
	b.DateAdded, err = time.Parse(time.RFC3339Nano, added)
	if err != nil {
		return Bookmark{}, err
	}
	b.DateModified, err = time.Parse(time.RFC3339Nano, modified)
	return b, err
}
func loadTags(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, id int64) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.name FROM tags t JOIN bookmark_tags bt ON bt.tag_id=t.id WHERE bt.bookmark_id=? ORDER BY t.name COLLATE BINARY,t.id`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	tags := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}
func replaceTags(ctx context.Context, tx *sql.Tx, userID, bookmarkID int64, names []string, now time.Time) error {
	unique := make(map[string]struct{}, len(names))
	clean := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("%w: empty tag", ErrInvalidBookmark)
		}
		if _, ok := unique[name]; !ok {
			unique[name] = struct{}{}
			clean = append(clean, name)
		}
	}
	sort.Strings(clean)
	if _, err := tx.ExecContext(ctx, `DELETE FROM bookmark_tags WHERE bookmark_id=?`, bookmarkID); err != nil {
		return err
	}
	for _, name := range clean {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tags(user_id,name,date_added) VALUES(?,?,?) ON CONFLICT(user_id,name) DO NOTHING`, userID, name, stamp(now)); err != nil {
			return err
		}
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO bookmark_tags(bookmark_id,tag_id) SELECT ?,id FROM tags WHERE user_id=? AND name=?`)
		if err != nil {
			return err
		}
		_, err = stmt.ExecContext(ctx, bookmarkID, userID, name)
		closeErr := stmt.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
