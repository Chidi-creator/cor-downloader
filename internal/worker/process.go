package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgtype"

	"cor-downloader/internal/downloader"
	"cor-downloader/internal/resolver"
	"cor-downloader/internal/store"
)

// Uploader uploads a local file and returns a key identifying where it
// ended up. The real implementation (MinIO) lives in internal/storage -
// this package only depends on the interface, not that concrete type.
type Uploader interface {
	Upload(ctx context.Context, jobID, localPath string) (objectKey string, err error)
}

// Processor does the actual work for one job: resolve, download, upload,
// and record the outcome in Postgres.
type Processor struct {
	Queries            *store.Queries
	Uploader           Uploader
	DownloadDir        string
	CookiesFromBrowser string
}

// ProcessJob runs one job, identified by jobID, end to end.
func (p *Processor) ProcessJob(ctx context.Context, jobID string) error {
	var id pgtype.UUID
	if err := id.Scan(jobID); err != nil {
		return fmt.Errorf("parsing job id: %w", err)
	}

	job, err := p.Queries.GetJob(ctx, id)
	if err != nil {
		return fmt.Errorf("fetching job: %w", err)
	}

	info, err := resolver.Resolve(ctx, job.Url, p.CookiesFromBrowser)
	if err != nil {
		p.markFailed(ctx, id, err)
		return err
	}

	// Each job gets its own scratch directory - jobs processed concurrently
	// must never share a destination path, since the filename is derived
	// from the post title, not the job ID.
	jobDir := filepath.Join(p.DownloadDir, jobID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		p.markFailed(ctx, id, err)
		return fmt.Errorf("creating job dir: %w", err)
	}
	defer os.RemoveAll(jobDir)

	onProgress := func(downloaded, total int64) {
		_ = p.Queries.UpdateProgress(ctx, store.UpdateProgressParams{
			ID:              id,
			DownloadedBytes: downloaded,
			TotalBytes:      pgtype.Int8{Int64: total, Valid: total >= 0},
		})
	}

	localPath, err := downloader.Download(ctx, info, jobDir, onProgress)
	if err != nil {
		p.markFailed(ctx, id, err)
		return err
	}

	objectKey, err := p.Uploader.Upload(ctx, jobID, localPath)
	if err != nil {
		p.markFailed(ctx, id, err)
		return err
	}

	if err := p.Queries.MarkDone(ctx, store.MarkDoneParams{
		ID:        id,
		ObjectKey: pgtype.Text{String: objectKey, Valid: true},
	}); err != nil {
		return fmt.Errorf("marking job done: %w", err)
	}

	return nil
}

func (p *Processor) markFailed(ctx context.Context, id pgtype.UUID, cause error) {
	_ = p.Queries.MarkFailed(ctx, store.MarkFailedParams{
		ID:           id,
		ErrorMessage: pgtype.Text{String: cause.Error(), Valid: true},
	})
}
