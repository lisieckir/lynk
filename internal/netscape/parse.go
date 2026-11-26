package netscape

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"
	"time"
)

// Parse decodes a Netscape bookmark file. The whole input is bounded by
// limits.MaxBytes before tokenization, and the number of anchors is bounded by
// limits.MaxBookmarks. Structural errors are reported with an input line number
// and no partial records.
func Parse(r io.Reader, limits Limits) ([]Record, error) {
	effective, err := limits.withDefaults()
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, &ParseError{Line: 1, Err: fmt.Errorf("%w: nil reader", ErrMalformed)}
	}
	data, err := io.ReadAll(io.LimitReader(r, effective.MaxBytes+1))
	if err != nil {
		return nil, &ParseError{Line: 1, Err: fmt.Errorf("%w: read input: %v", ErrMalformed, err)}
	}
	if int64(len(data)) > effective.MaxBytes {
		return nil, &ParseError{Line: 1, Err: fmt.Errorf("%w: input exceeds %d bytes", ErrLimitExceeded, effective.MaxBytes)}
	}
	p := &parser{data: data, line: 1, limits: effective, ddIndex: -1}
	if err := p.run(); err != nil {
		return nil, err
	}
	return p.records, nil
}

type tokenKind uint8

const (
	tokText tokenKind = iota
	tokStart
	tokEnd
	tokEOF
)

type token struct {
	kind  tokenKind
	name  string
	text  string
	attrs map[string]string
	line  int
}

type frame struct {
	folder string
}

type parser struct {
	data   []byte
	pos    int
	line   int
	limits Limits

	records  []Record
	frames   []frame
	pending  *Record
	title    strings.Builder
	openH3   bool
	h3       strings.Builder
	h3Folder string
	ddIndex  int
	desc     strings.Builder
	last     int
}

func (p *parser) fail(line int, format string, args ...any) error {
	return &ParseError{Line: line, Err: fmt.Errorf("%w: %s", ErrMalformed, fmt.Sprintf(format, args...))}
}

func (p *parser) limit(line int, format string, args ...any) error {
	return &ParseError{Line: line, Err: fmt.Errorf("%w: %s", ErrLimitExceeded, fmt.Sprintf(format, args...))}
}

func (p *parser) advance(n int) {
	for _, c := range p.data[p.pos : p.pos+n] {
		if c == '\n' {
			p.line++
		}
	}
	p.pos += n
}

func (p *parser) run() error {
	for {
		tok, err := p.next()
		if err != nil {
			return err
		}
		switch tok.kind {
		case tokEOF:
			return p.finish()
		case tokText:
			if err := p.text(tok); err != nil {
				return err
			}
		case tokStart:
			if err := p.start(tok); err != nil {
				return err
			}
		case tokEnd:
			if err := p.end(tok); err != nil {
				return err
			}
		}
	}
}

func (p *parser) finish() error {
	if p.pending != nil {
		return p.fail(p.line, "unclosed <A> element")
	}
	if p.openH3 {
		return p.fail(p.line, "unclosed <H3> element")
	}
	if len(p.frames) != 0 {
		return p.fail(p.line, "unclosed <DL> element")
	}
	p.flushDescription()
	return nil
}

func (p *parser) text(tok token) error {
	decoded := html.UnescapeString(tok.text)
	switch {
	case p.pending != nil:
		if len(decoded) > MaxTitleBytes {
			return p.limit(tok.line, "title exceeds %d bytes", MaxTitleBytes)
		}
		p.title.WriteString(decoded)
	case p.openH3:
		if p.h3.Len()+len(decoded) > MaxTitleBytes {
			return p.limit(tok.line, "folder title exceeds %d bytes", MaxTitleBytes)
		}
		p.h3.WriteString(decoded)
	case p.ddIndex >= 0:
		if p.desc.Len()+len(decoded) > MaxDescriptionBytes {
			return p.limit(tok.line, "description exceeds %d bytes", MaxDescriptionBytes)
		}
		p.desc.WriteString(decoded)
	}
	return nil
}

