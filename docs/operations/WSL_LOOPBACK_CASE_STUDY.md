# 一次真实环境故障排查：WSL 回环新端口首连被拒（完整研究链）

> 维护层：human | last-reviewed：2026-08-19（32 号：Docker 排除、探针 wincheck 缺陷、Grafana 竞态机制精确化）| 事实源：docs/journal/2026-08-18-wsl-loopback-relay.md、docs/lessons/process-wsl-loopback-fresh-listen-refused.md

## 一、背景与影响

本机开发环境是 Windows + WSL2（Ubuntu，内核 6.18.33.1-microsoft-standard-WSL2，NAT 模式）。某次改动交付后，本地 Go 测试出现两个 Grafana 反向代理测试偶发失败（`TestGrafanaProxyPreservesSubPathAndForwards` / `TestGrafanaProxyRootPath`），而 CI（GitHub Actions）全绿。

最初只把它当作"本地偶发抖动"记入变更归档。用户要求核实"WSL 回环 TCP 当前整体不可用"这个断言是否成立——这触发了一次跨两个阶段的完整研究：**首轮定位**（收敛到 WSL localhost 转发中继）与**恢复验证**（`wsl --shutdown` 后确认恢复、推翻两个旧探针结论、发现健康态可复现瞬态）。

这个案例的价值不在"一个环境 bug"，而在研究方法论：如何把"偶发、语言无关、只在特定地址族出现"的问题收敛到系统组件层面，并**如何用健康对照推翻自己的早期结论**——后者的诚实度比结论本身更稀缺。

## 二、现象（两种形态）

**严重形态（原始故障，小时级）**：
- 模式：IPv4 回环（`127.0.0.1`）+ 新监听端口 + 创建后立即连接 → 间歇性 `connection refused`（Go 的 `dial tcp: connect: connection refused`，Python 的 `Errno 111`）。
- 只有 `127.0.0.0/8` 受影响（下表读数来自旧探针"立即拨号 ×10"，**已被健康对照推翻**——健康态该探针同样 10/10 失败，不能作为故障证据）：

| 地址 | 旧读数（作废） | 有效结论 |
| --- | --- | --- |
| `127.0.0.1` | 0/10（connection refused） | 严重形态下新端口首连被 RST |
| `127.0.0.2` | 0/10（i/o timeout） | 严重形态下 SYN 被丢弃 |
| `192.168.10.227`（eth0） | 10/10 成功 | 不受影响（有效） |
| `169.254.73.153`（loopback0 vNIC） | 10/10 成功 | 不受影响（有效） |
| `[::1]`（IPv6 回环） | 全部成功 | 不受影响（有效） |

- 长存活端口（8080 Dashboard、18080 脚本端口）稳定可达；Windows→WSL 的 localhost 转发正常。
- dmesg 持续增长：`WSL (Relay) ERROR: UtilAcceptVsock: Waiting for abnormally long accept(11)`。**`accept(11)` 的 11 是 fd 编号，不是 errno 11=EAGAIN**（早期文档误写"错误码 11"，已修正）。

**健康态瞬态（wsl --shutdown 恢复后仍可复现，2–5s 自愈）**：
- 同一进程内连续"listen→连接→curl→关闭"多轮：**第 1 轮必过，第 2 轮起新端口注册停滞 >2s**，2–5s 后自愈（Go 探针 5/5 命中；第 3 轮部分恢复）。
- 独立进程（间隔 1s）4/4 全过；进程内间隔 3s 依然失败（3/3）→ 不是时间恢复，是进程内"会话拆除竞争"。
- 全程 dmesg `UtilAcceptVsock` 计数 = 0 → 与严重形态错误路径无关。

## 三、排查方法论（重点）

### 第 1 步：先复现，再归因

写最小探针量化失败率，再用 Python 做同样的事确认语言无关。如果只有 Go 失败可能是运行时问题；Go/Python 一致失败说明在网络栈层面。

### 第 2 步：对照组设计（关键实验）

同一段代码、同一时机，只换监听/连接地址：`eth0` → 成功；`[::1]` → 成功；`127.0.0.2`（同属 127/8）→ 失败但表现不同（丢弃 vs RST）。结论收窄：**问题限定在 IPv4 回环地址族入口处理**。

### 第 3 步：排除法

- 无 raw socket（`ss -w` 空）、无 eBPF 可见 → 排除用户态劫持。
- iptables/nft 对 `127.0.0.0/8` 无拦截（Docker nat 规则明确排除它）→ 排除本机防火墙。
- Relay 进程 fd 数不变 → 排除句柄耗尽。

### 第 4 步：证据收敛

- `listen` 后立即查 `/proc/net/tcp`，端口已是 `0A`（LISTEN），connect 仍被 RST → 拦截发生在 netfilter/内核 hook 层。
- dmesg `UtilAcceptVsock` 时间线起点与现象出现时间吻合 → 锁定 localhost 转发中继组件。
- 官方 issue（microsoft/WSL#12837）症状一致 → 属于已知问题类别。

### 第 5 步：健康对照推翻旧结论（本案例最重要的方法论）

