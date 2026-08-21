# 变更总览：AIOps 前端 M4——AI 洞察分区与契约先行（#98 前端部分）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- 前端重构合入主仓库后（2026-08-21 同批次），开始 AIOps 前端（#98 契约先行部分）：在 Observatory 增加 AI 洞察分区，落地切面分析展示、一句话起实验入口、警戒列表与气泡 AI 分级外圈。
- 后端 #93（L1/L2）已实现未提交、#94/#95（M2/M3）未开始：本次按 #98 的"契约先行"策略，类型与后端 model/迁移对齐，组件走真实 API 层；后端未就绪的端点（/aiops/alerts）404 时优雅显示未接入，dev:mock 提供契约演示数据。

## 改成什么

1. 契约类型 `src/types/aiops.types.ts`：analysis/entity/scores/window/alert/command + AgentVerdict（字段与后端 `internal/model/aiops.go` + 迁移 005 对齐）；`AgentGrade/AgentVerdict` 从 ClusterBubbleField 移入契约，trace.types 的 Pod/Node/Tenant 增加可选 `agentVerdict`。
2. API 层：`src/api/endpoints/aiopsApi.ts`（analyses 列表/详情/按切面查询）+ `src/api/queries/aiopsQueries.ts`（15s 列表轮询、详情进行中 10s 轮询、404 不重试）。
3. 组件（features/observatory/）：`AiInsightPanel`（状态机/进度/L2 四维分数+总分+理由/L1 实体分类列表）、`CommandInput`（一句话入口，本地规则演示解析并标注 DEMO）、`AlertList`（M3 未接入空态 + 演示数据）。
4. 气泡 AI 分级：ObservatoryPage 取最新 completed 分析的 L1 实体总结，注入 overview 的 `agentVerdict` → ClusterBubbleField 外圈按 classification 着色（healthy→normal/suspect→odd/problem→problematic）。
5. dev:mock：`plugins/mock-fixtures.ts` 增加 `/aiops/analyses`、`/aiops/analyses/{id}`、`/aiops/alerts` 路由；新增 `aiops-analyses.json`、`aiops-analysis-ana-20260821-0001.json`、`aiops-alerts.json`（meta.warnings 标注契约演示，实体名与 overview fixtures 对齐）。
6. 顺带修复：ClusterBubbleField 两处既有依赖数组 bug（podAliasById 自引用依赖、podItemsById 缺 podAliasById 依赖），lint warning 归零。

## 验证

- `npm run check`（lint 0 warning + build + verify:state）全绿。
- dev:mock 实测：`/api/v1/aiops/analyses`（3 条）、`?segmentId=`（completed + 6 实体）、`/aiops/alerts`（2 条）、`/overview` 均返回正确。
- `make docs-sync` / `make docs-check` 通过；DATA_FLOW/FRONTEND_ARCHITECTURE/PAGE_STRUCTURE 同步（MAP 门禁）。

## 回滚

- 未推送：`git reset --hard HEAD~1`（仅本分支前端 + 文档，不含其他会话内容）。
- 后端 M2/M3 就绪后：删除 `aiops-*.json` fixtures 与 mock 路由即切真实链路，组件逻辑不变。

## 未验证/待办

- 真实后端联调：AIOPS_ENABLED 未开启（本机无 LLM Key），analyses 真实数据与气泡分级待 #93 提交后验证。
- M2 意图执行（#94）：CommandInput 换真实解析→确认→GATE→执行链路。
- M3 时间聚合与警戒（#95）：AlertList 接真实 /aiops/alerts。
