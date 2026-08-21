-- AIOps 调用审计（#110 阶段四）：
-- aiops_audit_log 记录同步对话/分析调用的模型、耗时、会话与结果，用于用量校准与问题归因。
-- 幂等：IF NOT EXISTS，可重复应用（沿用 001-006 约定）。

CREATE TABLE IF NOT EXISTS aiops_audit_log (
    audit_id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL DEFAULT 'chat',
    model TEXT NOT NULL DEFAULT '',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    message_len INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'ok',
    error_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS aiops_audit_log_created_idx
    ON aiops_audit_log (created_at DESC);
