package worker

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	"github.com/lisieckir/lynk/internal/metadata"
)

// MetadataJob represents a metadata fetch job.
type MetadataJob struct {
	BookmarkID int64
	URL        string
}

type MetadataWorker struct {
	db      *sql.DB
	fetcher *metadata.Fetcher
	logger  *slog.Logger
	jobs    chan MetadataJob
	wg      sync.WaitGroup
}

func NewMetadataWorker(db *sql.DB, fetcher *metadata.Fetcher, logger *slog.Logger, queueSize int) *MetadataWorker {
	if queueSize <= 0 {
		queueSize = 100
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &MetadataWorker{
		db:      db,
		fetcher: fetcher,
		logger:  logger,
		jobs:    make(chan MetadataJob, queueSize),
	}
}

func (w *MetadataWorker) Start(ctx context.Context, workers int) {
	if workers <= 0 {
		workers = 2
	}
	for i := 0; i < workers; i++ {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-w.jobs:
					if !ok {
						return
					}
					w.process(ctx, job)
				}
			}
		}()
	}
}

func (w *MetadataWorker) process(ctx context.Context, job MetadataJob) {
	res, err := w.fetcher.Fetch(ctx, job.URL)
	if err != nil {
		w.logger.Debug("metadata fetch failed", "bookmark_id", job.BookmarkID, "url", job.URL, "error", err)
		return
	}
	// Update bookmark with fetched metadata (only set non-empty fields)
	_, err = w.db.ExecContext(ctx, `
		UPDATE bookmarks
		SET favicon_url = CASE WHEN ? != '' THEN ? ELSE favicon_url END,
		    preview_image_url = CASE WHEN ? != '' THEN ? ELSE preview_image_url END
		WHERE id = ?`,
		res.FaviconURL, res.FaviconURL,
		res.PreviewImageURL, res.PreviewImageURL,
		job.BookmarkID,
	)
	if err != nil {
		w.logger.Debug("metadata update failed", "bookmark_id", job.BookmarkID, "error", err)
	}
}

func (w *MetadataWorker) Enqueue(job MetadataJob) bool {
	select {
	case w.jobs <- job:
		return true
	default:
		// Queue full, drop
		w.logger.Debug("metadata job queue full, dropping", "bookmark_id", job.BookmarkID)
		return false
	}
}

func (w *MetadataWorker) Stop() {
	close(w.jobs)
	w.wg.Wait()
}
