package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cor-downloader/internal/resolver"
)

// fixedChunkReader returns exactly one chunk of data per Read call,
// regardless of how big the caller's buffer is - giving us full control
// over how many Read calls happen, unlike a real network connection.
type fixedChunkReader struct {
	chunks [][]byte
}

func (r *fixedChunkReader) Read(buf []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(buf, r.chunks[0])
	r.chunks = r.chunks[1:]
	return n, nil
}

func TestProgressReaderReportsEachRead(t *testing.T) {
	chunks := [][]byte{
		make([]byte, 100),
		make([]byte, 200),
		make([]byte, 300),
	}
	pr := &progressReader{
		reader: &fixedChunkReader{chunks: chunks},
		total:  600,
	}

	var calls int
	var lastDownloaded, lastTotal int64
	pr.onProgress = func(downloaded, total int64) {
		calls++
		lastDownloaded = downloaded
		lastTotal = total
	}

	written, err := io.Copy(io.Discard, pr)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if written != 600 {
		t.Fatalf("expected 600 bytes copied, got %d", written)
	}
	if calls != 3 {
		t.Fatalf("expected exactly 3 progress calls (one per chunk), got %d", calls)
	}
	if lastDownloaded != 600 || lastTotal != 600 {
		t.Fatalf("expected final downloaded=600 total=600, got downloaded=%d total=%d", lastDownloaded, lastTotal)
	}
}

func TestDownloadReportsFinalProgress(t *testing.T) {
	const totalSize = 10 * 1024

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", totalSize))
		w.Write(make([]byte, totalSize))
	}))
	defer srv.Close()

	dir := t.TempDir()
	info := &resolver.MediaInfo{
		Title:          "progress test",
		SelectedFormat: resolver.Format{URL: srv.URL, Ext: "mp4"},
	}

	var calls int
	var lastDownloaded, lastTotal int64
	onProgress := func(downloaded, total int64) {
		calls++
		lastDownloaded = downloaded
		lastTotal = total
	}

	_, err := Download(context.Background(), info, dir, onProgress)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	if calls < 1 {
		t.Fatal("expected onProgress to be called at least once")
	}
	if lastDownloaded != totalSize {
		t.Fatalf("expected final downloaded=%d, got %d", totalSize, lastDownloaded)
	}
	if lastTotal != totalSize {
		t.Fatalf("expected total=%d, got %d", totalSize, lastTotal)
	}
}
