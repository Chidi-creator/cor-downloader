package downloader

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"cor-downloader/internal/resolver"
)

func TestDownloadCancelledContextCleansUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for i := 0; i < 50; i++ {
			w.Write([]byte("some-bytes-pretending-to-be-video-data-"))
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	info := &resolver.MediaInfo{
		Title:          "test video",
		SelectedFormat: resolver.Format{URL: srv.URL, Ext: "mp4"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := Download(ctx, info, dir, nil)
	elapsed := time.Since(start)

	t.Logf("Download returned after %v with err: %v", elapsed, err)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if elapsed > 1*time.Second {
		t.Fatalf("expected cancellation to stop quickly, took %v", elapsed)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("expected no leftover .part file, found: %v", entries)
	}
}
