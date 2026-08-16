# 实现修改明细

## 1. 改动前状态

- `Simulator.simulationElapsed` 是进程内累计模拟时间，从 Leader 进程启动起算（初始 0）。
- Leader Pod 重启或租约切换会创建新的 Simulator，冷启动曲线从头开始，Score 骤降、TTFT 被 factor 放大，可能触发 Orchestrator 虚假扩缩容。

## 2. 修改

- `api/v1/simulatorinstance_types.go`：`SimulatorInstanceStatus` 新增 `SimulationElapsedMs *int64`（`json:"simulationElapsedMs,omitempty"`），注释明确由 Simulator Leader 持久化。
- `simulator/simulator.go`：
  - `Simulator` 增加 `elapsedRestored bool`，首个成功读取实例的 tick 从 `status.simulationElapsedMs` 恢复 `simulationElapsed`（无历史值保持 0）。
  - `updateOwnedStatus` 每 Tick 把 `s.simulationElapsed` 的毫秒值写回 Status，与其他 Simulator 自有字段一起 Patch，不碰 Controller 字段。
- 重新生成：`make manifests generate YEAR=2026`，差异为 `platform.study.com_simulatorinstances.yaml` 新增 `simulationElapsedMs` 与 `zz_generated.deepcopy.go` 对应 DeepCopy。

## 3. 未做

- 引擎队列/随机序列的 Leader 切换恢复与 checkpoint（需要持久化 engine 状态，改动面大）。
- 逐副本年龄分布与扩容后新副本独立预热（需要按 Pod 建模，超出本轮）。
- Backend/Frontend 展示 `simulationElapsedMs`（本轮只保证控制回路语义）。
