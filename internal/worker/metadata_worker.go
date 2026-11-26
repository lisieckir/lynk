package worker

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/lisieckir/lynk/internal/metadata"
)

// MetadataJob represents a metadata fetch job.
type MetadataJob struct {
	BookmarkID int64
	URL        string
}

type MetadataWorker struct {
	db       *sql.DB
	fetcher  *metadata.Fetcher
	logger   *slog.Logger
	jobs     chan MetadataJob
	wg       sync.WaitGroup
	stopped  atomic.Bool
	stopOnce sync.Once
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
					func() {
						defer func() {
							if r := recover(); r != nil {
								w.logger.Error("metadata worker panic recovered", "bookmark_id", job.BookmarkID, "panic", r)
							}
						}()
						w.process(ctx, job)
					}()
				}
			}
		}()
	}
}

func (w *MetadataWorker) process(ctx context.Context, job MetadataJob) {
	res, err := w.fetcher.Fetch(ctx, job.URL)
	if err != nil {
		errClass := "fetch_failed"
		if errors.Is(err, metadata.ErrBlockedURL) {
			errClass = "blocked"
		} else if errors.Is(err, context.Canceled) {
			errClass = "canceled"
		} else if errors.Is(err, context.DeadlineExceeded) {
			errClass = "timeout"
		}
		w.logger.Debug("metadata fetch failed", "bookmark_id", job.BookmarkID, "error_class", errClass)
		return
	}
	title := res.OGTitle
	if title == "" {
		title = res.Title
	}
	desc := res.OGDescription
	if desc == "" {
		desc = res.Description
	}
	_, err = w.db.ExecContext(ctx, `
		UPDATE bookmarks
		SET title = CASE WHEN ? != '' AND (title = '' OR title IS NULL) THEN ? ELSE title END,
		    description = CASE WHEN ? != '' AND (description = '' OR description IS NULL) THEN ? ELSE description END,
		    favicon_url = CASE WHEN ? != '' THEN ? ELSE favicon_url END,
		    preview_image_url = CASE WHEN ? != '' THEN ? ELSE preview_image_url END
		WHERE id = ?`,
		title, title,
		desc, desc,
		res.FaviconURL, res.FaviconURL,
		res.PreviewImageURL, res.PreviewImageURL,
		job.BookmarkID,
	)
	if err != nil {
		w.logger.Debug("metadata update failed", "bookmark_id", job.BookmarkID, "error", err)
	}
}

func (w *MetadataWorker) Enqueue(job MetadataJob) bool {
	if w.stopped.Load() {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			// channel closed concurrently
			w.stopped.Store(true)
		}
	}()
	select {
	case w.jobs <- job:
		return true
	default:
		w.logger.Debug("metadata job queue full, dropping", "bookmark_id", job.BookmarkID)
		return false
	}
}

func (w *MetadataWorker) Stop() {
	w.stopOnce.Do(func() {
		w.stopped.Store(true)
		close(w.jobs)
	})
	w.wg.Wait()
}
