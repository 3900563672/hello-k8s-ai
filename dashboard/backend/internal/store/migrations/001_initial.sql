CREATE TABLE IF NOT EXISTS resource_events (
    sequence BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    occurred_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    operation TEXT NOT NULL,
    api_version TEXT NOT NULL,
    kind TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    uid TEXT NOT NULL DEFAULT '',
    resource_version TEXT NOT NULL DEFAULT '',
    generation BIGINT NOT NULL DEFAULT 0,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS resource_events_time_idx
    ON resource_events (occurred_at DESC, sequence DESC);
CREATE INDEX IF NOT EXISTS resource_events_object_idx
    ON resource_events (kind, namespace, name, occurred_at DESC);
CREATE INDEX IF NOT EXISTS resource_events_uid_idx
    ON resource_events (uid, occurred_at DESC) WHERE uid <> '';

CREATE TABLE IF NOT EXISTS resource_snapshots (
    snapshot_id TEXT PRIMARY KEY,
    captured_at TIMESTAMPTZ NOT NULL,
    logical_time TIMESTAMPTZ NOT NULL,
    source_versions JSONB NOT NULL DEFAULT '{}'::jsonb,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS resource_snapshots_time_idx
    ON resource_snapshots (captured_at DESC);

CREATE TABLE IF NOT EXISTS audit_log (
    sequence BIGSERIAL PRIMARY KEY,
    operation_id TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    api_version TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    uid TEXT NOT NULL DEFAULT '',
    outcome TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    details JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS audit_log_time_idx
    ON audit_log (occurred_at DESC, sequence DESC);
CREATE INDEX IF NOT EXISTS audit_log_object_idx
    ON audit_log (kind, namespace, name, occurred_at DESC);

CREATE TABLE IF NOT EXISTS trace_index (
    trace_id TEXT PRIMARY KEY,
    start_time TIMESTAMPTZ NOT NULL,
    duration_ms DOUBLE PRECISION NOT NULL,
    root_service TEXT NOT NULL DEFAULT '',
    root_operation TEXT NOT NULL DEFAULT '',
    tenant_name TEXT NOT NULL DEFAULT '',
    model_name TEXT NOT NULL DEFAULT '',
    simulator_instance_name TEXT NOT NULL DEFAULT '',
    error_span_count INTEGER NOT NULL DEFAULT 0,
    indexed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS trace_index_time_idx
    ON trace_index (start_time DESC);
CREATE INDEX IF NOT EXISTS trace_index_tenant_idx
    ON trace_index (tenant_name, start_time DESC) WHERE tenant_name <> '';

CREATE TABLE IF NOT EXISTS clock_state (
    clock_id TEXT PRIMARY KEY,
    mode TEXT NOT NULL,
    state TEXT NOT NULL,
    actual_anchor TIMESTAMPTZ NOT NULL,
    logical_anchor TIMESTAMPTZ NOT NULL,
    rate DOUBLE PRECISION NOT NULL,
    version BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (rate > 0)
);

INSERT INTO clock_state (
    clock_id, mode, state, actual_anchor, logical_anchor, rate, version
) VALUES (
    'default', 'actual', 'running', clock_timestamp(), clock_timestamp(), 1, 1
) ON CONFLICT (clock_id) DO NOTHING;
