package netscape

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

const sampleHTML = `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=UTF-8">
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
    <DT><H3 ADD_DATE="1600000000">Work &amp; Fun</H3>
    <DL><p>
        <DT><A HREF="https://example.com/a?x=1&amp;y=2" ADD_DATE="1600000001" LAST_MODIFIED="1600000002" TAGS="alpha, beta">A &amp; B 😀</A>
        <DD>Notes &lt;script&gt;alert(1)&lt;/script&gt; with é
        <DT><A HREF='http://example.org/b' TAGS='gamma'>Second</A>
    </DL><p>
</DL><p>
`

func TestParseSample(t *testing.T) {
	records, err := Parse(strings.NewReader(sampleHTML), Limits{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	first := records[0]
	if want := "https://example.com/a?x=1&y=2"; first.URL != want {
		t.Errorf("URL = %q, want %q", first.URL, want)
	}
	if want := "A & B 😀"; first.Title != want {
		t.Errorf("Title = %q, want %q", first.Title, want)
	}
	if want := "Notes <script>alert(1)</script> with é"; first.Description != want {
		t.Errorf("Description = %q, want %q", first.Description, want)
	}
	if want := time.Unix(1600000001, 0).UTC(); first.DateAdded == nil || !first.DateAdded.Equal(want) {
		t.Errorf("DateAdded = %v, want %v", first.DateAdded, want)
	}
	if want := time.Unix(1600000002, 0).UTC(); first.DateModified == nil || !first.DateModified.Equal(want) {
		t.Errorf("DateModified = %v, want %v", first.DateModified, want)
	}
	wantTags := []string{"alpha", "beta", "folder:Work & Fun"}
	if !equalStrings(first.Tags, wantTags) {
		t.Errorf("Tags = %q, want %q", first.Tags, wantTags)
	}
	if records[1].DateAdded != nil {
		t.Errorf("DateAdded = %v, want nil for missing ADD_DATE", records[1].DateAdded)
	}
	if got, want := records[1].Tags, []string{"gamma", "folder:Work & Fun"}; !equalStrings(got, want) {
		t.Errorf("second Tags = %q, want %q", got, want)
	}
}

func TestParseNestedFolders(t *testing.T) {
	html := `<DL><p>
<DT><H3>Outer</H3><DL><p>
<DT><H3>Inner</H3><DL><p>
<DT><A HREF="https://example.com/nested">Nested</A>
</DL><p></DL><p>
<DT><A HREF="https://example.com/root">Root</A>
</DL><p>`
	records, err := Parse(strings.NewReader(html), Limits{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if got, want := records[0].Tags, []string{"folder:Outer/Inner"}; !equalStrings(got, want) {
		t.Errorf("nested Tags = %q, want %q", got, want)
	}
	if got, want := records[1].Tags, []string{}; !equalStrings(got, want) {
		t.Errorf("root Tags = %q, want no folder tag", got)
	}
}

func TestParseNumericEntity(t *testing.T) {
	html := `<DL><p><DT><A HREF="https://example.com/é" TAGS="caf&#233;">&#x1F600; Caf&#233;</A></DL><p>`
	records, err := Parse(strings.NewReader(html), Limits{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if got, want := records[0].Title, "😀 Café"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
	if got, want := records[0].URL, "https://example.com/é"; got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
	if got, want := records[0].Tags, []string{"café"}; !equalStrings(got, want) {
		t.Errorf("Tags = %q, want %q", got, want)
	}
}

func TestParseMalformedDeterministic(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{"unclosed anchor", `<DL><DT><A HREF="https://e.example">Title</DL>`, "unclosed <A>"},
		{"stray close anchor", `<DL><DT></A></DL>`, "</A> without matching <A>"},
		{"unclosed dl", `<DL><DT><A HREF="https://e.example">Title</A>`, "unclosed <DL>"},
		{"stray close dl", `</DL>`, "</DL> without matching <DL>"},
		{"missing href", `<DL><DT><A>Title</A></DL>`, "<A> without HREF"},
		{"anchor outside dl", `<A HREF="https://e.example">Title</A>`, "<A> outside <DL>"},
		{"unclosed tag", `<DL><A HREF="https://e.example"`, "unterminated tag"},
		{"unclosed comment", `<DL><!-- oops`, "unterminated comment"},
		{"dd outside dl", `<DD>orphan`, "<DD> without a preceding bookmark"},
		{"nested h3", `<DL><H3>A<H3>B</H3></DL>`, "nested <H3>"},
		{"bad attribute", `<DL><A HREF="https://e.example" =bad>Title</A></DL>`, "unexpected character"},
		{"unterminated attribute", `<DL><A HREF="https://e.example>Title</A></DL>`, "unterminated tag"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.html), Limits{})
			if err == nil {
				t.Fatalf("Parse succeeded for %s", tc.name)
			}
			if !errors.Is(err, ErrMalformed) {
				t.Errorf("error %v does not wrap ErrMalformed", err)
			}
			var parseErr *ParseError
			if !errors.As(err, &parseErr) || parseErr.Line < 1 {
				t.Errorf("error %v is not a line-numbered ParseError", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
			// The same input must produce the same error text every time.
			_, again := Parse(strings.NewReader(tc.html), Limits{})
			if again == nil || again.Error() != err.Error() {
				t.Errorf("non-deterministic error: %v vs %v", err, again)
			}
		})
	}
}

func TestParseLimits(t *testing.T) {
	two := `<DL><DT><A HREF="https://a.example">A</A><DT><A HREF="https://b.example">B</A></DL>`
	if _, err := Parse(strings.NewReader(sampleHTML), Limits{MaxBytes: 32}); !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("byte limit: got %v, want ErrLimitExceeded", err)
	}
	if _, err := Parse(strings.NewReader(two), Limits{MaxBookmarks: 1}); !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("bookmark limit: got %v, want ErrLimitExceeded", err)
	}
	if _, err := Parse(strings.NewReader(sampleHTML), Limits{MaxBytes: HardMaxBytes + 1}); !errors.Is(err, ErrInvalidOptions) {
		t.Errorf("byte hard cap: got %v, want ErrInvalidOptions", err)
	}

	longTitle := `<DL><A HREF="https://e.example">` + strings.Repeat("x", MaxTitleBytes+1) + `</A></DL>`
	if _, err := Parse(strings.NewReader(longTitle), Limits{}); !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("title limit: got %v, want ErrLimitExceeded", err)
	}
	longDesc := `<DL><A HREF="https://e.example">T</A><DD>` + strings.Repeat("x", MaxDescriptionBytes+1) + `</DL>`
	if _, err := Parse(strings.NewReader(longDesc), Limits{}); !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("description limit: got %v, want ErrLimitExceeded", err)
	}
	manyTags := make([]string, MaxTagsPerBookmark+1)
	for i := range manyTags {
		manyTags[i] = "tag" + strconv.Itoa(i)
	}
	tagHTML := `<DL><A HREF="https://e.example" TAGS="` + strings.Join(manyTags, ",") + `">T</A></DL>`
	if _, err := Parse(strings.NewReader(tagHTML), Limits{}); !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("tag count limit: got %v, want ErrLimitExceeded", err)
	}
	deep := strings.Repeat("<DL><H3>F</H3>", MaxFolderDepth+1) + `<A HREF="https://e.example">T</A>` + strings.Repeat("</DL>", MaxFolderDepth+1)
	if _, err := Parse(strings.NewReader(deep), Limits{}); !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("folder depth limit: got %v, want ErrLimitExceeded", err)
	}
}

