# 已知坑位清单

> 维护层：agents ｜ 最后同步：2026-08-18 ｜ 对应变更：change-history/2026-08-18-host-toolchain-recovery/
> 记录格式：现象 → 原因 → 解决 → 验证 → 日期。新坑按日期倒序追加到对应主题。
> 这是"踩坑即记录"的流水账，不替代 `docs/` 的正式说明。
> 领域层面的"已知易误判点"（来自原 AI_CONTEXT 第 8 节）见本文最后一节。

## 宿主与工具链（2026-08-18）

### 2026-08-18 Codex 应用更新后 notify 路径过期，exec 报 helper_unknown_error
- 现象：整机重启后 Codex 桌面应用反复重启（15 分钟内 8 个实例）；exec 工具报 `helper_unknown_error: setup refresh had errors`（cmd.exe / pwsh.exe 全部被拒）；回合结束尝试启动不存在的 `codex-computer-use.exe`。
- 原因：应用自动更新运行时（`cua_node/2f053e67fec2d258` → `1cb4becc994cbb02`），`C:\Users\hh\.codex\config.toml` 里 `[model_providers.deepseek] notify` 仍是旧路径；重启后沙箱初始化窗口（`windowsSandbox/setupStart` → `cap_sid` 生成，约 30-60 秒）内 exec 一律失败；重启早期 github.com 443 超时（代理未就绪）导致应用反复重启。
- 解决：notify 路径改为现有运行时 `1cb4becc994cbb02`（与顶层 notify 一致），改前备份 `config.toml.bak-20260817-2359`；沙箱初始化完成后 exec 自愈。
- 验证：notify 目标文件存在；exec 与 node_repl 正常；DeepSeek API 实测 `deepseek-chat` 仍是别名（返回 `deepseek-v4-flash`），`/models` 当前仅 `deepseek-v4-flash` / `deepseek-v4-pro`。
- 备注：`[windows] sandbox = "elevated"` 是官方合法值（elevated/unelevated），不是错误；`thread_tools` 未知 feature 警告与远程插件 401 均为无害噪音。

### 2026-08-18 整机重启后环境恢复顺序（Docker Desktop / 内置 K8s / 端口转发）
- 现象：重启后 `docker-desktop` WSL 发行版 Stopped；引擎起来后 kubectl 一度报 `nodes is forbidden`（RBAC 未就绪）；controller-manager Pod Error（`dial tcp 10.96.0.1:443: i/o timeout`）；`localhost:8080` 无监听（port-forward 进程随重启消失）。
- 原因：Docker Desktop 引擎需手动启动；内置 K8s 节点容器逐个拉起约 40-60 秒（期间 API server 可连但 RBAC/controller 未就绪）；controller-manager 在 API 未就绪时启动即退出；端口转发是前台进程，重启即丢。
- 解决顺序：启动 Docker Desktop → 等 `docker version` server 非空 → 等 `kubectl get nodes` 全 Ready（约 40s）→ `kubectl -n hello-k8s-ai-system rollout restart deploy/hello-k8s-ai-controller-manager` → `make cluster-open`（恢复 8080/18080 单入口）→ `/api/v1/health/ready` 验收。
- 验证：10 worker 节点 Ready；controller/backend/frontend/可观测组件全 1/1；ready API 200；PostgreSQL 历史完整（34.6 万事件、2223 快照）。
- 备注：Ubuntu 发行版重启后自动 Running；Kind 集群（e2e/minikserve）容器自动恢复；`make cluster-down` 只缩负载不删 CR，重启后 CR 全在。

### 2026-08-18 gocyclo 圈复杂度 31 > 30：给高复杂度函数加分支前先评估
- 现象：CI lint 失败 `gocyclo: cyclomatic complexity 31 > 30`（`internal/controller/simulatorinstance_placement.go`）。
- 原因：`reconcileDeploymentObjects` 原有复杂度 ~28（计划解码/校验/物化多职责叠加），修"replicas=0 与放置计划失配死锁"时内联"暂停清理"分支（多条件 && 链 + Update 错误处理）后越过阈值。
- 解决：提取 `clearPlacementPlanWhenPaused` helper（提交 `d4dc41c`），主函数只留一行调用，逻辑零变化。
- 验证：CI 全绿；`TestSimulatorInstancePauseClearsPlacementPlan` 通过。
- 备注：改这类接近阈值的函数前先 `gocyclo` 评估；超阈值优先按职责拆函数，不用 `//nolint` 掩盖。

## 宿主内存治理（2026-08-17）

### 2026-08-17 整机内存爆满根因链（Commit 打满 → C 盘 pagefile 暴涨）
- 现象：物理内存 31.4GB 被占满（空闲 0.4GB），C 盘被 pagefile 吃掉 20-30GB，WSL 内 kubectl 频繁超时（Wsl/Service/0x8007274c），Commit Charge 打满 68.7/68.7GB。
- 原因（四层叠加）：
  1. 无 `.wslconfig`：WSL2 默认占宿主 50% 内存（15.7GB）且**永不归还**（vmmem 只增不减，`autoMemoryReclaim` 未开启）；
  2. Docker Desktop 内置 K8s 配置 `KubernetesNodesCount=10`：10 个节点容器（kubelet/containerd/kindnet）本身吃掉 VM 内 8-10GB；
  3. 长跑测试遗留 `SimulatorInstance` CR（`spec.replicas=200`、rate=20）一直没清理，200 个模拟器 Pod 吃 5-8GB；
  4. Jaeger limit 512Mi < badger 默认 BlockCacheSize 256MB + MemTable 64MB，反复 OOM CrashLoop 加剧压力。
- 解决：`wsl --shutdown` 回收全部 WSL 内存（vmmemWSL 9.6GB→0.5GB）；新建 `.wslconfig`（`memory=12GB` + `autoMemoryReclaim=gradual`）；关闭 Docker AI（`EnableDockerAI=false`/`InferenceCanUseGPUVariant=false`，备份在 settings-store.json.memguard.bak）；`make cluster-down` + CR 副本归零清掉 200 Pod；Jaeger limit 升 1Gi + `GOMEMLIMIT=805306368`（768Mi）。
- 验证：空闲内存 0.4GB→9.9GB；负载清零后 VM 内 10 节点仍占 ~11.7GB（12GB 上限），证明**节点数必须缩减**（见 RESILIENCE.md 内存预算节）。
- 备注：Windows 自动管理 pagefile 会保留峰值大小（实测分配 38GB、当前仅用 3GB），C 盘被占是正常机制不是泄漏；固定大小需重启电脑，列为待办。

### 2026-08-17 GOMEMLIMIT 不接受 K8s 风格 Mi 后缀（malformed GOMEMLIMIT）
- 现象：Jaeger 容器启动即崩：`fatal error: malformed GOMEMLIMIT; see go doc runtime/debug.SetMemoryLimit`，CrashLoopBackOff。
- 原因：`GOMEMLIMIT` 是 Go runtime 环境变量，只接受**十进制字节数**（如 `805306368`），`768Mi` 是 K8s 资源格式，Go 不认。
- 解决：manifest 写字节数 `805306368`（=768Mi）并注释说明。
- 验证：修正后 Jaeger 正常 Ready（0 重启）。
- 备注：Go 相关 env（GOMEMLIMIT/GOGC）一律查 `go doc runtime/debug.SetMemoryLimit` 再写，不要套 K8s 单位。

### 2026-08-17 SimulatorInstance replicas=0 不是"停止"态（已修复失配死锁；停止请删 TenantModelPolicy）
- 现象：把长跑遗留 CR `spec.replicas` 从 200 改为 0 后，controller 持续报错：`simulator instance "tenant-core-model-lite" has 0 replicas but its node placement plan contains 200`，reconcile 直接失败、不缩容。
- 原因：replicas 与持久化放置计划（注解 `platform.study.com/node-placements`）失配时的一致性校验过于严格：`replicas=0` 是**合法最小值**（新实例骨架就是 0，Orchestrator 按流量扩容），但旧计划非空时校验死锁，无法缩也无法扩。
- 解决（已修复）：`replicas=0` 时先清除历史放置计划注解再按空计划收敛（Deployment 缩 0、清理逐节点 Deployment）；Orchestrator 对 0 副本实例不再报错、暂停实例不参与资源预留；新增测试 `TestSimulatorInstancePauseClearsPlacementPlan`。
- **重要结论（实测）**：`replicas=0` 不是"暂停"——Orchestrator 看到流量（qps>0）会从 0 自动扩容。**停止一个实例的正确方式是删除其 TenantModelPolicy**（Deny 时 `reconcileTenantModelPair` 自动删除 SimulatorInstance 并清理 Deployment）；只删 SimulatorInstance 会被策略立即重建。
- 验证：删除 `tenantmodelpolicy tenant-core-model-lite` 后实例/Deployment/Pod 全部消失，Tenant/Model/WorkerNode 保留；新 controller（修复后）0 报错。
- 备注：集群层面"全部停止"仍用 `make cluster-down`（停 controller 与全部工作负载）。

