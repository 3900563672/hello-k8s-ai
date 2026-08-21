-- AIOps 审计补 token 用量（#110 阶段四）：流式响应的 usage 字段（include_usage）。
-- 幂等：IF NOT EXISTS，可重复应用（沿用 001-008 约定）。

ALTER TABLE aiops_audit_log
    ADD COLUMN IF NOT EXISTS prompt_tokens INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS completion_tokens INT NOT NULL DEFAULT 0;
