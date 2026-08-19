# gh api 传文件正文必须用 --input + jq，不要用 -f body=@file

> 提升日期：2026-08-19 ｜ 来源：WSL 回环评论修订过程 ｜ 适用对象：本地 Agent
> 触发条件（Use when）：用 gh api 传文件正文（POST / PATCH body）或 GitHub 写操作后回读校验时

## 现象

`gh api -X PATCH repos/.../comments/<id> -f body=@file.md` 没有读取文件，而是把字面字符串 `@/path/file.md` 写进了 GitHub 评论正文（回读 len=60）。

## 根因

gh api 的 `-f/-F` 不展开 `@file` 语法（那不是 gh api 的文件读取方式），值被原样作为字符串提交。

## 可复用规则

- 传文件正文：`jq -n --rawfile b /path/file '{body:$b}' > /tmp/payload.json && gh api -X PATCH <endpoint> --input /tmp/payload.json`。
- 任何对 GitHub 的写操作后必须回读校验（长度、首尾、乱码、无本地路径）。
- 含 `$` 的 shell 命令在 PowerShell→wsl 环境一律写成脚本文件执行（见 process-wsl-powershell-quoting.md）。

## 验证方法

PATCH 后 `gh api <endpoint> --jq .body | diff - <(cat /path/file)`。
