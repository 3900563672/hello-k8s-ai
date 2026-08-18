# IMPLEMENTATION_DETAILS：参数实验细节与数据

## 环境

Windows 26200.9168 / WSL 2.7.8.0 / Ubuntu 26.04（Consomme 模式，健康态，未重启环境）。Go 1.26.4。

## 实验 1：bind() 系统调用耗时（GNS 同步注册链）

工具：bind_latency.go / syscall_bench.go

| 场景 | min | p50 | p95 | max | avg |
| --- | --- | --- | --- | --- | --- |
| seq 200 轮 127.0.0.1 | 2.4ms | 10.4ms | 21.5ms | 33.5ms | 13.7ms |
| churn 8×200 | 43.4ms | 93.3ms | 93.9ms | 126.1ms | 90.4ms |
| getpid 对照 | - | 83ns | - | - | - |
| socket+close 对照 | - | 1.08µs | - | - | - |
| bind 127.0.0.2 | 5.0ms | 10.4ms | 20.9ms | 31.7ms | - |

结论：guest seccomp 拦截所有 bind；Windows 侧才区分 127.0.0.1。

## 实验 2：去分配窗口（c_bind_timeout_seconds=60）

- 2a：bind→close→立即重 bind→保持：Windows listener 全程 LISTENING；最终 close 后 ~1.4s 消失。
- 2b：bind→close→不重 bind 观察 70s：close 后 ~1.8s 消失（2s 轮询粒度）。
- 修正：60s 是 bind 完成至 sock_diag 确认活跃前的竞态保护窗口，非 listener 保留期。

## 实验 3：TIME_WAIT 与同四元组重连

- 服务端主动关闭（45681/55551）：TIME_WAIT 自 00:28:28 可见，00:29:47 仍在，00:30:27 消失 → 约 81~121s。
- 客户端主动关闭（45682/55552）：close 00:31:58.878，+145s 仍 10048，00:35:0x 消失 → 约 145~187s。
- 同四元组重连（SO_REUSEADDR 可 bind）：connect 12ms 内 WSAEADDRINUSE(10048)。

## 实验 4：极端 churn（churn_heavy.go）

| 并发 | 轮数 | p50 | p95 | p99 | max | >1s 占比 | 吞吐 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 64 | 100 | 666ms | 687ms | 709ms | 735ms | 0% | 96 bind/s |
| 128 | 100 | 1.322s | 1.355s | 1.380s | 1.420s | 99.27% | 96 bind/s |

模型：延迟 ≈ 并发 × 10.4ms；吞吐恒定为 1/10.4ms。

## 实验 5：WPR/ETW 抓包（最小 profile：TCPIP + Kernel-Network，27MB ETL）

- T1 两轮（55554/55556，SO_REUSEADDR）：事件序列为 bind 成功 → requested to connect → `listener ... accept failed: connection insertion. Duplicate TCB` → connect attempt failed（无法打开已存在的传输地址）→ 无 loopback Nbl 发送。**SYN 未发出**。
- 基线 20 连：全部 0ms；SYN/SYN-ACK 各 1 次。
- 对照：本机其他进程 127.0.0.1:8080 连接 `retransmitting connect attempt, RexmitCount = 4`，RTO 300ms→6s。
- 开放观察：128×50 churn 窗口 BindEndpoint(WFP) 事件约 744，远小于 6400 次 bind；疑为端点复用，未定论。

## 抓包方法（沉淀，供复现）

1. 写最小 wprp（EventProvider Name="Microsoft-Windows-TCPIP"/"Microsoft-Windows-Kernel-Network"，Keyword 0xFFFFFFFF，LoggingMode=File）。
2. 提权（UAC + Start-Process -Verb RunAs -WindowStyle Hidden + 脚本文件 + %TEMP% 日志轮询）执行 wpr -start/-stop。
3. 非提权 Get-WinEvent -Path x.etl -Oldest 解析 Message；注意 PowerShell 内联命令的 $ 变量会被外层 shell 吞，用脚本文件。
4. 无需 Npcap/Wireshark；pktmon 看不到 AFD loopback。
