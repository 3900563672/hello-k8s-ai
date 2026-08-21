-- AIOps 任务语义补强（#110 阶段一）：
-- aiops_analyses 即任务队列。attempts：失败重试计数（claim 时 +1，达上限转 failed）；
-- kind：任务类型（segment=切面 L1/L2 分析；预留窗口聚合等任务类型）。
-- 幂等：ADD COLUMN IF NOT EXISTS，可重复应用（沿用 001-005 约定）。

ALTER TABLE aiops_analyses
    ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'segment';
