# 一次真实环境故障排查：WSL 回环新端口首连被拒

> 维护层：human | last-reviewed：2026-08-18 | 事实源：docs/journal/2026-08-18-wsl-loopback-relay.md、docs/lessons/process-wsl-loopback-fresh-listen-refused.md

## 一、背景与影响

本机开发环境是 Windows + WSL2（Ubuntu，内核 6.18.33.1-microsoft-standard-WSL2，NAT 模式）。某次改动交付后，本地 Go 测试出现两个 Grafana 反向代理测试偶发失败（`TestGrafanaProxyPreservesSubPathAndForwards` / `TestGrafanaProxyRootPath`），而 CI（GitHub Actions）全绿。

最初只把它当作"本地偶发抖动"记入变更归档。用户要求核实"WSL 回环 TCP 当前整体不可用"这个断言是否成立——这触发了一次完整排查。最终结论：**业务代码无错，问题出在 WSL2 的 localhost 转发中继组件降级，修复手段只有重置 WSL 网络栈**。

这个案例的价值不在"一个环境 bug"，而在排查方法论：如何把一个"偶发、语言无关、只在特定地址族出现"的问题，一步步收敛到内核与系统组件层面，并拿出证据证明业务代码无错。

## 二、现象

- 新写的探针（`/tmp/nd_*.go`）全部通过，但更早的探针（`/tmp/probe2.go`）稳定失败——看似矛盾，实际暴露了一个特定模式。
- **模式**：IPv4 回环（`127.0.0.1`）+ 新监听端口 + 创建后立即连接 → 间歇性 `connection refused`（Go 的 `dial tcp: connect: connection refused`，Python 的 `Errno 111`）。
- 失败窗口约 0–300ms，但**不是固定窗口**：`sleep 300ms` 后重试仍可能失败（中继注册队列拥塞型抖动）。
- 只有 `127.0.0.0/8` 受影响：

| 地址 | 结果（新端口立即拨号 ×10） |
| --- | --- |
| `127.0.0.1` | 0/10 成功（connection refused，SYN 被 RST） |
| `127.0.0.2` | 0/10 成功（i/o timeout，SYN 被丢弃） |
| `192.168.10.227`（eth0） | 10/10 成功 |
| `169.254.73.153`（loopback0 vNIC） | 10/10 成功 |
| `[::1]`（IPv6 回环） | 全部成功 |

- 长存活端口（8080 Dashboard、18080 脚本端口）稳定可达；Windows→WSL 的 localhost 转发（`curl.exe`）正常，**日常使用不受影响**。
- 同一个端口只要"首连成功过"一次，之后关闭再 listen、再立即连接也成功（端口级注册）。

## 三、排查方法论（重点）

### 第 1 步：先复现，再归因

不基于"感觉"下结论，先写最小探针把失败率量化：

```bash
# Go 探针：新建 127.0.0.1 随机端口，立即拨号一次
go run /tmp/nd_addr.go 127.0.0.1:0 127.0.0.1 10
```

再用 Python（socket 模块）做同样的事，确认**语言无关**。如果只有 Go 失败，可能是 Go 运行时的坑；Go/Python 一致失败，说明问题在网络栈层面。

### 第 2 步：对照组设计（关键实验）

同一段代码、同一时机，只换监听/连接地址：

- 换 `eth0` 地址 → 成功
- 换 IPv6 `[::1]` → 成功
- 换 `127.0.0.2`（同属 127/8）→ 失败但表现不同（丢弃 vs RST）

结论立刻收窄：**问题被限定在 IPv4 回环地址族**，且 `127.0.0.1` 与 `127.0.0.2` 行为不同说明拦截点在 127/8 网段的入口处理，而非通用 TCP 栈。

### 第 3 步：排除法

- 无 raw socket（`ss -w` 空）、无 eBPF 可见 → 排除用户态进程劫持。
- iptables/nft 对 `127.0.0.0/8` 无拦截规则（Docker 的 nat 规则明确 `ip daddr != 127.0.0.0/8` 排除它）→ 排除本机防火墙规则。
- Relay 进程 fd 数在失败期间不变 → 排除句柄耗尽。

### 第 4 步：证据收敛

