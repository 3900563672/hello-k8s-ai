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
## 阶段三：试探评论正式发出与演练清理（2026-08-18 晚）

- **正式评论已发出并验证**：microsoft/WSL 回环 issue 评论区 issuecomment-5327431363（编号与链接见桌面 11 号文档） ，作者账号 3900563672，发送时间 2026-08-18T11:20:40Z（北京时间 19:20:40），正文 4012 字节，回读校验无乱码、无截断。
- 发送方式：`gh issue comment <编号> --repo microsoft/WSL（编号见桌面 11 号文档） --body-file <终稿>`；先在自有仓库建演练 issue #57 用同一命令发评论验证渲染（4031 字节），通过后才正式发送（零风险流程）。
- **演练 issue #57 删除**：GitHub REST API 没有删除 issue 的端点（`DELETE /repos/.../issues/57` 必 404/410）；正确方式是 GraphQL mutation `deleteIssue`（入参 node_id）；`DeleteIssuePayload` 无 `issue` 字段，查询 `repository { nameWithOwner }` 即可。删除后 GET 返回 HTTP 410 "This issue was deleted" 确认。详见 journal/2026-08-16-github-api-gh.md 与桌面 11 文档。
- **观察窗口**：2026-08-18 → ~09-01（2 周）。我们是非作者追加评论，官方不会自动通知，需订阅或主动轮询（检查命令见桌面 11 文档）。三种回应路径见 09 文档 4.2。
## 阶段四：源码机制分析（2026-08-18 深夜，2.7.8 tag 逐行核对）

- **机制链（4 环节）**：① `bind(127.0.0.1,0)` 被 seccomp 拦截后立即恢复进程，端口解析 + allocate 在后台异步完成（`GnsPortTracker.cpp:396-410,532`）→ 探针 `net.Listen` 返回时注册可能未完成；② 注销靠 sock_diag 500ms 轮询（29-58 行）；③ Windows 侧 `StopPortListener` 在全局 SRW 锁内 `Worker.join()`（`wslrelay/localhost.cpp:217-237`，锁定义 localhost.h:60）；④ worker 退出要等 pending AcceptEx 取消完成（`SingleAcceptHandle` 析构 `CancelIoEx` + `WSAGetOverlappedResult(fWait=TRUE)`，relay.cpp:736-750）→ join 可能拖 2–5s → 期间所有 allocate 排队 → 新端口注册停滞。
- **判定**：真实设计缺陷（锁内做慢操作），触发条件 = 进程内快速 churn 新端口（close 后 ~500ms 内再 bind）。
- **全部 7 项实测观测与机制预测吻合**（第 1 轮必过 / 第 2 轮起停滞 / 2–5s 自愈 / 进程内 3s 仍失败 / 跨进程 1s 通过 / dmesg 计数 0 / 重启后仍复现）。
- **issue 概率评估**：纯观察 3–4 成 → 有代码机制 + 修复建议 5–6 成；剩余不确定性 = `WSAGetOverlappedResult` 等待时长无运行时证据 + 官方对 churn 场景态度。
- **验证计划**：V1 探针复现 / V2 guest GNS 日志时间戳 / V3 wslrelay ETW（PortRelayBind/UnBind）/ V4 停滞窗口线程栈 dump / V5 端口复用检查 / V6 跨进程对照。验证通过后再决定是否追加评论（把"推断"升级为"代码依据 + 运行时证据"）。
- 详细文档：桌面 12_源码机制分析.md（Documents/ 有同步副本）。
### 阶段四补充：运行时验证结果（2026-08-18 深夜）

