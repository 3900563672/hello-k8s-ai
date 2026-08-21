# WSL/Docker 僵尸进程锁 vhdx：识别方法 + 系统服务卡死时的处置边界

> 提升日期：2026-08-21 ｜ 来源：WSL 彻底重启根治期间宿主机崩溃重启 ｜ 适用对象：本地 Agent
> 触发条件（Use when）：`wsl` 命令挂死 / `wsl --shutdown` 后服务卡死 / Docker Desktop 起不来（`ERROR_SHARING_VIOLATION`）/ 服务停在 StopPending 时

## 现象

- `wsl --shutdown` 后 `wslservice` 进入僵尸状态：所有 `wsl` 命令无限挂起，服务接口无响应。
- 重启 `wslservice` 后 Docker Desktop 引擎仍起不来：docker-desktop 发行版挂载 `ext4.vhdx` 报 `ERROR_SHARING_VIOLATION`（文件被占用）。
- 追查发现孤儿 VM 进程锁着 vhdx：`vmwp.exe`（昨日的 Docker 引擎 VM）+ `vmmemWSL` 常驻，`taskkill /F` 杀不掉（报"没有实例在运行"/拒绝访问）。
- 连管它们的 Hyper-V 宿主服务 `vmcompute` 也卡死在停止中（StopPending），形成「vmcompute → vmwp → vmmemWSL → vhdx 锁」僵尸链。

## 根因

- Docker Desktop 的 VM（vmwp/vmmemWSL 由 vmcompute 托管）异常退出后成为孤儿，进程未回收且持续占用 `ext4.vhdx`；WSL 服务重启无法释放该句柄。
- `vmwp`/`vmmemWSL` 是 Hyper-V 虚拟机进程，受 vmcompute 服务保护，`taskkill /F` 无法直接终止（表面"进程不存在"但句柄仍存活）。
- 系统级服务（wslservice / vmcompute）卡在 StopPending 时，强杀其主进程会破坏 Windows 虚拟化服务链 → 可能导致系统崩溃（本次实测：强杀 vmcompute 后电脑直接崩溃重启）。

## 正确处置

1. **识别**：Docker Desktop 起不来时先查占用：`Get-Process vmwp,vmmemWSL` 看是否存在孤儿进程；vhdx 被占用可用 `openfiles` 或 Sysinternals handle 确认。
2. **先停手，走正常重启**：系统服务卡死（StopPending）时，正确路径是「保存工作 → 正常重启 Windows」。重启会物理清掉僵尸进程与 vhdx 锁，这是最干净、最快的根治（本次实测：崩溃/重启后环境恢复，锁全部释放）。
3. 若不想重启：只能尝试 `net stop vmcompute` 优雅停止（不要强杀进程）；无效就放弃，绝不 `taskkill /F` 系统级服务主进程。
4. 重启后按规避顺序拉起：先 Docker Desktop（引擎），再 WSL distro；`make doctor` 自检。

## 边界（什么能杀、什么不能杀）

- 可杀：应用层残留进程（node/vite/port-forward）、wslservice 卡死时提权重启该服务（仅服务自身，不影响 VM 链）。
- 不可杀：`vmcompute.exe`（Hyper-V 主机计算服务）主进程、`vmwp.exe`（VM 工作进程）——强杀会破坏虚拟化服务链，高风险，本次已实测踩坑。
- 判断标准：目标进程是否被系统服务托管（Services 会话）；被托管的一律走服务停止或重启系统，不直接杀进程。