### 2026-08-17 cluster-down 后 kubectl apply config/dev 会复活 controller 并按 CR 重建模拟器
- 现象：`make cluster-down` 后负载确实归零；但随后为部署 Jaeger 修复执行 `kubectl apply -f config/dev`，controller-manager 恢复 1 副本，Reconcile 看到 `SimulatorInstance.spec.replicas=200` 立即重建全部 200 个模拟器 Pod。
- 原因：`stop_stack()` 只缩 Deployment 不删 CR；apply 全量清单把 controller 拉回，CR 是用户配置，controller 忠实执行。
- 解决：先处理 CR（`replicas=0` 或删除）再恢复 controller；或 `cluster-down` 后不要直接 apply 全量清单，只 apply 目标组件。
- 验证：CR 归零后 apply 不再拉起模拟器。
- 备注：沉淀进 WORKFLOW 4.2 长跑结束清单。

### 2026-08-17 .wslconfig 会被 Docker Desktop GUI 内存设置覆盖
- 现象/原因：Docker Desktop 资源设置在 GUI 修改后会重写 `%USERPROFILE%\.wslconfig`，手动写入的 `memory`/`autoMemoryReclaim` 可能丢失。
- 解决：`%USERPROFILE%\.wslconfig` 是唯一内存治理入口（当前 `memory=12GB` + `autoMemoryReclaim=gradual`），改 Docker GUI 内存后必须同步本文件；已在本文件注释说明。
- 验证：wsl --shutdown 后 `vmmemWSL` 峰值从 15.7GB 降到 12GB 上限。

## 可观测性与存储

### 2026-08-17 Prometheus 单副本 + RWO PVC：滚动更新 TSDB 锁冲突 CrashLoop（Recreate 策略）
- 现象：给 Prometheus Deployment 换配置后 `rollout restart`，新 Pod CrashLoopBackOff，日志 `Fatal error: opening storage failed: lock DB directory: resource temporarily unavailable`；旧 Pod 仍 Running 持锁，新 Pod 抢不到同一 PVC 上的 TSDB 锁。
- 原因：默认 RollingUpdate 先起新 Pod 后停旧 Pod；单副本 + RWO PVC 时新旧 Pod 同时挂载同一数据卷。
- 解决：Deployment `strategy.type: Recreate`（先删后建），配置变更直接 `rollout restart` 即可；与 Jaeger badger 的 scale-to-zero 同类问题，Prometheus 用 Recreate 免去手工缩扩。
- 验证：改为 Recreate 后 `successfully rolled out`，TSDB 数据保留（重启前序列重启后仍可查）。

### 2026-08-17 preflight 实现：grep -v 无匹配在 pipefail + set -e 下返回 1 导致脚本静默退出
- 现象：`NOT_READY=$(... | grep -v ' Ready' | wc -l)` 在全部节点 Ready 时，`grep -v` 无匹配返回 1，配合 `set -Eeuo pipefail` 直接中止脚本——"无问题"反而让体检退出。
- 解决：命令替换末尾加 `|| true`（`... | wc -l || true`），并确认 `NOT_READY` 是 0 而不是空串。
- 验证：10 节点全 Ready 时 preflight 正常输出 `19 通过 / 0 失败 / 1 警告`。

### 2026-08-17 preflight 实现：bc 可能不存在，浮点比较用 awk
- 现象：体检脚本若用 `bc` 比较 Windows 空闲内存会因环境缺 bc 失败；`awk 'BEGIN{exit !(f < x)}'` 可直接在 `if` 里做浮点判断且无外部依赖。
- 解决：`awk -v f="$FREE_GB" 'BEGIN{exit !(f < 1.0)}'`；先校验输入是数字（`=~ ^[0-9.]+$`）再比较。
- 验证：Windows 空闲内存 3.5GB 时正确落在 WARN（<3GB 假）之外、PASS 分支。

## 夜间长时运行（night-run）
### 2026-08-17 宿主机空闲 15 分钟自动睡眠会冻结 WSL（值守事故根因）
- 现象：2026-08-17 00:50 后值守会话与 keepalive 全部停滞约 7 小时，07:48 恢复；keepalive.log 无新检查记录、sleep 等待命令 7 小时不返回；恢复后系统本身无故障（18 Pod 全 Running）。
- 原因：Windows 电源计划"平衡"下交流空闲 15 分钟自动睡眠（powercfg 实测 STANDBYIDLE AC=900s）、3 小时自动休眠（HIBERNATEIDLE=3h）；睡眠冻结整个 WSL VM。
- 解决：已落地 `hack/night-run/sleep-guard.ps1/.sh`（方案 A，2026-08-17 预授权执行 `on`，当前 guard=on）：`bash hack/night-run/sleep-guard.sh status|on|off`，on/off 走 UAC 提权并写 `%TEMP%\sleep-guard.log`；原值存 `%LOCALAPPDATA%\night-run-sleep-guard.json`。提示词前提从"App 必须保持运行"扩展为"App 运行 + 宿主机不睡眠"；PowerToys Awake 可作免管理员备选（方案 B，未选）。
- 注意：`powercfg /change` 只认 `standby-timeout-ac`/`hibernate-timeout-ac` 专用名，不认 `STANDBYIDLE` 等 SUB_SLEEP GUID 别名（实测报"参数无效"）。
- 验证：07:48 唤醒后系统自动恢复，队列自动回排；未实测禁用睡眠后的整夜值守。
- 备注：合盖行为由"合盖操作"设置单独控制，改空闲睡眠不影响合盖逻辑。

### 2026-08-17 nohup 挡不住 exec 会话进程组回收，keepalive 必须 setsid 启动
- 现象：`nohup node keepalive.mjs --loop ... &` 启动的常驻进程（PID 120061）在命令会话结束后静默消失，日志无第二轮检查、无错误输出。
- 原因：Codex exec 在命令返回后回收该会话的进程组，nohup 只能忽略 SIGHUP、挡不住进程组终止。
- 解决：`setsid nohup node ... < /dev/null >> log 2>&1 &`（setsid 脱离会话）；已在 phase_a_prompt.md 与 hack/night-run/README.md 同步。
- 验证：setsid 重启（PID 122828）跨命令存活，首轮检查全绿。

### 2026-08-17 snapshot.mjs 漏定义 sleep 导致采集崩溃（node --check 查不出）
- 现象：`node hack/night-run/snapshot.mjs --once --summary` 在 port-forward 偶发失败走重试路径时崩溃：`ReferenceError: sleep is not defined`（snapshot.mjs:47），整份快照丢失。
- 原因：从 keepalive.mjs 拷贝 httpGet 时漏拷 `const sleep` 辅助函数；`node --check` 只查语法不查运行时引用，所有 fetch 首试成功时脚本可跑通，掩盖了 bug。
- 解决：snapshot.mjs 补 `const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));`。
- 验证：修复后实跑 `snapshot.mjs --once --summary` 成功（2026-08-16T23:55:54Z 快照 ok:true）。
- 备注：同类脚本改动后除 `node --check` 外，至少实跑一次触发重试路径（或直接跑通一次）。


### 2026-08-16 kubectl port-forward 偶发连接复用失败
- 现象：Node fetch 连续请求 Backend 时偶发 `TypeError: fetch failed`，curl 同时刻正常。
- 原因：port-forward 单监听 127.0.0.1:8080，Node 长连接复用被后端/隧道偶发重置。
- 解决：脚本 httpGet 网络层错误自动重试 3 次（间隔 500ms）；keepalive 启动阶段健康探针失败最多恢复 3 轮（间隔 10s）。
- 验证：重试后快照/健康检查稳定输出全绿。

### 2026-08-16 自动化会话模型：cron 型每次触发都是全新会话
- 现象/原因：project 型 cron 自动化按 rrule 触发新会话，不复用既有会话；thread heartbeat 型才会复用指定线程。
- 解决：提示词必须自包含（先读 AGENTS.md / README / phase prompt / problems.md），信息落仓库与 `.runtime/`；不要假设新会话有对话记忆。
- 验证：`$CODEX_HOME/automations/<id>/automation.toml` 逆向 app.asar 确认 `kind="cron"` + `target={type="project"}` 语义。

