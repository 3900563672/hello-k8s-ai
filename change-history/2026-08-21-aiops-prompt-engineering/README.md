# 变更总览：AIOps 提示词与上下文工程——Schema 契约、Token 预算与记忆边界（#112 阶段 A/B）

> 日期：2026-08-21 ｜ 级别：P1

## 背景与决策

- 四层链式提示词（L1 实体 / L2 切面 / L3 窗口 / L4 日）与对话/警戒/命令提示词此前是 Go 常量字符串：
  输出 schema 写在提示词正文里，模型可随意偏离；改提示词就是改代码；无 token 预算，实体一多可能超上下文；
  「该读什么、不该读什么」只靠注释约定，没有强制边界。
- 决策（#112）：提示词当作工程资产——目录化模板 + 版本/渲染哈希；输出契约化（Go struct + 运行时校验 + 兜底链）；
  每层显式 token 预算与截断优先级；上下文组装保持「只读结论型」并统一收口。

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
  非流式响应解析 usage；每次调用 `recordTokenUsage` 结构化日志（层/提示词 ID/prompt/completion token）。
- **预算表与截断（阶段 B）**：`internal/aiops/budget.go` 预算常量（L1 单实体 200 rune、L2 摘要区 4000、
  L3 ≤24 子级、L4 ≤96 窗口、对话 6000）；截断优先级「分数 > 结论 > 现象 > 事件」——L2 先丢现象再截结论，
  L3/L4 超窗口数保留最近 N 个，对话上下文超预算硬截断并记日志。
- **上下文组装器收口（阶段 C 雏形）**：L1 `extractEntities`、L2 `l2UserPrompt`、聚合 `aggregateChildren`、
  对话 `ChatBuildContext` 均只读结论型数据；对话上下文裁剪提取为纯函数 `truncateChatContext`（可单测）。

## 测试与验证

- 后端 `go test ./...` 全绿；新增：prompts 包渲染/哈希稳定性 + 目录注入、四层 validate 正反用例、
  兜底链重试恢复与双失败、L2 摘要裁剪（丢现象→截结论）、对话上下文截断、L3/L4 窗口数裁剪、
  CompleteJSON usage 解析（有/无 usage 两态）。
- 静态检查：`gofmt -l` 无输出；`go vet ./...` 通过。前端未改动。

## 迁移与回滚

- 无数据库迁移：本次不新增表（`aiops_chat_messages` 属 #112 阶段 D，另立 P2）。
- 回滚：删除 prompts 包并恢复各层常量即可；LLM 接口签名变化涉及调用点，回滚需一并还原 llm.go / worker.go /
  aggregator.go / alerts.go / command.go / chat.go。
- 行为变化：模型输出必须过 schema 校验（原来只做宽松解析），解析失败会多消耗一次重试调用；
  该重试计入 `MaxCallsPerAnalysis` 预算（调用点保持原有 usedCall 语义）。
