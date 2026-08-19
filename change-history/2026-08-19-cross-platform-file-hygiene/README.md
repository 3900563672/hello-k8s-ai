# 跨平台文件卫生沉淀：UTF-8 BOM / Git 执行位噪音 规则与修复

> 日期：2026-08-19 ｜ 关联：docs/lessons/process-cross-platform-file-hygiene.md、docs/agents/WORKFLOW.md §5

## 为什么做

- 仓库体检（docs-hygiene 分支）用 PowerShell 改写 `TROUBLESHOOTING.md` / `change-history/README.md` 时引入 UTF-8 BOM，导致 `make docs-sync-check` CI 失败（llms.txt 标题回退成文件名）。
- 同一时段，Windows Git 经 UNC 访问仓库产生 251 个文件的幽灵执行位差异（100755→100644），一旦 `git add` 会整体污染提交。

## 改成什么

1. 修复：剥离 `change-history/README.md` 与 `docs/operations/TROUBLESHOOTING.md` 的 BOM；`git config core.fileMode false` + WSL 内恢复 251 个文件执行位；docs-hygiene 分支补 CI 修复提交（docs-sync 一致性恢复）。
2. 沉淀：新增 lesson「跨平台文件写入卫生」（BOM + fileMode 双通道）；WORKFLOW §5 增加文件写入卫生规则（WSL Python 无 BOM 写入、fileMode 噪音处理）。

## 关键行为

- 全部为仓库卫生改动，未触碰集群、直方图实验与运行时组件。
- `core.fileMode=false` 是仓库本地配置，不进提交；执行位保留在 Git index（100755），此后新 chmod 不再被跟踪。

## 验证

- `make docs-sync-check` 与 `make docs-check` 在 docs-hygiene 分支 CI 全绿（BOM 修复提交后）。
- `git status --short` 干净；`git diff --summary | grep 'mode change'` 无输出。
- 抽查剥离文件 `head -c 3 | xxd` 无 `efbbbf`。

## 回滚

- 删除 lesson 文件与 WORKFLOW 规则行、移除 change-history 条目即可；BOM 剥离与 fileMode 配置为本地状态，无运行时影响。