### 2026-08-17 自动化 TOML 必须显式写 model，否则用默认模型（自定义 Provider 会失败）
- 现象：`night-run-phase-a-keepalive` 00:00 触发后只创建了线程（`state_5.sqlite` threads 表可见 `01a00b4d`，`model` 列为 None），turn 未启动，任务"没跑起来"；UI 显示模型为 SOL。
- 原因：用户全局 `config.toml` 是自定义 Provider（`model_provider="deepseek"`、`model="deepseek-chat"`），但手写 automation.toml 未带 `model` 字段，自动化会话回落到 App 默认模型（SOL，用户实际不可用），线程启动失败。
- 解决：automation.toml 显式加 `model = "deepseek-chat"`、`reasoning_effort = "max"`（与全局配置一致）。
- 验证：tomllib 解析通过；最早实测点为 04:30 Phase B 空跑（若线程 `model` 列为 deepseek-chat 即生效），其次次日 00:00 Phase A。
- 备注：用户"先创建会话再指定会话"的 heartbeat 方案可作备选（复用线程模型、不涉及默认模型），但会累积线程上下文。

### 2026-08-16 自动化触发前提：Codex 桌面 App 必须保持运行
- 现象：App 退出/合盖时到点不触发自动化。
- 解决：夜间运行前确认 App 保持运行；Phase A 开工先拉起 nohup 常驻 keepalive，会话中断脚本仍继续。
- 验证：首次夜间运行（2026-08-17 00:00）实测。

### 2026-08-16 Windows 宿主没有 gh，GitHub 操作必须在 WSL 内执行
- 现象：Codex 自动化/会话的工作目录是 `\\wsl.localhost\...`（UNC），Windows 侧 `gh` 不存在（`C:\Program Files\GitHub CLI\gh.exe` 也没有）。
- 解决：一切 git/gh 操作走 `wsl -d Ubuntu -- bash -lc "..."`；WSL 内 `gh` 已认证（account 3900563672，scopes 含 repo）。
- 验证：PR #24 创建成功。

### 2026-08-16 gh 偶发 TLS handshake timeout（代理链路）
- 现象：`gh pr create` / GraphQL 请求偶发 `TLS handshake timeout`，重试即成功；与 WSL http_proxy=127.0.0.1:7890（Clash）链路有关。
- 解决：失败后等 5–8 秒重试，最多 5 次；不要怀疑认证或重复创建 PR（会报 "a pull request already exists"）。
- 验证：PR #24 第 2 次尝试成功。

## 命令与终端（WSL / Windows 宿主）

### 2026-08-16 apply_patch 无法写 UNC 路径
- 现象：在 `\\wsl.localhost\Ubuntu\...` 工作目录用 `apply_patch` 写文件，报 "UNC paths are not supported. Defaulting to Windows directory." 后 "Access is denied"。
- 原因：apply_patch 底层走 cmd.exe，不支持 UNC 当前目录。
- 解决：写新文件用 PowerShell here-string + `[System.IO.File]::WriteAllText`（UTF-8 无 BOM）；复杂脚本内容多时 base64 传 WSL。
- 验证：本次 UI 验证链路文档与脚本全程使用该方式。

### 2026-08-16 PowerShell 直传 wsl.exe 引号被拆
- 现象：`wsl.exe -d Ubuntu -- bash -lc "..."` 内含引号或括号时 bash 报 `syntax error`。
- 原因：Windows 到 wsl.exe 的原生参数传递会重排引号，内部双引号被拆开。
- 解决：脚本内容 base64 编码后 `echo <b64> | base64 -d > /tmp/x.sh && bash /tmp/x.sh`；含中文或引号的脚本一律走 base64。
- 验证：本次文档体系交付全程使用该方法无失败。
- 备注：wsl 启动时的 "localhost 代理未镜像" 是环境噪音，可忽略。

### 2026-08-16 git commit 中文提交信息丢失
- 现象：PowerShell 传参提交后，GitHub 上提交标题只剩英文前缀。
- 原因：Windows → WSL 参数编码吞掉非 ASCII 字符。
- 解决：`echo "提交信息" | base64 -w0 > /tmp/msg.b64 && git commit -F <(base64 -d /tmp/msg.b64)`。
- 验证：`dc5308e`、`89916cc` 中文完整。

### 2026-08-16 Docker Desktop WSL2 数据盘迁移：DataFolder 配置无效，必须用 Junction
- 现象：C 盘 vhdx 占 112GB；在 `settings-store.json` 设置 `DataFolder=D:\DockerData` 并重启后，`docker ps` / `docker volume ls` 全部为空。数据没丢，只是引擎挂到了默认位置的新空盘。
- 原因：Docker Desktop 的 WSL2 引擎硬编码在 `%LOCALAPPDATA%\Docker\wsl\disk` 找 `docker_data.vhdx`，忽略 DataFolder 配置。
- 解决：把 vhdx 移动到目标盘后，在 `%LOCALAPPDATA%\Docker\wsl\disk` 建立指向新位置的目录 Junction。
- 验证：`dir C:\Users\hh\AppData\Local\Docker\wsl\disk` 可见 vhdx 且 LinkType=Junction；`docker volume ls` 恢复 15 个卷。
- 备注：以后容器/卷显示为 0，先查 Junction 与目标 vhdx 是否存在，不要贸然重置 Docker。

### 2026-08-16 禁止 wsl --shutdown：会关闭所有发行版
- 现象：为"清理空间"执行 `wsl --shutdown`，用户的 Ubuntu（跑着 Agent 与项目）被一起关掉。
- 解决：除非用户明确同意，只对单个发行版操作（`wsl -d Ubuntu -- ...`），不执行 `wsl --shutdown`。
- 备注：用户环境约束：不要重启电脑（有 Agent 项目在跑）；确需重启必须先征得同意。

### 2026-08-16 强杀 Docker Desktop 后内置 Kubernetes 不会自动恢复
- 现象：Stop-Process 强杀 Docker Desktop/com.docker.backend 后，desktop-control-plane、desktop-worker* 等内置 K8s 容器全部消失，kubectl 连不上。
- 解决：不要强杀 Docker Desktop；重启走正常流程。恢复需在 Settings → Kubernetes 重新启用（本次随 Docker Desktop 正常重启自愈）。
- 验证：恢复后 10 个节点 Ready，kubectl cluster-info 正常。

### 2026-08-16 非提权进程写不了 D:\ 根目录
- 现象：普通进程在 D:\ 创建目录失败。
- 原因：ACL 只有 Administrators 可写、Everyone 只读。
- 解决：UAC 提权创建目录并 icacls 授权；提权操作写成 .ps1 脚本，用 `Start-Process -Verb RunAs` 执行，脚本写日志到 %TEMP% 后轮询确认。

### 2026-08-16 PowerShell 复杂内联命令被安全策略拦截
- 现象：含 Remove-Item 组合、多层引号嵌套的内联 PowerShell 被安全策略拦截。
- 解决：删除/移动文件改用 Node.js fs 模块；复杂逻辑写成 .ps1 脚本文件再执行。

### 2026-08-16 系统代理（127.0.0.1:7890）不能动
- 现象：代理配置被改后网络全断。
- 解决：保持 ProxyEnable=1 不变；Docker Hub 拉取失败多为瞬时故障（auth.docker.io 超时），预拉 + 重试即可，不要改代理。

## Go 构建与 CRD 生成

### 2026-08-16 gofmt 对齐问题被 lint 拦下
- 现象：`internal/controller/simulationclock_controller_test.go` 等 3 个文件报 "File is not properly formatted (gofmt)"。
- 原因：手工编辑导致对齐与 gofmt 不一致；lint 使用自定义 golangci-lint（`.custom-gcl.yml`）。
- 解决：`make fmt`；lint 前先 `make golangci-lint` 编译带自定义插件的二进制。
- 验证：修复后 `make lint` 通过。

### CRD 修改后必须重新生成
- 现象：只改 `api/v1/*_types.go` 会导致清单与生成结果不一致。
- 解决：`make manifests generate YEAR=2026`；`config/crd/bases/*.yaml`、`config/rbac/role.yaml`、`zz_generated.deepcopy.go` 只由生成器维护，不手改。
- 验证：CI "源码与部署验证" 会核对生成一致性。

## GitHub API 与 gh

### 2026-08-16 gh 偶发 TLS handshake timeout / EOF
- 现象：批量创建 label、编辑 issue 时部分请求失败。
- 解决：失败项重试即可；批量脚本要容忍失败，最后统一核对清单。
- 验证：重试后 17 个 label、5 个 issue 标签全部就位。

### gh 权限不足不等于无数据
- 现象：Projects v2 字段查询报权限错误；SSH keys 列表 404。
- 原因：token scopes 缺 `read:project`、`admin:public_key`。
- 解决：先查 `totalCount` 判断"有没有"；缺 scope 需 `gh auth refresh` 或重新授权，不要根据报错断定无数据。
- 验证：`projectsV2 totalCount=0` 确认真无 Projects。

