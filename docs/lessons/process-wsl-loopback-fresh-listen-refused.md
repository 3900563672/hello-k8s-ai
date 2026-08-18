# WSL2 新监听端口首连被拒：IPv4 回环 SYN 被 localhost 转发中继拦截

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-18-wsl-loopback-relay.md ｜ 适用对象：本地 Agent / 远程 AI

## 现象

- WSL Ubuntu 内 `127.0.0.1` 上新 `listen` 的端口，**创建后立刻连接会间歇性 `connection refused`**（Go / Python 一致，语言无关），失败窗口约 0–300ms 且**不是固定窗口**（`sleep 300ms` 后仍偶发失败）。
- 本地 `TestGrafanaProxyPreservesSubPathAndForwards` / `TestGrafanaProxyRootPath` 失败（httptest server 刚启动就被 proxy dial → 502 / handler 未到达），CI 正常。
- 对照全部正常：`eth0`（192.168.x.x）与 IPv6 `::1` 新端口**立即连接 10/10 成功**；长存活监听（8080/18080）稳定可达；**同一端口注册成功一次后，重新 listen 再立即连接也成功**。
- `127.0.0.2`（同属 127/8）表现为 `i/o timeout`（SYN 被丢弃）而非 RST。

## 根因

- 监听端口在 `/proc/net/tcp` 中立即可见（LISTEN 状态），SYN 仍被 RST/丢弃 → 拦截发生在 netfilter/内核 hook 层，不是"无监听"路径。
- 无 raw socket、无 eBPF、iptables/nft 对 127.0.0.0/8 无拦截规则（Docker 的 nat 规则明确排除 127/8）。
- 时间线吻合 WSL 内部 localhost 转发中继降级：dmesg 自 2026-08-18 ~11:38（boot 后 43695s）起持续 `WSL (Relay) ERROR: UtilAcceptVsock: Waiting for abnormally long accept(11)`，每 10s 一条、多进程（`/init` 子进程 `Relay`）轮换，至检查时 1600+ 条。
- 结论：WSL2 的宿主机/跨发行版 localhost 转发机制在**新端口注册阶段**拦截 IPv4 回环 SYN；中继降级时首连失败，已注册/长存活端口不受影响。修复 = 重置 WSL 网络栈，业务代码无错。

## 可复用规则

- 本地脚本/测试连 `127.0.0.1` 新端口时，对首个连接做 ≥100ms 的重试（或先自连一次完成"注册"）；连长存活端口（kubectl port-forward 等）无需处理。
- 遇到"刚 listen 就 refused"先按本环境问题排查，不要改业务代码、不要误判为 Go/Python 差异。
- 根因修复 = `wsl --shutdown` 或整机重启；执行前必须征得用户同意（会中断所有发行版与 Docker Desktop K8s，需重新启用内置 K8s）。

## 验证方法

```bash
go run /tmp/probe2.go          # 环境故障时稳定复现 connection refused
go run /tmp/nd_timeline.go     # 时间线：t=0 FAIL，t≥100ms 全 OK
go run /tmp/nd_addr.go 127.0.0.1:0 127.0.0.1 10   # 立即拨号 10 次统计失败率
dmesg | grep UtilAcceptVsock | wc -l   # 中继错误计数，持续增长即降级中
```

完整调查记录（探针、证据、对照实验）见 `docs/journal/2026-08-18-wsl-loopback-relay.md`。
