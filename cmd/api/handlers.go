package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"cor-downloader/internal/queue"
	"cor-downloader/internal/store"
)

type api struct {
	queries *store.Queries
	queue   *queue.Queue
}

func (a *api) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("POST /jobs", a.createJob)
	mux.HandleFunc("GET /jobs/{id}", a.getJob)
	mux.HandleFunc("GET /jobs/{id}/file", a.downloadFile)
	return mux
}

func (a *api) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type createJobRequest struct {
	URL string `json:"url"`
}

type jobResponse struct {
	ID              string  `json:"id"`
	URL             string  `json:"url"`
	Status          string  `json:"status"`
	DownloadedBytes int64   `json:"downloaded_bytes"`
	TotalBytes      *int64  `json:"total_bytes,omitempty"`
	ObjectKey       *string `json:"object_key,omitempty"`
	Error           *string `json:"error,omitempty"`
}

func toJobResponse(j store.Job) jobResponse {
	resp := jobResponse{
		ID:              j.ID.String(),
		URL:             j.Url,
		Status:          j.Status,
		DownloadedBytes: j.DownloadedBytes,
	}
	if j.TotalBytes.Valid {
		resp.TotalBytes = &j.TotalBytes.Int64
	}
	if j.ObjectKey.Valid {
		resp.ObjectKey = &j.ObjectKey.String
	}
	if j.ErrorMessage.Valid {
		resp.Error = &j.ErrorMessage.String
	}
	return resp
}

func (a *api) createJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	job, err := a.queries.CreateJob(r.Context(), req.URL)
	if err != nil {
		http.Error(w, "failed to create job", http.StatusInternalServerError)
		return
	}

	if err := a.queue.Push(r.Context(), job.ID.String()); err != nil {
		http.Error(w, "failed to queue job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toJobResponse(job))
}

func (a *api) getJob(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")

	var id pgtype.UUID
	if err := id.Scan(idParam); err != nil {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}

	job, err := a.queries.GetJob(r.Context(), id)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toJobResponse(job))
}

func (a *api) downloadFile(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")

	var id pgtype.UUID
	if err := id.Scan(idParam); err != nil {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}

	job, err := a.queries.GetJob(r.Context(), id)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	if job.Status != "done" {
		http.Error(w, fmt.Sprintf("job is %s, not done", job.Status), http.StatusConflict)
		return
	}

	if !job.ObjectKey.Valid {
		http.Error(w, "no file available", http.StatusNotFound)
		return
	}

	filePath := job.ObjectKey.String
	jobDir := filepath.Dir(filePath)

	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "file not found on server", http.StatusNotFound)
		return
	}

	// clean up the scratch dir once we're done streaming — success or failure
	defer func() {
		f.Close()
		if err := os.RemoveAll(jobDir); err == nil {
			log.Printf("job %s: scratch dir cleaned up", idParam)
		}
	}()

	filename := filepath.Base(filePath)

	info, err := f.Stat()
	if err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))

	log.Printf("job %s: streaming %s to client", idParam, filename)
	if _, err := io.Copy(w, f); err != nil {
		if !strings.Contains(err.Error(), "broken pipe") {
			log.Printf("job %s: stream error: %v", idParam, err)
		}
	}
}
