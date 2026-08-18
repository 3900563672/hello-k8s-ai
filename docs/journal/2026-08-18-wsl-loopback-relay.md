# 2026-08-18 WSL 回环 TCP：新监听端口首连被拒（localhost 转发中继）——完整研究链

> 日期：2026-08-18 ｜ 触发者：本地 Agent（用户要求核实环境遗留断言）｜ 相关：change-history/2026-08-18-wsl-loopback-case-study/ ｜ promoted: lessons/process-wsl-loopback-fresh-listen-refused.md

## 触发

用户要求核实 change-history 中"WSL 回环 TCP 当前整体不可用（同进程 listen→dial 被拒）"的断言（GrafanaProxy 两个测试本地失败、CI 正常）。核实过程分两个阶段：**首轮定位**（16:00-16:40）与**后续研究**（晚 20:00 后，含恢复验证、探针推翻、瞬态复现）。

## 阶段一：首轮定位（2026-08-18 16:00-16:40 CST）

1. **不是整体不可用，是特定模式**：新写的探针（/tmp/nd_*.go）全部通过，但原始 `/tmp/probe2.go` 稳定失败 → 模式 = IPv4 回环 + 新监听端口 + 立即连接。
2. **语言无关**：Go 立即 dial → refused；Python（socket 模块）立即 connect → `Errno 111`；延迟 300ms 后两者都 OK。
3. **地址对照**（每次新建端口立即拨号 ×10，读数后续被推翻，见阶段二）：
   - `127.0.0.1` → 0/10 成功（connection refused）；
   - `127.0.0.2` → 0/10（i/o timeout，SYN 被丢弃）；
   - `192.168.10.227`（eth0）→ 10/10；`169.254.73.153`（loopback0 vNIC）→ 10/10；`[::1]` → 成功。
4. **内核已注册但 SYN 被拒**：listen 后立即查 `/proc/net/tcp`，端口已是 `0A`（LISTEN），但同一时刻 connect 仍被 RST → 拦截在 netfilter/内核 hook 层。
5. **间歇性而非固定窗口**：单监听端口时间线 t=0 失败、t≥100ms 起全 OK；但"每轮新建端口"的 sweep 在 sleep 300ms 后仍偶发失败（50/100/300ms 均出现过失败）→ 中继注册队列拥塞型抖动。
6. **端口级注册**：同一端口"首连成功"一次后，关闭并重新 listen，再立即连接成功。
7. **排除项**：无 raw socket（`ss -w` 空）、无 eBPF 可见、iptables/nft 对 127.0.0.0/8 无拦截（Docker nat 规则 `ip daddr != 127.0.0.0/8` 明确排除）；Relay 进程 fd 数在失败期间不变。
8. **dmesg 症状**：自 2026-08-18 ~11:38（boot 后 43695s）起持续输出 `WSL (Relay) ERROR: UtilAcceptVsock: Waiting for abnormally long accept(11)`，每 ~10s 一条，6+ 个 `/init` 子进程 `Relay`（PID 轮换）轮流报错，累计 1600+ 条。**注意：`accept(11)` 的 11 是 fd 编号，不是 errno 11=EAGAIN**；消息语义是"在 fd 11 上等待 accept 异常久（poll 60s 超时循环）"。早期文档误写为"错误码 11"，已统一修正。
9. **用户侧当时正常**：Windows→WSL localhost 转发正常（`curl.exe http://127.0.0.1:8080/` 与 `:18080/` 均 HTTP 200），Dashboard 访问不受影响。

## 阶段二：恢复验证 + 探针推翻（2026-08-18 晚）

经用户同意执行 `wsl --shutdown` 后：

1. **恢复验证（缺口②）**：Windows→WSL 新端口数据转发恢复（`echo 18530` → HTTP 200，~2ms；`netstat` 显示 dllhost 37696 承载监听）。
2. **旧探针被推翻（H1）**：健康对照 10 轮显示 WSL 内新端口注册延迟约 **50–100ms**，`t=0` 立即拨号 10/10 必失败 → 旧探针"listen 后立即拨号 ×10"在**健康态也必然 FAIL**，属探针设计缺陷。阶段一中 E1/E3a/E3b 的客侧读数（0/10 等）**全部作废**。
3. **vsock 探针被推翻（H2/H2b）**：H2 扫描宿主端口 1–512 全 ETIMEDOUT；H2b 发现 **50000/50001/50002/50005 可 CONNECTED** → guest→host vsock 通道活着；对**未监听端口**超时是正常行为。旧"新D：vsock 失效"结论撤销。
4. **仍有效的故障证据（原始小时级故障）只剩**：Windows 侧新端口 curl ≥10s 仍返回 000 + Windows netstat 无新端口（新C）+ 长存活 8080 正常（E9）+ 复用端口可注册。
5. **健康态可复现瞬态（重大发现，可作 issue Repro from healthy）**：
   - 同一进程内连续"listen→连接→curl→关闭"多轮，**第 1 轮必过，第 2 轮起新端口注册停滞 >2s**（Go 探针 5 次运行 5/5 命中；第 3 轮部分恢复）。
   - 进程间隔离对比：独立进程（间隔 1s）4/4 全过；进程内间隔 3s 依然失败（3/3）→ **不是时间恢复，是进程内"会话拆除竞争"**。
   - 瞬态**自愈**：t+2s/5s/15s/30s 后注册恢复；健康态全程 dmesg `UtilAcceptVsock` 计数 = 0。
   - 结论：原始小时级故障是严重形态；"健康态可复现的瞬态注册失效 2–5s"是独立、可复现的形态，可作微软 issue 的 Repro from healthy。

## 最终结论（修正后）

- 业务代码无错；CI 无此问题。两个 Grafana 测试按环境性跳过。
- WSL2 localhost 转发机制存在两种已知形态：**严重形态**（小时级、dmesg 持续报错、Windows 侧新端口不可达）与**健康态瞬态**（进程内连续端口 churn 第 2 轮起注册停滞 2–5s 后自愈，无 dmesg 错误）。
- 立即拨号 ×10 的探针语义无效（健康态必然误报），已重写为"单轮 + 时延测量 + Windows 侧 curl 校验"语义。
- 修复手段仍是重置 WSL 网络栈（`wsl --shutdown` 或整机重启），须用户同意。

## 处理

- 探针重写为单轮语义：`hack/wsl-loopback-probe`（默认 `-attempts 1`，健康态 4/4 PASS 已验证；`-attempts 3 -delay 0` 作压力复现，稳定触发 WARN 1/3）。
- 沉淀本条目 + `lessons/process-wsl-loopback-fresh-listen-refused.md` + `docs/operations/WSL_LOOPBACK_CASE_STUDY.md` + `change-history/2026-08-18-wsl-loopback-case-study/`。
- 微软 issue 投稿稿：以"健康态瞬态注册失效"为 Repro from healthy，原始小时级故障为严重形态；环境 Windows 26200.9168 / WSL 2.7.8.0 / Ubuntu 26.04 / 内核 6.18.33.1。