- **V1 探针复现** ✅：压力模式第 3 轮 STALL；单轮新鲜进程 3/3 PASS（52ms 基线）。
- **V2 GNS 日志** ⚠️ 不可用：g_TelemetryFd 仅 Utility VM（main.cpp:1630）。
- **V3 注册时序** ✅：netstat 100ms 采样，STALL 轮端口（34307/40873/45213/43891）**从未出现在 Windows 侧** → allocate 排队；OK 轮端口 ~0.5s 出现。
- **V4 线程栈** ⚠️ 环境受限：procdump ThreadContext 全 0（1663B 非标准 CONTEXT，两次复现），gdb 不识别；放弃（行为学证据足够）。
- **V5 端口复用** ✅：8 轮随机端口全不同，Count++ 路径不触发。
- **V6 固定 vs 随机** ✅ **决定性**：固定端口 19432 连续 8 轮全 OK（0.7–1.1ms）；随机端口 8 轮 5/8 STALL。固定端口 rebind 快于 sock_diag 500ms → deallocate 被抑制 → 无锁竞争。
- **V7 注销耗时** ✅ **核心运行时证据**：8 无连接端口 kill 后 1.1s 全部注销；4 有连接端口 kill 后 60s–4min 未注销（23000 系列 57s+ 持续 LISTENING、23100 系列 3–4min、39037 单端口 >10min）。有连接历史 → StopPortListener join 慢 → 锁占 → allocate 排队。
- **把握度更新**：5–6 成 → **7–8 成**。剩余不确定性：join 慢的确切内部原因未直接观测线程栈；官方对 churn 场景态度。
- **追加评论决策**：待用户确认。建议把"推断"升级为"机制已定位 + 运行时证据"（机制链 4 环节 + 关键对照数据 + 修复建议，保持克制标注未直接观测点）。

### 阶段四补充 2：架构修正 + 实证重做（2026-08-18 深夜）

**重大架构发现**：本机 2.7.8.0 实际转发实现是**闭源 wsldevicehost.dll（Rust consomme TCP 栈）**，由 dllhost.exe（COM 类 WslDeviceHost_Net，CLSID {16479D2E-F0C3-4DBA-BF7A-04FFF0892B07}）宿主；**wslrelay.exe 从未运行**（二进制含 PortRelayBind ETW 字符串但不启动）。之前基于开源 wslrelay 的机制链（GnsPortTracker/StopPortListener/SRW 锁 join/AcceptEx 取消）**不适用于本机实际代码**，机制推断全部降级；已发评论的 "Source Context"（localhost.cpp:52）与 "per-process" 论断需要修正（详见桌面 12 文档）。

**V8–V12 + 对照组结果（对照完备）**：
- V8 悬挂注销不阻塞新注册（1ms）；V9 多端口注销并行（两端口 69.6/70.8s）；V10a 无连接端口 1.3s 注销；V10b/c 有连接历史端口 63s 注销（连接早已关闭也一样）；V11 陈旧窗口内 dllhost 同端口 **CloseWait+Listen 并存**、Windows curl 立即失败、重启后恢复正常（42ms）；V12 陈旧窗口内 Windows bind 同端口 **WSAEADDRINUSE**。
- 控制组：单轮新鲜进程 5/5 OK（~52ms）；5 进程同时注册 5/5 OK（无注册-注册争用）；单进程 churn 第 2 轮起 >30s STALL；无连接 churn 也卡（概率性 5-6s）；双进程并发 churn 双双 STALL（**推翻"per-process"论断**）。

**实证模型**：注册/注销在闭源 worker 上重叠 → 注册概率性卡 2-30s+；有连接历史端口注销时连接 socket 卡 CloseWait、监听器保留 60-75s、阻塞 Windows bind。行为层面把握度 8-9 成；闭源内部机制 ~6 成（无法源码验证）。

**决策建议**：暂不追加评论；若要追加，先修正 per-process 与 Source Context 两处，以纯外部可验证事实（netstat/socket 状态/WSAEADDRINUSE）表述。

### 阶段四补充 3：T1/T3/T5 决定性实验（2026-08-18 深夜）

