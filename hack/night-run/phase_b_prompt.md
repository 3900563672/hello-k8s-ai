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
3. 若 problems.md 不完整（Phase A 会话可能提前中断）：以快照目录 + `keepalive.log` 为准补齐时间线，再判断每个问题的证据是否足够；证据不足的按"存疑"处理，不强行修复。
4. 需要复现时，先确认集群当前状态（`kubectl get pods -A`、`curl http://localhost:8080/api/v1/health/ready`）。

## 决策矩阵

| 问题类型 | 处理方式 |
| --- | --- |
| 文档错字、表述不清 | 直接改，`docs:` 提交，不建 issue |
| 小逻辑缺陷、错误修复（不改 CRD/API/契约） | 直接改，`fix:` 提交，视影响决定是否建 issue |
| CRD/API/数据库结构/字段所有权变化 | 先建 design/bug issue，再实现，提交关联 `Fixes #N` |
| 需要用户决策（UI 风格、验收口径、资源投入） | 不擅自动手，记入遗留清单，最终汇报 |
| 无法复现或证据不足 | 不修，记入"未验证/存疑"，标注需要什么证据 |

## 交付方式：全部走 PR，禁止直接推 main

- Phase B 的所有改动一律通过 PR 交付，**严禁 push main**；合并留给用户早上审阅后决定。
- 每个逻辑问题一个分支/PR：分支名 `fix/<主题>` / `docs/<主题>` / `chore/<主题>`（kebab-case）；问题间有依赖时按依赖顺序创建。
- 一个 PR 流程：`git checkout -b <分支>` → 提交（中文 `git commit -F`）→ `git push origin <分支>` → `gh pr create`（标题带 `fix:`/`docs:` 前缀 + 中文描述；正文写：问题/改动/验证/关联 `Fixes #N`）→ 等 PR 的 CI 绿（30s 轮询），不绿就在同一分支补丁重推。
- **执行环境**：Windows 侧没有 `gh`，所有 git/gh 操作一律在 WSL 内执行（`wsl -d Ubuntu -- bash -lc "..."` 或先 `wsl -d Ubuntu`）；`gh` 已在该 WSL 内认证（account 3900563672）。
- **gh 偶发超时**：`gh` 偶发 `TLS handshake timeout`（代理链路不稳），失败就等 5–8 秒重试，最多 5 次，不要因此怀疑认证。
- PR 数量控制：一般 1–5 个；改动琐碎且同主题时可合并为 1 个 PR（如"夜间运行发现的问题一批"）。
- 创建完 PR 后不点 Merge；最终汇报列出 PR 链接清单。

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