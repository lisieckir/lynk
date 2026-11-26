package netscape

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/bookmark"
	"github.com/lisieckir/lynk/internal/database"
)

// Parse and Export are the two hot paths used by the CLI migration workflow.
// Both benchmarks use real bounded inputs and, for export, the same migrated
// SQLite database and service used by the command.
func benchmarkDocument(records int) []byte {
	var doc strings.Builder
	doc.WriteString("<!DOCTYPE NETSCAPE-Bookmark-file-1>\n<META HTTP-EQUIV=\"Content-Type\" CONTENT=\"text/html; charset=UTF-8\">\n<TITLE>Bookmarks</TITLE>\n<H1>Bookmarks</H1>\n<DL><p>\n")
	for i := 0; i < records; i++ {
		fmt.Fprintf(&doc, "    <DT><A HREF=\"https://example.com/bench/%06d\" ADD_DATE=\"%d\" LAST_MODIFIED=\"%d\" TAGS=\"bench%02d,needle\">Benchmark bookmark %06d</A>\n",
			i, 1760000000+i, 1760000001+i, i%10, i)
		if i%2 == 0 {
			fmt.Fprintf(&doc, "    <DD>Representative description for benchmark record %06d\n", i)
		}
	}
	doc.WriteString("</DL><p>\n")
	return []byte(doc.String())
}

func benchmarkExportService(b *testing.B, records int) (*bookmark.Service, context.Context) {
	b.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Path: filepath.Join(b.TempDir(), "export-bench.db")})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users(id,username,password_hash,created_at,updated_at) VALUES(1,?,?,?,?)`,
		"bench", "!", now, now); err != nil {
		b.Fatal(err)
	}
	svc, err := bookmark.NewService(db)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < records; i++ {
		_, err := svc.Create(ctx, 1, bookmark.CreateInput{
			URL:         fmt.Sprintf("https://example.com/bench/%06d", i),
			Title:       fmt.Sprintf("Benchmark bookmark %06d", i),
			Description: fmt.Sprintf("Representative description for benchmark record %06d", i),
			Tags:        []string{fmt.Sprintf("bench%02d", i%10), "folder:Bench/Phase8", "needle"},
		})
		if err != nil {
			b.Fatal(err)
		}
	}
	return svc, ctx
}

func BenchmarkNetscapeParse(b *testing.B) {
	for _, records := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("records=%d", records), func(b *testing.B) {
			data := benchmarkDocument(records)
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parsed, err := Parse(bytes.NewReader(data), Limits{})
				if err != nil {
					b.Fatal(err)
				}
				if len(parsed) != records {
					b.Fatalf("parsed %d records, want %d", len(parsed), records)
				}
			}
		})
	}
}

func BenchmarkNetscapeExport(b *testing.B) {
	for _, records := range []int{100, 1000} {
		b.Run(fmt.Sprintf("records=%d", records), func(b *testing.B) {
			svc, ctx := benchmarkExportService(b, records)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				written := 0
				err := Export(ctx, svc, 1, io.Discard, ExportOptions{
					Filter:  FilterAll,
					Written: &written,
				})
				if err != nil {
					b.Fatal(err)
				}
				if written != records {
					b.Fatalf("exported %d records, want %d", written, records)
				}
			}
		})
	}
}
