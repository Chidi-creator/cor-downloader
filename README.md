# cor-downloader

A production-grade media downloader for X (Twitter) and Instagram. Submit a URL, the server resolves and downloads the video in the background, and you retrieve the file over HTTP — from the CLI, Postman, or a Chrome extension.

## Architecture

```
CLI / Chrome extension / Postman
        │
        │  HTTP
        ▼
   ┌─────────────────────────────────────┐
   │           API Server (cmd/api)      │
   │  POST /jobs     → create + enqueue  │
   │  GET  /jobs/:id → poll status       │
   │  GET  /jobs/:id/file → stream file  │
   │  GET  /health   → liveness check    │
   └─────────┬──────────────────┬────────┘
             │                  │
             ▼                  ▼
       ┌───────────┐     ┌────────────┐
       │  Postgres │     │   Redis    │
       │ (job state│     │  (queue)   │
       └───────────┘     └─────┬──────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │   Worker Pool (x5)  │
                    │  pop → resolve URL  │
                    │  → stream download  │
                    │  → mark done        │
                    └─────────────────────┘
```

**Flow:**
1. Client sends `POST /jobs` with a URL
2. API creates a `pending` job in Postgres and pushes the job ID to Redis
3. A worker pops the ID, resolves the media via `yt-dlp`, streams the video to a temp directory
4. Job is marked `done` in Postgres
5. Client calls `GET /jobs/:id/file` — API streams the file back, then deletes it

## Prerequisites

### Local development
- [Go 1.26+](https://go.dev/dl/)
- [Docker + Docker Compose](https://docs.docker.com/get-docker/)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)
- [ffmpeg](https://ffmpeg.org/)

```bash
# macOS
brew install go yt-dlp ffmpeg
```

## Local setup

**1. Clone and install dependencies**

```bash
git clone https://github.com/Chidi-creator/cor-downloader.git
cd cor-downloader
go mod download
```

**2. Start infrastructure**

```bash
docker compose up -d postgres redis
```

This starts Postgres on port `5433` and Redis on port `6379`.

**3. Start the API server**

```bash
go run ./cmd/api
```

The server starts on `http://localhost:8080`. The database schema is applied automatically on startup.

**4. Download a video**

```bash
go run ./cmd/cor 'https://x.com/username/status/123456789'
```

The file is saved to `~/Downloads/`.

## Running everything in Docker

```bash
docker compose up --build
```

This builds the API image and runs the full stack (Postgres, Redis, API + workers) in containers. The API is accessible at `http://localhost:8080`.

## CLI usage

```bash
# Basic usage
go run ./cmd/cor '<url>'

# Or build the binary first
go build -o cor ./cmd/cor
./cor '<url>'

# Install globally (type 'cor' from anywhere)
go install ./cmd/cor
cor '<url>'

# Point at a hosted server
COR_API_URL=https://your-server.com cor '<url>'
```

Supported platforms: X (Twitter), Instagram, and any platform supported by yt-dlp.

## API reference

### `POST /jobs`
Submit a URL for download.

```bash
curl -X POST https://your-server.com/jobs \
  -H "Content-Type: application/json" \
  -d '{"url": "https://x.com/username/status/123456789"}'
```

Response:
```json
{
  "id": "abc-123",
  "url": "https://x.com/...",
  "status": "pending",
  "downloaded_bytes": 0
}
```

### `GET /jobs/:id`
Poll job status and progress.

```bash
curl https://your-server.com/jobs/abc-123
```

Response:
```json
{
  "id": "abc-123",
  "status": "downloading",
  "downloaded_bytes": 8200000,
  "total_bytes": 26200000
}
```

Status values: `pending` → `downloading` → `done` / `failed`

### `GET /jobs/:id/file`
Retrieve the finished file. Only available when `status` is `done`. The file is streamed as an attachment and deleted from the server after delivery.

```bash
curl -OJ https://your-server.com/jobs/abc-123/file
```

### `GET /health`
Liveness check. Returns `200 ok`. Used by uptime monitors to keep the server awake.

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://cor:cor@localhost:5433/cor` | Postgres connection string |
| `REDIS_URL` | — | Redis URL with TLS (Upstash format: `rediss://...`) |
| `REDIS_ADDR` | `localhost:6379` | Plain Redis address (used if `REDIS_URL` is not set) |
| `QUEUE_KEY` | `cor:jobs` | Redis list key for the job queue |
| `HTTP_ADDR` | `:8080` | Address the HTTP server listens on |
| `WORKER_CONCURRENCY` | `3` | Number of parallel download workers |
| `COR_API_URL` | `http://localhost:8080` | API URL used by the `cor` CLI |

Copy `.env.example` to `.env` for local development:

```bash
cp .env.example .env
```

## Deployment (Render)

The app is designed to be self-contained. All services run in Docker containers on a single machine.

**Services needed:**
- Render Web Service (this repo, Docker) — the API + workers
- Render PostgreSQL — job state
- [Upstash Redis](https://upstash.com) (free tier) — job queue

**Steps:**
1. Create a Render PostgreSQL database (Postgres 16, free tier)
2. Create an Upstash Redis database — copy the `rediss://` TCP URL
3. Create a Render Web Service connected to this GitHub repo
   - Environment: Docker
   - Port: `8080`
   - Set environment variables (see table above)
4. Set up a cron job at [cron-job.org](https://cron-job.org) to ping `GET /health` every 14 minutes to prevent the free tier from sleeping

## Project structure

```
cmd/
  api/          → HTTP server + worker pool (the deployable binary)
  cor/          → CLI client (submit jobs, poll status, save to ~/Downloads)
internal/
  resolver/     → shells out to yt-dlp to resolve media URLs
  downloader/   → streams the video from CDN to disk with retry + progress
  store/        → Postgres access via sqlc-generated type-safe queries
  queue/        → Redis job queue (push/pop job IDs)
  worker/       → worker pool — processes one job end to end
  storage/      → S3-compatible object storage client (available for future use)
Dockerfile      → multi-stage build: Go compiler → Alpine runtime with yt-dlp
docker-compose.yml → local dev stack (Postgres, Redis, MinIO, API)
sqlc.yaml       → sqlc config for generating type-safe DB code
```

## How it works

**Why yt-dlp?** X and Instagram constantly change their internal APIs, use signed/expiring CDN URLs, and require authentication. yt-dlp maintains extractors for 1000+ platforms. We use it only for URL resolution (`yt-dlp -j`) — the actual download is done by our own Go streaming code.

**Why a queue?** Without a queue, simultaneous requests would all start downloading immediately, overwhelming the server. Redis acts as a buffer — jobs wait in the queue, workers process them at a controlled rate (`WORKER_CONCURRENCY`).

**Why stream?** Files are never fully loaded into memory. `io.Copy` moves bytes from X's CDN directly to disk in chunks. This keeps memory usage flat regardless of file size.

**Why temp files + HTTP streaming?** The worker downloads to a temp directory on the server. When you call `GET /jobs/:id/file`, the API streams the file over HTTP to your machine, then deletes it. This means one network hop (server → you) and no permanent storage needed.
