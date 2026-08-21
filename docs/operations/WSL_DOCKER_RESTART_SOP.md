# WSL/Docker 宿主层安全重启与残留检查 SOP

> 维护层：human ｜ last-reviewed：2026-08-21 ｜ 适用对象：本地 Agent 与人工运维
> 关联：docs/lessons/deploy-wsl-zombie-vhdx-lock.md（现象与根因）｜ 外部查重：microsoft/WSL#11082、docker/for-win#14024 / #14669 / #14827 / #14656（同类已知问题，无需新开 issue）

## 0. 这份 SOP 解决什么

宿主层（Windows + WSL + Docker Desktop）是开发环境的最底层。它出问题时，所有 `wsl` / `docker` / `kubectl` 命令都会连带失败，症状容易被误判成业务故障。

本 SOP 覆盖三类场景：

1. **例行重启**：需要彻底重启 WSL/Docker 时的安全顺序（避免把系统弄进僵尸状态）。
2. **故障识别**：`wsl` 命令挂死、Docker 引擎起不来（`ERROR_SHARING_VIOLATION`）、服务卡在 StopPending 时，如何快速判定问题归属。
3. **处置边界**：什么进程可以杀、什么进程绝对不能强杀（2026-08-21 实测：强杀 vmcompute 导致宿主机崩溃重启）。

## 1. 重启前：现状快照（只读）

动手前先记录基线，5 条命令、全部只读：

```bash
# 1. 三个 distro 的状态（正常应 1 秒内返回）
wsl -l -v

# 2. 运行中的 distro（quiet 模式，无表头，便于脚本解析）
wsl -l --running -q

# 3. 宿主 VM 进程（Docker 引擎在跑时 vmwp/vmmemWSL 各 1 个属正常）
powershell.exe -NoProfile -Command "Get-Process vmwp,vmmemWSL -ErrorAction SilentlyContinue | Select-Object Name,Id,StartTime"

# 4. Docker 引擎可达性
docker info

# 5. 环境自检基线（含第 2.1 节宿主 VM 残留检查）
make doctor
```

快照作用：重启后能立刻对照「恢复成什么样算成功」；同时确认没有长任务（night-run / 长时间实验）在跑——**有未保存结果的长任务时，先等它结束或确认数据已落库，再重启**。

## 2. 故障识别速查表

| 症状 | 判定命令 | 判定标准 | 归属 |
| --- | --- | --- | --- |
| `wsl` 命令无限挂起 | `timeout 8 wsl.exe -l -v` | 8 秒内不返回 / 非零退出 | wslservice 疑似僵尸化 |
| Docker Desktop 报 unable to start | `docker info` | 引擎持续 503 / 连接失败 | Docker 引擎层 |
| 启动引擎报 `ERROR_SHARING_VIOLATION` | `Get-Process vmwp,vmmemWSL` | vmwp 存在且引擎不可达 | vhdx 被孤儿 VM 锁 |
| 服务卡在 StopPending | `sc query wslservice` / `sc query vmcompute` | STATE=STOP_PENDING 超 2 分钟 | 系统服务僵尸化 |
| 进程杀不掉（报没有实例在运行） | `tasklist /svc /fi "IMAGENAME eq vmwp.exe"` | 显示在 Services 会话 | Hyper-V 托管进程，不可强杀 |

`make doctor` 第 2.1 节会自动跑其中三项（vmwp 数量 + vmmemWSL 数量 + wsl.exe 响应性），输出 PASS/FAIL 可直接作为判定依据。

## 3. 标准重启顺序（正常路径）

原则：**先停 Docker（释放它的 VM 和 vhdx 句柄），再停 WSL；启动时反过来，先 Docker 引擎、后 Ubuntu**。顺序颠倒（先 WSL 后 Docker）是 Docker 引擎起不来的高概率诱因。

### Step 0 保存工作

- 确认没有长任务在跑（`make doctor` + 业务侧无进行中的实验）。
- 记录第 1 节快照，尤其是 `wsl -l -v` 与 `make doctor` 输出。

### Step 1 优雅退出 Docker Desktop

- 托盘图标 → Quit Docker Desktop，等 UI 退出。
- 验证引擎与 VM 已停（约 10-30 秒）：
  ```powershell
  Get-Process vmwp,vmmemWSL,com.docker.backend -ErrorAction SilentlyContinue
  ```
- 期望：无输出。若 Docker Desktop UI 无响应，可只强杀 UI 壳（`Stop-Process -Name "Docker Desktop" -Force`），再观察引擎是否自行停止；**不要动 vmwp/vmmemWSL**。

### Step 2 确认无 VM 残留

- 上一步验证失败（vmwp/vmmemWSL 仍在）→ 走第 4.2 节（vhdx 锁分支），**不要继续 `wsl --shutdown` 硬冲**。

### Step 3 `wsl --shutdown`

```bash
wsl --shutdown
```

- 验证：`wsl -l -v` 应显示全部 distro Stopped。
- 若 10 秒内命令不返回，先 `timeout 8 wsl.exe -l -v` 确认 wslservice 状态，再走第 4.1 节。

