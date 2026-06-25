CREATE TABLE IF NOT EXISTS jobs (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    url              text NOT NULL,
    status           text NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'downloading', 'done', 'failed')),
    downloaded_bytes bigint NOT NULL DEFAULT 0,
    total_bytes      bigint,
    object_key       text,
    error_message    text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