- T1：固定端口同步注册永不卡（15/15 <1ms）；ephemeral 延迟注册 27-52ms、只有它中招；间隔扫描非单调 → 陈旧累积污染。
- T3：8 轮 churn 6/8 轮 >90s 不可达（第 1、3 轮 52ms）；第 5-8 轮陈旧已排空仍卡 → 中继内部状态累积（netstat 不可见）。原评论"2-5s 自愈"只反映低积压状态。
- T5 剂量响应：0 陈旧 → 0 卡；1-3 陈旧 → 3/8 卡 >30s；5-6 陈旧 → 3/3 卡。陈旧概率性阻塞 ephemeral 注册。
- 最终模型：固定端口同步/免疫；ephemeral 延迟注册受陈旧积压 + churn 内部状态影响；有连接历史端口保留 60-75s（CloseWait + WSAEADDRINUSE）。
- 已发评论修正清单：self-heals 2-5s（错误）、per-process（撤回）、Source Context 开源代码（不适用）。修正版评论草稿见桌面 13 号文件。

### 阶段五：Consomme 开源代码核验（2026-08-18 深夜）

- **推翻"完全闭源"**：链路三层中两层开源——microsoft/WSL master `src/windows/common/ConsommeNetworking.cpp`（端口编排，569 行）与 OpenVMM master `vm/devices/net/net_consomme/`（Rust 用户态 TCP 栈，openvmm.dev rustdoc 同源）；仅 wsldevicehost glue 闭源（WSL 仓库只有 wsldevicehoststub）。wsldevicehost.dll 内联路径 `oss\vm\devices\net\net_consomme\consomme\src\tcp.rs` 与依赖字符串吻合。
- **环境复验 = Consomme 模式**：guest `enP42845p0s0`（virtio-net）192.168.10.227/24 + `loopback0` 169.254.73.153/30 + 无 wslrelay.exe（监听由 dllhost/wsldevicehost 持有）；.wslconfig 无 networkingMode（Consomme 为 2.7.x 默认演进方向）。
- **开源机制锚点（带行号）**：① 全局独占锁 `ConsommeNetworking.cpp:282`（ModifyOpenPorts 内 m_lock.lock_exclusive）串行化所有 bind/unbind（L290 BindPort / L299 UnbindPort），与旧 wslrelay 同型；② guest 端口追踪器 500ms sock_diag 轮询（GnsPortTracker.cpp:25,71）；③ `tcp.rs:713` `BindError::PortAlreadyBound` = dllhost 内 "Duplicate TCP bind for port" 字符串；④ `tcp.rs:2004-2006/2021-2022` TimeWait `// TODO: start timer` —— TimeWait 连接无到期清理，只能靠 host socket EOF；⑤ loopback = 同进程自连 + accept 双 socket（`tcp.rs` ~428-440、~981），解释 Listen+CloseWait 并存。
- **解释力升格**：60-75s 保留（等连接排空）、WSAEADDRINUSE（旧 listener 未移除）、重复绑定失败（PortAlreadyBound）、注册卡顿（全局锁 + 慢注销 + 状态累积）均有开源代码 + 行为双向证据；机制把握度 6 → 7.5-8 成。
- **对最初问题的一句话解释**：Consomme 模式下 churn 时，每次 bind/close 都过 500ms 轮询 + GNS 通知 + 全局锁串行 Bind/Unbind；有连接历史端口注销要等连接排空（TimeWait 无定时器），新注册被压 → 低积压 2-5s、累积后 90s+ 不可达。
- **13 草稿升级 v2**（桌面 + Documents）：三处修正 + 开源锚点 + 剂量响应/WSAEADDRINUSE + 官方式结尾（collect-networking-logs.ps1 / 邮件）。**未发送，等用户确认。**
- 源码本地副本：`/tmp/wsl-master`（sparse：src/windows/common、src/linux/init）、`/tmp/openvmm`（sparse：vm/devices/net/net_consomme）；已发评论 ID issuecomment-5327431363，官方尚未回复。

