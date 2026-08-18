# 2026-08-18 WSL 回环 TCP：新监听端口首连被拒（localhost 转发中继降级）

> 日期：2026-08-18 ｜ 触发者：本地 Agent（用户要求核实环境遗留断言）｜ 相关：change-history/2026-08-18-issue51-segment-lifecycle/（遗留项定因）｜ promoted: lessons/process-wsl-loopback-fresh-listen-refused.md

## 触发

用户要求核实 change-history 中"WSL 回环 TCP 当前整体不可用（同进程 listen→dial 被拒）"的断言（GrafanaProxy 两个测试本地失败、CI 正常）。

## 调查过程与证据（2026-08-18 16:00-16:40 CST）

1. **不是整体不可用，是特定模式**：新写的探针（/tmp/nd_*.go）全部通过，但原始 `/tmp/probe2.go` 稳定失败 → 模式 = IPv4 回环 + 新监听端口 + 立即连接。
2. **语言无关**：Go 立即 dial → refused；Python（socket 模块）立即 connect → `Errno 111`；延迟 300ms 后两者都 OK。
3. **地址对照**（每次新建端口立即拨号 ×10）：
   - `127.0.0.1` → 0/10 成功（connection refused）；
   - `127.0.0.2` → 0/10（i/o timeout，SYN 被丢弃）；
   - `192.168.10.227`（eth0）→ 10/10；`169.254.73.153`（loopback0 vNIC）→ 10/10；`[::1]` → 成功。
4. **内核已注册但 SYN 被拒**：listen 后立即查 `/proc/net/tcp`，端口已是 `0A`（LISTEN），但同一时刻 connect 仍被 RST → 拦截在 netfilter/内核 hook 层。
5. **间歇性而非固定窗口**：单监听端口时间线 t=0 失败、t≥100ms 起全 OK；但"每轮新建端口"的 sweep 在 sleep 300ms 后仍偶发失败（50/100/300ms 均出现过失败）→ 中继注册队列拥塞型抖动。
6. **端口级注册**：同一端口"首连成功"一次后，关闭并重新 listen，再立即连接成功。
7. **排除项**：无 raw socket（`ss -w` 空）、无 eBPF 可见、iptables/nft 对 127.0.0.0/8 无拦截（Docker nat 规则 `ip daddr != 127.0.0.0/8` 明确排除）；Relay 进程 fd 数在失败期间不变。
8. **中继降级证据**：dmesg 自 2026-08-18 ~11:38（boot 后 43695s）起持续 `WSL (Relay) ERROR: UtilAcceptVsock: Waiting for abnormally long accept(11)`，每 ~10s 一条，6+ 个 `/init` 子进程 `Relay`（如 833511/1283916/1428884/1428979/1429041/1437624）轮换，累计 1600+ 条；内核 `6.18.33.1-microsoft-standard-WSL2`，WSL NAT 模式。
9. **用户侧不受影响**：Windows→WSL localhost 转发正常（`curl.exe http://127.0.0.1:8080/` 与 `:18080/` 均 HTTP 200），Dashboard 访问不受影响。

## 结论

WSL2 内部 localhost 转发中继（guest 侧 `Relay`，/init 子进程）降级，导致新 IPv4 回环监听端口的首连被 RST（127.0.0.1）或丢弃（127.0.0.2）。业务代码无错；CI 无此问题。修复需重置 WSL 网络栈（`wsl --shutdown` 或整机重启），须用户同意（影响运行中发行版与 Docker Desktop 内置 K8s）。

## 处理

- 不改业务代码、不改测试逻辑；两个 Grafana 测试继续按环境性跳过。
- 沉淀本条目 + `lessons/process-wsl-loopback-fresh-listen-refused.md`。
- 复测时机：整机重启或 `wsl --shutdown` 后，重跑 `go run /tmp/probe2.go` 与 `cd dashboard/backend && go test ./internal/api/ -run TestGrafanaProxy -v`；若中继错误计数归零且 probe2 通过，即确认修复。
