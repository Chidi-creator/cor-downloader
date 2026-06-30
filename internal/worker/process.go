package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"cor-downloader/internal/downloader"
	"cor-downloader/internal/resolver"
	"cor-downloader/internal/store"
)

// Processor does the actual work for one job: resolve, download, and record
// the outcome in Postgres. The file stays in DownloadDir until the API
// streams it to the client, which then cleans it up.
type Processor struct {
	Queries            *store.Queries
	DownloadDir        string // scratch space; files live here until downloaded
	CookiesFromBrowser string
}

func (p *Processor) ProcessJob(ctx context.Context, jobID string) error {
	start := time.Now()

	var id pgtype.UUID
	if err := id.Scan(jobID); err != nil {
		return fmt.Errorf("parsing job id: %w", err)
	}

	job, err := p.Queries.GetJob(ctx, id)
	if err != nil {
		return fmt.Errorf("fetching job: %w", err)
	}

	log.Printf("job %s: resolving %s", jobID, job.Url)
	info, err := resolver.Resolve(ctx, job.Url, p.CookiesFromBrowser)
	if err != nil {
		p.markFailed(ctx, id, err)
		return err
	}
	log.Printf("job %s: resolved — title=%q format=%s", jobID, info.Title, info.SelectedFormat.Ext)

	jobDir := filepath.Join(p.DownloadDir, jobID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		p.markFailed(ctx, id, err)
		return fmt.Errorf("creating scratch dir: %w", err)
	}

	var lastProgressUpdate time.Time
	onProgress := func(downloaded, total int64) {
		now := time.Now()
		if !shouldSendProgressUpdate(now, lastProgressUpdate, downloaded, total) {
			return
		}
		lastProgressUpdate = now

		log.Printf("job %s: downloading — %.1f MB / %.1f MB",
			jobID,
			float64(downloaded)/1e6,
			float64(total)/1e6,
		)

		_ = p.Queries.UpdateProgress(ctx, store.UpdateProgressParams{
			ID:              id,
			DownloadedBytes: downloaded,
			TotalBytes:      pgtype.Int8{Int64: total, Valid: total >= 0},
		})
	}

	log.Printf("job %s: starting download", jobID)
	localPath, err := downloader.Download(ctx, info, jobDir, onProgress)
	if err != nil {
		os.RemoveAll(jobDir)
		p.markFailed(ctx, id, err)
		return err
	}

	log.Printf("job %s: download complete in %.1fs — ready to serve", jobID, time.Since(start).Seconds())

	if err := p.Queries.MarkDone(ctx, store.MarkDoneParams{
		ID:        id,
		ObjectKey: pgtype.Text{String: localPath, Valid: true},
	}); err != nil {
		os.RemoveAll(jobDir)
		return fmt.Errorf("marking job done: %w", err)
	}

	return nil
}

func (p *Processor) markFailed(ctx context.Context, id pgtype.UUID, cause error) {
	log.Printf("job failed: %v", cause)
	_ = p.Queries.MarkFailed(ctx, store.MarkFailedParams{
		ID:           id,
		ErrorMessage: pgtype.Text{String: cause.Error(), Valid: true},
	})
}
