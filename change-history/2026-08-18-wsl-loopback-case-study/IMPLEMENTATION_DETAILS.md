# IMPLEMENTATION_DETAILS：WSL 回环中继研究链

## 改动前状态

- `hack/wsl-loopback-probe/main.go`：旧版"每次新建 127.0.0.1 随机端口并立即拨号 ×10"，默认 `-attempts 10`，加 dmesg 计数。实测输出 `RESULT: FAIL`（10/10 失败 + dmesg 计数 1620），当时认为是有效复现。
- 文档四处均为旧结论：`accept(11)` 被误读为"错误码 11=EAGAIN"；客侧 0/10 读数被当作故障证据；vsock 未监听端口超时被当作"vsock 失效"。
- 未提交工作区：`hack/wsl-loopback-probe/main.go` 已有重写版（单轮语义）但未提交。

## 实现

### 探针重写（hack/wsl-loopback-probe/main.go）

- 新语义：
  1. 每轮新建 `127.0.0.1` 随机端口 + HTTP 应答，测量 WSL 内首次连接成功时延（0→2s，每 50ms 重试）；
  2. 若有 `curl.exe`（Windows 互操作），以 Windows 侧视角请求新端口，验证 localhost 转发注册已落地；
  3. 读取 dmesg `UtilAcceptVsock` 计数作为症状计数（不单独判级）。
- 参数：`-attempts`（默认 1）、`-delay`（轮间间隔毫秒，默认 0，诊断用）。
- 平台行为注释：健康态下进程内连续 churn 第 2 轮起注册停滞 >2s（会话拆除竞争），因此默认单轮；多轮仅诊断。
- 兼容 `preflight.sh` 契约：只认 `RESULT: FAIL/WARN/PASS/SKIP`。

### 文档修正（四处 + change-history）

| 文件 | 修正内容 |
| --- | --- |
| docs/journal/2026-08-18-wsl-loopback-relay.md | 补阶段二研究；`accept(11)`=fd 编号；旧读数标注作废；两级形态结论 |
| docs/operations/WSL_LOOPBACK_CASE_STUDY.md | 现象表标注旧读数作废；新增"健康对照推翻旧结论"方法论（H1/H2/H3）；探针附录更新 |
| docs/lessons/process-wsl-loopback-fresh-listen-refused.md | 两级形态；踩坑教训（健康对照、vsock 探针、瞬态≠故障）；验证命令更新 |
| docs/operations/TROUBLESHOOTING.md §3.3 | 两级形态症状；快速判断命令与语义；多轮结果不可判故障 |

## 研究关键数据（2026-08-18）

- 健康态注册延迟：约 50–100ms（H1，10 轮对照）。
- 进程内瞬态：第 2 轮起停滞 >2s，2–5s 自愈（Go 探针 5/5；进程内间隔 3s 仍失败 3/3；独立进程 1s 间隔 4/4 全过）。
- vsock：host 1–512 全 ETIMEDOUT（未监听端口正常）；50000/50001/50002/50005 CONNECTED。
- 严重形态 dmesg：`UtilAcceptVsock: Waiting for abnormally long accept(11)`（11 = fd 编号）。
- 恢复验证：`wsl --shutdown` 后 Windows→WSL 新端口转发恢复（echo 18530 → HTTP 200 ~2ms；netstat dllhost 37696）。
