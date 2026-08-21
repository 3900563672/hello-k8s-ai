# AIOps 智能分析层总览

> 维护层：human | last-reviewed：2026-08-21 | 事实源：dashboard/backend/internal/aiops/、dashboard/frontend/my-app/src/components/features/aiops/、change-history/

AIOps 是 Dashboard 上的可选智能分析层：用 LLM + 硬指标规则把切面（Segment）实验、时间窗口与日期的调度数据压缩成结构化结论，并支持"一句话起实验"与右下角对话浮窗。默认关闭，不依赖它也能完整使用控制面与 Simulator。

## 1. 它能做什么

| 能力 | 说明 | 主文档 |
| --- | --- | --- |
| 分层总结（L1-L4） | 实体总结 → 切面分数 → 窗口认知 → 日总结，逐层压缩上下文 | [Backend 架构](../backend/BACKEND_ARCHITECTURE.md) 第 13 节 |
| 意图执行（M2） | 一句话起实验：解析 → 模板目录校验 → 确认 gate → 写流量/调倍速/创建实验 | [API 设计](../backend/API_DESIGN.md) `/aiops/commands` |
| 时间聚合警戒（M3） | 窗口/日总结 + 分数序列规则（连续低分/趋势下滑）触发警戒 | [API 设计](../backend/API_DESIGN.md) `/aiops/windows`、`/aiops/alerts` |
| 同步对话 | 浮窗 SSE 流式问答，工具步骤可见，会话历史可回填 | [Frontend 数据流](../frontend/DATA_FLOW.md) 6.1、[页面结构](../frontend/PAGE_STRUCTURE.md) |
| 异步任务可见性 | `aiops_jobs` 队列（attempts 重试、stale 回收），前端 10s 轮询 | [Backend 架构](../backend/BACKEND_ARCHITECTURE.md) 第 13 节 |
| 运行时开关 | 面板配置 LLM 并启停分析入队；key 仅存服务端内存 | [API 设计](../backend/API_DESIGN.md) `/aiops/settings` |

## 2. 分层总结（L1-L4）

- L1 实体总结：对切面内 Pod/Node/Tenant 批量一次 LLM 调用产出结构化摘要（现象/issueFlag/healthy|suspect|problem 分类/结论）；LLM 失败用规则兜底补齐，单实体失败不影响其它。
- L2 切面总结：硬指标（错误率/TTFT p95/QPS 达成/事件数/重启数）规则先算，LLM 基于 L1 摘要 + 硬指标出分（goal/stability/efficiency/anomaly + overall + verdict + reason）。
- L3 窗口总结：定时把窗口内各切面 L2 聚合为窗口认知；L4 日总结把当日窗口聚合为日认知。
- 全部 Upsert 幂等、失败按 attempts 重试、`l1_done/l1_total` 进度落库（前端可显示）；粒度与阈值可配置。

## 3. 意图执行链路

用户一句话 → `POST /aiops/commands`（LLM 严格 JSON 解析 + 模板目录校验，编造 id 拒绝）→ 落库 `parsed` → `POST /aiops/commands/{id}/confirm` 时 gate 校验（节点/租户存在）→ 顺序执行：写流量（`SetTenantQPS`）→ 调倍速（`SetSimulationRate`）→ 创建并启动实验，每步追加 `steps`，任一步失败整体 `failed`。执行复用既有写通道，不新增越权入口。

## 4. 对话浮窗与异步

- 浮窗在 `MainLayout` 全局挂载；设置视图配置 API Key/模型/地址并开启运行时开关，设置接口只回显掩码状态。
- `POST /aiops/chat` SSE 流式（lifecycle/tool/text 事件）；回答成功后问答对与引用的 window/alert/command ID 落 `aiops_chat_messages`；打开面板时 `GET /aiops/chat/messages` 回填历史。
- 限流 6 次/分钟/会话、消息 ≤4000 字符、模型白名单；失败不影响主流程（存储不可用不阻塞对话、审计失败只记日志）。

## 5. 提示词与上下文工程（#112）

- 提示词模板在 `internal/aiops/prompts/templates/`（go:embed，分层版本化，渲染 sha256 哈希随调用日志记录）。
- 输出过运行时 schema 校验（枚举/范围/长度），解析或校验失败重试 1 次再规则兜底。
- 输入预算与截断优先级「分数 > 结论 > 现象 > 事件」；温度分层（分析层 0.1、对话层 0.5）。

## 6. 开关、配置与安全

- `AIOPS_ENABLED` 默认 false；开启需 `AIOPS_OPENAI_API_KEY`。关闭时不启动 worker、不触发入队；`/aiops/settings` 路由始终注册，保证面板能重新打开开关。
- 面板写入的 key 仅存 Backend 进程内存，重启恢复部署级环境变量；不落 PostgreSQL、不进日志与 Trace。
- 日配额保护（#124）：AIOPS_DAILY_MAX_CALLS（默认 300 次/24h）与 AIOPS_DAILY_MAX_TOKENS（默认 200 万/24h）统计 iops_audit_log 用量，超限时对话返回 429、分析不再入队——防止 key 被刷爆；0 表示不限。
- 单向依赖：只读 segments 数据 + 写 `aiops_*` 表；LLM 输出不反向驱动控制面。
- 完整 env 参数见 [配置参考](../reference/CONFIGURATION_REFERENCE.md) 第 11 节；部署见 [DEPLOYMENT](../getting-started/DEPLOYMENT.md)。

## 7. 边界与不变量

- `aiops_*` 是辅助结论，不是事实源：Kubernetes API Server 仍是唯一事实源，PostgreSQL 保存历史与 AI 结论。
- LLM、存储、遥测失败都不能阻断控制面、Simulator 或对话主流程。
- 前端不直接访问 LLM，所有 AI 调用都经 Dashboard Backend。
