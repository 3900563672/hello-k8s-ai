# 失败模式注册表（Failure Registry）

> 维护层：agent ｜ last-reviewed：2026-08-19 ｜ 事实源：docs/lessons/、docs/journal/、change-history/
> 用法：**每次开工扫本表末尾 3 条**（末尾=最新），检查本次任务是否触及相同代码路径 / 产物类型；命中先看证据链再动手。

## 读取约定

- 每条 = 现象 / 触发条件 / 根因 / 必须动作 / 证据链 / 状态。
- 状态：`active`（仍会踩，靠自觉）／`guarded`（已被机器门禁拦截：lint / doctor / CI / 硬规则）。
- 新增条件：同一现象第二次出现，或教训被验证可复用；一次性环境偶然不登记。
- 追加：新条目写在**文件末尾**（"最后 3 条" = 最新 3 条 = 最高危）。已 guarded 的条目保留，证明门禁存在。

## 注册表

### FR-001 交付"假完成"：改了码没实跑就声称验证（2026-08-19 建档）
- 现象：改动无运行证据，汇报里却写"已通过"；长跑/高负载项用短测结果冒充。
- 触发条件：任何交付前。
- 根因：验证靠自觉，无强制清单。
- 必须动作：交付前逐项勾 done-check 三问——实跑命令？输出证据？未验证范围？勾不上不算完。
- 证据链：WORKFLOW 8.6、AGENTS.md 交付检查。
- 状态：guarded（done-check 已写入 WORKFLOW/AGENTS）

### FR-002 引用外部 issue 编号泄露 cross-reference（2026-08-19 建档）
- 现象：提交/PR/issue 正文含外部 `#数字`，GitHub 自动在对方时间线登记，第三方可见，删除源才消失。
- 触发条件：提交信息、PR/issue 标题正文评论、diff 中出现外部编号。
- 根因：GitHub 的 cross-reference 机制 + 习惯性写编号。
- 必须动作：外部 issue 一律用描述语或 URL；提交前检查 git diff 与提交信息。
- 证据链：docs/journal/2026-08-19-github-crossref-external-issue-number.md
- 状态：guarded（WORKFLOW 5 已硬规则）

### FR-003 告警/规则只验证"加载成功"未验证"真实触发"（2026-08-19 建档）
- 现象：内存告警对无 limit 容器假阳性、重启告警永不触发；规则加载 OK 但语义错。
- 触发条件：新增/修改 Prometheus 告警规则、Grafana 表达式。
- 根因：验证停在做完即检查，没做触发演练。
- 必须动作：规则改动必须 `promtool check rules` + 表达式实时查询 + 触发演练（停组件/改值观察告警出现与恢复）。
- 证据链：docs/lessons/observability-prom-memory-alert.md、observability-prom-restart-alert.md
- 状态：active

### FR-004 长时/高负载才暴露的资源上限（2026-08-18 建档）
- 现象：ENOBUFS（100+ Pod）、模拟器无资源 limit（宿主 OOM 风险）、PVC 单副本——短测不爆长跑爆。
- 触发条件：长时运行、大副本数、大并发验证。
- 根因：验证范围没覆盖峰值；资源核算缺失。
- 必须动作：长跑前 `make doctor` + preflight；大列表 kubectl 输出分页/限长；模拟器类负载必须带资源 limit。
- 证据链：docs/lessons/process-kubectl-enobufs.md、simulator-scale-node-capacity.md、observability-pvc-single-replica.md
- 状态：active

### FR-005 环境层脆弱且不自愈（2026-08-18 建档）
- 现象：节点容器强杀/重建后 CNI 混乱、双节点同 IP、宿主机睡眠冻结 WSL、端口冲突、Docker bind 丢 PVC。
- 触发条件：重启 Docker Desktop / WSL / 整机后继续开发。
- 根因：环境层无自愈，恢复依赖 SOP 手工执行。
- 必须动作：恢复序列按文档走（docker start 5 节点 → umount → chown → 删坏 Pod）；开工先 `make doctor`。
- 证据链：docs/journal/2026-08-18-docker-bind-pvc-loss.md、docs/lessons/deploy-docker-desktop-k8s-recovery.md、deploy-docker-data-junction.md
- 状态：guarded（doctor 已覆盖磁盘/Docker/回环/端口/内存/tmpfs）

### FR-006 长跑结束不清理，负载复活（2026-08-18 建档）
- 现象：cluster-down 后再 apply 全量清单，controller 按 CR 重建负载；replicas=0 不是停止态。
- 触发条件：任何长时运行结束、cluster-down、内存回收。
- 根因：CR 是声明式事实源，controller 必然收敛到 CR 描述的状态。
- 必须动作：长跑结束必须① `make cluster-down` ② 删 `TenantModelPolicy` ③ 验证只剩系统组件 ④ 确认内存回落。
- 证据链：docs/lessons/deploy-cluster-down-revive.md
- 状态：active

