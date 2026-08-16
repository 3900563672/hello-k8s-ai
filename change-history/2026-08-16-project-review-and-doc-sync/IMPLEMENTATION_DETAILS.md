# 实现修改明细

## 1. 改动前状态

- `project-review/`（2026-08-13 深度审查的 10 条记录 + README）被 `.gitignore` 第 11 行 `/project-review/` 忽略，从未进入 Git，GitHub 上不可见。
- 根 `README.md` 部署后访问表仍是 4 个独立地址（Dashboard 8080 / Grafana 3000 / Prometheus 9090 / Jaeger 16686），与 2026-08-16 可观测单入口改造（Fixes #14，Dashboard「监控面板」「数据回显」页内嵌）不一致。
- `docs/agents/` 工作流只有单任务闭环，没有"看板 + 批量任务"的接手指引；文档同步规则是"人类文档默认不改"，没有区分"访问方式/架构描述过期必须修"。

## 2. 修改内容

### .gitignore 与 project-review/

- 删除 `/project-review/` 一行，保留用户其他未提交规则（`/AI_AUDIT_PACKAGE/`、`A.sh`、移除 `/change-history/` 忽略）。
- `project-review/` 全部文件进入版本控制：README + issue-01～10。
- `project-review/README.md` 已含"与 GitHub Issue / 看板的关联（2026-08-16 更新）"小节。

### README.md

- 访问表改为单入口：`Dashboard（唯一入口） http://localhost:8080`。
- 增加说明：Grafana 监控面板在 Dashboard「监控面板」页内嵌，Prometheus 与 Jaeger 数据在「数据回显」页展示，不再单独暴露端口。

### AGENTS.md

- 开始前第 5 步：涉及 GitHub Issue / Project 看板 / 批量任务时，先读 `docs/agents/PROJECT_REVIEW.md`。
- 文档维护边界：README 与 `docs/` 中访问方式、架构、行为描述因改动过期时，必须同步更新并纳入本次提交（文档漂移检查是强制步骤）。

### docs/agents/WORKFLOW.md

- 归档与同步节：人类文档默认只在用户明确要求时代笔，但 README / `docs/` 描述过期必须修；交付前做文档漂移检查。

### docs/agents/SYNC.md

- 触发条件新增"发现文档漂移（README 或 docs/ 描述与当前行为不一致）"。
- 同步步骤 5 强化：能直接修的过期文档直接修并纳入本次提交；其余列待同步清单。

### docs/agents/PROJECT_REVIEW.md

- 新增第 0 节"新 Agent 接手指引（先读）"：6 步快速进入状态，明确只动 `Approved`、每批 10 个、`Fixes #N` 闭环、归档与文档核对。

## 3. 未做

- 未创建第一批 issue / 看板卡片（下一步单独执行）。
- 未开始自动化执行 Project Review 条目。
