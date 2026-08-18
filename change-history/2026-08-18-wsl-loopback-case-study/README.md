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

## 后续进展（2026-08-18 深夜 → 08-19 凌晨，详见 journal 阶段五至九）

- **架构修正**：本机 2.7.8.0 实际转发实现为闭源 wsldevicehost.dll（Consomme，dllhost.exe 承载），wslrelay.exe 从未运行 → 早期开源机制链推断不适用本机（journal 阶段四补充 2）。
- **决定性实验**：T1/T3/T5 证明固定端口免疫、ephemeral 受陈旧积压 + churn 内部状态影响、有连接历史端口保留 60-75s（CloseWait + WSAEADDRINUSE）。
- **官方日志收集成功**（22:27-22:31，`-LogProfile networking`，102MB 包已发微软评论区）并在录制窗口内复现（8 轮 churn，前 3 轮半黑洞、后 5 轮全黑洞）。
- **v3 评论已正式发送**：微软 WSL 回环 issue（编号见桌面 11 号文档） issuecomment-5329767813（22:40，7306 字节）；观察窗口至 ~09-01。
- **日志包分析（阶段九）**：WPR trace 内确认探针前 3 轮端口（43723/39489/39027）的 Windows 侧连接失败事件（TCPIP `requested to connect` 36 条 + loopback NBL 双向消息）→ "虚拟握手黑洞"运行时实证在官方包内；pktmon 负结果（中继不经 NDIS/vmswitch）；HNS "WSL (Hyper-V firewall)" 创建失败 0x80070002（环境异常，仅备注）。
- 材料归档：`Desktop\WSL\`（16 文档 + logs/evidence/），仓库 `Documents/16_日志包分析结果.md`。

## 后续进展（2026-08-19 凌晨：#61 DLL 核验 + #60 复现手册 + 回滚指南）

- **#61 DLL 签名核验（完成，决定性）**：`C:\Program Files\WSL\wsldevicehost.dll`（v1.2.14.0，正被 dllhost 加载）字符串确认内嵌 openvmm net_consomme：96 处 `oss\vm\devices\net\net_consomme\...` 源码路径、`Duplicate TCP bind for port` 翻译串、TCP 状态机字段串、带行号 WPP 事件名、微软内部 CI/Cargo 路径（rustc 1.94.0-ms-20260303）。wslrelay.exe 无任何标记；两二进制导出表 0 项。"可能包含相同代码"→"**已确认包含**"。局限：行号与 openvmm master 漂移，无法定位精确 commit（文档 `Documents/17_dll签名核验.md`）。
- **#60 实验复现手册（完成）**：12 项实验 → 命令/预期/耗时/依赖态矩阵 + 7 条常见误判清单；退化态标注"不可一键复现"（`Documents/18_实验复现手册.md`）。
- **回滚指南（完成，用户强调）**：权限矩阵、WSL 恢复/版本更新回滚、Docker/K8s/数据盘 Junction、C 盘防护（WPR 成对）、实验回滚清单、#62 恢复序列（`Documents/19_回滚与恢复指南.md`）。
- issue：#61/#60 已评论并关闭；#62/#63 待用户明天决定。

## 后续进展（2026-08-19：外部引用泄露清洗 + 规矩沉淀）

- **事件**：本仓库 PR #59 与演练 issue 标题含"微软 WSL 回环 issue 完整编号"，触发 GitHub 自动交叉引用，对方 issue 时间线出现指向本仓库的事件（第三方可见）。演练 issue 删除后事件消失；PR 只能 close 不能删除，close 不移除事件（API 复核无残留）。
- **清洗**：PR #59 改名、正文去编号、分支重写为单提交（diff 与提交信息零外部编号）；演练 issue 删除；issue #60 标题去编号；全仓 main 零残留；本地备份分支 backup/pre-cleanup 禁止推送。
- **沉淀**：规矩写入 docs/agents/WORKFLOW.md 第 5 节（提交前检查项）+ journal 条目 docs/journal/2026-08-19-github-crossref-external-issue-number.md；此后公开仓库内容（标题/正文/评论/提交信息/提交 diff）禁止外部 issue 完整编号。
