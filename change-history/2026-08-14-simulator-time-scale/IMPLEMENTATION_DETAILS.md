# 实现修改明细

## 1. API 与 CRD

### `api/v1/simulationclock_types.go`

新增 cluster-scoped `SimulationClock`：

- 名称通过 CEL 固定为 `default`；
- `spec.rate` 默认 1，Minimum=1，Maximum=20；
- Status 保存 observed generation、applied rate、同步/总实例数和标准 Conditions；
- 常量统一默认名称、默认倍速与最大倍速。

### `api/v1/simulatorinstance_types.go`

`SimulatorInstanceSpec` 新增必填 `timeScale`，同样带 1..20 校验和默认 1。该字段是运行配置的派生副本，不是用户入口。`kubectl get simulatorinstances` 新增 Rate 列。

### 生成物与样例

同步更新：

- `api/v1/zz_generated.deepcopy.go`；
- `config/crd/bases/platform.study.com_simulationclocks.yaml`；
- `config/crd/bases/platform.study.com_simulatorinstances.yaml`；
- `config/crd/kustomization.yaml`、`PROJECT`；
- SimulationClock admin/editor/viewer 辅助角色及 Manager RBAC；
- `config/samples/platform_v1_simulationclock.yaml`、样例 Kustomization 和 Instance 样例。

生成文件与 Kubebuilder 标记保持一一对应。目标环境仍应执行 `make manifests generate` 做生成一致性门禁。

## 2. SimulationClock Controller

### `internal/controller/simulationclock_controller.go`

新增 `SimulationClockReconciler`：

1. 只处理 `default`；缺失时创建 rate=1。
2. CRD 校验之外再次防御 1..20 非法值，并通过 Ready=False/InvalidRate 报告。
3. List 非删除中的全部 SimulatorInstance。
4. 对每个实例执行冲突重试、重新读取和 `MergeFrom` Patch，只修改 `spec.timeScale`。
5. 汇总同步结果并写 Clock Status；Status 无变化时不发起 Patch。
6. 任一实例失败时 Ready=False、保留旧 appliedRate、返回聚合错误让运行时退避重试。

Watch 做了范围控制：

- Clock 主资源只监听 generation，Status 写不会自触发循环；
- Instance 只在 create/delete/timeScale 变化时触发；
- replicas、QPS 和每 5 秒 Status 变化不会导致 O(N) 全量同步。

### `cmd/main.go` 与 `tenant_controller.go`

Manager 注册第 7 个 Reconciler。TenantModelPolicy 创建新 Instance 时写 timeScale=1，保证没有 Clock 事件前对象也满足 schema；后续唯一所有者是 Clock Controller。

### 不重启约束

`SimulatorInstanceReconciler` 的 Deployment template 不包含 timeScale，其 Update predicate 也有意忽略只变更 timeScale 的事件。这样倍速变化不会生成无意义 Deployment Patch 或滚动替换 Pod。

## 3. Simulator

### `simulator/simulator.go`

Simulator 新增进程内 `simulationElapsed`：

```text
timeScale = clamp(instance.spec.timeScale, 1, 20)
simulationStep = fixedRealTick × timeScale
simulationElapsed = saturatingAdd(simulationElapsed, simulationStep)
```

每轮先读取 Instance/Model，再推进模拟时间。固定 Tick 而非实际墙钟差可避免进程暂停、CPU 卡顿或调试恢复后一次性补算大量请求。倍速变化只影响后续增量，不重置已有进度。

SimEngine 的 `StepRate` 接收 simulationStep，因此 Poisson 请求量、队列和 TTFT 真正按倍率推进。QPS 的含义保持“每模拟秒请求数”。

### `simulator/coldstart.go`

冷启动计算拆出 `coldStartFactorForElapsed`，从墙钟 `now-startTime` 改为累计模拟时长。原确定性入口保留给既有测试，公式本身未改。

### Metrics 与 Trace

新增：

- `hello_k8s_ai_simulator_time_scale`；
- `hello_k8s_ai_simulator_simulation_step_seconds`；
- `hello_k8s_ai_simulator_simulation_elapsed_seconds`；
- Tick Span 属性 `simulator.time_scale`、`simulator.step_seconds`、`simulator.elapsed_seconds`。

Reporter/leader 切换后这些进程内值从新任期开始；当前没有 checkpoint。

## 4. Dashboard Backend

### Kubernetes cache 与读模型

