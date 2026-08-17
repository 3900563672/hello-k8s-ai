# 中文提交信息必须用 git commit -F 文件方式

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-command-and-terminal.md ｜ 适用对象：本地 Agent

## 现象

`git commit -m "中文..."` 在 PowerShell/WSL 混合环境下提交信息变成乱码或丢失。

## 根因

终端编码（PowerShell UTF-16 / WSL UTF-8）在参数传递时被转换丢失。

## 可复用规则

- 中文提交信息写文件（`git commit -F /tmp/msg.txt`，文件用 UTF-8），不要用 `-m` 直传。
- gh issue 正文同理：`--body-file`。
- 提交信息格式保持仓库风格：`feat:` / `fix:` / `docs:` / `chore:` / `refactor:` + 中文描述 +（`Fixes #N`）。

## 验证方法

提交后 `git log -1 --format=%B` 检查中文完整。
