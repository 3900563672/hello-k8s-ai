-- AIOps 智能分析层（总纲 #92 / #93）：
-- aiops_analyses：一次切面的 L1/L2 分析主记录（状态机 pending→running→aggregating→completed/failed）。
-- aiops_entity_summaries：L1 实体总结；aiops_window_summaries：L3/L4 时间聚合（M3 启用）。
-- aiops_alerts：持续低分警戒（M3 启用）；aiops_commands：意图执行记录（M2 启用）。
-- 幂等：所有对象 IF NOT EXISTS，可重复应用（沿用 001-004 约定）。

CREATE TABLE IF NOT EXISTS aiops_analyses (
    analysis_id TEXT PRIMARY KEY,
    segment_id TEXT NOT NULL UNIQUE REFERENCES segments(segment_id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    l1_total INT NOT NULL DEFAULT 0,
    l1_done INT NOT NULL DEFAULT 0,
    scores JSONB,
    summary JSONB,
    error_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (status IN ('pending', 'running', 'aggregating', 'completed', 'failed'))
);

CREATE INDEX IF NOT EXISTS aiops_analyses_status_idx
    ON aiops_analyses (status, created_at ASC);
CREATE INDEX IF NOT EXISTS aiops_analyses_segment_idx
    ON aiops_analyses (segment_id);

CREATE TABLE IF NOT EXISTS aiops_entity_summaries (
    summary_id TEXT PRIMARY KEY,
    analysis_id TEXT NOT NULL REFERENCES aiops_analyses(analysis_id) ON DELETE CASCADE,
    entity_kind TEXT NOT NULL,
    entity_name TEXT NOT NULL,
    classification TEXT NOT NULL DEFAULT 'healthy',
    phenomenon TEXT NOT NULL DEFAULT '',
    issue_flag BOOLEAN NOT NULL DEFAULT FALSE,
    conclusion TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (analysis_id, entity_kind, entity_name)
);

CREATE INDEX IF NOT EXISTS aiops_entity_summaries_analysis_idx
    ON aiops_entity_summaries (analysis_id);

CREATE TABLE IF NOT EXISTS aiops_window_summaries (
    window_id TEXT PRIMARY KEY,
    level TEXT NOT NULL CHECK (level IN ('L3', 'L4')),
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    scores JSONB,
    summary JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS aiops_window_summaries_window_idx
    ON aiops_window_summaries (level, window_start DESC);

CREATE TABLE IF NOT EXISTS aiops_alerts (
    alert_id TEXT PRIMARY KEY,
    rule TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'warning',
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    analysis_id TEXT REFERENCES aiops_analyses(analysis_id) ON DELETE SET NULL,
    interpretation JSONB,
    acked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS aiops_alerts_triggered_idx
    ON aiops_alerts (triggered_at DESC);

CREATE TABLE IF NOT EXISTS aiops_commands (
    command_id TEXT PRIMARY KEY,
    raw_input TEXT NOT NULL,
    parsed JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'parsed',
    steps JSONB NOT NULL DEFAULT '[]'::jsonb,
    error_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (status IN ('parsed', 'confirmed', 'gate', 'executing', 'verified', 'done', 'rejected', 'failed'))
);