- `resources.go` 将 SimulationClock 加入第 11 个 dynamic informer。
- `cache.go` 读取 desired/applied/sync counts/resourceVersion/Ready。
- convergence 必须同时满足 Ready=True、observedGeneration 等于当前 generation、desired=applied、synchronized=total，避免 Spec 刚改时复用旧 Ready。
- Configuration/Overview 把 SimulationClock 纳入当前态和历史快照 DTO。

### Clock 投影

`internal/clock` 保持 server/actual/logical time 为真实 UTC，只从 cache 投影 Simulator rate：

- `rate`：期望值；
- `appliedRate`：Controller 已同步值；
- `resourceVersion`、`converged`、同步计数；
- `canSetRate/simulatorAcceleration=true`；
- `canPause/canSeek=false`、`simulationTime=nil`。

数据库不可用时 Server 会把写能力关闭，因为命令无法满足幂等与审计保证。

### 写 API

新增 `PATCH /api/v1/clock/rate`：

- strict JSON：rate、resourceVersion、dryRun；
- 范围 1..20；
- Clock 缺失且没有版本时 create；存在时 update；
- Controller 与 Backend 并发补建 Clock 时，Backend 读取胜出的单例并继续 update；
- 版本不一致返回 Kubernetes Conflict；
- 支持 API Server dry-run、Idempotency-Key、审计和 OperationReceipt；
- 只操作 `SimulationClock/default.spec.rate`，不开放通用配置删除或 Status 权限。

### Prometheus catalog

新增 `simulator.timeScale`，按 tenant/model/simulator_instance 取当前 reporter gauge 的 max，不向浏览器开放任意 PromQL。

## 5. Frontend

### `ExecutionControls.tsx`

新增倍速选择。禁用条件包括：

- Historical 模式；
- Backend/Kubernetes 未连接；
- capabilities 未开放；
- 当前命令 pending；
- 已存在 Clock 但上一 generation 尚未收敛。

Tooltip 显示目标/已应用值和同步实例数。API 返回未知但合法整数时也能回显，不会因 UI 只列常用档位而变为空值。

### API 与状态

- `controlPlaneApi.ts` 映射 Clock 全部收敛字段，并发送带 Idempotency-Key/resourceVersion 的 PATCH。
- `controlPlaneSlice.ts` 管理提交 phase、乐观目标值、冲突错误和下一次 SSE/REST 收敛。
- 类型定义新增 SimulationRateReceipt 和 Clock 字段。
- Data Overview 展示 SimulationClock 资源，并增加 Time Scale 指标卡。

Frontend 不修改时间轴插值；历史游标和运行倍速保持两个独立概念。

## 6. 部署与权限

- Dashboard Backend RBAC 对 SimulationClock 有 get/list/watch/create/update/patch，无 delete/status。
- Manager RBAC 可创建/读取 Clock、更新 Clock Status，并 Patch Instance。
- `config/demo` 应用 rate=1 的默认 Clock。
- `hack/local-cluster.sh` 等待新 CRD，检查 Clock desired/applied/Ready、Instance timeScale、Backend capability 和 Prometheus time-scale metric。

## 7. 测试覆盖

| 层 | 新增覆盖 |
| --- | --- |
| Controller | 自动创建默认 Clock；只修改拥有字段；Status 收敛；非法 rate 不改实例。 |
| Simulator | 2x 后动态 8x，累计达到 10 秒且墙钟 observedAt 不变；边界钳制与饱和累计。 |
| Backend cache/clock | desired/applied 映射；旧 Ready/generation 不误判；Clock 缺失 fallback；墙钟不被倍速改变。 |
| Backend gateway | singleton create/update、并发创建收敛、范围拒绝、旧 resourceVersion 冲突。 |
| Prometheus provider | timeScale 命名查询和 label matcher。 |
| Frontend contract | 不支持时拒绝；支持时实际发送 PATCH、版本和幂等键，并进入 pending convergence。 |
| E2E | 1x -> 10x 动态同步；指标为 10、step=50s；Pod UID 不变。 |

## 8. 明确未修改

- 未修改 Orchestrator 评分、选点、cooldown 或 pending plan。
- 未修改 Traffic 分配和 Performance 聚合公式。
- 未修改 Deployment 架构、CRD scope、数据库 schema 或可观测技术栈。
- 未实现 pause、Seek、SimulationRun、确定性 seed 或 checkpoint。
- 未把高倍速解释为生产吞吐能力；真实校准仍是独立工作。