## GitHub Actions 与 CI

### 2026-08-16 golangci-lint 预置普通二进制会跳过自定义插件编译
- 现象：CI 直接把官方 golangci-lint 二进制放到 `bin/` 后，`make lint` 不再编译 `.custom-gcl.yml` 里的 logcheck 插件，lint 语义悄悄变化。
- 原因：Makefile 的 `golangci-lint` 目标以文件存在性判断是否安装；v2.12.2 官方 release 没有 with-plugins 预编译资产。
- 解决：CI 中先用官方二进制执行 `golangci-lint custom --destination bin --name golangci-lint-custom` 再 `mv` 覆盖 `bin/golangci-lint`，与本地 `make lint` 一致；`bin/` 用 actions/cache 缓存（key 含 `.custom-gcl.yml` 哈希）。
- 验证：golang 容器内完整复现 CI 步骤，`golangci-lint run` 0 issues。

### 2026-08-16 CI 里 go install kind 每次都要编译
- 现象：E2E workflow 用 `go install sigs.k8s.io/kind@v0.32.0`，每次冷编译浪费约 1 分钟。
- 解决：curl 官方 release 预编译二进制（`kind-linux-amd64`）到 `$HOME/.local/bin` 并加入 `GITHUB_PATH`。
- 验证：`kind version` 通过，E2E 不再有 go install 编译阶段。

### 2026-08-16 BuildKit gha 缓存要先 docker buildx create
- 现象：`--cache-from/--cache-to=type=gha` 在 builder 未初始化时不生效或报错。
- 解决：工作流先 `docker buildx create --use --name ci-builder`（`|| true` 容忍已存在）；Makefile 通过 `DOCKER_BUILD_CACHE` 变量注入缓存参数，本地默认空不受影响。
- 验证：镜像构建 job 启用 gha 缓存；注意 `|| true` 会掩盖创建失败，出错时先查 builder。

### 2026-08-16 等待 CI 不要长 sleep
- 现象：Agent 等 CI 用固定长 sleep，用户看到"全部跑完了但还在等"。
- 解决：每 30 秒轮询一次（`gh run list` / `gh run view --json jobs`），预期 3-6 分钟，超过 10 分钟无结论再停下排查；失败取 `gh run view <run-id> --log-failed`。
- 验证：本次交付全程 30 秒轮询，无空等。

### 2026-08-16 墙钟由最慢 job 决定，优化墙钟以下的 job 不改变总耗时
- 现象：lint / controller / verify-deploy 各自提速明显，但整次 push 墙钟仍约 5 分 20 秒。
- 原因：三个 workflow 并行，墙钟 = 最慢 job = E2E（5m22s）；其余 job 本来就在墙钟内跑完。
- 解决：先量各 job 耗时找出瓶颈（`gh run view --json jobs`），再决定优化对象；本次结论是 E2E 的 Go 编译（约 3 分钟）为硬成本。
- 验证：lint 从 2m13s 降到 24s，墙钟不变；E2E 内部并行化后墙钟仍不变。

### 2026-08-16 E2E 并行构建 Go 镜像无墙钟收益（CPU 密集）
- 现象：BeforeSuite 并行构建 manager/simulator 镜像（两个 goroutine），构建阶段仍 2m55s，与串行相同。
- 原因：Go 编译是 CPU 密集，4 vCPU runner 上并行两个编译 = 总 CPU 时间不变。
- 解决：并行仍保留（收益在"与 CertManager 安装重叠"），但不要指望并行编译本身省时间；要省只能复用镜像产物（架构级改动，暂不做）。
- 验证：f111704 实测构建阶段 2m55s（串行基线 1m46s + 1m09s）。

### 2026-08-16 gha 镜像缓存对 Go 编译层无效
- 现象：`--cache-from/--cache-to=type=gha` 参数正确传入、builder 用 docker-container，但层输出几乎没有 `CACHED`（4 个镜像仅 3 层）。
- 原因：Dockerfile 里 `COPY . .` 之后是 `go build`；源码每次提交都变，编译层必然重跑。gha 能缓存的只有 base 镜像与 `go mod download` 层，收益约 1 分钟内。
- 解决：保留 cache 参数（对依赖层有少量收益）；不要把镜像构建提速押在 gha 缓存上。
- 备注：actions/cache 的命中匹配除 key 外还包含由 path 派生的 version；修改 cache path 会导致旧缓存 miss（属正常失效，不要误判为 bug）。

### 2026-08-16 E2E 测试阶段约 1 分钟是就绪轮询
- 现象：E2E 测试执行约 1m17s，其中约 24 秒的 spec 里有 11 次每秒一次的轮询。
- 原因：spec 等待资源就绪（deployment/pod/端点），每次轮询间隔 1 秒，属于稳定逻辑。
- 解决：不要为省时间调小轮询间隔（有偶发失败风险）；如确需优化，应改为"按事件等待"（需要较大重构）。
- 验证：多次运行该 spec 稳定约 24 秒。

### 2026-08-16 E2E 偶发 make undeploy 静默挂起直到套件超时（runner 环境 flake）
- 现象：4ab7ec9 的 E2E 全部用例通过后，`make undeploy` 静默挂起约 6 分钟直到 ginkgo 10 分钟超时；`gh run rerun --failed` 重跑同一 commit 直接全绿。
- 原因：`kubectl delete -k config/default` 在 GitHub runner 上偶发不返回（疑似 kind API 或 docker 环境瞬时故障）；同一代码在其他多次运行中 undeploy 均 15 秒内完成。
- 解决：先重跑确认（`gh run rerun <run-id> --failed`）；若同一 commit 复现再排查，看 `gh run view <run-id> --log` 超时前最后一步与挂起命令。
- 验证：31942621277 重跑 success；369c158 等历史运行 undeploy 正常。

## 可观测性与 Grafana 嵌入

### 2026-08-16 Grafana 13 面板无 data-panelid，屏外懒渲染读不全
- 现象：`querySelectorAll('[data-panelid]')` 返回 0；未滚动 iframe 时 `innerText` 缺下半屏面板（如 Leader 面板）。
- 原因：Grafana 13 面板容器是 `[class*="panel-container"]` 而非 data-panelid；屏外面板懒渲染不产出文本。
- 解决：先 `iframe.contentWindow.scrollTo(0, iframe.contentDocument.body.scrollHeight)` 再读；选择器用 `[class*="panel-container"]`。已内置到 `hack/ui-check/grafana-panels.mjs`。
- 验证：滚动后 12 个面板文本全部可读，Leader 面板 10 行 0/1 完整。

### 2026-08-16 本环境 view_image 不可用，视觉验证以 DOM 读取为主
- 现象：`view_image` 连最小 PNG 都返回 Unsupported Image。
- 解决：截图 + DOM 读取双通道；Agent 判定以 DOM 渲染文本为准，截图复制给用户核实。
- 验证：Leader 面板问题通过 DOM 文本确认（10 行 0/1，hjf2g=1）。

### 2026-08-16 Chrome --screenshot 多 target 报错，统一走 CDP 脚本
- 现象：`chrome --headless=new --screenshot` 报 "Multiple targets are not supported in headless mode"；in-app browser 的 node_repl 报 "failed to write kernel assets: 系统找不到指定的路径 (os error 3)"。
- 解决：统一用 `hack/ui-check/grafana-panels.mjs`（WSL Node + Windows Chrome CDP）：`Page.captureScreenshot` 截图 + `Runtime.evaluate` 读 DOM。
- 验证：脚本输出 12 面板文本 + 1578×902 截图。

### 2026-08-16 Grafana Live WebSocket 经反代握手 400
- 现象：控制台反复 `WebSocket connection to 'ws://localhost:8080/grafana/api/live/ws' failed: ... 400`。
- 原因：Backend 反代未对 `/grafana/api/live` 做 WS 升级，Grafana 自动退回轮询刷新。
- 解决：无需处理（面板 10s 刷新正常）；若要修，反代对该路径放行 Upgrade 头。
- 验证：仅控制台噪音，面板数据正常。

### 2026-08-16 Grafana sub-path 反代必须保留 /grafana 前缀
- 现象：iframe 白屏或显示控制台首页；`/grafana/d/...` 返回 301 到 `http://localhost:8080/grafana/...`，静态资源 404。
- 原因：Grafana 以 `GF_SERVER_SERVE_FROM_SUB_PATH=true` 部署时，页面与静态资源只认 `/grafana/...` 路径；反代剥前缀后 Grafana 把面板页 301 回外部入口，经 nginx SPA fallback 变成控制台首页。
- 解决：Backend 反代保留 `/grafana` 前缀原样转发；`GF_SERVER_ROOT_URL=http://localhost:8080/grafana/` 与前端 iframe 路径必须一致。
- 验证：`curl localhost:8080/grafana/d/hello-k8s-ai-overview` 200 且 HTML 含 `<base href="/grafana/" />`。

