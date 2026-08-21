-- AIOps 任务队列表（#110 阶段一）：
-- aiops_jobs 是「任务级状态表」：记录 切面 + 分析类型 + status（pending/running/done/failed）
-- + attempts + last_error + 起止时间；DB 即队列，worker 用 FOR UPDATE SKIP LOCKED 认领，
-- 崩溃遗留的 running 由启动时 RequeueStale 回收。aiops_analyses 仍是结果表（幂等唯一 segment）。
-- 幂等：IF NOT EXISTS，可重复应用（沿用 001-007 约定）。

CREATE TABLE IF NOT EXISTS aiops_jobs (
    job_id TEXT PRIMARY KEY,
    segment_id TEXT NOT NULL UNIQUE REFERENCES segments(segment_id) ON DELETE CASCADE,
    kind TEXT NOT NULL DEFAULT 'analysis',
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (status IN ('pending', 'running', 'done', 'failed'))
);

CREATE INDEX IF NOT EXISTS aiops_jobs_status_idx
    ON aiops_jobs (status, created_at ASC);
CREATE INDEX IF NOT EXISTS aiops_jobs_updated_idx
    ON aiops_jobs (updated_at);