func (p *parser) start(tok token) error {
	switch tok.name {
	case "dl":
		if len(p.frames) >= MaxFolderDepth {
			return p.limit(tok.line, "folder nesting exceeds %d levels", MaxFolderDepth)
		}
		p.flushDescription()
		p.frames = append(p.frames, frame{folder: p.h3Folder})
		p.h3Folder = ""
	case "dt":
		p.flushDescription()
	case "h3":
		if p.openH3 {
			return p.fail(tok.line, "nested <H3> element")
		}
		p.openH3, p.h3 = true, strings.Builder{}
	case "a":
		return p.anchor(tok)
	case "dd":
		return p.description(tok)
	}
	return nil
}

func (p *parser) end(tok token) error {
	switch tok.name {
	case "a":
		if p.pending == nil {
			return p.fail(tok.line, "</A> without matching <A>")
		}
		p.pending.Title = strings.TrimSpace(p.title.String())
		p.records = append(p.records, *p.pending)
		p.last, p.pending = len(p.records)-1, nil
	case "h3":
		if !p.openH3 {
			return p.fail(tok.line, "</H3> without matching <H3>")
		}
		p.h3Folder = strings.TrimSpace(p.h3.String())
		p.openH3 = false
	case "dl":
		if len(p.frames) == 0 {
			return p.fail(tok.line, "</DL> without matching <DL>")
		}
		p.flushDescription()
		p.frames = p.frames[:len(p.frames)-1]
	}
	return nil
}

func (p *parser) anchor(tok token) error {
	if p.openH3 {
		return p.fail(tok.line, "<A> inside unfinished <H3>")
	}
	if len(p.frames) == 0 {
		return p.fail(tok.line, "<A> outside <DL>")
	}
	if len(p.records) >= p.limits.MaxBookmarks {
		return p.limit(tok.line, "more than %d bookmarks", p.limits.MaxBookmarks)
	}
	href := strings.TrimSpace(tok.attrs["href"])
	if href == "" {
		return p.fail(tok.line, "<A> without HREF")
	}
	tags, err := splitTags(tok.attrs["tags"], tok.line, p)
	if err != nil {
		return err
	}
	rec := Record{
		URL:          href,
		Tags:         tags,
		DateAdded:    unixTime(tok.attrs["add_date"]),
		DateModified: unixTime(tok.attrs["last_modified"]),
	}
	folders := make([]string, 0, len(p.frames))
	for _, f := range p.frames {
		if f.folder != "" {
			folders = append(folders, f.folder)
		}
	}
	if len(folders) > 0 {
		folderTag := folderPrefix + strings.Join(folders, "/")
		if len(folderTag) > MaxFolderTagBytes {
			return p.limit(tok.line, "folder tag exceeds %d bytes", MaxFolderTagBytes)
		}
		rec.Tags = appendUnique(rec.Tags, folderTag)
	}
	p.flushDescription()
	p.pending = &rec
	p.title = strings.Builder{}
	return nil
}

func (p *parser) description(tok token) error {
	if p.last < 0 || len(p.frames) == 0 {
		return p.fail(tok.line, "<DD> without a preceding bookmark")
	}
	p.flushDescription()
	p.ddIndex = p.last
	p.desc = strings.Builder{}
	return nil
}

func (p *parser) flushDescription() {
	if p.ddIndex < 0 {
		return
	}
	text := strings.TrimSpace(p.desc.String())
	if text != "" {
		if existing := p.records[p.ddIndex].Description; existing != "" {
			p.records[p.ddIndex].Description = existing + "\n" + text
		} else {
			p.records[p.ddIndex].Description = text
		}
	}
	p.ddIndex, p.desc = -1, strings.Builder{}
}

const folderPrefix = "folder:"

func splitTags(raw string, line int, p *parser) ([]string, error) {
	var tags []string
	for _, part := range strings.Split(raw, ",") {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		if len(tag) > MaxTagBytes {
			return nil, p.limit(line, "tag exceeds %d bytes", MaxTagBytes)
		}
		if len(tags) >= MaxTagsPerBookmark {
			return nil, p.limit(line, "more than %d tags", MaxTagsPerBookmark)
		}
		tags = appendUnique(tags, tag)
	}
	return tags, nil
}

