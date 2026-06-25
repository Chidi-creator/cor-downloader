package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"cor-downloader/internal/resolver"
)

func TestDownloadRetriesThenSucceeds(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Write([]byte("fake video bytes"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	info := &resolver.MediaInfo{
		Title:          "test video",
		SelectedFormat: resolver.Format{URL: srv.URL, Ext: "mp4"},
	}

	path, err := Download(context.Background(), info, dir, nil)
	if err != nil {
		t.Fatalf("expected eventual success, got: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(data) != "fake video bytes" {
		t.Fatalf("unexpected file contents: %q", data)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDownloadDoesNotRetryPermanentFailure(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	info := &resolver.MediaInfo{
		Title:          "test video",
		SelectedFormat: resolver.Format{URL: srv.URL, Ext: "mp4"},
	}

	_, err := Download(context.Background(), info, dir, nil)
	if err == nil {
		t.Fatal("expected an error for a 404")
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 attempt (no retries on permanent failure), got %d", attempts)
	}
}

func TestDownloadGivesUpAfterMaxAttempts(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	info := &resolver.MediaInfo{
		Title:          "test video",
		SelectedFormat: resolver.Format{URL: srv.URL, Ext: "mp4"},
	}

	_, err := Download(context.Background(), info, dir, nil)
	if err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
	if attempts != maxAttempts {
		t.Fatalf("expected %d attempts, got %d", maxAttempts, attempts)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("expected no leftover .part file, found: %v", entries)
	}
}
