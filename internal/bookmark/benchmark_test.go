package bookmark

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lisieckir/lynk/internal/database"
)

// Benchmarks use the same migrated SQLite database, service and triggers as
// production. Setup is always outside the measured region; only CRUD, list and
// search calls made by the API are timed.
const benchmarkOwnerID int64 = 1

func benchmarkService(b *testing.B) (*Service, context.Context) {
	b.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Path: filepath.Join(b.TempDir(), "bench.db")})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO users(id,username,password_hash,created_at,updated_at) VALUES(?,?,?,?,?)`,
		benchmarkOwnerID, "bench", "!", now, now); err != nil {
		b.Fatal(err)
	}
	svc, err := NewService(db)
	if err != nil {
		b.Fatal(err)
	}
	return svc, ctx
}

func benchmarkCreateInput(i int) CreateInput {
	return CreateInput{
		URL:         fmt.Sprintf("https://example.com/bench/%06d", i),
		Title:       fmt.Sprintf("Benchmark bookmark %06d", i),
		Description: "Representative bookmark body used for CRUD benchmarking",
		Notes:       "p2-011 benchmark note",
		Tags:        []string{fmt.Sprintf("bench%02d", i%10), "folder:Bench/Phase8"},
	}
}

func benchmarkSeed(b *testing.B, svc *Service, ctx context.Context, count int) []int64 {
	b.Helper()
	ids := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		created, err := svc.Create(ctx, benchmarkOwnerID, benchmarkCreateInput(i))
		if err != nil {
			b.Fatal(err)
		}
		ids = append(ids, created.ID)
	}
	return ids
}

func BenchmarkBookmarkCreate(b *testing.B) {
	svc, ctx := benchmarkService(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Create(ctx, benchmarkOwnerID, benchmarkCreateInput(i)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBookmarkGet(b *testing.B) {
	svc, ctx := benchmarkService(b)
	ids := benchmarkSeed(b, svc, ctx, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Get(ctx, benchmarkOwnerID, ids[i%len(ids)]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBookmarkUpdate(b *testing.B) {
	svc, ctx := benchmarkService(b)
	ids := benchmarkSeed(b, svc, ctx, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		title := fmt.Sprintf("Updated benchmark bookmark %06d", i)
		if _, err := svc.Patch(ctx, benchmarkOwnerID, ids[i%len(ids)], UpdateInput{Title: &title}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBookmarkDelete(b *testing.B) {
	svc, ctx := benchmarkService(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Setup is excluded: each measured call deletes a real, freshly
		// committed row so no bookkeeping or reuse distorts the result.
		b.StopTimer()
		created, err := svc.Create(ctx, benchmarkOwnerID, benchmarkCreateInput(i))
		b.StartTimer()
		if err != nil {
			b.Fatal(err)
		}
		if err := svc.Delete(ctx, benchmarkOwnerID, created.ID); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBookmarkListPagination(b *testing.B) {
	svc, ctx := benchmarkService(b)
	benchmarkSeed(b, svc, ctx, 2000)
	for _, tc := range []struct {
		name     string
		limit    int
		offset   int
		archived bool
	}{
		{name: "first_page_limit_100", limit: 100, offset: 0},
		{name: "deep_page_limit_100", limit: 100, offset: 1000},
		{name: "large_page_limit_500", limit: 500, offset: 1500},
		{name: "archived_page", limit: 100, offset: 0, archived: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			opts := PageOptions{Limit: tc.limit, Offset: tc.offset}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := svc.List(ctx, benchmarkOwnerID, tc.archived, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkBookmarkHeapSnapshot reports a process-level runtime.MemStats
// snapshot after a realistic paginated read workload. It complements -benchmem
// (per-op allocations) with the live Go heap/system footprint, and is only a
// test instrumentation hook: production code never calls runtime.ReadMemStats.
func BenchmarkBookmarkHeapSnapshot(b *testing.B) {
	svc, ctx := benchmarkService(b)
	benchmarkSeed(b, svc, ctx, 2000)
	opts := PageOptions{Limit: 100}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.List(ctx, benchmarkOwnerID, false, opts); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	b.ReportMetric(float64(stats.HeapAlloc)/(1<<20), "heap-MiB")
	b.ReportMetric(float64(stats.Sys)/(1<<20), "sys-MiB")
}

func BenchmarkBookmarkSearch(b *testing.B) {
	svc, ctx := benchmarkService(b)
	for i := 0; i < 2000; i++ {
		in := benchmarkCreateInput(i)
		if i%10 == 0 {
			in.Title = "Needle in title " + in.Title
			in.Tags = append(in.Tags, "needletag")
		}
		if _, err := svc.Create(ctx, benchmarkOwnerID, in); err != nil {
			b.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name  string
		query string
	}{
		{name: "prefix", query: "need"},
		{name: "two_terms", query: "needle benchmark"},
		{name: "tag", query: "needletag"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			opts := PageOptions{Limit: 100, Query: tc.query}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := svc.List(ctx, benchmarkOwnerID, false, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
