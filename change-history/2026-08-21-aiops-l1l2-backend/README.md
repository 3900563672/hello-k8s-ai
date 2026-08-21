# 变更总览：AIOps 分层总结后端骨架（M0+M1，Fixes #93）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- AIOps 总纲 #92 定义了两个能力（一句话起实验 / 切面分层总结+打分），本次落地能力 2 的后端骨架（M0+M1）：切面 completed/failed 后自动产出 L1 实体总结（全量覆盖）与 L2 切面分数（含理由），状态机可查进度，LLM 不可用时规则分兜底。
- 与业界成熟 AIOps 方案对齐（#105 实施路线）：预算硬限制（借鉴 vigil）、规则先行 + LLM 判断（借鉴 Zenjoy/Aurora）、GATE 校验思想留待 M2 意图执行。

## 改成什么

1. 新增 `dashboard/backend/internal/aiops/`：OpenAI 兼容 Provider（`llm.go`，json_object 强制、429/5xx 重试、4xx 不重试）、L1/L2 提示词与实体提取（`prompts.go`）、硬指标与规则兜底打分（`scoring.go`）、分析 worker（`worker.go`：轮询 pending → 认领 → L1 批量 → L2 混合打分 → 落库）。
2. 新增迁移 `005_aiops.sql`：`aiops_analyses`（状态机 pending→running→aggregating→completed/failed + L1 进度）、`aiops_entity_summaries`（L1，按分析+实体唯一）、`aiops_window_summaries`（L3/L4，M3 用）、`aiops_alerts`（警戒，M3 用）、`aiops_commands`（意图执行，M2 用）。
3. `store.Store` 新增 11 个 AIOps 方法（Postgres 实现 + Disabled 兜底），模型类型入 `internal/model/aiops.go`。
4. 配置新增 AIOps 段（`AIOPS_ENABLED` 默认 false，开启必须 `AIOPS_OPENAI_API_KEY`；预算/轮询参数可配），`dashboard/deploy/backend.yaml` 已暴露（默认关闭 + Secret 注入示例注释）。
5. API：`GET /api/v1/aiops/analyses`（列表，可按 status 过滤）、`GET /api/v1/aiops/analyses/{id}`（详情 + L1 实体，支持 `?segmentId=`）；实验 complete/fail 后自动入队分析。

## 关键行为

- 入队幂等：`aiops_analyses.segment_id` 唯一，重复 complete/fail 不产生重复分析。
- L1 全量覆盖：不做健康过滤；LLM 遗漏的实体用规则兜底补齐（保证每个实体都有总结）。
- L2 混合打分：硬指标（错误率/TTFT p95/QPS 达成/事件计数/重启数）规则先算，LLM 基于摘要与硬指标出分；LLM 失败或预算耗尽 → 规则分兜底，分析不阻塞。
- 预算硬限制：单次分析 LLM 调用次数 ≤ `AIOPS_MAX_CALLS_PER_ANALYSIS`（默认 8），单次 token ≤ `AIOPS_MAX_TOKENS_PER_CALL`。
- 崩溃恢复：启动时回收 running/aggregating 超时（`AIOPS_STALE_REQUEUE_INTERVAL`，默认 10min）的任务回 pending。

## 验证

- `go build ./...`、`go vet ./...`、`gofmt -l` 全干净。
- 单元测试 10 个包全绿；新增 aiops 包测试：LLM 可用全链路（L1 3 实体 + L2 分数）、LLM 全失败规则兜底、入队幂等、LLM 客户端重试（500 重试成功/400 不重试）、实体提取合并、分数钳制。
- 集成测试 `TestAIOpsStoreLifecycle`（迁移 005 + 表读写 + 幂等）已写，需 `TEST_DATABASE_URL`（本机无 PostgreSQL，未实跑）。
- `kubectl kustomize dashboard/deploy` 渲染通过。

## 回滚

- 关闭 `AIOPS_ENABLED` 即完全停用（不建服务、不注册路由、不触发入队）。
- 迁移 005 为追加式（IF NOT EXISTS），回滚只需删 `aiops_*` 五张表与 `schema_migrations` 中 `005_aiops.sql` 一行（未写入任何生产数据时可忽略）。
- 未推送、未合并：`git reset --hard` 前请确认不包含其它会话未提交内容。

## 未验证/待办

- 真实 LLM 调用需 `AIOPS_OPENAI_API_KEY`（本次未配置，规则兜底路径已覆盖）。
- M2 意图执行（#94）、M3 时间聚合与警戒（#95）未开始；前端 AI 面板（#96-98/100）待 API 稳定后接入。