### Step 4 启动 Docker Desktop 并等引擎就绪

- 从开始菜单 / 快捷方式启动 Docker Desktop。
- 轮询引擎就绪（最多 120 秒，正常 30-90 秒）：
  ```bash
  for i in $(seq 1 24); do docker info >/dev/null 2>&1 && break; sleep 5; done; docker info
  ```
- 90 秒未就绪且报 `ERROR_SHARING_VIOLATION` → 走第 4.2 节。

### Step 5 拉起 Ubuntu

```bash
wsl -d Ubuntu -- exit   # 触发启动并立即退出；或直接开 Ubuntu 终端
wsl -l --running -q     # 应包含 Ubuntu
```

### Step 6 环境自检 + 业务链路

```bash
make doctor     # 应全 PASS（允许个别 WARN）
make preflight  # 集群深度体检：5 节点 Ready、8 工作负载、3 PVC Bound
```

全部通过后再继续业务；任何 FAIL 先解决再开工。

## 4. 异常分支

### 4.1 wslservice 僵尸化（`wsl` 命令挂死）

- 判定：`timeout 8 wsl.exe -l -v` 不返回；`sc query wslservice` 卡死或异常。
- 处置（按顺序尝试，每步验证）：
  1. 提权重启服务：管理员 PowerShell 执行 `Restart-Service wslservice`（会弹 UAC）。注意：这是重启服务自身，允许；强杀 vmcompute/vmwp 这类托管链进程，不允许。
  2. 若卡 StopPending 超 2 分钟：**停手**。保存工作 → 正常重启 Windows（最干净且已被实测验证的根治方式）。
- 验证：重启服务后 `wsl -l -v` 立即返回、distro 显示 Stopped。

### 4.2 vhdx 被孤儿 VM 锁（`ERROR_SHARING_VIOLATION`）

- 判定：Docker Desktop 起不来；`Get-Process vmwp,vmmemWSL` 有进程；`handle.exe ext4.vhdx`（Sysinternals）能看到占用者。
- 根因：Docker 的 VM（vmwp/vmmemWSL，由 vmcompute 托管）异常退出后成孤儿，进程未被回收且持续占用 `ext4.vhdx`；`taskkill /F` 表面无效（报「没有实例在运行」）。
- 处置：
  1. 先试优雅停：管理员 PowerShell `net stop vmcompute`（等它自然停止，不强杀）；成功后 `net start vmcompute` 再启动 Docker Desktop。
  2. 无效 → **保存工作，正常重启 Windows**。重启物理清掉僵尸进程与 vhdx 锁（2026-08-21 实测：崩溃/重启后环境全部恢复）。
  3. 重启后按第 3 节顺序拉起（先 Docker 再 Ubuntu），跑 `make doctor`。

### 4.3 vmcompute 卡 StopPending

- 判定：`sc query vmcompute` 长时间 STOP_PENDING。
- 处置：**不要强杀 vmcompute.exe 主进程**（2026-08-21 实测直接导致系统崩溃）。只有两条路：等它自行完成，或保存工作后重启 Windows。

### 4.4 崩溃/强制重启后的恢复

- 系统重启后僵尸进程与 vhdx 锁会被物理清掉，环境通常直接恢复。
- 恢复检查：`wsl -l -v` 正常返回 → 启动 Docker Desktop → `make doctor` → `make preflight` → 业务验证（Grafana 有历史数据、`/api/v1/replay` 可查）。

## 5. 可杀 / 不可杀边界

| 对象 | 可否强杀 | 理由 |
| --- | --- | --- |
| node / vite / port-forward / kubectl 等应用层残留 | 可 | 普通用户进程，杀错最多重开 |
| 容器（`docker stop`） | 可（优雅优先） | 应用层 |
| `wslservice` 服务本身 | 可（提权重启服务） | 仅服务自身，不涉及 VM 托管链 |
| `vmcompute.exe` 主进程 | **不可** | Hyper-V 主机计算服务，强杀实测导致系统崩溃 |
| `vmwp.exe` / `vmmemWSL` | **不可** | vmcompute 托管，taskkill 无效且强杀破坏虚拟化链 |
| Windows 内核级僵尸句柄 | 不可 | 只能通过重启系统回收 |

判断标准一句话：**目标进程是否被系统服务托管（Session Name = Services）？被托管的一律走服务停止或重启系统，绝不直接杀进程。**

## 6. 外部已知问题对应（查重结论）

本地踩的这条链与开源社区已知问题同源，**无需新开 issue**：

- microsoft/WSL#11082：vmwp.exe 锁 vhdx + `ERROR_SHARING_VIOLATION`（最贴合）
- docker/for-win#14024：WSL 持有 vhdx 句柄
- docker/for-win#14669：vhdx open in another process
- docker/for-win#14827 / #14656：Resource Saver 模式与 WSL 锁死（补充解释）

本 SOP 是把社区已知问题本地化、操作化的结果：识别命令、判定标准、安全顺序与处置边界都在上面。