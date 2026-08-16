# Phase B 提示词模板：夜间问题分析与修复

> 用途：Codex 桌面自动化在 04:30 触发时使用本模板生成任务提示词。
> 槽位：窗口 / 输入 / 决策矩阵 / 输出。填好四槽位即可复用。

## 窗口

- 运行日期：2026-08-17（Asia/Shanghai）。
- 执行窗口：04:30 – 09:00。
- 若 `.runtime/night-run/2026-08-17/problems.md` 不存在或不是当天产物，直接结束（空跑），不执行任何动作。

## 输入

1. 先读 `.runtime/night-run/2026-08-17/problems.md`（Phase A 的交接档案），再读 `docs/agents/WORKFLOW.md` 与 `docs/agents/KNOWN_PITFALLS.md`。
2. 对照快照目录 `.runtime/night-run/2026-08-17/snapshots/` 核对指标曲线。
3. 需要复现时，先确认集群当前状态（`kubectl get pods -A`、`curl http://localhost:8080/api/v1/health/ready`）。

## 决策矩阵

| 问题类型 | 处理方式 |
| --- | --- |
| 文档错字、表述不清 | 直接修，`docs:` 提交，不建 issue |
| 小逻辑缺陷、错误修复（不改 CRD/API/契约） | 直接修，`fix:` 提交，视影响决定是否建 issue |
| CRD/API/数据库结构/字段所有权变化 | 先建 design/bug issue，再实现，提交关联 `Fixes #N` |
| 需要用户决策（UI 风格、验收口径、资源投入） | 不擅自动手，记入遗留清单，最终汇报 |
| 无法复现或证据不足 | 不修，记入"未验证/存疑"，标注需要什么证据 |

## 红线

- 不改 UI 视觉（除非数据完全出不来，属于功能错误）；不截图验证。
- 不 `wsl --shutdown`；不强杀 Docker Desktop；不动代理（127.0.0.1:7890）；不重建集群。
- 不删 PVC、不重置数据库；不手改生成文件（CRD bases、role.yaml、zz_generated.deepcopy.go、PROJECT）。
- 改动最小、可回滚；遵循 AGENTS.md 边界（Reconcile 幂等、字段所有权、Telemetry 不阻塞控制面）。
- 修复后必须本地验证（fmt/vet/test/lint 或对应前端检查），验证结果写进汇报。
- 提交按仓库风格：中文 `git commit -F`，docs 与 hack 分开提，`Fixes #N` 关联 issue。
- 推送后等 CI（30s 轮询），失败必须修或回滚；CI 未绿前不得结束。
- 归档：改了什么必须同步 docs 与 `change-history/` 条目（含踩坑记录）。

## 输出

- 每个处理过的问题：结论（修了/不修/存疑）+ 证据 + 涉及文件。
- 提交清单与 CI 结果；未验证范围与真实风险。
- 最终汇报：本次夜间运行解决了哪些问题、遗留什么、下一次夜间运行建议。