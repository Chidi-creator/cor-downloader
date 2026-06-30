package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"

	"github.com/redis/go-redis/v9"

	"cor-downloader/internal/queue"
	"cor-downloader/internal/store"
	"cor-downloader/internal/worker"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log.Println("connecting to postgres...")
	dbPool, err := store.Connect(ctx, getEnv("DATABASE_URL", "postgres://cor:cor@localhost:5433/cor"))
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}
	defer dbPool.Close()
	queries := store.New(dbPool)
	log.Println("postgres ready")

	log.Println("connecting to redis...")
	redisClient := redis.NewClient(&redis.Options{Addr: getEnv("REDIS_ADDR", "localhost:6379")})
	defer redisClient.Close()
	q := queue.New(redisClient, getEnv("QUEUE_KEY", "cor:jobs"))
	log.Println("redis ready")

	downloadDir := filepath.Join(os.TempDir(), "cor-downloader")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		log.Fatalf("creating scratch dir: %v", err)
	}
	log.Printf("scratch dir: %s", downloadDir)

	concurrency, err := strconv.Atoi(getEnv("WORKER_CONCURRENCY", "3"))
	if err != nil {
		log.Fatalf("invalid WORKER_CONCURRENCY: %v", err)
	}

	pool := &worker.Pool{
		Queue: q,
		Processor: &worker.Processor{
			Queries:     queries,
			DownloadDir: downloadDir,
		},
		Concurrency: concurrency,
	}
	log.Printf("starting %d workers", concurrency)

	poolDone := make(chan struct{})
	go func() {
		pool.Run(ctx)
		close(poolDone)
	}()

	srv := &api{queries: queries, queue: q}
	httpServer := &http.Server{
		Addr:    getEnv("HTTP_ADDR", ":8080"),
		Handler: srv.routes(),
	}

	go func() {
		<-ctx.Done()
		log.Println("shutting down http server...")
		_ = httpServer.Shutdown(context.Background())
	}()

	log.Printf("api listening on %s", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server failed: %v", err)
	}

	log.Println("waiting for in-flight jobs to finish...")
	<-poolDone
	log.Println("shutdown complete")
}