恢复验证（`wsl --shutdown` 后）阶段发现：
- **H1 健康对照**：健康态新端口注册本身需 **50–100ms**，`t=0` 立即拨号 10/10 必失败 → 旧探针"立即拨号 ×10"在健康态必然 FAIL，属**探针设计缺陷**，阶段一客侧读数全部作废。
- **H2 vsock 对照**：扫描宿主端口 1–512 全超时是**正常行为**（未监听端口本就超时）；用已知存活端口（50000/50001/50002/50005）验证 guest→host vsock 通道活着 → 旧"vsock 失效"结论撤销。
- **H3 瞬态复现**：健康态下进程内连续端口 churn，第 2 轮起注册停滞 2–5s 自愈 → 这是独立于严重形态的**可复现瞬态**，可作微软 issue 的 Repro from healthy。
- **仍有效的故障证据只剩**：Windows 侧新端口 curl ≥10s 仍 000 + Windows netstat 无新端口 + 长存活 8080 正常 + 复用端口可注册。

### 第 6 步：区分环境问题与代码问题

CI（干净 Linux 容器）全绿 → 不是业务代码；业务长存活端口、Windows→WSL 转发正常 → 不是配置错误；官方 issue 症状一致 → 不是本项目引入。

## 四、结论与修复

- **根因**：WSL2 内部 localhost 转发中继（guest 侧 `Relay`，`/init` 子进程）存在两级形态：**严重形态**（中继降级：dmesg 持续报错、Windows 侧新端口不可达、小时级）与**健康态瞬态**（进程内会话拆除竞争：第 2 轮起注册停滞 2–5s 自愈、无 dmesg 错误）。
- **修复（32 号修正边界）**：`wsl --shutdown` 或整机重启只对**严重形态**成立（08-18 已验证恢复）；**健康态端口 0 注册失败窗口不因重启消除**（32 号：重启后探针 1/8~3/8 成功），修复方向 = 升级 WSL 2.9.5+（含 #41051/#41125）；Docker 已通过 ON/OFF 三轮对照排除。均须用户同意。本次完成的是**定位 + 规避 + 可复现探针**；业务侧无法"修好"微软组件。
- **对项目的影响**：两个 Grafana 测试按环境性跳过；本地失败机制 = 注册时序竞态（bind 返回先于 Windows listener 就绪 ~200ms，t+0 拨号必 refused），CI 无此竞态所以全绿；变更归档中"整体不可用""网络栈残留"的断言已修正为精确结论。

## 五、规避与工程经验

- 本地脚本/测试连 `127.0.0.1` 新端口时，对首个连接做 ≥100ms 重试，或先自连一次完成"端口注册"；连长存活端口无需处理。
- 遇到"刚 listen 就 refused"先按环境问题排查，**不要改业务代码、不要误判为 Go/Python 差异**。
- 探针语义：单轮 + 时延测量 + Windows 侧 curl 校验（`hack/wsl-loopback-probe`，健康态 PASS）；多轮模式仅作诊断，**其结果不能判定环境故障**。
- 环境断言必须可复现，且**必须带健康对照**：先证明"健康态下探针会通过"，再谈"故障态探针失败"。
- **探针工具本身也要验证（32 号）**：wincheck 曾因只 Listen 不 Accept + `-o /dev/null`（Windows 侧 rc=23）从未真正工作，历史 `win=UNREACHABLE` 读数作废；工具结论必须带地面真值（真实服务 + Windows netstat/curl）对照。

## 六、附录

### 探针参数与自动接入（2026-08-18 晚更新）

- 运行：`go run ./hack/wsl-loopback-probe`，默认单轮：测量 WSL 内首次连接成功时延（0→2s 每 50ms 重试）+ Windows 侧 `curl.exe` 校验新端口 + dmesg 计数。
- 结果分级：`PASS` / `WARN` / `FAIL` / `SKIP`（非 WSL 环境）；`-attempts 3 -delay 0` 为压力复现模式（稳定触发 WARN 1/3，仅诊断用）。
- 自动接入：`make preflight` 第 9 节与 `make selfcheck` 会自动运行探针；探针只输出结果、不设置退出码，由调用方决定门禁（preflight 中 FAIL 阻止启动，WARN 仅提示）。

### 官方依据

- WSL 官方技术文档 [localhost 转发说明](https://wsl.dev/technical-documentation/localhost/)：NAT 模式下 WSL 会监听绑定的 TCP 端口并通过 `wslrelay.exe` 转发到 Windows；guest 侧对应 `localhost.cpp` 实现的 `Relay` 进程。
- [microsoft/WSL#12837](https://github.com/microsoft/WSL/issues/12837)：症状与本例严重形态一致（localhost 失效但 eth IP 正常、`UtilAcceptVsock` 报错、`wsl --shutdown` 恢复）。
- 环境版本：Windows 26200.9168 / WSL 2.7.8.0 / Ubuntu 26.04 / 内核 6.18.33.1。

### 复现与验证命令

```bash
go run ./hack/wsl-loopback-probe          # 单轮健康检查：健康态 PASS
go run ./hack/wsl-loopback-probe -attempts 3 -delay 0   # 压力复现瞬态（诊断用，预期 WARN）
dmesg | grep UtilAcceptVsock | wc -l      # 严重形态计数持续增长；健康态为 0/恒定
cd dashboard/backend && go test ./internal/api/ -run TestGrafanaProxy -v
```

### 关联记录

- 流水账（完整证据链）：[journal/2026-08-18-wsl-loopback-relay.md](../journal/2026-08-18-wsl-loopback-relay.md)
- 蒸馏规则（Agent 视角）：[lessons/process-wsl-loopback-fresh-listen-refused.md](../lessons/process-wsl-loopback-fresh-listen-refused.md)
- 变更归档：[change-history/2026-08-18-wsl-loopback-case-study/](../../change-history/2026-08-18-wsl-loopback-case-study/README.md)
- 排障入口：[TROUBLESHOOTING.md](TROUBLESHOOTING.md)
