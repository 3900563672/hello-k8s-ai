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

模板目录（`GET /aiops/templates`）预置 model/node/tenant 各 10 条，模板 id 与集群 Model/Tenant/WorkerNode CR 名一一对应（`preset-model-001..010` 等）；集群侧由 `hack/aiops-templates-seed.sh` 幂等预置（含 ModelNodePolicy/TenantNodePolicy/TenantModelPolicy 关系策略），租户 `qps` 预置 0（空环境，无预置流量）。gate 的节点校验同时接受真实 Node 与 WorkerNode CR，AI 可直接选中任一预置节点模板。

流量形状与上限：`traffic` 支持 `steady`（固定 qps）/ `tidal`（潮汐）/ `spike`（脉冲）/ `ramp`（斜坡），波形用 `shape`+`peakQps`（+`periodMinutes` 潮汐周期，默认 30 分钟）；单命令峰值 QPS 上限 200、倍速上限 100（解析校验拒绝超限，防 AI 把流量设得离谱打爆环境）。非平稳波形由执行端调度器按模拟时间推进（墙钟 = 模拟时长/倍速），到点自动把租户 QPS 归零，不留残留流量。

限制可见性：`GET /aiops/limits` 返回上述全部硬限制与能力（单一事实源），确认面板「可执行范围」提示条直接展示（峰值 QPS ≤ 200 / 倍速 1-100 / 波形 / 潮汐周期 / 时长不限 / 随时可停止），任何约束都不会让用户凭空猜测（#134）。超上限不再拒绝而是钳制：`NormalizeTrafficIntent` 返回 applied（请求值→生效值+原因：超上限已钳制/未指定用默认），命令卡片展示「要求 500 → 生效 200（超上限，已钳制）」；未给数字的字段（峰值默认 20、倍速默认 1、非稳态时长默认一个周期）同样可见。流量目录含小时级模板（`preset-traffic-tidal-2h`：2 小时潮汐，峰值 50 QPS，30 分钟周期，真实正弦采样），Traffic 模板库预览图支持小时级时间轴。
波形由 AI 描绘：解析出 shape/peak/period/duration 后后端 `GenerateTrafficCurve` 生成采样曲线（自适应粒度），前端直接渲染预览，用户无需手画（手画/模板编辑保留为兜底）。运行中命令卡片显示模拟进度、当前 QPS（按生效曲线插值）、墙钟剩余，可一键停止（`POST /aiops/commands/{id}/stop`：QPS 归零 + 恢复倍速 + 状态 stopped）。日配额用量通过 `GET /aiops/quota` 展示在面板顶部。

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

## 8. 演示与降级预案（#124）

- 历史预生成：`bash hack/aiops-preseed.sh [数量]` 批量创建并完成切面实验，worker 自动产出 AI 分析历史（前置：AIOps 已启用且配置 Key；API 地址用 `AIOPS_API_BASE` 覆盖）。
- 失败态文案：浮窗对话错误已区分配额超限（429 `DAILY_QUOTA_EXCEEDED`）、限流（429 `CHAT_RATE_LIMITED`）、未启用（404）、网络/超时四类，不再裸抛后端原文。
- 开关关闭时：分析接口返回 404，前端各区块显示"未启用/空态"，页面本身不报错；重新打开开关后恢复。