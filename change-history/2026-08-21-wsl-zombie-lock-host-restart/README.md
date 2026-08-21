# WSL/Docker 僵尸进程锁 vhdx 识别 + 系统服务卡死处置边界（崩溃重启清场）

> 日期：2026-08-21 ｜ 关联：docs/lessons/deploy-wsl-zombie-vhdx-lock.md

## 为什么做

- 执行「WSL 彻底重启根治」时，`wsl --shutdown` 后 wslservice 僵尸化（所有 wsl 命令挂死），提权重启后 Docker Desktop 引擎仍起不来：docker-desktop 发行版挂载 `ext4.vhdx` 报 `ERROR_SHARING_VIOLATION`。
- 追查确认孤儿 VM 链锁死 vhdx：昨日的 `vmwp.exe` + `vmmemWSL` 常驻且杀不掉，其宿主服务 `vmcompute` 卡在 StopPending；处置中强杀 `vmcompute` 主进程导致电脑崩溃重启。

## 改成什么

1. 沉淀 lesson：`docs/lessons/deploy-wsl-zombie-vhdx-lock.md`（现象 / 根因 / 正确处置 / 可杀与不可杀边界）。
2. 环境恢复：崩溃重启后僵尸进程与 vhdx 锁被系统物理清掉，WSL/Docker/集群待自愈验证（见本条目验证节，状态检查随任务完成）。

## 关键行为

- 本条目为宿主层（Windows/WSL/Docker）教训沉淀，未触碰仓库代码、集群清单与运行时。
- 不改变任何配置；lesson 仅记录识别方法与处置边界。

## 验证

- 崩溃重启后 `wsl -l -v` 正常返回（不再挂死），两个 distro 干净 Stopped。
- Docker Desktop 引擎在 vhdx 锁释放后正常拉起。
- 本条目与 lesson 通过 `make docs-sync` / `make docs-check`。

## 回滚

- 删除 lesson 与本条目即可，无运行时影响；环境侧状态与仓库无关。