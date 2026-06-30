package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: cor <url>")
		os.Exit(1)
	}
	url := os.Args[1]

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := &apiClient{
		baseURL:    getEnv("COR_API_URL", "http://localhost:8080"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	fmt.Println("submitting job...")
	jobID, err := client.submitJob(ctx, url)
	if err != nil {
		log.Fatalf("submit failed: %v", err)
	}
	fmt.Printf("job created: %s\n", jobID)
	fmt.Println("waiting for download to complete...")

	for {
		job, err := client.getJob(ctx, jobID)
		if err != nil {
			log.Fatalf("polling job: %v", err)
		}

		switch job.Status {
		case "done":
			fmt.Println()

			homeDir, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("finding home dir: %v", err)
			}

			fmt.Println("retrieving file from server...")
			path, err := client.downloadFile(ctx, jobID, homeDir+"/Downloads")
			if err != nil {
				log.Fatalf("downloading file: %v", err)
			}

			fmt.Printf("saved: %s\n", path)
			return

		case "failed":
			fmt.Println()
			if job.Error != nil {
				log.Fatalf("job failed: %s", *job.Error)
			}
			log.Fatalf("job failed: unknown error")

		default:
			if job.TotalBytes != nil && *job.TotalBytes > 0 {
				fmt.Printf("\r  %s — %.1f MB / %.1f MB   ",
					job.Status,
					float64(job.DownloadedBytes)/1e6,
					float64(*job.TotalBytes)/1e6,
				)
			} else {
				fmt.Printf("\r  %s — %.1f MB   ",
					job.Status,
					float64(job.DownloadedBytes)/1e6,
				)
			}
			time.Sleep(time.Second)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
