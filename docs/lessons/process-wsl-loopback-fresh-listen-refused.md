# WSL2 新监听端口首连被拒：IPv4 回环 localhost 转发中继的两级形态

> 提升日期：2026-08-18（晚更新：修正错误码表述、推翻旧探针、补充健康态瞬态）｜ 来源：journal/2026-08-18-wsl-loopback-relay.md ｜ 适用对象：本地 Agent / 远程 AI

## 现象（两种形态，务必区分）

**严重形态（原始故障，小时级）**：
- WSL Ubuntu 内 `127.0.0.1` 上新 `listen` 的端口，创建后立刻连接间歇性 `connection refused`（Go / Python 一致，语言无关）。
- 本地 `TestGrafanaProxyPreservesSubPathAndForwards` / `TestGrafanaProxyRootPath` 失败（httptest server 刚启动就被 proxy dial → 502），CI 正常。
- `127.0.0.2`（同属 127/8）表现为 `i/o timeout`（SYN 被丢弃）而非 RST；`eth0`、IPv6 `::1`、长存活端口（8080/18080）正常。
- dmesg 持续增长：`WSL (Relay) ERROR: UtilAcceptVsock: Waiting for abnormally long accept(11)`。**`accept(11)` 的 11 是 fd 编号，不是 errno 11=EAGAIN**（语义：在 fd 11 上等待 accept 异常久，poll 60s 超时循环）。

**健康态瞬态（wsl --shutdown 后仍可复现，2–5s 自愈）**：
- 同一进程内连续"listen→连接→curl→关闭"多轮：**第 1 轮必过，第 2 轮起新端口注册停滞 >2s**，约 2–5s 后自愈（Go 探针 5/5 命中；第 3 轮部分恢复）。
- 独立进程（间隔 1s）4/4 全过；进程内间隔 3s 依然失败 → **不是时间恢复，是进程内"会话拆除竞争"**。
- 全程 dmesg `UtilAcceptVsock` 计数 = 0 → 与严重形态错误路径无关。

## 根因（收敛结论）

- 监听端口在 `/proc/net/tcp` 中立即可见（LISTEN 状态），SYN 仍被 RST/丢弃 → 拦截发生在 netfilter/内核 hook 层，不是"无监听"路径。
- 无 raw socket、无 eBPF、iptables/nft 对 127.0.0.0/8 无拦截规则（Docker 的 nat 规则明确排除 127/8）。
- WSL2 localhost 转发中继（guest 侧 `Relay`，/init 子进程）存在两个层次的问题：严重形态（中继降级，dmesg 报错、Windows 侧新端口不可达）与健康态瞬态（端口 churn 时注册停滞，无错误日志）。
- 修复 = 重置 WSL 网络栈（`wsl --shutdown` 或整机重启），业务代码无错。

## 踩坑教训（2026-08-18 晚修正）

- **"listen 后立即拨号 ×10"的探针在健康态也必然 FAIL**：健康态新端口注册本身需 50–100ms，t=0 立即拨号 10/10 必失败 → 旧探针的客侧读数（0/10 等）全部作废。**探针必须先做健康对照（H1），再下故障结论**。
- **vsock 连通性探针无效**：对未监听端口 vsock connect 超时是正常行为，不能当作"vsock 失效"证据；正确做法是连已知存活端口（如 50000/50001/50002/50005）。
- **可复现的瞬态不等于持久故障**：区分"进程内连续 churn 的瞬态"与"跨进程、小时级的严重故障"，两者证据集不同（前者看进程内时序，后者看 dmesg + Windows 侧 netstat/curl）。

## 可复用规则

- 本地脚本/测试连 `127.0.0.1` 新端口时，对首个连接做 ≥100ms 的重试（或先自连一次完成"注册"）；连长存活端口（kubectl port-forward 等）无需处理。
- 遇到"刚 listen 就 refused"先按本环境问题排查，不要改业务代码、不要误判为 Go/Python 差异。
- 根因修复 = `wsl --shutdown` 或整机重启；执行前必须征得用户同意（会中断所有发行版与 Docker Desktop K8s，需重新启用内置 K8s）。
- 探针语义：`hack/wsl-loopback-probe` 默认单轮（测量首次成功时延 + Windows 侧 curl 校验 + dmesg 计数），健康态应 PASS；`-attempts 3 -delay 0` 仅作诊断/压力复现（稳定触发 WARN 1/3），**不要用多轮结果判定环境故障**。

## 验证方法（2026-08-18 晚更新）

```bash
go run ./hack/wsl-loopback-probe        # 默认单轮：健康态 PASS；-attempts 3 -delay 0 复现瞬态 WARN
dmesg | grep UtilAcceptVsock | wc -l    # 严重形态下持续增长；健康态应为 0 或恒定
cd dashboard/backend && go test ./internal/api/ -run TestGrafanaProxy -v
```

完整调查记录（探针、证据、对照实验）见 `docs/journal/2026-08-18-wsl-loopback-relay.md`；人类可读案例见 `docs/operations/WSL_LOOPBACK_CASE_STUDY.md`。
