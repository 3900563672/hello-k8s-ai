CREATE TABLE IF NOT EXISTS resource_states (
    state_id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    uid TEXT NOT NULL DEFAULT '',
    resource_version TEXT NOT NULL DEFAULT '',
    generation BIGINT NOT NULL DEFAULT 0,
    captured_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (kind, namespace, name)
);

CREATE INDEX IF NOT EXISTS resource_states_object_idx
    ON resource_states (kind, namespace, name);
CREATE INDEX IF NOT EXISTS resource_states_time_idx
    ON resource_states (updated_at DESC);