- `listen` 后立即查 `/proc/net/tcp`，端口已是 `0A`（LISTEN），但同一时刻 connect 仍被 RST → **内核已注册监听，拦截发生在 netfilter/内核 hook 层**。
- `dmesg` 时间线：自 boot 后 43695s（2026-08-18 11:38 左右）起持续输出 `WSL (Relay) ERROR: UtilAcceptVsock: Waiting for abnormally long accept(11)`，每 ~10s 一条，6+ 个 `/init` 子进程 `Relay`（PID 轮换）轮流报错，累计 1600+ 条。
- 错误时间起点与现象出现时间吻合 → 锁定 WSL 内部 localhost 转发中继组件降级。

### 第 5 步：区分环境问题与代码问题

- CI（干净 Linux 容器）全绿 → 不是业务代码。
- 业务长存活端口、Windows→WSL 转发全部正常 → 不是配置错误，是特定路径降级。
- 官方 issue（microsoft/WSL#12837）症状一致（localhost 失效但 eth IP 正常、`UtilAcceptVsock` 报错、`wsl --shutdown` 恢复）→ 属于已知问题类别，不是本项目引入。

## 四、结论与修复

- **根因**：WSL2 内部 localhost 转发中继（guest 侧 `Relay`，`/init` 子进程）降级，新 IPv4 回环监听端口的首连被 RST（`127.0.0.1`）或丢弃（`127.0.0.2`）。
- **修复**：`wsl --shutdown` 或整机重启（会中断所有发行版与 Docker Desktop 内置 Kubernetes，需要用户在场确认）。本次完成的是**定位 + 规避**，没有、也无法在业务侧"修好"微软组件。
- **对项目的影响**：两个 Grafana 测试按环境性跳过，其余全部正常；已在变更归档中把"整体不可用"修正为精确结论。

## 五、规避与工程经验

- 本地脚本/测试连 `127.0.0.1` 新端口时，对首个连接做 ≥100ms 重试，或先自连一次完成"端口注册"；连长存活端口（kubectl port-forward 等）无需处理。
- 遇到"刚 listen 就 refused"先按环境问题排查，**不要改业务代码、不要误判为 Go/Python 差异**。
- 坑的终点是自动化：沉淀为探针 `hack/wsl-loopback-probe`（每次新建端口立即拨号 ×10 + dmesg 中继错误计数），已接入 `make preflight` 与 `make selfcheck`，非 WSL 环境自动 SKIP。
- 环境断言必须可复现：把"整体不可用"这种模糊描述，改写成可执行的探针路径，避免下次重复踩坑。

## 六、附录


### 探针参数与自动接入

- 运行：`go run ./hack/wsl-loopback-probe`，默认新建端口立即拨号 10 次并统计失败率；`-attempts N` 可调整次数。
- 结果分级：`PASS`（首连正常）/ `WARN`（间歇失败或 dmesg 存在历史错误）/ `FAIL`（全部失败）/ `SKIP`（非 WSL 环境）。
- 自动接入：`make preflight` 第 9 节与 `make selfcheck` 会自动运行探针；探针只输出结果、不设置退出码，由调用方决定门禁（preflight 中 FAIL 阻止启动，WARN 仅提示）。
### 官方依据

- WSL 官方技术文档 [localhost 转发说明](https://wsl.dev/technical-documentation/localhost/)：NAT 模式下 WSL 会监听绑定的 TCP 端口并通过 `wslrelay.exe` 转发到 Windows；guest 侧对应 `localhost.cpp` 实现的 `Relay` 进程。
- [microsoft/WSL#12837](https://github.com/microsoft/WSL/issues/12837)：症状与本例一致（localhost 失效但 eth IP 正常、`UtilAcceptVsock` 报错、`wsl --shutdown` 恢复）。

### 复现与验证命令

```bash
go run ./hack/wsl-loopback-probe          # 探针：10 次新端口立即拨号 + dmesg 中继错误计数
dmesg | grep UtilAcceptVsock | wc -l      # 中继错误计数；持续增长 = 降级中
cd dashboard/backend && go test ./internal/api/ -run TestGrafanaProxy -v
```

### 关联记录

- 流水账（完整证据链）：[journal/2026-08-18-wsl-loopback-relay.md](../journal/2026-08-18-wsl-loopback-relay.md)
- 蒸馏规则（Agent 视角）：[lessons/process-wsl-loopback-fresh-listen-refused.md](../lessons/process-wsl-loopback-fresh-listen-refused.md)
- 变更归档：[change-history/2026-08-18-wsl-loopback-case-study/](../../change-history/2026-08-18-wsl-loopback-case-study/README.md)
- 排障入口：[TROUBLESHOOTING.md](TROUBLESHOOTING.md)