### 2026-08-16 Grafana 嵌入开关变量名
- 现象：设置了 `GF_SERVER_ALLOW_EMBEDDING=true`，面板响应仍带 `X-Frame-Options: deny`。
- 原因：`allow_embedding` 属于 `[security]` 段，环境变量是 `GF_SECURITY_ALLOW_EMBEDDING`；`GF_SERVER_*` 前缀对应 `[server]` 段，不生效。
- 解决：改用 `GF_SECURITY_ALLOW_EMBEDDING=true`。
- 验证：改后直接请求 Grafana 无 X-Frame-Options 头。

### 2026-08-16 Backend 安全中间件会覆盖 Grafana 放行
- 现象：Grafana 已放行嵌入，但经 Dashboard 8080 访问面板仍带 `X-Frame-Options: DENY`。
- 原因：Backend `securityHeadersMiddleware` 对所有响应强制加 DENY，包括 `/grafana/*` 反代响应。
- 解决：中间件对 `/grafana/` 前缀跳过 X-Frame-Options；API 路径保持 DENY。
- 验证：`security_headers_test.go` 覆盖两条路径；8080 全链路无 X-Frame-Options。
- 补充（同日）：引入幂等与写认证中间件后，Grafana 前端查询（`POST /api/ds/query`）被 `MISSING_IDEMPOTENCY_KEY` 400 拦截，面板全部 No data。修复：`idempotencyMiddleware` 与 `authMiddleware` 对 `/grafana/` 前缀直接放行（上游 UI 流量不是 Dashboard 命令），API 命令路径保持原有约束。
- 验证：`idempotency_test.go` / `auth_test.go` 覆盖；真实集群 `POST /grafana/api/ds/query` 返回 10 个 frame 时间序列，控制器/编排器/模拟器面板查询均返回 1000+ 数据点。

### 2026-08-16 Grafana 384MiB 内存上限运行中打满
- 现象：集群运行一段时间后 Grafana 探针间歇失败（`context deadline exceeded` / HTTP 503），日志大量 `http: Handler timeout` 与 8-10s 请求超时；RESTARTS 可能仍为 0。
- 排查：`kubectl exec <grafana-pod> -- cat /sys/fs/cgroup/memory.current /sys/fs/cgroup/memory.max`，实测 383MiB/384MiB（99.7%）。
- 解决：`config/observability/grafana.yaml` 限额提到 memory 1024Mi / requests 256Mi / cpu 1000m；其余组件按实测水位判断，不超限不动。
- 验证：滚动后 0 重启、无探针告警，水位稳定约 547MiB/1GiB。
- 教训：不要只看 RESTARTS 与就绪状态判断“意外停止”；先看 cgroup 水位与最近事件。

### 2026-08-16 Docker Desktop kubelet 缓存 :dev 标签 digest
- 现象：重建镜像并 `rollout restart` 后，Pod 仍在跑旧镜像（`imageID` 与本地 `docker image inspect` 不一致）。
- 原因：Docker Desktop 内嵌 kubelet 的 containerd 按标签缓存 digest，`imagePullPolicy: IfNotPresent` 不重新解析。
- 解决：dev 部署清单（backend/frontend）改 `imagePullPolicy: Always`；排查时对比 Pod `imageID` 与本地镜像 ID。
- 验证：改后重启 Pod 的 imageID 与本地一致。

### 2026-08-16 滚动更新会杀死端口转发
- 现象：`kubectl rollout restart` 后 `localhost:8080` 连接拒绝，转发日志报 "network namespace ... is closed"。
- 原因：`port-forward svc/` 在首次连接时 pin 到具体 Pod，Pod 被滚动更新删除后转发即断。
- 解决：部署/重启后重新执行 `make cluster-open`（脚本的存活检查只覆盖"检查时刻"，检查后 Pod 再被删仍会断）。
- 验证：重启后重开转发，8080 恢复。
## YAML 与模板

### 2026-08-16 issue form description 中 `bug: ` 冒号被 YAML 解析
- 现象：`yaml.safe_load` 报 "mapping values are not allowed here"。
- 原因：plain scalar 里 `bug: ` 被当成嵌套 mapping。
- 解决：description 整行加双引号包裹。
- 验证：3 个 issue 模板 YAML 校验通过。

## 数据库与容器验证

### 2026-08-16 WSL 无 Go：用 golang 容器编译与测试
- 现象：WSL 里没有 go 命令，无法本地编译/跑测试。
- 解决：`docker run --rm -v $PWD:/app -w /app golang:1.26 go test ./...`；版本与 Dockerfile 保持一致（当前 golang:1.26）。
- 验证：Backend 全量测试在容器内通过。

### 2026-08-16 PostgreSQL 集成测试
- 方法：`docker run -d --name hk8s-pg-test -e POSTGRES_USER=dashboard -e POSTGRES_PASSWORD=dashboard -e POSTGRES_DB=dashboard -p 55432:5432 postgres:17-alpine`，然后 `TEST_DATABASE_URL=postgres://dashboard:dashboard@localhost:55432/dashboard?sslmode=disable go test ./internal/store/ -run TestPostgresLifecycle -v`。
- 注意：测试容器访问宿主端口用 `--network host`（WSL 原生 docker 没有 host.docker.internal）。
- 测试用专用数据库，不要指向真实集群的库。

### 2026-08-16 当前态读路径的降级边界
- `/configuration`、`/overview`、`/traffic` 无 `at` 参数时优先从数据库 `resource_states` 重建当前态；数据库可用但表为空（快照循环未跑过）时同样回退实时聚合，不要误判为故障。
- `asOf` 是数据库最新 `captured_at`（数据时间），可能比墙钟晚最多一个快照周期（默认 30s），不是 bug。
- 修改读路径时保持响应结构与实时路径一致（空集合输出 `[]` 而非 `null`）。

### 2026-08-16 resource_states 只增不改导致已删除资源变幽灵数据
- 现象：删除 Model/WorkerNode/策略等资源后，`/configuration` 仍显示已删除对象；集群 `kubectl get` 已无该对象，数据库 `resource_states` 残留旧行。
- 原因：`UpsertResourceStates` 只有 INSERT ... ON CONFLICT DO UPDATE，没有删除路径；快照循环每 30s 把当前态 upsert 进 `resource_states`，读路径又优先从该表重建当前态，已删除资源永远不会消失。
- 解决：`persistSnapshot` 每次写当前态后调用新增的 `PruneResourceStates`，按本次快照的活跃业务资源集合删除库中不存在的业务行（Model/WorkerNode/Tenant/三种 Policy/Orchestrator/SimulationClock/SimulatorInstance/Performance/Runtime/Traffic）；Node/Deployment/Pod 系统遥测保留。无业务资源时同样清理，保证删除全部资源后读路径为空。
- 验证：真实集群创建 model-prune-test → 快照写入 → API 删除 → 一个快照周期后库中与 `/configuration` 均消失；`TestPostgresLifecycle` 覆盖清理行为。
- 备注：已部署环境的存量幽灵行需要手动 DELETE 一次，新代码只负责增量清理。

### 迁移规则（本次确立）
- 新表/结构变更只追加 `migrations/NNN_*.sql`，不修改已应用的迁移（`schema_migrations` 已记录）。
- 迁移必须幂等（`IF NOT EXISTS` / `ON CONFLICT`），Backend 启动自动应用。
- 验证：`TestPostgresLifecycle` 覆盖"迁移幂等 + 重启后历史仍在"。

### 2026-08-16 一键启动脚本的两个坑
- `hack/local-cluster.sh` 可能丢失执行位（Windows 侧操作后 100644）：`setup.sh` 报 `Permission denied` 时先 `chmod +x hack/*.sh` 并提交 mode 变化。
- 端口转发存活检查只看 ps 会误判：进程死亡但 PID 文件残留时 `cluster-open` 不会重建转发（8080 无监听但日志说"已在运行"）。修复后检查包含 `/dev/tcp` 端口探测；遇到 8080 无响应先看 `.runtime/port-forward-*.pid` 与 `ps aux | grep port-forward`。
- 并行构建四个镜像后，构建日志会交错输出；判断失败以退出码与最终镜像存在为准，不要按日志顺序读。
### 2026-08-16 rollout restart 会杀死 kubectl port-forward 进程
- 现象：`rollout restart` backend/frontend 后，8080 变 000；`ps` 里 port-forward 进程消失，日志结尾是 `lost connection to pod`。
- 原因：kubectl port-forward 在 pod 重建后不会自动恢复连接，进程直接退出；脚本的 pid 文件仍残留旧 PID。
- 解决：部署后检查 `/dev/tcp/127.0.0.1/8080` 与 `ps aux | grep port-forward`；进程不存在就重建（`bash setup.sh open` 或手动 nohup + 更新 pid 文件）。
- 验证：本次部署后手动重建转发，8080/guide 恢复 200。