func TestParseCumulativeFieldCapsAcrossChunks(t *testing.T) {
	// A title split by inline tags must be bounded by the decoded total, not
	// by the largest chunk.
	chunk := strings.Repeat("x", 4096)
	splitTitle := `<DL><A HREF="https://e.example">`
	for i := 0; i < 5; i++ {
		if i > 0 {
			splitTitle += "<b></b>"
		}
		splitTitle += chunk
	}
	splitTitle += `</A></DL>`
	if _, err := Parse(strings.NewReader(splitTitle), Limits{}); !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("split title: got %v, want ErrLimitExceeded", err)
	}

	// Several <DD> blocks for the same record must also share one cap.
	splitDesc := `<DL><A HREF="https://e.example">T</A><DD>` +
		strings.Repeat("a", MaxDescriptionBytes-1) +
		`<DD>` + strings.Repeat("b", 8) + `</DL>`
	if _, err := Parse(strings.NewReader(splitDesc), Limits{}); !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("split description: got %v, want ErrLimitExceeded", err)
	}

	// Exactly at the cap is accepted, including the newline joining two blocks.
	atCap := `<DL><A HREF="https://e.example">` + strings.Repeat("t", MaxTitleBytes/2) +
		`<i></i>` + strings.Repeat("t", MaxTitleBytes-MaxTitleBytes/2) +
		`</A><DD>` + strings.Repeat("a", 32000) +
		`<DD>` + strings.Repeat("b", MaxDescriptionBytes-32001) + `</DL>`
	records, err := Parse(strings.NewReader(atCap), Limits{})
	if err != nil {
		t.Fatalf("exactly at cap: %v", err)
	}
	if len(records) != 1 || len(records[0].Title) != MaxTitleBytes || len(records[0].Description) != MaxDescriptionBytes {
		t.Fatalf("at-cap record lengths = title %d, description %d (records=%d)",
			len(records[0].Title), len(records[0].Description), len(records))
	}
}

func TestParseRejectsInvalidUTF8(t *testing.T) {
	tests := []struct {
		name string
		html string
		line int
	}{
		{"title byte", `<DL><A HREF="https://e.example">bad` + string([]byte{0xff}) + `</A></DL>`, 1},
		{"description byte", "<DL><A HREF=\"https://e.example\">T</A><DD>ok\n" + string([]byte{0xc3, 0x28}) + "</DL>", 2},
		{"attribute byte", `<DL><A HREF="https://e.example/` + string([]byte{0xff}) + `">T</A></DL>`, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.html), Limits{})
			if err == nil || !strings.Contains(err.Error(), "not valid UTF-8") {
				t.Fatalf("error = %v, want invalid UTF-8 error", err)
			}
			if !errors.Is(err, ErrMalformed) {
				t.Errorf("error %v does not wrap ErrMalformed", err)
			}
			var parseErr *ParseError
			if !errors.As(err, &parseErr) || parseErr.Line != tc.line {
				t.Errorf("error %v: got line %d, want %d", err, parseErr.Line, tc.line)
			}
			// Deterministic on reparse.
			_, again := Parse(strings.NewReader(tc.html), Limits{})
			if again == nil || again.Error() != err.Error() {
				t.Errorf("non-deterministic UTF-8 error: %v vs %v", err, again)
			}
		})
	}
	// An encoded U+FFFD is valid UTF-8 and stays valid after decoding.
	valid, err := Parse(strings.NewReader(`<DL><A HREF="https://e.example">`+string([]byte{0xef, 0xbf, 0xbd})+`</A></DL>`), Limits{})
	if err != nil || len(valid) != 1 || valid[0].Title != "\uFFFD" {
		t.Fatalf("encoded U+FFFD: records=%v err=%v", valid, err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
