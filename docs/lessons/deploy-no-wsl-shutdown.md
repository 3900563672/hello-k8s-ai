# 禁止 wsl --shutdown：会关闭所有发行版

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-command-and-terminal.md ｜ 适用对象：本地 Agent

## 现象

想"重置 WSL 网络"执行 `wsl --shutdown`，结果用户的 Ubuntu（Agent 与项目所在）也被关闭，长时任务中断。

## 根因

`wsl --shutdown` 关闭全部发行版与 docker-desktop，不是单发行版操作。

## 可复用规则

- 重启 WSL 只针对目标发行版：`wsl -t <发行版名>`；docker-desktop 引擎单独在 Docker Desktop GUI 重启。
- 任何涉及 WSL 整体重启的操作先征求用户同意（用户可能有 Agent/项目在跑）。

## 验证方法

操作后 `wsl -l -v` 确认无关发行版仍 Running。