### 阶段六：WPR 残留事故复盘（2026-08-18 深夜，C 盘被写满）

- **症状**：C 盘仅剩 4.4MB 空闲（Used 429.5GB）。
- **原因**：`C:\Users\hh\AppData\Local\Temp` 下 WPR 录制残留，合计 **112.9GB**：
  - `WPR_initiated_WprApp_WPR System Collector.etl` 95.8GB（最后修改 21:56，录制直到磁盘满/被停才停止）
  - `WPR_initiated_WprApp_WPR Event Collector.etl` 13.2GB
  - `wpr_merged.etl` 3.9GB（早前合并产物）
  - 另有 dllhost_v4/v4b.dmp 崩溃转储共 ~213MB（WSL 研究时杀 dllhost 的残留）
- **根因**：之前研究 WSL relay 时启动的 WPR 录制（System Collector profile，为抓 wslrelay ETW）**只 start 未 stop/cancel**，会话退出后 ETL 文件残留且持续增长（System Collector 无大小上限），直到把启动盘写满。`wpr -status` 显示 not recording，说明无活动会话，纯残留。
- **处理**：`wsl -d Ubuntu -- bash -lc "rm -fv ..."` 删除（PowerShell Remove-Item 被本环境策略拦截；node 不在 PATH；WSL /mnt/c 删除是可靠通道）→ C 盘恢复 **110.4GB** 空闲。
- **预防（写死规则）**：
  1. WPR 录制必须成对：`wpr -start` 后无论如何要 `wpr -stop` 或 `wpr -cancel`，并在结束时立即检查 %TEMP% 下 *.etl 大小；
  2. 需要长录制时用 `-FileMode` 或设置 MaxFileSize，禁止无上限 System Collector 直录 C 盘；
  3. 每次长会话结束前，例行扫描 `%TEMP%` 与 `%LOCALAPPDATA%\Temp` 大于 1GB 的文件（PowerShell 一行可查，删除走 WSL rm）；
  4. ETL/转储等研究产物默认放 D:\WSL 研究目录或仓库 Documents（gitignore），不落 C 盘 Temp。
- **遗留**：`wslrelay_etw.etl`（40KB，早前抓取的证据）保留在 Temp，后续可归档到研究目录。

### 阶段七：系统进入"新端口注册失效"退化态（2026-08-18 23:1x，插曲）

- **现象**：健康态对照失败——全新固定端口 28999（从未用过）guest 1ms 注册成功，但 Windows 侧 30s 不可达；单轮 ephemeral 35791 同样（guest 12ms / Windows 30s 不可达）。8 轮 churn：第 1-4 轮 guest 正常但 Windows 全部不可达，第 5 轮起 guest 侧也 STALL。
- **netstat 决定性证据**：35791 在 Windows 侧**无任何 LISTEN**，但有 10+ 条 curl 客户端的 **TIME_WAIT**（127.0.0.1:55xxx→35791）——连接"完成握手"但无响应 = **consomme 虚拟握手黑洞**（监听未真正落地，SYN-ACK 由用户态栈虚拟生成，数据不转发）。
- **对照**：28080（实验早期注册）在服务存活期间 Windows curl 200（旧注册残留正常）；当前 28080 无 LISTEN（服务已停，正常）。
- **定性**：与 04 文档"阶段 2c A1 新端口注册未落地"一致 = 回环严重形态（注册链路失效）的现场复现。最可能触发源 = **本会话压力实验**（kill dllhost 多次、V8-V12/T1/T3/T5 大量 churn、WPR 95GB 录制、C 盘写满），非用户自然状态。
- **影响**：① 瞬态实验（健康态 19:00-21:00 数据）仍然有效；② 200 轮直方图需健康态，当前跑无意义；③ 评论中退化态需谨慎定性（不宣称自然触发）。
- **决策点**：恢复健康态需 `wsl --shutdown`（中断 Docker/K8s 项目）→ 建议**先收集官方日志（捕捉退化态现场），再决定是否恢复**。

