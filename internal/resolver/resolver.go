package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

// ErrNeedsExternalDownload means no direct progressive URL was found
// (HLS/DASH-only). Caller should run yt-dlp itself end-to-end instead.
var ErrNeedsExternalDownload = errors.New("resolver: no progressive format available")

// Format is the subset of yt-dlp's per-format JSON fields we care about.
type Format struct {
	URL         string            `json:"url"`
	Protocol    string            `json:"protocol"`
	VCodec      string            `json:"vcodec"`
	ACodec      string            `json:"acodec"`
	Ext         string            `json:"ext"`
	Height      int               `json:"height"`
	Filesize    int64             `json:"filesize"`
	HTTPHeaders map[string]string `json:"http_headers"`
}

// ytdlpOutput mirrors yt-dlp's `-j` output shape. It never leaves this
// package - callers only see MediaInfo.
type ytdlpOutput struct {
	Title   string   `json:"title"`
	Formats []Format `json:"formats"`
}

// MediaInfo is the resolved, ready-to-download result handed to the rest
// of the app.
type MediaInfo struct {
	Title          string
	SelectedFormat Format
}

// Resolve shells out to yt-dlp to find a direct, progressive media URL for
// the given post URL. cookiesFromBrowser is optional (e.g. "safari",
// "chrome") for content that requires a logged-in session; pass "" to skip.
func Resolve(ctx context.Context, url, cookiesFromBrowser string) (*MediaInfo, error) {
	args := []string{"-j", "--no-playlist"}
	if cookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", cookiesFromBrowser)
	}
	args = append(args, url)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp failed: %w (stderr: %s)", err, stderr.String())
	}

	var out ytdlpOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("parsing yt-dlp output: %w", err)
	}

	best, ok := pickBestFormat(out.Formats)
	if !ok {
		return nil, ErrNeedsExternalDownload
	}

	return &MediaInfo{Title: out.Title, SelectedFormat: best}, nil
}

// pickBestFormat picks the highest-resolution progressive format (direct
// http/https URL with both video and audio). ok=false means none exist.
func pickBestFormat(formats []Format) (best Format, ok bool) {
	for _, f := range formats {
		if f.Protocol != "http" && f.Protocol != "https" {
			continue
		}
		if f.VCodec == "none" {
			continue
		}
		if f.ACodec == "none" {
			continue
		}
		if !ok || f.Height > best.Height {
			best, ok = f, true
		}
	}
	return best, ok
}
