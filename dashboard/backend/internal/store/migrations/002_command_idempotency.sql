CREATE TABLE IF NOT EXISTS command_idempotency (
    idempotency_key TEXT PRIMARY KEY,
    request_hash TEXT NOT NULL,
    state TEXT NOT NULL,
    status_code INTEGER,
    response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    completed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    CHECK (state IN ('pending', 'completed')),
    CHECK (
        (state = 'pending' AND status_code IS NULL AND response IS NULL AND completed_at IS NULL)
        OR
        (state = 'completed' AND status_code BETWEEN 100 AND 599 AND response IS NOT NULL AND completed_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS command_idempotency_expiry_idx
    ON command_idempotency (expires_at);