### 阶段七补充：官方日志收集成功（2026-08-18 22:27-22:31）

- **产物**：`C:\Users\hh\OneDrive\Desktop\WSL\logs\WslLogs-2026-08-18_22-27-23.tar.gz`（102.4MB，官方 collect-wsl-logs.ps1 -LogProfile networking，WPR trace logs.etl 976MB 压缩后约 90MB + pktmon 7MB + service trace 8.7MB + 系统状态 56 文件）。已解压归档同目录；**精选子集 4.0MB**（wsl-netlogs-subset.zip，pktmon+service etl+网络/防火墙/注册表状态+探针复现证据）供 GitHub 贴（限 25MB）。
- **过程踩坑（重要，防再犯）**：
  1. 首次运行卡死：`wsl.exe -- id -nu 0` 的 stderr 代理警告在 PS 5.1 重定向下产生 RemoteException 记录（不影响赋值，但 collect.log 里吓人）；真凶是 WPR "profiles already running"（管理员上下文残留 session）+ **WSLg 日志收集无超时地挂起**（官方脚本 350 行 Start-Job + wsl --system，第一次卡 10+ 分钟）。
  2. 修复：superUser 硬编码 root + 提权 wrapper 先 `taskkill` 旧进程 + `wpr -cancel` + `pktmon stop` 清残留；WSLg 收集第二次超时自动跳过（脚本有 60s 超时？），正常完成。
  3. wsl.wprp 缓冲 512KB 不会爆盘（与之前 95GB System Collector 事故不同）；官方脚本有 wpr -start 失败重试逻辑。
- **录制窗口内复现数据**（repro_during_capture2.log）：8 轮 churn，前 3 轮 guest 12ms 成功但 Windows 30s 不可达，第 4-8 轮 guest 侧也 STALL；固定端口 28999 guest 1ms / Windows 不可达。与"新端口注册失效"一致。

### 阶段八：v3 评论已正式发送（2026-08-18 22:40）

- 正式评论：微软 WSL 回环 issue（编号见桌面 11 号文档） issuecomment-5329767813（7306 字节，作者 3900563672，UTC 14:40:05Z）
- 演练 issue #58（用户仓库）渲染校验后删除（GraphQL deleteIssue；本次踩坑：gh api graphql 变量 `$id` 在 bash 双引号/gh 间转义失败，改用 query 内联字面量解决）
- 观察窗口：~09-01；官方无回应 → 独立 issue 路线（探针 `/tmp/probe_repro.go` 已备，200 轮直方图待健康态系统）
- 文档同步：桌面 11/13 + Documents 副本已更新

### 阶段九：官方日志包分析——复现现场提取结果（2026-08-18 深夜至 2026-08-19 凌晨）

- **复现成功且失败现场被官方日志包收录**：WPR trace（logs.etl，22:27:43→22:30:16，8,015,751 事件 0 丢失）内确认探针 repro2 前 3 轮（43723/39489/39027，半黑洞）的 Windows 侧连接失败事件；第 4-8 轮（STALL）无 Windows 侧事件（符合探针逻辑：guest 未连上 → 不跑 wincheck）。
- **决定性运行时证据**（trace 全量解码后提取，1,312 条消息归档 `Desktop\WSL\logs\evidence/wpr-probe-evidence.txt`）：TCPIP `requested to connect` 36 条；IP `Transmitting loopback Nbl` 每端口 55 条 Dst + 44 条反向 Src（中继虚拟回应）→ "虚拟握手黑洞"的运行时实证，且位于微软官方日志包内。
- **pktmon 负结果（方法论）**：两个 pktmon 共 53.8 万包，中继路径帧为 0 → Consomme 中继（wsl_devicehost/dllhost 用户态栈）不经过 NDIS/vmswitch 可见组件；pktmon 对该问题无效，证据只能来自 ETW/WPR。
- **环境异常**：HNS "WSL (Hyper-V firewall)" 每次启动创建失败 0x80070002（hns_events.log 17:53/18:30/18:31/22:18/22:27 全失败），vEthernet 对应网卡 Not Present；Consomme 不依赖 HNS，仅作环境备注。
- **trace provider 清单**：HvSocket 286.5 万、TCPIP 65.9 万、wsl_devicehost 2.6 万、FlowSteering 3 万、HNS 70、WSL 服务 WPP 数百万条；PortTrackerServer_WPP 未录（与"本机不跑 wslrelay"一致）。
- **时间线修正**：22:18:40 第一次收集 WPR 启动失败（残留会话）但 pktmon 录 8.5 分钟（34.9MB，445,657 包）；22:27:16 recollect 清残留；22:27:23 第二次收集（最终包）。探针 repro2 前 3 轮落在 WPR 窗口内（各 25 条字段事件）。
- 详细文档：桌面 16_日志包分析结果.md（Documents/ 有同步副本）；8.8GB 解码 XML 已删（C 盘保护），可重新生成。

