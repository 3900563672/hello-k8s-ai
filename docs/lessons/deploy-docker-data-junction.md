# Docker Desktop WSL2 数据盘迁移必须用 Junction，DataFolder 配置无效

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-command-and-terminal.md ｜ 适用对象：本地 Agent

## 现象

`settings-store.json` 设置 `DataFolder: D:\DockerData` 后重启，`docker ps` / `docker volume ls` 全部为 0（数据没丢，只是没挂载）。

## 根因

Docker Desktop 的 WSL2 引擎硬编码在 `%LOCALAPPDATA%\Docker\wsl\disk` 找数据盘，忽略 DataFolder；目标目录没有它认识的 vhdx 时会在默认位置静默建新空盘。

## 可复用规则

- 迁移数据盘 = 移动 `docker_data.vhdx` 到目标盘 + 在默认位置建**目录联接（Junction）**指向它。
- 验证：`dir C:\Users\hh\AppData\Local\Docker\wsl\disk` 能看到 vhdx 且 `LinkType=Junction`。
- D:\ 根目录默认不可写：先提权创建目录并授权（`icacls`），非提权进程写不了。
- 迁移后若容器/卷显示 0：先检查 Junction 与 vhdx 是否存在，不要贸然重置 Docker。

## 验证方法

`docker ps` / `docker volume ls` 恢复迁移前的容器与卷列表；`Get-Item <junction> | Select LinkType` 为 Junction。
