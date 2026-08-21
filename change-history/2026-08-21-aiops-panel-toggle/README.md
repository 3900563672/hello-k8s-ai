# 变更总览：AIOps 运行时开关（面板启用/停用分析入队）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- 用户要求面板配置 API Key 后可一键开启/关闭 AIOps：关闭时实验照常运行，但不入队 AI 分析，避免占用 LLM 预算与数据库写放大。
- 同时清空历史业务数据（TRUNCATE 全部 18 张业务表，保留 `schema_migrations`），避免开关开启后旧数据触发大量聚合。

## 改成什么

1. 后端开关：`internal/aiops/Service` 增加 `enabled` 字段（`NewService` 默认 true）；`EnqueueAnalysis` 开头短路（关闭时不入队，实验生命周期不受影响）；`GET /aiops/settings` 返回 `enabled`；`POST /aiops/settings` 接受 `enabled`（指针字段，仅传开关也合法）。
2. 路由守卫：`requireAIOps` 增加 `!Enabled()` 判断，关闭时 `/aiops/*` 返回 404 `AI_OPS_DISABLED`（文案含「面板开关未关闭」）。
3. 前端开关：`AiChatWidget` 设置面板新增「AI 分析开关」switch（暗色风格一致），保存时随 settings 一起提交；`AIOpsSettings` 类型与 `updateAIOpsSettings` payload 增加 `enabled`。
4. 测试：`TestAIOpsJobsDisabledReturns404` 覆盖开关关闭时 jobs 返回 404。
5. 历史数据清理：`TRUNCATE` 全部 18 张业务表（保留 `schema_migrations`），replay 快照/segments/aiops_*/resource_snapshots 归零。

## 验证

- `go build ./...`、`go test ./...` 全绿（含新增开关测试）。
- `npm run check`（oxlint + tsc + vite build + verify:state）全绿。
- `make docs-check`（MAP 门禁）通过：API_DESIGN / BACKEND_ARCHITECTURE / API_EXAMPLES / DATA_FLOW / FRONTEND_ARCHITECTURE / EVENT_FLOW 已同步。

## 回滚

- 未合并：`git reset --hard HEAD~1`。
- 运行时开关回滚：`POST /aiops/settings {"enabled":true}` 即可恢复；重启后端恢复部署级启用态（`AIOPS_ENABLED`）。

## 未验证/待办

- 集群内实测开关（部署新镜像后）：关闭后跑一个实验，确认不入队；开启后恢复入队。
- 历史数据清空后无回填逻辑（L1/L2 仅由实验 complete/fail 触发），无需额外清理。
