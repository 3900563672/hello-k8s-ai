# 实现细节：宿主内存治理与稳定性校准

## 改动前状态

- 无 `.wslconfig`：WSL2 默认 memory = 宿主 50%（15.7GB），无 autoMemoryReclaim，vmmem 只增不减。
- Docker Desktop `settings-store.json`：`KubernetesEnabled=true`、`KubernetesNodesCount=10`、`EnableDockerAI=true`、`InferenceCanUseGPUVariant=true`。
- 集群内长跑遗留：`SimulatorInstance tenant-core-model-lite`（`spec.replicas=200`、rate=20、24h）→ 208 个 Pod（两个 Deployment 各 100 副本）。
- Jaeger Deployment：request 128Mi / limit 512Mi，无 GOMEMLIMIT；badger 默认 `BlockCacheSize=268435456`（256MB）+ `MemTableSize=67108864`（64MB）→ OOMKilled 反复 CrashLoop（exit 137）。

## 实现

### 1. 宿主配置（不入库，记录于 change-history）

`%USERPROFILE%\.wslconfig`：

```ini
[wsl2]
memory=12GB
autoMemoryReclaim=gradual
```

- `memory=12GB`：31.4GB 机器给 Windows 侧留 ~19GB；实测 10 节点集群空载就占 ~11.7GB，12GB 上限是当前节点数下的硬约束（见待办）。
- `autoMemoryReclaim=gradual`：空闲页逐步归还宿主，解决"WSL 内存只增不减"。
- 生效方式：`wsl --shutdown` 后重启（本次已执行）；Docker Desktop GUI 改内存会覆盖本文件（KNOWN_PITFALLS 有记录）。

Docker Desktop：`EnableDockerAI=false`、`InferenceCanUseGPUVariant=false`（原文件备份 `settings-store.json.memguard.bak`）。

### 2. 负载清零

- `make cluster-down`（停 controller + 全部 Deployment/StatefulSet + port-forward）。
- 为部署 Jaeger 修复执行 `kubectl apply -f config/dev` 后 controller 复活并按 CR 重建 200 Pod（坑，已沉淀）。
- `kubectl patch simulatorinstance tenant-core-model-lite --type=merge -p '{"spec":{"replicas":0}}'` —— 触发 controller 校验错误 `has 0 replicas but its node placement plan contains 200`（replicas=0 不是合法停止态，已沉淀待修）。
- 手动 `kubectl scale deployment -n hello-k8s-ai-system simulator-tenant-core-model-lite simulator-tenant-core-model-lite-node-a4ad59a91231 --replicas=0` 止血；controller 因校验前置失败不再拉起。

### 3. Jaeger 资源校准（config/observability/jaeger.yaml）

- request：128Mi → 256Mi；limit：512Mi → 1Gi。
- 新增 env `GOMEMLIMIT=805306368`（768Mi 字节数；badger cache 在 Go 堆内，GC 软限是 cgroup limit 之外的第二道防线）。
- 校验依据：jaeger v2.20.0 源码 `internal/storage/v1/badger/config.go`/`options.go` 确认 badger 缓存大小**不暴露**为 YAML 配置，只能靠内存上限治理。
- 重启遵循既有注解：单副本 + RWO PVC 必须 scale 0→1。

## 关键决策

- **不迁移 D 盘**：D 盘剩余空间小，pagefile 固定大小列为待办（需重启电脑）。
- **不杀用户进程**：保留微信/Chrome/GoLand/ChatGPT；只关豆包与 Edge。
- **节点数缩减暂缓**：`KubernetesNodesCount` 变更可能重置内置 K8s（CRD/CR/PVC 风险），必须先备份并单独评估。
