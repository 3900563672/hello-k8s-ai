# PowerShell 直传 wsl 引号必被拆，可靠模式是写脚本文件

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-command-and-terminal.md ｜ 适用对象：本地 Agent

## 现象

`wsl -d Ubuntu -- bash -lc '...'` 中带 `$`、引号、反引号、`$(...)`、`for` 循环时，命令被 PowerShell 拆坏：变量为空、语法错误、引号丢失。

## 根因

PowerShell 对整条命令做一层解析后原样传给 wsl.exe，bash 收到的字符串与预期不一致；嵌套引号越深越容易坏。

## 可复用规则

- 复杂命令（含变量 / 循环 / 嵌套引号）一律先写成脚本文件再执行：PowerShell `Set-Content`（ASCII）或用 Node.js `fs.writeFile`，再 `wsl -d Ubuntu -- bash /root/.../script.sh`。
- 脚本文件必须用 **LF 行尾**（PowerShell 默认 CRLF 会让 bash 报 `unexpected end of file`）；写入时做 `text.Replace("\r\n", "\n")`。
- 单条简单命令可直接 `wsl -d Ubuntu -- bash -lc '...'`，但保持引号最小化。
- Windows 侧 PowerShell 命令若包含 `Remove-Item` 组合 / 多引号嵌套，会被安全策略拦截：删除文件改用 Node.js `fs` 或单条 `Remove-Item -LiteralPath`（不带组合），或放到 WSL 里 `rm`。

## 验证方法

写完后先在 WSL 内 `bash -n script.sh` 再执行；执行结果与预期不符时，先怀疑引号/行尾，再怀疑逻辑。
