# 迁移与回滚：夜间运行修复

## 1. 迁移

无数据迁移、无 CRD 变化、无数据库变化。本条目只涉及 Agent 工具脚本与文档。

## 2. 回滚

- `hack/night-run/snapshot.mjs`：删除补入的 `const sleep` 一行即回滚（不推荐，脚本会回归崩溃）。
- `hack/night-run/phase_a_prompt.md` / `README.md`：纯文档/提示词，回滚无运行时影响；旧启动命令（无 setsid）在交互终端仍可用，仅 Codex exec 环境需要 setsid。
- `docs/agents/KNOWN_PITFALLS.md`：文档回滚无影响。
- 若通过 PR 合并，整体回滚用 `git revert`。

## 3. 风险

- 未实测禁用睡眠后的整夜值守；若用户选择方案 A（powercfg 自动开关），提权脚本需 UAC 弹窗一次，夜间无人值守时弹窗未确认会失败——实施时需考虑（可提前一次性授权或改用计划任务）。
- keepalive PID 122828 当前仍在运行，属首次执行的遗留进程；不影响本修复，用户可在确认系统健康后按 PID 终止。
