package metadata

import (
	"fmt"
	"strings"
	"testing"
)

// parseHTML is the CPU/allocation hot path after a bounded metadata response
// body has been read. Fixtures mirror the 2 MiB FETCH body cap and a smaller
// typical page; no network or SSRF code is involved.
func benchmarkHTML(targetBytes int) []byte {
	var doc strings.Builder
	doc.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8">`)
	doc.WriteString(`<title>Benchmark page title with representative length</title>`)
	doc.WriteString(`<meta name="description" content="A representative meta description used for metadata parser benchmarking.">`)
	doc.WriteString(`<meta property="og:title" content="Open Graph benchmark title">`)
	doc.WriteString(`<meta property="og:description" content="Open Graph benchmark description">`)
	doc.WriteString(`<meta property="og:image" content="/images/preview.png">`)
	doc.WriteString(`<link rel="icon" href="/favicon.ico"><link rel="apple-touch-icon" href="/apple.png">`)
	doc.WriteString(`</head><body>`)
	filler := `<div class="content"><p>Some realistic page body text that the parser scans but does not extract. Lorem ipsum dolor sit amet, consectetur adipiscing elit.</p><a href="/next">Next</a></div>` + "\n"
	for doc.Len()+len(filler) < targetBytes {
		doc.WriteString(filler)
	}
	doc.WriteString(`</body></html>`)
	return []byte(doc.String())
}

func BenchmarkParseHTML(b *testing.B) {
	for _, tc := range []struct {
		name  string
		bytes int
	}{
		{name: "small_page", bytes: 16 << 10},
		{name: "body_cap", bytes: 2 << 20},
	} {
		b.Run(fmt.Sprintf("%s_%dKiB", tc.name, tc.bytes>>10), func(b *testing.B) {
			data := benchmarkHTML(tc.bytes)
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				res := parseHTML(data)
				if res.Title == "" || res.FaviconURL == "" {
					b.Fatal("benchmark fixture lost title or favicon")
				}
			}
		})
	}
}
