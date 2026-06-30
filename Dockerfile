## Stage 1 — build the Go binary
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Download dependencies first (cached if go.mod/go.sum unchanged)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and compile
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api


## Stage 2 — minimal runtime image
FROM alpine:3.21

# ffmpeg is needed by yt-dlp for some formats; ca-certificates lets us
# make HTTPS requests to X/Instagram CDNs; curl is used to install yt-dlp
RUN apk add --no-cache ffmpeg ca-certificates curl python3

# Install the yt-dlp binary directly from its GitHub releases
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp \
    -o /usr/local/bin/yt-dlp \
    && chmod +x /usr/local/bin/yt-dlp

# Copy only the compiled binary from the builder stage
COPY --from=builder /api /api

ENTRYPOINT ["/api"]
