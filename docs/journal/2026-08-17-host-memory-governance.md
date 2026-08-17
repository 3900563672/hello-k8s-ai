# 宿主内存治理（2026-08-17）

> 日期：2026-08-17 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

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
