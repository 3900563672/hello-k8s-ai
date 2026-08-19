# 升级与回滚

## 1. 迁移

- 无迁移需求：纯文档 + 本机工具脚本，不涉及 CRD、数据库、配置或部署清单。
- 上下文包已重新生成（`make context-pack`），远程 AI 下次收包自动包含 `docs/agents/UI_VERIFICATION.md`（打包脚本 `cp -r docs/agents` 覆盖）。

## 2. 回滚

- `git revert` 本提交即可完整回滚；无数据或 Schema 需要回退。
- 回滚后 `hack/ui-check/` 脚本与 UI_VERIFICATION.md 一并消失，坑位清单恢复旧版；不影响任何构建或运行。

## 3. 风险与注意

- 风险极低：新增脚本不在任何 Makefile 目标与 CI 路径内。
- 脚本依赖本机 Windows Chrome 与 WSL 镜像网络；换机器或 Chrome 路径不同时用 `CHROME_PATH` 覆盖（已写进脚本与文档）。
- 面板现状快照会随时间过期：每次"看面板"任务顺手更新 UI_VERIFICATION.md 的状态列，别让它变成僵尸文档。
