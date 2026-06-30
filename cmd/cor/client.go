package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type apiClient struct {
	baseURL    string
	httpClient *http.Client
}

type jobResponse struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"`
	DownloadedBytes int64   `json:"downloaded_bytes"`
	TotalBytes      *int64  `json:"total_bytes,omitempty"`
	ObjectKey       *string `json:"object_key,omitempty"`
	Error           *string `json:"error,omitempty"`
}

func (c *apiClient) submitJob(ctx context.Context, url string) (string, error) {
	body, err := json.Marshal(map[string]string{"url": url})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/jobs", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("api returned status %d", resp.StatusCode)
	}

	var job jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	return job.ID, nil
}

func (c *apiClient) getJob(ctx context.Context, jobID string) (jobResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/jobs/"+jobID, nil)
	if err != nil {
		return jobResponse{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return jobResponse{}, fmt.Errorf("calling api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return jobResponse{}, fmt.Errorf("api returned status %d", resp.StatusCode)
	}

	var job jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return jobResponse{}, fmt.Errorf("decoding response: %w", err)
	}

	return job, nil
}

// downloadFile fetches the finished file from the API and saves it to destDir.
// It uses a separate client with no timeout since large files can take time.
func (c *apiClient) downloadFile(ctx context.Context, jobID, destDir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/jobs/"+jobID+"/file", nil)
	if err != nil {
		return "", err
	}

	streamClient := &http.Client{} // no timeout — streaming can take a while
	resp, err := streamClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api returned status %d", resp.StatusCode)
	}

	filename := filenameFromHeader(resp.Header.Get("Content-Disposition"), jobID)
	destPath := filepath.Join(destDir, filename)

	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return destPath, nil
}

// filenameFromHeader parses the filename from a Content-Disposition header.
// Falls back to fallback if the header is missing or malformed.
func filenameFromHeader(header, fallback string) string {
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "filename=") {
			name := strings.TrimPrefix(part, "filename=")
			name = strings.Trim(name, `"`)
			if name != "" {
				return name
			}
		}
	}
	return fallback
}
