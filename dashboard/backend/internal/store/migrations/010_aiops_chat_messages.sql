-- AIOps 对话持久化（#112 阶段 D）：aiops_chat_messages 保存同步对话的问答对与上下文引用。
-- window_ids / alert_ids / command_ids 记录回答生成时注入的结论型上下文引用（窗口总结 /
-- 警戒 / 意图命令的 ID 数组），不存原始事件；用于事后回溯「这条回答当时引用了什么」。
-- 幂等：IF NOT EXISTS，可重复应用（沿用 001-009 约定）。

CREATE TABLE IF NOT EXISTS aiops_chat_messages (
    message_id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    window_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    alert_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    command_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS aiops_chat_messages_session_idx
    ON aiops_chat_messages (session_id, created_at ASC);
