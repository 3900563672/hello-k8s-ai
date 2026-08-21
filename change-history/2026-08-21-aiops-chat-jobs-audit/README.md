# 变更总览：AIOps 助手浮窗 + 异步任务可见性 + 配置审计（#110 全阶段）

> 日期：2026-08-21 ｜ 级别：P1

## 背景与决策

- 控制台原只有观测页内嵌「AI 洞察」，无全局对话入口，也看不到异步分析正在干什么；面板里配 API 若把密钥放进浏览器，等于公开账单凭证。
- 决策：双模式——异步任务队列（DB 即队列，worker 轮询驱动）+ 同步 SSE 对话浮窗；密钥只存服务端内存/环境变量，前端只显示掩码；任务队列采用 `FOR UPDATE SKIP LOCKED`（参考 pg-boss/pgqueuer 同款模式），analyses 保留为结果表。

## 实现摘要

- **阶段一（异步可见性）**：迁移 008 `aiops_jobs`（segment 唯一、status/attempts/max_attempts/last_error/起止时间）；worker 改任务队列驱动（SKIP LOCKED 认领 → 复用 analyses 状态机 → 回写 done/failed），启动时 `RequeueStaleAIOpsJobs` 回收崩溃遗留；`GET /api/v1/aiops/jobs`（status 过滤）；前端 `AIOpsJobList` 挂 AiInsightPanel（10s 轮询，进行中计数/状态徽章/重试次数/失败原因）。
- **阶段二/三（同步对话）**：`POST /api/v1/aiops/chat` SSE 流（lifecycle/tool/text，AG-UI 轻量子集）；`AiChatWidget` 浮窗挂 MainLayout（气泡 → 面板，工具步骤指示器，会话 localStorage 不含 key）；限制：消息长度上限、会话限流、模型白名单。
- **阶段四（配置与审计）**：`GET/POST /api/v1/aiops/settings` 掩码态（key 仅内存，重启由环境变量恢复）；`aiops_audit_log`（迁移 007）记录模型/耗时/消息长度/结果，迁移 009 补 prompt/completion token（流式请求带 `stream_options.include_usage`，末 chunk usage 解析）；前端「设置」入口（对话/配置双视图）。
- **工程修复**：`hack/gen-docs.py` 只统计 git 已跟踪的 change-history 条目，多会话共享工作树时派生文件互不污染（README 时间线段 / docs/status.md）。

## 测试与验证

- 后端 `go build` / `go vet` / `go test -count=1 ./...` 全绿；新增：任务生命周期（pending→done）与失败回写（failed + last_error）、流式 usage 解析（prompt=11/completion=3）、配置掩码态与审计落库。
- 前端 `npm run check`（oxlint + tsc + vite build + state check）通过；`make lint`（shellcheck/markdownlint/PSScriptAnalyzer/golangci-lint）与 `make docs-check` 全绿。
- 合入 main 后复核（f88caa0，#113）：attempts 重试上限语义生效——worker 单次 poll 重试 3 次达上限回写 failed，未达上限回 pending 续跑；对应测试同步更新。

## 迁移与回滚

- 迁移 007（audit 表）/ 008（jobs 表）/ 009（audit token 列）：全部 `IF NOT EXISTS` 幂等，可重复应用；schema_migrations 记录版本，删除失败可重跑。
- 回滚：按版本删除对应对象（表/列）并清 migration 版本行即可；新 API 只读或写白名单，不改变既有 CR/Controller 语义，不影响模拟时钟与 replay。