## 集群操作与部署
### 2026-08-17 WSL 内访问 localhost:8080 与 Windows dllhost 转发冲突（脚本必须走 18080）
- 现象：day-watch 10:03 起 GET traffic 全部失败（`read ECONNRESET`/`fetch failed`），keepalive 每轮却 ok:true；WSL 内 curl 8080 时好时坏（首个 200、后续全 000）；Windows 侧访问 localhost:8080 始终 200。
- 原因：Windows 侧 `dllhost.exe` 监听 `127.0.0.1:8080`（WSL2 localhost 转发宿主）；WSL 内 kubectl port-forward 也监听 8080，同端口冲突，WSL 内连接被抢占/重置。keepalive 每轮是独立新进程，首个连接成功即报 ok（假阳性）。
- 解决：WSL 内脚本一律走 `18080`（`local-cluster.sh` 新增 `dashboard-internal` 转发 18080:80）；8080 保留给 Windows 浏览器。day-watch 默认 `--base-url http://localhost:18080` 并透传给 keepalive/snapshot。
- 验证：18080 连续 4 次 200；PATCH 50→35 全链路成功；Windows 8080 保持 200。
- 备注：kubectl port-forward 日志“Handling connection”增加但连接仍失败 = 端口冲突特征；先查 Windows `netstat -ano | findstr 8080`。


### 2026-08-17 kubectl 输出超过 1MB 时 spawnSync/execFileSync 报 ENOBUFS（Pod 多时 keepalive pods 检查必现）
- 现象：副本扩到 141 后 `keepalive.mjs --once` 的 pods 检查失败 `spawnSync kubectl ENOBUFS`，其余检查全绿（假阴性）。
- 原因：`execFileSync`/`spawnSync` 默认 maxBuffer=1MB；`kubectl get pods -o json` 在 100+ 模拟器 Pod 时 JSON 远超 1MB。
- 解决：所有 kubectl 子进程调用加 `maxBuffer: 32 * 1024 * 1024`（keepalive.mjs runKubectl、day-watch.mjs kubeSnapshot）。
- 验证：141 Pod 时 keepalive 全绿（simulatorPods=141 running=141 ready=141）。
- 备注：副本少时不会触发，容易被漏测；长时测试扩到 100+ 副本后首次暴露。


### 2026-08-17 批量扩容已上线：扩容会停在"节点容量上限"，不是 maxReplicas 的问题
- 现象：400 QPS 压测下副本 16→18→20 后停止，队列 2 分钟冲到 7 万、TTFT 小时级；Orchestrator Ready=True 但不再扩。
- 原因：单副本吞吐 = maxConcurrency ÷ 平均服务时长（model-lite 约 3.7 qps）；400 QPS 需 ≈108 副本，而 2 个 WorkerNode（各 maxConcurrency=160）÷ 模型 16 = 全租户最多 20 副本，扩容到节点容量即返回 `no_feasible_placement`（正常容量不足，不是错误）。`maxReplicas=0` 只解除策略上限，节点配置才是真实天花板。
- 解决：测试前把 WorkerNode `spec.maxConcurrency`（和 gpu）调大，例如目标 N 副本 × 模型 maxConcurrency × 节点数；前端"配置详解-模拟条件下怎么填"有换算公式。
- 验证：调大节点后副本应随批量扩容（每批 1..10，冷却 60s 一批）持续增长。
- 备注：压测后队列清空需要时间（排水速度 = 副本×单副本吞吐）；TTFT 平均值会带峰值尾巴，看 queue 回落为准。

### 2026-08-17 压测走 Backend API 需要 Idempotency-Key 头
- 现象：`curl -X PATCH .../traffic` 返回 `MISSING_IDEMPOTENCY_KEY`。
- 原因：Backend 写接口要求命令幂等键。
- 解决：加 `-H 'Idempotency-Key: <任意唯一值>'`；day-watch.mjs 内部已处理，手工压测脚本要带上。
- 验证：带键后返回 `state: accepted`，Tenant.spec.qps 已更新。


### 2026-08-17 更新 Controller 必须用 config/dev 部署，make deploy(config/default) 会丢掉 SIMULATOR_IMAGE env
- 现象：`make deploy`（kustomize config/default）更新 controller 后，SimulatorInstance Controller 重建模拟器 Deployment，新 Pod 用 `simulator:latest`（本地很老、无 9090 端点的镜像）→ readiness/liveness 探针 connection refused → 29s 优雅退出循环（Exit 0 + Completed，易误判为正常退出）。
- 原因：dev 栈的 `SIMULATOR_IMAGE=hello-k8s-ai-simulator:dev` env 在 `config/dev/manager-observability-patch.yaml` 里，`make deploy` 用 `config/default` 不含该 patch，apply 覆盖 Deployment 后 env 丢失，controller 回落默认 `simulator:latest`；本地 `simulator:latest` 是过期镜像。
- 解决：dev 集群更新 controller 一律 `kubectl kustomize config/dev | kubectl apply -f -`（幂等），之后 rollout restart；不要用 `make deploy`。判断依据：`kubectl get deploy hello-k8s-ai-controller-manager -n hello-k8s-ai-system -o yaml | grep SIMULATOR_IMAGE` 应有值。
- 验证：config/dev 重部署后 controller 重建模拟器 Deployment 模板为 `hello-k8s-ai-simulator:dev`，CrashLoop 的 RS 缩到 0，实例恢复 Running 并继续扩缩（本次实测 REPLICAS 10→12）。
- 备注：`simulator:latest` 默认值仅用于 CI/隔离环境；本机 dev 栈镜像 tag 是 `hello-k8s-ai-simulator:dev`（Makefile SIMULATOR_IMG）。

### 2026-08-17 节点 DNS 故障导致 nginx 启动失败：先验节点再怪代码
- 现象：重新部署 frontend 后新 Pod `CrashLoopBackOff`，日志 `nginx: [emerg] host not found in upstream "hello-k8s-ai-dashboard-backend"`；同镜像旧 Pod（另一节点）一直正常，集群内 nslookup FQDN 也正常。
- 原因：新 Pod 被调度到 `desktop-worker6`，该节点 kindnet 网络故障（kindnet/kube-proxy 均重启过 6 次），Pod 内 DNS 全超时（`busybox nslookup` 实测 `connection timed out`）。
- 解决：`kubectl cordon desktop-worker6` 后 rollout restart，新 Pod 落到正常节点即恢复；根因修复后 `kubectl uncordon desktop-worker6`。
- 验证：cordon 后 frontend rollout 成功、nginx 200；worker6 上 busybox 探针复现 DNS 全超时。
- 备注：`nginx: emerg host not found in upstream` 先查节点与 DNS（`kubectl run` 探针），不要直接改 nginx.conf/重写 upstream。

### 2026-08-17 镜像 tag 相同（:dev）时 kubectl apply 不触发滚动，必须 rollout restart
- 现象：本地重新构建 `hello-k8s-ai-dashboard-backend:dev` 后 `kubectl kustomize config/dev | kubectl apply -f -` 显示 configured，但 Pod 仍是旧镜像内容（新路由 404）。
- 原因：Kubernetes 按镜像 tag 判断 spec 是否变化，`:dev` tag 内容更新不改变 spec。
- 解决：apply 后显式 `kubectl -n hello-k8s-ai-system rollout restart deployment <name>`。
- 验证：restart 后新 Pod 生效（/segment 200）。

### 2026-08-17 make lint 触发 golangci-lint 重下载时 GOSUMDB 校验失败（本机）
- 现象：`make lint` 报 `invalid GOSUMDB: malformed verifier id`，且下载规则失败后把 `bin/golangci-lint` 符号链接删掉。
- 原因：本机 Go 环境 GOSUMDB/代理校验异常，Makefile 按 mtime 判断需要重建工具。
- 解决：直接运行已有二进制 `bin/golangci-lint-v2.12.2 run`（缺失时 `ln -sf bin/golangci-lint-v2.12.2 bin/golangci-lint`）。
- 验证：`bin/golangci-lint run` → `0 issues`。