// next returns the next HTML token, skipping comments and declarations.
func (p *parser) next() (token, error) {
	for {
		if p.pos >= len(p.data) {
			return token{kind: tokEOF, line: p.line}, nil
		}
		rel := bytes.IndexByte(p.data[p.pos:], '<')
		if rel < 0 {
			text := string(p.data[p.pos:])
			line := p.line
			p.advance(len(text))
			return token{kind: tokText, text: text, line: line}, nil
		}
		if rel > 0 {
			text := string(p.data[p.pos : p.pos+rel])
			line := p.line
			p.advance(rel)
			return token{kind: tokText, text: text, line: line}, nil
		}

		start, line := p.pos, p.line
		if bytes.HasPrefix(p.data[start:], []byte("<!--")) {
			end := bytes.Index(p.data[start+4:], []byte("-->"))
			if end < 0 {
				return token{}, p.fail(line, "unterminated comment")
			}
			p.advance(4 + end + 3)
			continue
		}
		if start+1 < len(p.data) && (p.data[start+1] == '!' || p.data[start+1] == '?') {
			end := bytes.IndexByte(p.data[start+1:], '>')
			if end < 0 {
				return token{}, p.fail(line, "unterminated declaration")
			}
			p.advance(end + 2)
			continue
		}
		if start+1 < len(p.data) && p.data[start+1] == '/' {
			end := findTagEnd(p.data, start+2)
			if end < 0 {
				return token{}, p.fail(line, "unterminated closing tag")
			}
			raw := string(p.data[start+2 : end])
			p.advance(end + 1 - start)
			return token{kind: tokEnd, name: readName(raw), line: line}, nil
		}
		if start+1 < len(p.data) && isNameStart(p.data[start+1]) {
			end := findTagEnd(p.data, start+1)
			if end < 0 {
				return token{}, p.fail(line, "unterminated tag")
			}
			raw := string(p.data[start+1 : end])
			p.advance(end + 1 - start)
			name := readName(raw)
			attrs, err := parseAttrs(raw[len(name):], line, p)
			if err != nil {
				return token{}, err
			}
			return token{kind: tokStart, name: name, attrs: attrs, line: line}, nil
		}
		// A literal '<' that is not a tag is ordinary text; consume one byte.
		p.advance(1)
		return token{kind: tokText, text: "<", line: line}, nil
	}
}

func findTagEnd(data []byte, from int) int {
	var quote byte
	for i := from; i < len(data); i++ {
		c := data[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return i
		}
	}
	return -1
}

func isNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isNameByte(c byte) bool {
	return isNameStart(c) || c >= '0' && c <= '9' || c == ':' || c == '-' || c == '_'
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

func readName(raw string) string {
	i := 0
	for i < len(raw) && isNameByte(raw[i]) {
		i++
	}
	return strings.ToLower(raw[:i])
}

func parseAttrs(raw string, line int, p *parser) (map[string]string, error) {
	attrs := make(map[string]string)
	for i := 0; i < len(raw); {
		for i < len(raw) && isSpace(raw[i]) {
			i++
		}
		if i >= len(raw) {
			break
		}
		start := i
		for i < len(raw) && isNameByte(raw[i]) {
			i++
		}
		if start == i {
			return nil, p.fail(line, "unexpected character %q in tag", raw[i])
		}
		name := strings.ToLower(raw[start:i])
		for i < len(raw) && isSpace(raw[i]) {
			i++
		}
		value := ""
		if i < len(raw) && raw[i] == '=' {
			i++
			for i < len(raw) && isSpace(raw[i]) {
				i++
			}
			if i >= len(raw) {
				return nil, p.fail(line, "missing value for attribute %q", name)
			}
			if raw[i] == '"' || raw[i] == '\'' {
				quote := raw[i]
				i++
				start = i
				for i < len(raw) && raw[i] != quote {
					i++
				}
				if i >= len(raw) {
					return nil, p.fail(line, "unterminated value for attribute %q", name)
				}
				value = raw[start:i]
				i++
			} else {
				start = i
				for i < len(raw) && !isSpace(raw[i]) {
					i++
				}
				value = raw[start:i]
			}
		}
		attrs[name] = html.UnescapeString(value)
	}
	return attrs, nil
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func unixTime(raw string) *time.Time {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || seconds <= 0 {
		return nil
	}
	t := time.Unix(seconds, 0).UTC()
	return &t
}
