# 业务层优雅降级闭环：物理水位联动 + 资源受限标记（#49）

> 日期：2026-08-18

## 为什么做

- 资源紧张时系统表现是"错误 + 丢失"（ENOBUFS、事件丢弃）而不是"降级"。模拟器本质是模拟负载，达到物理上限时的正确表现应是：TTFT 上升、队列积压、扩不起来，但**不报错、不丢数据、不崩**。
- 已确认前提（用户决策）：无网关、模拟器全量接收 QPS；WorkerNode 是模拟资源可无限扩；真实机器有物理天花板，必须靠资源水位联动保护，而不是硬编码上限。
- 模拟器已有无限排队 + 虚拟队列（不丢请求，无需改）；缺口在：物理水位（真实 Node 资源）联动与降级可观测。

## 改成什么

1. **WorkerNode 物理水位字段**（`api/v1/workernode_types.go`）：`status.memoryUsagePercent` / `status.cpuUsagePercent`（0-100，CRD validation Minimum/Maximum）。
2. **WorkerNodeUsage 控制器**（`workernodeusage_controller.go`）：读同名真实 Node 的 `allocatable` + 该节点全部非终态 Pod 的 `requests`，算内存/CPU 水位百分比写入 Status；超阈值（90%，系统常量 `physicalPressureThresholdPercent`）时置 `PhysicalPressure=True` condition；找不到同名 Node 时保持缺省，不阻塞模拟用量更新。新增 RBAC `core/v1 nodes get;list;watch`。
3. **Orchestrator 资源受限降级**（`orchestrator_decision.go` / `orchestrator_controller.go`）：`needUp && 任一可调度节点 PhysicalPressure` → `NoOp/resource_limited`（RequeueAfter=orchestratorSyncPeriod），不再扩容；缩容与重平衡不受影响。置 `ResourceLimited=True` 条件（同时 `Ready=True/ResourceLimited`：降级是预期行为不是故障），恢复后自动清除。
4. **可观测**（`observability.go`）：新指标 `worker_node_memory_usage_percent`、`worker_node_cpu_usage_percent`、`orchestrator_resource_limited{tenant}`；常量收敛 `decisionReasonResourceLimited`、`metricsSubsystemWorkerNode`、`metricLabelNode`。
5. **单测**（`orchestrator_decision_unit_test.go`）：无压力扩、有压力停扩、恢复后自动扩、压力下缩容不受影响。

## 关键行为

- 水位阈值 90% 是系统常量（宿主机安全边界）而非用户配置；如后续需要按环境调整，再参数化到 WorkerNode spec。
- WorkerNode 与真实 Node 同名（部署脚本按节点名生成）；找不到同名 Node 时水位保持缺省 0，只影响物理保护不阻塞逻辑用量。
- 降级期间 QPS 照常接收，负载由现有副本排队消化；水位恢复后下一轮决策自动退出降级，无需人工干预。

## 验证

- `make fmt` / `go vet ./...` / `go test ./...` 全绿（含新增 `TestDecideAtResourceLimited`）。
- `make lint`：0 issues（golangci-lint v2.12.2 + 自定义 logcheck 插件）。
- `make manifests generate YEAR=2026`：生成差异仅 CRD +12（两字段）、RBAC +1（nodes 权限）。
- `make docs-check` 通过；`make docs-sync` 已重新生成派生文档。
- 未验证：真实集群上的超物理上限长时压测（≥2h，验收"只降速不报错"）与 Grafana 降级面板，属后续超限压测轮次。

## 回滚

- `git revert` 对应 commit 后需重新 `make manifests generate YEAR=2026`；存量 CR 若已写入新 status 字段，CRD 回退后字段会被忽略（status 不校验拒绝）。
- RBAC nodes 权限随 role.yaml 一并回退；WorkerNodeUsage 找不到同名 Node 时自动退化为旧行为（不写水位）。