### 2026-08-17 Prometheus emptyDir 重启即丢历史：段/历史查询出现"区间无数据"先查 Prometheus 存活时间【已修复：PVC 化】
- 现象：段查询 06:00Z-10:00Z 区间 qps/queue/ttft 全部 0 series，只有 errorRate 有 121 个常量 0 点（`or on() vector(0)` 空集保护产生）。
- 原因：Prometheus 数据卷是 emptyDir，11:41Z 部署时重启过，重启前的原始指标全部丢失；errorRate 的空集保护会在无数据时填常量 0 系列，容易误读成"指标为 0"。
- 解决（2026-08-17 同日）：Prometheus 数据卷改 PVC（`hello-k8s-ai-prometheus-data` 20Gi），retention 24h→168h；Jaeger 同步改 badger + PVC。旧 emptyDir 数据不迁移，切换时从空开始属预期。查询"区间无数据"时仍先核对"窗口是否早于组件首次建库时间"与"是否超出 168h 保留窗口"。
- 验证：PVC 化后 scale 0→1 重启，`count(up)` 205 不变、重启前 30 分钟采样仍在（见 change-history/2026-08-17-observability-persistence/TEST_REPORT.md）。
- 备注：历史窗口早于 12:54Z（本次切 PVC 时间）的指标仍缺失，是历史数据损失，不是新问题。



### 2026-08-17 Jaeger badger 单副本 + RWO PVC：重启/升级必须先 scale 到 0 再扩回 1
- 现象：Deployment 滚动更新（rollout restart）时新 Pod CrashLoopBackOff，日志 `Cannot acquire directory lock on "/tmp/jaeger/". Another process is using this Badger database`；旧 Pod 一直 Terminating，rollout 卡死。
- 原因：badger 在数据目录写 LOCK 文件；单副本 + RWO PVC 滚动更新会短暂出现新旧两个 Pod 同时挂载同一 PVC，新 Pod 抢不到锁即退出。
- 解决：重启/升级 Jaeger 用 `kubectl scale deploy hello-k8s-ai-jaeger --replicas=0`（等 Pod 清空）→ `--replicas=1`，不要 rollout restart；Prometheus TSDB 同样有目录锁，按同一流程操作。清单已加注解 `platform.study.com/restart-procedure: scale-to-zero`。
- 验证：scale 0→1 后 Jaeger 正常启动，重启前 Trace 仍在（badger 持久化生效）。
- 备注：若已陷入"新旧 RS 抢锁"死锁，直接 scale 0 清空所有 Pod 再扩回即可，无需删除 RS。

### 2026-08-17 Jaeger v2 显式配置后 OTLP receiver 默认只绑 127.0.0.1
- 现象：给 Jaeger 加 config.yaml 后 otel-collector 持续 `connection refused`（`dial tcp ...:4317`），Jaeger 日志显示 OTLP 只监听 `127.0.0.1:4317/4318`。
- 原因：Jaeger v2 带配置文件时 OTLP receiver 默认绑定 localhost；之前无配置时内置默认绑 0.0.0.0，掩盖了差异。
- 解决：配置里显式写 `receivers.otlp.protocols.grpc.endpoint: 0.0.0.0:4317`、`http.endpoint: 0.0.0.0:4318`。
- 验证：改后 Jaeger 监听 `[::]:4317/4318`（含 IPv4 映射），collector 导出自愈，`/api/services` 有数据。

### 2026-08-17 prometheus/client_golang CounterVec 无 label 实例时不出现在 /metrics
- 现象：Backend 加了 `promauto.NewCounterVec`，但 `/metrics` 里找不到该指标（以为没注册）；二进制 strings 又能搜到指标名。
- 原因：CounterVec 的 Gather 只输出"已有 label 组合"的系列；从未 `WithLabelValues(...)` 过就不输出，静默期看不到 0 值。
- 解决：需要"始终可见的 0 值"的计数用普通 `promauto.NewCounter`（无 label）；需要按 kind 拆分的场景要在启动时预建 label 实例或接受"首次发生才可见"。
- 验证：改用普通 Counter 后 `/metrics` 立刻出现两个计数器（0 值）。

### 2026-08-17 desktop-worker6 kindnet 网络持续故障（2026-08-17 13:10Z 复测），保持 cordon
- 现象：cordon 后复测（busybox 调度到 worker6 执行 nslookup）仍 `connection timed out; no servers could be reached`；kindnet 日志持续 `lookup desktop-control-plane: i/o timeout`。
- 原因：worker6 节点网络栈（kindnet/kube-proxy 均重启过 6 次）未自愈；根因未定位，疑似 Docker Desktop 内置 K8s 节点容器重建后的 CNI 混乱（历史上 desktop-worker6/9 曾同 IP）。
- 解决：继续 `kubectl cordon desktop-worker6`；不要重启节点/集群（用户未批准）；后续从 kindnet/kube-proxy 日志与节点容器侧排查。
- 验证：cordon 状态在 `kubectl get nodes` 可见（Ready,SchedulingDisabled），业务 Pod 全部跑在健康节点。

### 2026-08-17 本机 go get 新依赖必须 GOSUMDB=off
- 现象：`go get github.com/prometheus/client_golang@latest` 报 `verifying module: invalid GOSUMDB: malformed verifier id`（GOSUMDB=sum.goproxy.cn 本机异常）。
- 原因：本机 Go 环境 sumdb 校验问题（与 `make lint` 的 GOSUMDB 失败同源）。
- 解决：`export GOSUMDB=off` 后执行 `go get`/`go mod tidy`；go.sum 正常入库，CI 侧校验不受影响。
- 验证：GOSUMDB=off 后依赖拉取成功、`go build ./...` 通过。

### 2026-08-16 Simulator Pod 调度绑定 WorkerNode 名：虚拟节点名无法调度
- 现象：用虚拟节点名（如 node-gpu-1）创建 WorkerNode 并建 TenantNodePolicy 后，SimulatorInstance 副本一直 Pending，`describe` 显示 node selector 匹配不到真实节点。
- 原因：Simulator 物化的 Pod 通过 affinity/nodeSelector 绑定 WorkerNode 名称，虚拟名在真实集群里不存在。
- 解决：WorkerNode 的 name 必须使用集群真实节点名（docker-desktop 为 desktop-worker、desktop-worker2 ...）；建 WorkerNode 前先 `kubectl get nodes` 核对。
- 验证：改为真实节点名后 SimulatorInstance 副本正常调度并 Running。
- 备注：Docker Desktop 内置 K8s 的节点名是 desktop-worker*，与 Kind 测试集群（hello-k8s-ai-test-e2e 的 kind-control-plane/worker）不同，切换环境要重查。

### 2026-08-16 Docker Desktop 重建节点后可能出现重复主机 IP（双节点同 172.18.0.4）
- 现象：rollout 新 Pod 卡 Init:0/1，init 容器 `wait-for-postgresql` 报 `no response`；Pod IP 属于其他节点网段（worker6 上的 Pod 拿到 10.244.2.x，而 worker6 的 CIDR 是 10.244.8.0/24）。
- 原因：此前强杀 Docker Desktop 后内置 K8s 节点容器重建，desktop-worker6 与 desktop-worker9 主机 IP 都是 172.18.0.4（`kubectl get pods -A -o wide` 可见两个节点同 IP），CNI 分配混乱。
- 解决：删除卡住的 Pod 让它重新调度到健康节点；必要时 `kubectl cordon desktop-worker6 desktop-worker9`。不要为此重建集群或重启 Docker。
- 验证：新 Pod 落到 worker9（10.244.2.5）后 rollout 成功；演练数据（模拟器/数据库）全程不受影响。
- 备注：彻底修复需 Docker Desktop → Settings → Kubernetes 重新启用节点容器（会中断全部工作负载），本次未做。

### 2026-08-16 清理演示 CR 必须在 Controller 在线时进行
- 现象：`cluster-down` 后删除 Tenant/Model/Policy 卡在 DeletionTimestamp，对象不消失。
- 原因：tenant-model-policy、simulator-instance-controller、performance-collector、traffic-distribution 四个 finalizer 依赖 Controller 处理。
- 解决：先恢复 Controller（`make cluster-up`）再按顺序删除：orchestrator → tenantmodelpolicy → 等 instance 消失 → tenant/model/派生 CR → 动态策略与 WorkerNode。
- 验证：本次清理 10 类业务 CR 全部归零。
- 备注：`simulationclock/default` 是系统默认对象，删除后控制器会自动重建，不需要也不建议清理。

### 2026-08-16 空配置不再写历史快照（干净环境预期）
- 现象：干净环境（无业务 CR）下 `/replay` 没有 `snapshot-*`。
- 原因：`persistSnapshot` 新增 `snapshotHasBusinessData` 判定，无模型/租户/节点/策略/编排器/实例时跳过写快照。
- 解决：这是预期行为；`resource_events` 仍会记录真实系统 Lease/Node 心跳事件，不属于假数据。
- 验证：`bash setup.sh` 干净模式验收通过。
- 备注：旧版本后端写入的残留快照不会自动消失；发现 `resource_snapshots` 有历史行时可 TRUNCATE 后再验收（脚本断言只检查 `/replay` 响应，不检查库表行数）。`resource_states` 的业务行已由 `PruneResourceStates` 自动清理，无需手工处理。

