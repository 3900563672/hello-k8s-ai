-- 切面生命周期（Issue #51）：segments 是一次调度实验的不可变归档单元，
-- 配 segment_events（六类事件）/ segment_metrics（1min 桶聚合）/ trace 关联。
-- 幂等：所有对象 IF NOT EXISTS，可重复应用。

CREATE TABLE IF NOT EXISTS segments (
    segment_id TEXT PRIMARY KEY,
    tenant TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    reason TEXT NOT NULL DEFAULT '',
    config_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    start_snapshot JSONB,
    end_snapshot JSONB,
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (status IN ('pending', 'running', 'completed', 'failed'))
);

CREATE INDEX IF NOT EXISTS segments_status_idx
    ON segments (status, created_at DESC);
CREATE INDEX IF NOT EXISTS segments_tenant_idx
    ON segments (tenant, created_at DESC);

-- 切面内事件：decision/alert/error/gap/burst/phase_change 六类，Pod 个体不进切面。
CREATE TABLE IF NOT EXISTS segment_events (
    event_id TEXT PRIMARY KEY,
    segment_id TEXT NOT NULL REFERENCES segments(segment_id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    entity TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL DEFAULT 'info',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS segment_events_segment_idx
    ON segment_events (segment_id, occurred_at DESC);

-- 切面内指标聚合：1min 桶 min/max/avg/p95。
CREATE TABLE IF NOT EXISTS segment_metrics (
    metric_id TEXT PRIMARY KEY,
    segment_id TEXT NOT NULL REFERENCES segments(segment_id) ON DELETE CASCADE,
    metric_name TEXT NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    bucket_end TIMESTAMPTZ NOT NULL,
    min DOUBLE PRECISION NOT NULL,
    max DOUBLE PRECISION NOT NULL,
    avg DOUBLE PRECISION NOT NULL,
    p95 DOUBLE PRECISION NOT NULL,
    UNIQUE (segment_id, metric_name, bucket_start)
);

CREATE INDEX IF NOT EXISTS segment_metrics_segment_idx
    ON segment_metrics (segment_id, metric_name, bucket_start);

-- trace_index 增加 segment_id 关联（幂等；已有 trace 保持 NULL，按需回填）。
ALTER TABLE trace_index ADD COLUMN IF NOT EXISTS segment_id TEXT;

CREATE INDEX IF NOT EXISTS trace_index_segment_idx
    ON trace_index (segment_id) WHERE segment_id IS NOT NULL;