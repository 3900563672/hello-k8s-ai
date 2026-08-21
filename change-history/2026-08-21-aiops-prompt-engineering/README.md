# 变更总览：AIOps 提示词与上下文工程——Schema 契约、Token 预算、温度分层与对话持久化（#112 全阶段）

> 日期：2026-08-21 ｜ 级别：P1

## 背景与决策

- 四层链式提示词（L1 实体 / L2 切面 / L3 窗口 / L4 日）与对话/警戒/命令提示词此前是 Go 常量字符串：
  输出 schema 写在提示词正文里，模型可随意偏离；改提示词就是改代码；无 token 预算，实体一多可能超上下文；
  「该读什么、不该读什么」只靠注释约定，没有强制边界。
- 决策（#112）：提示词当作工程资产——目录化模板 + 版本/渲染哈希；输出契约化（Go struct + 运行时校验 + 兜底链）；
  每层显式 token 预算与截断优先级；上下文组装保持「只读结论型」并统一收口；生成参数分层（分析层低温度、
  对话层默认）；对话问答对与上下文引用落库（阶段 D 追溯）。

## 实现摘要

- **提示词目录化（阶段 A）**：新增 `dashboard/backend/internal/aiops/prompts/` 包（`go:embed` 模板 +
  `Definition` 不可变定义：ID/Version/渲染函数；`Render` 输出带 sha256 前 8 字节的渲染哈希）。
  七层模板：l1_entity / l2_scores / l3_window / l4_day / alert_interpretation / command_intent
  （支持 `{{ .Catalog }}` 模板目录注入）/ chat_assistant。prompts.go / aggregator.go / alerts.go /
  command.go / chat.go 的常量全部迁移，调用点改为 `definition.Render(nil).System`。
- **Schema 契约化 + 兜底链（阶段 A）**：`internal/aiops/schema.go` 每层手写 validate（L1 枚举/长度、
  L2 分数 0-100 + verdict 枚举、L3/L4 趋势枚举与条数/字数、警戒解读长度）。统一泛型 `callStructured`：
  解析/校验失败 → 重试 1 次 → 返回失败原因由调用方规则兜底（L1 `ruleSummaries`、L2 `fallbackScores`、
  L3/L4 `ruleWindowAggregation`、警戒规则文本），不再直接 failed 浪费整轮。
- **Token 用量（阶段 B 前置）**：`LLM.CompleteJSON` 返回 `Completion{Content, Usage}`，
  非流式响应解析 usage（流式在阶段 B 收尾补齐）；每次调用 `recordTokenUsage` 结构化日志
  （层/提示词 ID/版本/哈希/prompt/completion token）。
- **预算表与截断（阶段 B）**：`internal/aiops/budget.go` 预算常量（L1 单实体 200 rune、L2 摘要区 4000、
  L3 ≤24 子级、L4 ≤96 窗口、对话 6000）；截断优先级「分数 > 结论 > 现象 > 事件」——L2 先丢现象再截结论，
  L3/L4 超窗口数保留最近 N 个，对话上下文超预算硬截断并记日志。
- **上下文组装器收口（阶段 C）**：新增 `internal/aiops/assembler.go`——`assembleEntityFacts` /
  `assembleL2Input` / `assembleAggregationInput` 统一委托 budget 与截断；对话检索器补
  `ListAIOpsCommands`（store + Postgres + Disabled stub），`ChatBuildContext` 增加 recentCommands 块；
  `ChatBuildContext` 返回 `*ChatContext`（截断文本 + `ChatContextRefs`：window/alert/command ID，
  引用与文本截断无关，始终完整收集）。
- **温度分层（阶段 C 收尾）**：`LLM` 接口与请求体支持 `Temperature *float64`（0 省略走服务端默认）；
  分析层 0.1、对话层 0.5，所有调用点（`callStructured` / `ChatStream` / `ParseCommand`）传入
  `prompt.Temperature`；fake 与单元测试同步更新。
- **对话持久化（阶段 D）**：新增迁移 `010_aiops_chat_messages.sql`（问答对 +
  `window_ids` / `alert_ids` / `command_ids` JSONB 引用数组，幂等 IF NOT EXISTS）；
  模型 `AIOpsChatMessage`；store 接口 + Postgres 实现 `CreateAIOpsChatMessage` /
  `ListAIOpsChatMessages`（按会话倒序取最近 N 条再正序返回）；Service 层 `ChatRecord` 在
  SSE 回答成功后落库 user+assistant 两条（引用只在 assistant 消息上，失败只记日志）；
  API handler 累积流式增量并在审计后调用 `ChatRecord`。
  读侧：新增 `GET /api/v1/aiops/chat/messages`（`sessionId` 必填、`limit` 1..200，按时间正序）与 Service `ChatHistory`，前端 `AiChatWidget` 打开面板时拉取服务端历史回填空会话（失败静默降级，不覆盖本地新消息）；前端契约新增 `AIOpsChatMessage` / `AIOpsChatMessagesEnvelope` 与 `fetchAIOpsChatMessages`。

## 测试与验证

- 后端 `go test ./...` 全绿；新增：prompts 包渲染/哈希稳定性 + 目录注入、四层 validate 正反用例、
  兜底链重试恢复与双失败、L2 摘要裁剪（丢现象→截结论）、对话上下文截断、L3/L4 窗口数裁剪、
  CompleteJSON usage 解析（有/无 usage 两态）、temperature 分层进入请求体（非流式 0.1 / 流式 0.5 /
  0 省略）、`ChatBuildContext` 引用收集（空数据与种子数据两态）、`ChatRecord` 双消息落库与匿名回退。
- 静态检查：`gofmt -l` 无输出；`go vet ./...` 通过；golangci-lint 0 issues。前端未改动。
- 真实 DB 验证：迁移 010 已在本机 Kind 集群 PostgreSQL 实际应用，`CreateAIOpsChatMessage` +
  `ListAIOpsChatMessages` 读写回环通过（测试行已清理）。

## 迁移与回滚

- 数据库迁移：`010_aiops_chat_messages.sql`（新表，幂等；启动时 Migrate 自动应用，已在真实 PG 验证）。
- 回滚：删除 prompts 包并恢复各层常量即可；LLM 接口签名变化涉及调用点，回滚需一并还原 llm.go / worker.go /
  aggregator.go / alerts.go / command.go / chat.go；阶段 D 回滚需同时删迁移文件与 store/模型/Service 改动
  （表可保留，不影响旧版本）。
- 行为变化：模型输出必须过 schema 校验（原来只做宽松解析），解析失败会多消耗一次重试调用；
  该重试计入 `MaxCallsPerAnalysis` 预算（调用点保持原有 usedCall 语义）。
