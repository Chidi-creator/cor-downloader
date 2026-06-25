package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cor-downloader/internal/resolver"
)

// httpClient is used instead of http.DefaultClient: the default transport
// has no limit on how long it'll wait for a response to start arriving,
// which can hang a download forever if a connection stalls.
func newHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: transport}
}

var httpClient = newHTTPClient()

// allowedContentTypePrefixes are the response Content-Type values we trust
// enough to save to disk as media.
var allowedContentTypePrefixes = []string{"video/", "audio/", "application/octet-stream"}

func isAllowedContentType(contentType string) bool {
	for _, prefix := range allowedContentTypePrefixes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	return false
}

// retryableError marks a failure as transient - worth trying again - as
// opposed to a permanent failure (bad auth, wrong content, etc.) where
// retrying would just waste time.
type retryableError struct{ err error }

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

func retryable(err error) error {
	return &retryableError{err}
}

const maxAttempts = 3

// ProgressFunc is called as bytes arrive. total is -1 if the server didn't
// report a Content-Length.
type ProgressFunc func(downloaded, total int64)

// Download fetches the resolved media format and saves it into destDir,
// deriving a safe filename from the post title. It downloads to a hidden
// temp file first and only renames it to the final name once the entire
// download has succeeded, so a half-written file never appears under a
// "real" name. Transient failures are retried with backoff. Returns the
// final file path. onProgress may be nil.
func Download(ctx context.Context, info *resolver.MediaInfo, destDir string, onProgress ProgressFunc) (string, error) {
	ext := info.SelectedFormat.Ext
	if ext == "" {
		ext = "mp4"
	}
	finalPath := filepath.Join(destDir, SanitizeFilename(info.Title)+"."+ext)
	tempPath := finalPath + ".part"

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := attemptDownload(ctx, info, tempPath, onProgress)
		if err == nil {
			if err := os.Rename(tempPath, finalPath); err != nil {
				return "", fmt.Errorf("finalizing file: %w", err)
			}
			return finalPath, nil
		}

		var re *retryableError
		if !errors.As(err, &re) {
			return "", err
		}
		lastErr = err

		if attempt == maxAttempts {
			break
		}

		backoff := time.Duration(attempt) * 200 * time.Millisecond
		select {
		case <-ctx.Done():
			os.Remove(tempPath)
			return "", ctx.Err()
		case <-time.After(backoff):
		}
	}

	os.Remove(tempPath)
	return "", fmt.Errorf("giving up after %d attempts: %w", maxAttempts, lastErr)
}

// attemptDownload makes one attempt at fetching info's selected format and
// writing it to tempPath. Errors worth retrying are wrapped with retryable.
func attemptDownload(ctx context.Context, info *resolver.MediaInfo, tempPath string, onProgress ProgressFunc) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.SelectedFormat.URL, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	for key, value := range info.SelectedFormat.HTTPHeaders {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return retryable(fmt.Errorf("requesting media: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return retryable(fmt.Errorf("unexpected status: %s", resp.Status))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	if !isAllowedContentType(contentType) {
		return fmt.Errorf("unexpected content-type: %s", contentType)
	}

	out, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}

	total := resp.ContentLength
	var source io.Reader = resp.Body
	if onProgress != nil {
		source = &progressReader{reader: resp.Body, total: total, onProgress: onProgress}
	}

	_, err = io.Copy(out, source)
	if err != nil {
		out.Close()
		os.Remove(tempPath)
		if ctx.Err() != nil {
			return err
		}
		return retryable(fmt.Errorf("writing file: %w", err))
	}

	if err := out.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("closing file: %w", err)
	}

	return nil
}
