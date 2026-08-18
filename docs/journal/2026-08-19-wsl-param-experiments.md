> 日期：2026-08-19
# WSL 回环参数验证：三 issue 闭环与诚实修正（2026-08-19）

## 背景

延续阶段五至九：源码机制链（20 号文档）已定，缺参数级实测。#66 查重、#65 指纹、#64 参数实验三个本仓库 issue 于本日凌晨全部完成并关闭。

## 关键结果

1. **查重**：本组合（Consomme + TCP 回环 churn 同步注册链）无人以源码层报告；官方 backlog 自认 TCP retry/timeout 未实现。投稿聚焦机制链增量。
2. **指纹**：DLL 精确锚定 openvmm commit 0bb5cf75（tcp/udp/dns_tcp 行号全中）——"分析的版本不对"质疑消除。
3. **参数实验**：
   - bind 固定 10.4ms 同步链；128 并发 p50=1.32s、>1s 占 99.27%（秒级停滞定量复现，吞吐恒定 96/s）。
   - 60s 窗口推断被推翻：listener <2s 释放，60s 是竞态保护窗口。
   - TIME_WAIT 实测 81~121s（服务端主动关闭）/ 145~187s（客户端主动关闭）；同四元组重连 10048。
   - 抓包（WPR+Get-WinEvent，免 Npcap）：健康态同四元组在 TCB 插入即被拒（Duplicate TCB），SYN 未发出；默认 Windows 有 SYN 重传（8080 对照 RexmitCount=4）→ Consomme 禁重传使黑洞无兜底。

## 遗留与下一步

- 退化态黑洞抓包（>30s 无响应）：需退化态复现 + 同步抓包，健康态不可复现。
- churn 窗口 BindEndpoint 事件 744/6400 差异：疑端点复用，未定论。
- 微软观察窗口至 ~09-01；#62/#63 待用户拍板。

## 证据

- Desktop\WSL\ 20~23 号文档（仓库 Documents/ 同步副本 gitignore 不入库）；logs/ 下 ETL 与解析日志；/root/research/exp/ 实验源码（仓库 research/exp/ 同步副本已入库）。