### 2026-08-16 本机 Go 测试 httptest 回环有约 300ms accept 延迟
- 现象：`TestGrafanaProxyPreservesSubPathAndForwards`、`TestGrafanaProxyRootPath` 在本机 WSL 报 502，CI 正常。
- 原因：本机 WSL/Docker Desktop 环境下 `httptest.NewServer` 的 127.0.0.1 监听刚建立时连接被拒（独立复现：延迟 300ms 后 dial 与反向代理均成功）。
- 解决：本地判断与本次改动无关时，以 CI 结果为准；不要为此改测试。
- 验证：stash 本次改动后同样失败；GitHub Actions 上历史提交均通过。

## 压测与长跑剧本设计（2026-08-17 容量校准确立）

### 2026-08-17 剧本无效：峰值 QPS ÷ 当前副本数 < 单副本容量（队列恒为 0）
- 现象：200/350 QPS 剧本在 141 副本下 queue 始终为 0、TTFT 恒等于基线，扩容一次都不触发，看似"稳定"实为无效负载。
- 原因：单副本容量 ≈ `maxConcurrency ÷ 平均服务时长`（model-lite ≈ 3.7 qps）；141 副本 × 3.7 ≈ 520 qps 总容量，剧本峰值远低于容量，请求永远不排队。
- 解决：写剧本前先算容量公式与所需副本：`平均服务时长 = prefillBaseMs + prefillPerTokenUs×0.5 + decodePerTokenMs×200`（prompt 500 / output 200 固定）；`所需副本 ≈ QPS × 平均服务时长 ÷ maxConcurrency`；保证 `峰值 QPS ÷ 当前副本数 > 单副本容量` 才会产生队列与扩容。
- 验证：650 QPS @ 141 副本（4.6/副本 = 1.25×容量）实测触发批量扩容 141→200（queue 峰值 ~2491、TTFT 峰值 ~678s，到顶后 ~6 分钟排空）；300 QPS @ 141 副本（0.57×容量）实测 queue 0。
- 备注：400 QPS @ 20 副本（5.4×容量）队列 2 分钟冲到 7 万、TTFT 小时级是数学结果，不是调度 bug；压测前先按公式放大 WorkerNode 容量（见 hack/night-run/README.md）。

### 2026-08-17 TTFT 只在排队时上升：TTFT=320ms 不代表没负载
- 现象：低负载下 TTFT 恒等于服务基线（model-lite ≈ 320ms），用 TTFT 判断是否扩容会误判"无压力"。
- 原因：TTFT 指标只在请求排队（每副本负载 ρ→1）后上升。
- 解决：判断负载以 queue 为主、TTFT 为辅；TTFT 变化只作为"已经排队"的旁证。
- 验证：300 QPS @ 141 副本 queue=0、TTFT 320ms；400 QPS @ 20 副本 queue 7 万、TTFT 小时级。

### 2026-08-17 缩容滞回：TTFT 基线高于缩容下阈值时峰值副本保持不回缩（预期）
- 现象：队列排空后副本数不回落到基线规模。
- 原因：Orchestrator 缩容同时看 TTFT 与 queue，model-lite TTFT 基线 320ms > 缩容下阈值 300ms，`needDown` 被 TTFT 挡住。
- 解决：这是观察到的预期行为（滞回），长跑结束后副本保持峰值规模不算故障；要恢复需改缩容阈值策略（本期未改）。
- 验证：2026-08-17 14:00-18:00 长跑观察中，结论以 summary.md 为准。

## 长跑工具与可观测性（2026-08-17 修复）

### 2026-08-17 --until 超时多跑一轮：轮间 sleep 不裁剪到截止时间（已修复）
- 现象：`--until 18:00` 实际 18:14 才恢复流量并退出，18:14:49 还短时 patch 650qps（7 秒后恢复 35）。
- 原因：旧版 do-while 循环里 `shouldStop()` 只在每轮结束后检查，轮间 sleep 固定 INTERVAL（900s），截止前最后一轮后仍睡满一轮。
- 解决：`msUntilStop()` 计算剩余时间，`sleep = min(轮次间隔补足, 剩余)`；循环顶部 `round > 0` 时提前判停；`remaining == 0` 直接 break（day-watch.mjs 已修）。
- 验证：`--until 19:34` 测试运行 19:34:07 整点停止（含恢复流量 + summary），无多余轮次。

### 2026-08-17 30 分钟快照错过峰值强度：summary 会严重低估峰值指标
- 现象：4 小时长跑 summary 显示 queue max=135，实际峰值 ~2491（PG `resource_events` 5s 序列）。
- 原因：快照每 2 轮（30 分钟）一次，恰好落在峰值起始 2 秒处；15 分钟轮次也覆盖不到 15 分钟峰值的中间段。
- 解决：day-watch 每轮轻量采样 6 个指标 + 进入峰值时预约「峰值中点」补采样（summary「轮内指标」节）；精确序列始终以 PG `resource_events`（5s）为准。
- 验证：测试剧本触发峰值中点采样并落盘；正式 run 已重生成 summary（快照局限已注明）。

### 2026-08-17 rounds 目录跨 run 复用：summary 混入历史轮次（已修复）
- 现象：正式 run 的 summary「总轮数 29」，混入 13:21-13:29 测试轮次；陈旧扩缩容事件（05:12:53Z）入表。
- 原因：rounds/ 按日期目录复用，多轮 run（测试/正式）文件混在一起，旧版 buildSummary 全量统计。
- 解决：启动写 `meta.json`（startIso/endIso/args），summary 只统计 `[startIso, endIso]` 窗口；扩缩容事件按事件时间过滤；新增 `--resummarize` 重生成模式（无 meta.json 会拒绝，需手工补）。
- 验证：正式 run 补 meta 后重生成：20 轮、事件 2 条、快照 10 个。

### 2026-08-17 PromQL 空集：零错误时 ratio 类指标塌成空（errorRate 恒为 null，已修复）
- 现象：controller.errorRate / simulator.errorRate 在快照与指标 API 里恒为 null，即使系统零错误。
- 原因：`sum(rate(x{outcome="error"}[5m]))` 在没有任何 error 系列时返回空集，分子空 → 整个 ratio 表达式无结果（Prometheus 经典空集问题）。
- 解决：分子分母都加 `or on() vector(0)`（`config/observability/prometheus.yaml` recording rule + `dashboard/backend/internal/providers/prometheus/client.go` simulator.errorRate 查询）。
- 验证：部署后两个指标均返回 1 条 series、值 0。
- 备注：以后写「错误比例 / 占比」类 PromQL 都要带空集保护，否则零错误时面板显示「无数据」而非 0。

## 领域已知易误判点（原 AI_CONTEXT 第 8 节）

- Traffic Overlay 是本地草稿；页面有真实数据不等于场景已写回控制平面。
- `TenantRuntime.status.instanceCount` 的实现含义是可用 Replica 总数。
- `Model.spec.absoluteScore` 是用户/Backend 提供的必填能力基准；旧 `status.absoluteScore` 仅用于滚动升级兼容，不应再写入。
- TenantNodePolicy、ModelNodePolicy 的 Status 当前没有 writer；空 Conditions 不等于失败。
- 前端只创建 Model/Tenant/Orchestrator 不会启动工作负载：SimulatorInstance 由 TenantModelPolicy(Allow) 物化，节点可调度性由 TenantNodePolicy(Allow) 决定（无显式 Allow 不可调度），模型-节点范围由 ModelNodePolicy 过滤；三类策略缺一不可，Orchestrator 在无可行 placement 时副本保持 0。
- 新租户没有 Simulator 时 Orchestrator 停在 MetricsNotReady 属正常引导态：性能指标来自运行中的 Simulator Pod，只有策略齐全后 bootstrap 扩容（到 minReplicas 地板）才会创建 Pod。
- Backend watch ReplicaSet 并记录事件，但 Workloads DTO 当前未直接展示 ReplicaSet。
- 数据库 `clock_state` 仍未驱动运行时。`SimulationClock/default` 只控制 Simulator 引擎倍速；Backend server/actual/logical time、Controller cooldown/freshness、Lease 和采集周期继续使用真实 UTC。
- 配置批次会先 dry-run 全部对象，再顺序写入；跨对象写入并非数据库式原子事务。
- SSE 是非持久通知流，慢客户端可能丢事件；30 秒轮询是安全网。
