package worker

import (
	"context"
	"testing"

	"github.com/lisieckir/lynk/internal/metadata"
)

func TestMetadataWorker_StopIdempotent(t *testing.T) {
	w := NewMetadataWorker(nil, metadata.NewFetcher(metadata.Config{}), nil, 10)
	w.Start(context.Background(), 1)
	w.Stop()
	w.Stop() // should not panic
}

func TestMetadataWorker_EnqueueAfterStop(t *testing.T) {
	w := NewMetadataWorker(nil, metadata.NewFetcher(metadata.Config{}), nil, 10)
	w.Start(context.Background(), 1)
	w.Stop()
	// should not panic, should return false
	ok := w.Enqueue(MetadataJob{BookmarkID: 1, URL: "http://example.com"})
	if ok {
		t.Error("expected enqueue to return false after stop")
	}
}
