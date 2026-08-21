# 2026-08-21 后端重启：pkill -f 自匹配把自己杀了

> 日期：2026-08-21 ｜ 触发者：本地 Agent ｜ 相关：.runtime/start-backend.sh

## 现象

`pkill -f /tmp/dashboard-server` 后同一命令里启动后端，进程没起来、日志为空、命令退出码 1。

## 上下文

- 用 `bash -lc "pkill -f /tmp/dashboard-server ; setsid nohup bash .runtime/start-backend.sh ... &"` 重启后端。
- pkill -f 按完整命令行匹配，外层 bash -lc 的命令行里本身就含 `/tmp/dashboard-server` 字样 → 匹配到自己 → 把整个外层 shell 杀了，后面的启动自然没执行。

## 处理

- 用 `pkill -f '[d]ashboard-server'`（括号技巧，命令行里不再是完整串）避开自匹配，进程正常启动。
- 教训：pkill -f 前先想「自己这行命令会不会被匹配到」；批量杀用 `pkill -x <进程名>` 更稳。