### 阶段十：DLL 签名核验 + 复现手册 + 回滚指南（2026-08-19 凌晨，#61/#60 完成）

- **#61 wsldevicehost.dll 核验完成（决定性）**：DLL 位于 `C:\Program Files\WSL\wsldevicehost.dll`（v1.2.14.0，签名 Valid，**当前被 dllhost 加载**）。字符串证据：96 处 `oss\vm\devices\net\net_consomme\...` 源码路径、`Duplicate TCP bind for port` 翻译串（开源仓无此串 → 闭源 glue 译 PortAlreadyBound 的证据）、TCP 状态机字段串（FinWait2/TimeWait/proxy_for_guest_port）、带行号 WPP 事件名（consomme::tcp::message_error 等）、微软内部 CI/Cargo 路径（`C:\__w\_temp\msrustup_home\...`、`microsoft.pkgs.visualstudio.com-8dd33d26ad55b096\`，rustc 1.94.0-ms-20260303）→ "可能包含"升级为"**已确认包含**"。对照组 wslrelay.exe 0 命中（旧架构残留）；两二进制导出表均 0 项（导出比对不适用）。局限：WPP 行号与 openvmm master 漂移（DLL 构建 2026-04），无法定位精确 commit；glue 仍闭源。文档：桌面 + Documents `17_dll签名核验.md`。
- **#60 复现手册完成**：12 项实验（E1 基线/E2 健康瞬态/E3 固定端口/E4 200 轮直方图/E5 双进程/E6 剂量响应/E7 端口保留/E8 退化态识别/E9 官方日志/E10 WPR 解码/E11 vsock 对照/E12 恢复验证）→ 命令/预期/耗时/依赖态矩阵 + 7 条常见误判清单 + 证据索引。退化态明确标注"**不可一键复现**"（数小时压力、概率性）。文档：桌面 + Documents `18_实验复现手册.md`。
- **回滚指南完成（用户强调的重点）**：敏感操作矩阵（授权要求）、10 秒快速健康检查、WSL 退化态恢复（引用 15 号）、WSL 版本更新/回滚（`wsl --update` / msixbundle + `Add-AppxPackage`）、Docker Desktop 与内置 K8s 恢复（节点不自动恢复）、**数据盘 Junction 检查（docker ps 全空先查 Junction，禁止重置）**、C 盘防护（WPR 成对启停/95GB 事故/删除走 WSL rm）、实验回滚清单、明天 #62 恢复序列（先不起 Docker 测 8 轮 → 起 Docker 复测 → 200 轮直方图）。文档：桌面 + Documents `19_回滚与恢复指南.md`。
- issue 状态：#61/#60 完成（已评论 + 关闭）；#62/#63 留待用户明天决定（#62 需授权 wsl --shutdown；#63 需拍板是否动本机 WSL）。