### FR-007 跨平台文件写入污染（BOM/CRLF/执行位）（2026-08-19 建档）
- 现象：`Set-Content` 写 BOM 破坏 gen-docs.py 解析；UNC 访问导致整批 `100755→100644` 幽灵差异。
- 触发条件：Windows 侧 PowerShell 写仓库文件、Windows Git 操作仓库。
- 根因：PowerShell 5.1 强制 BOM；9P/UNC stat 执行位不稳定。
- 必须动作：仓库文件一律 WSL 侧 Python `io.open(..., encoding="utf-8")`；`git config core.fileMode false`；提交前 `head -c 3` 抽查 + `git diff --summary`。
- 证据链：docs/lessons/process-cross-platform-file-hygiene.md
- 状态：guarded（规则已写入 WORKFLOW 5 / AGENTS）

### FR-008 PowerShell 多层转义拆命令（2026-08-18 建档）
- 现象：`wsl -d Ubuntu -- bash -lc '...'` 里 `$`、引号、反引号被拆坏：变量为空、语法错误。
- 触发条件：复杂命令（变量/循环/嵌套引号）经 PowerShell→wsl 直传。
- 根因：三层解析（工具→PowerShell→bash）各做一次转义，嵌套越深越容易坏。
- 必须动作：复杂命令先写脚本文件再执行；脚本用 LF；写后 `bash -n` / PS 解析器校验；单条简单命令保持引号最小化。
- 证据链：docs/lessons/process-wsl-powershell-quoting.md
- 状态：guarded（lint-sh / lint-ps1 已接入 make verify）

### FR-009 开工不扫坑位就动手，重复踩已归档的坑（2026-08-18 建档）
- 现象：文档里写过的坑（sleep-guard、回环、cluster-down 复活、CI 轮询节奏）反复出现。
- 触发条件：每次任务开工。
- 根因：教训是"提醒"不是"强制"，AI 每次新上下文不保证读全。
- 必须动作：开工按 WORKFLOW 第 1 节顺序：AGENTS.md → 本表末尾 3 条 → journal/lessons 相关条目 → 再动手。
- 证据链：docs/agents/README.md、WORKFLOW.md 第 1 节。
- 状态：guarded（已写入 AGENTS.md 必须项）

### FR-010 等待空转，长阻塞期间无事可做（2026-08-19 建档）
- 现象：CI/集群/构建等待时干等，用户看到"死等"。
- 触发条件：任何预计 >30s 的阻塞。
- 根因：没有并行工作清单。
- 必须动作：等待期间至少推进一件有用工作（查历史/查网/沉淀/清理）；先查证预期时长再定轮询节奏；长等待后台化。
- 证据链：docs/lessons/process-no-idle-wait.md、WORKFLOW 4.3。
- 状态：guarded（WORKFLOW 4.3 已硬规则）

### FR-011 环境自检缺失，问题"跑起来才爆"（2026-08-19 建档）
- 现象：磁盘爆、Docker 没起、回环降级、端口冲突——都是跑 30 分钟实验后才暴露。
- 触发条件：开工/长跑前未做环境检查。
- 根因：检查项散落在多个文档，没有一条命令的总入口。
- 必须动作：开工/长跑前先 `make doctor`（磁盘/Docker/回环/端口/内存/tmpfs/dmesg）；深度体检再 `bash hack/preflight.sh`。
- 证据链：hack/doctor.sh、hack/preflight.sh。
- 状态：guarded（make doctor 已接入）

### FR-012 脚本/命令未过静态检查就上线（2026-08-19 建档）
- 现象：night69.ps1 时间上限判断写错（06:30 只对 6 点生效）；脚本漏定义变量；引号被拆。
- 触发条件：改动/新建任何 .sh、.ps1、.mjs、多行命令。
- 根因：bash -n 只查语法不查语义；工具链无静态检查门禁。
- 必须动作：脚本类改动必须过 `make lint-sh lint-ps1`（已并入 `make verify`）；markdown 改动过 `make lint-md`。
- 证据链：docs/lessons/process-wsl-powershell-quoting.md、process-cross-platform-file-hygiene.md
- 状态：guarded（lint-sh/lint-md/lint-ps1 已接入 make lint/selfcheck/verify + CI）

### FR-013 WSL 回环中继降级不自知（2026-08-18 建档）
- 现象：新端口首连被拒、时好时坏；本地测试环境性失败被误判为代码问题。
- 触发条件：WSL 重启/Docker Desktop 重启/宿主机睡眠后；本地跑依赖 localhost 的测试。
- 根因：回环中继（dllhost/wsldevicehost）状态不稳定，无主动探测。
- 必须动作：`make doctor` / preflight 内置 `hack/wsl-loopback-probe`；环境性失败先自连一次完成端口注册再判。
- 证据链：docs/lessons/process-wsl-loopback-fresh-listen-refused.md、docs/operations/WSL_LOOPBACK_CASE_STUDY.md
- 状态：guarded（probe 已接入 doctor/preflight/selfcheck）
