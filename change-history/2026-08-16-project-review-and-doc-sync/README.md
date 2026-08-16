# Project Review 纳入版本控制与文档同步机制强化

- 变更日期：2026-08-16
- 关联问题：无（用户直接要求的工程基建与协作机制优化）
- 变更级别：P1 工程基建
- 变更范围：`.gitignore`、`project-review/`（纳入版本控制）、根 `README.md`、`AGENTS.md`、`docs/agents/`（WORKFLOW / SYNC / PROJECT_REVIEW）
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

- `project-review/` 从 `.gitignore` 移除，10 条深度审查记录（issue-01～10）与 README 进入版本控制，GitHub 与本地保持一致。
- 修复 README 文档漂移：部署后访问表从"Dashboard / Grafana / Prometheus / Jaeger 四个独立地址"收敛为 Dashboard 单入口，与可观测单入口改造（change-history/2026-08-16-observability-single-entry）对齐。
- 工作流强化（写进提示词）：`AGENTS.md` 开始前新增第 5 步（涉及 Project 看板先读 PROJECT_REVIEW.md）；`WORKFLOW.md` 与 `SYNC.md` 增加"文档漂移检查"硬约束——本次改动导致 README / `docs/` 描述过期时必须同步更新并纳入本次提交，禁止只归档不改文档。
- `docs/agents/PROJECT_REVIEW.md` 新增"新 Agent 接手指引"：接手 10 分钟内进入状态的 6 步（读文档 → 查看板 → 只动 Approved → issue 闭环 → 归档与文档核对 → 拿不准问用户）。

## 2. 关键行为

- 任何 Agent 接手时，`AGENTS.md` 强制指向 `docs/agents/PROJECT_REVIEW.md`，看板状态机成为任务范围的默认来源。
- 交付时 README 与 `docs/` 描述过期 = 必须修，不是可选项。
- `project-review/` 现在是仓库资产：issue 正文可引用 `project-review/issue-NN-*.md`，GitHub 上可直接点击。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| .gitignore | 移除 `/project-review/` 忽略规则 |
| project-review/ | 10 条审查记录 + README 首次进入版本控制 |
| README.md | 部署后访问表收敛为 Dashboard 单入口 |
| AGENTS.md | 开始前新增看板入口；文档维护边界强化 |
| docs/agents/ | WORKFLOW / SYNC 增加文档漂移硬约束；PROJECT_REVIEW 新增接手指引 |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `make docs-check` 通过；docs-only 提交触发文档检查 workflow。
- 停止线：本次只做机制与文档；第一批 issue 与看板卡片由后续步骤创建（状态 To do），不开始自动化执行。
