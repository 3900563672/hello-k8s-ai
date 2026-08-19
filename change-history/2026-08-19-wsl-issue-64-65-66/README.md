# WSL 回环研究三 issue 闭环（#66 查重 / #65 指纹 / #64 参数实验）

> 日期：2026-08-19 ｜ 关联：docs/journal/2026-08-19-wsl-param-experiments.md、Desktop\WSL\ 20~23 号文档（仓库 Documents/ 同步副本，gitignore 不入库）

## 为什么做

- #66 上游查重：确认本场景（Consomme 模式 TCP 回环 churn 的同步注册链）无人以源码层报告过，决定投稿策略。
- #65 指纹比对：消除"DLL 里不是分析那版"的版本漂移质疑。
- #64 参数实验：把源码推断（10.4ms 注册链 / 60s 窗口 / TIME_WAIT / 禁重传）用实验验证或修正。

## 改成什么

1. **#66 查重（完成并关闭）**：openvmm 官方 backlog 承认 "TCP retry and timeout logic" 未实现；WSL 侧有 IPv6 端口保留泄漏修复、seccomp 扩展拦 listen()、mirrored UDP 泄漏报告——但均为现象级或镜像/UDP/relay 场景，本组合无人报告。
2. **#65 指纹（完成并关闭，决定性）**：wsldevicehost.dll 精确匹配 openvmm commit 0bb5cf75（2026-04-19）：tcp 27/27、udp 7/7、dns_tcp 2/2 行号全中；HEAD eef8b22e 仅 0/27。TimeWait TODO、禁 SYN 重传、静默超时、无 SO_REUSEADDR 均在该版本存在。
3. **#64 参数实验（完成并关闭）**：
   - 实验 1：每个 bind 固定 ~10.4ms GNS 同步链往返（对照 socket+close 1µs）；churn 8 并发 p50=93ms≈8×10.4ms。
   - 实验 2：**修正源码推断**——listener close 后 <2s 释放，非 60s；c_bind_timeout_seconds=60 是 bind 完成至 sock_diag 确认活跃前的竞态保护窗口。
   - 实验 3（新增）：TIME_WAIT 实测 81~121s（服务端主动关闭）/ 145~187s（客户端主动关闭）；期间同四元组 bind/connect 必 WSAEADDRINUSE(10048)。
   - 实验 4（新增）：极端 churn 128 并发 p50=1.32s≈128×10.4ms，**>1s 停滞 99.27%**——秒级停滞 syscall 层定量复现；吞吐恒定 ~96 bind/s（串行锁模型）。
   - 实验 5（新增，WPR/ETW 抓包）：健康态同四元组重连在 TCB 插入阶段被拒（Duplicate TCB），SYN 未发出；对照默认 Windows loopback 有 SYN 重传（RexmitCount=4）→ Consomme 禁重传（源码级）使黑洞无兜底。
4. **新增实验代码**：research/exp/（churn_heavy.go / tw_rebind.go / tw_passive_listener.go 等），可复现。

## 关键行为

- 全部只读/受控实验，未重启环境、未动 Docker/K8s、未改代理。
- 抓包用系统自带 wpr（TCPIP + Kernel-Network 最小 profile）+ Get-WinEvent 解析，无需 Npcap/Wireshark。
- 诚实修正两处旧推断：60s 窗口语义、TIME_WAIT 时长（60-75s → 实测 81~187s 分场景）。

## 验证

- 实验数据与原始日志：桌面 WSL 23 号文档 + logs/（ETL 27MB、netstat 轮询日志）。
- WPR 证据：Duplicate TCB 拒绝序列（两轮复现）、基线 20 连 0ms、8080 重传对照。
- 未验证：退化态"黑洞"（>30s 无响应）健康态不可复现，需数小时压力/故障态 + 同步抓包；churn 期间 Windows 端点事件数与 bind 数差异（744/6400）未定位。

## 回滚

- 纯研究沉淀：新增 research/exp/ 与 change-history 条目；无运行时改动。删除上述目录即可回滚，零风险。
- 实验对系统的影响：WPR 抓包会话已停止、监听端口进程已退出、临时文件可删（%TEMP% 下 `wpr-*` / `tw-*` / `t1-*` / `churn*` 日志）。
