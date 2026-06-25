-- name: CreateJob :one
INSERT INTO jobs (url)
VALUES ($1)
RETURNING *;

-- name: GetJob :one
SELECT * FROM jobs
WHERE id = $1;

-- name: UpdateProgress :exec
UPDATE jobs
SET status = 'downloading',
    downloaded_bytes = $2,
    total_bytes = $3,
    updated_at = now()
WHERE id = $1;

-- name: MarkDone :exec
UPDATE jobs
SET status = 'done',
    object_key = $2,
    updated_at = now()
WHERE id = $1;

-- name: MarkFailed :exec
UPDATE jobs
SET status = 'failed',
    error_message = $2,
    updated_at = now()
WHERE id = $1;
