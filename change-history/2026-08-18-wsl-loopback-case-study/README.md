# WSL 回环中继：完整研究链 + 可复现探针（结论已修正）

> 日期：2026-08-18（晚修正：恢复验证 + 旧探针推翻 + 健康态瞬态发现）｜ 关联：docs/journal/2026-08-18-wsl-loopback-relay.md、docs/operations/WSL_LOOPBACK_CASE_STUDY.md、docs/lessons/process-wsl-loopback-fresh-listen-refused.md

## 为什么做

- 此前把"WSL 回环 TCP 当前整体不可用"作为环境遗留记入归档，断言模糊且未经核实。
- 用户要求核实断言，研究分两个阶段：首轮定位（收敛到 localhost 转发中继）+ 恢复验证（`wsl --shutdown` 后）。
- 恢复验证阶段**推翻了首轮的探针结论**（健康对照发现"立即拨号 ×10"在健康态必然失败；vsock 未监听端口超时是正常行为），并发现**健康态可复现瞬态**（进程内连续端口 churn 第 2 轮起注册停滞 2–5s 自愈）。

## 改成什么

1. **探针重写为单轮语义**：`hack/wsl-loopback-probe/main.go`——默认 `-attempts 1`：测量 WSL 内首次连接成功时延（0→2s 每 50ms 重试）+ Windows 侧 `curl.exe` 校验新端口 + dmesg 计数；`-attempts 3 -delay 0` 为压力复现模式（稳定触发 WARN 1/3，仅诊断）。健康态默认 4/4 PASS 已验证。
2. **文档修正四处**：journal / WSL_LOOPBACK_CASE_STUDY / lessons / TROUBLESHOOTING §3.3——统一修正"错误码 11=EAGAIN"为"fd 11（poll 60s 超时循环）"、标注旧读数作废、补充两级形态与健康对照方法论。
3. **微软 issue 投稿稿**（仓库外 Documents/）：以"健康态瞬态注册失效"为 Repro from healthy，原始小时级故障为严重形态；环境 Windows 26200.9168 / WSL 2.7.8.0 / Ubuntu 26.04 / 内核 6.18.33.1。

## 关键行为

- 探针只读、无副作用；默认单轮秒级完成，不阻塞 `make preflight` / `make selfcheck`。
- 结果分级：PASS / WARN / FAIL / SKIP（非 WSL 环境）；preflight 中 FAIL 阻止启动，WARN 仅提示。
- 多轮模式（`-attempts >1`）会命中健康态瞬态，**其结果不能判定环境故障**。

## 验证

- 健康态（wsl --shutdown 恢复后）：探针默认 4/4 PASS；压力模式 `-attempts 3 -delay 0` 稳定触发 WARN 1/3；`go vet` / `gofmt` 通过；`preflight.sh` 契约兼容（只认 `RESULT: FAIL/WARN/PASS/SKIP`）。
- 恢复验证：Windows→WSL 新端口转发恢复（~2ms HTTP 200）；vsock 已知存活端口 50000/50001/50002/50005 CONNECTED。
- 未验证：CI 上 selfcheck 新步骤实跑（探针非 WSL 自动 SKIP）；微软 issue 是否被接受（投稿前需用户确认）。

## 回滚

- 移除 `hack/wsl-loopback-probe/`、`hack/preflight.sh` 第 9 节、`make selfcheck` 对应步骤；文档同步删案例与入口。
- 纯文档与工具链改动，无数据迁移、无运行时行为变化，回滚零风险。
