# 变更总览：AIOps 启用 + 路由重复注册修复（#116）+ 历史数据安全验证

> 日期：2026-08-21 ｜ 级别：P1

## 背景与决策

- 集群部署已含 AIOps 全功能（#113/#114），但 `AIOPS_ENABLED` 默认 false（安全默认，开启必须提供 API Key）。本次决策：注入用户提供的 Key（Secret `hello-k8s-ai-dashboard-aiops`）并启用。
- 启用后立即暴露合并残留 bug：`/api/v1/aiops/jobs` 被注册两次（#114 注册 handleListAIOpsJobs；#113 合并时残留 handleListAIOpsAnalyses 挂同一 pattern），Go 1.22+ ServeMux 相同 pattern 重复注册直接 panic。此前默认关闭导致该路径从未被 CI/部署执行。

## 实现摘要

- **修复（#116）**：删除重复注册，保留 handleListAIOpsJobs；新增回归测试 `TestHandlerRegistersAIOpsRoutesOnce`（AIOps 启用时完整注册不 panic + jobs 返回 200）。
- **部署**：Secret 注入 openai-api-key；deployment env 加 `AIOPS_ENABLED=true` + key 引用；重建 backend 镜像并导入 5 个 kind 节点后 rollout。
- **验证**：`/api/v1/aiops/settings` → `keyConfigured: true`；`/aiops/jobs`、`/aiops/chat/messages` 均 200；后端 Pod Running。
- **历史数据安全验证**：L1/L2 仅由实验 complete/fail 入队触发，无历史回填；M3 只聚合最近 6 窗口且需窗口内有 analyses 才调 LLM；对话限流 6 次/分钟/会话；worker 单轮批量上限 + MaxAttempts=3 + 调用预算。存量历史数据不会触发批量 LLM 调用。

## 测试与验证

- 后端 `go test ./...` 全绿；新增回归测试覆盖 AIOps 启用路径。
- main CI 4 项全绿（文档检查/代码检查/源码与部署验证/E2E）。
- 环境脚本顺带修复：SimulationClock 无 CR 时跳过收敛检查；干净环境断言对保留 PVC 的历史快照降级为警告。

## 迁移与回滚

- Secret 与 deployment env 为集群本地配置，不落仓库清单；`setup.sh` 重跑会恢复 `AIOPS_ENABLED=false`，需按本条目步骤重新启用（或沉淀为脚本）。
- 回滚：删除 Secret + 移除 env 注入即可恢复默认关闭。
