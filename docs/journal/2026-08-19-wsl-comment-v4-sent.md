> 日期：2026-08-19 ｜ 触发者：本地 Agent ｜ 相关：change-history/2026-08-19-wsl-comment-v4-sent/

## 现象
- `gh api -f body=@file` 未展开文件内容，首次 PATCH 把官方评论正文写成了字面路径 "@/mnt/..."（len=60）。立即用 `--input` + jq `--rawfile` 修复，回读校验通过，无残留。
- PowerShell 双引号内 `\$b` 被 PS 当作变量展开为空，jq 脚本报 INVALID_CHARACTER（与 lessons/process-wsl-powershell-quoting.md 同源：含 `$` 的命令一律脚本文件执行）。

## 上下文
- 正式发送 WSL #41286 follow-up 评论 v4（编辑不新增），演练 issue #68 已渲染通过并删除。

## 处理
- 已解决：评论最终正文与定稿完全一致（10163 字符）。教训沉淀 docs/lessons/process-gh-api-file-body.md。
