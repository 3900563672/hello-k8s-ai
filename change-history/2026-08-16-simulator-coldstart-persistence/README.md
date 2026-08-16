# Simulator 冷启动进度与 reporter 生命周期解耦

- 变更日期：2026-08-16
- 关联问题：Fixes #19（Project Review issue-07，冷启动部分）
- 变更级别：P1 Simulator 状态语义
- 变更范围：`api/v1/simulatorinstance_types.go`、`simulator/simulator.go`、生成的 CRD/DeepCopy、相关文档
- CRD 变化：SimulatorInstance.status 新增 `simulationElapsedMs`
- 数据库变化：无

## 1. 完成结果

审查指出冷启动时钟属于 reporter（Leader 进程）：Leader Pod 重启或租约切换时，即使业务副本已运行很久，整个实例池也会重新进入冷启动，产生虚假 Score/TTFT 变化并可能触发扩缩容抖动。

本次把累计模拟时间持久化到 `SimulatorInstance.status.simulationElapsedMs`：Simulator Leader 每个 Tick 写回，新 Leader 接管时从 Status 恢复，冷启动进度成为实例属性而不是 reporter 进程属性。队列与随机序列仍随 Leader 切换重建（保持单 writer 与最小改动），不属于本轮范围。

## 2. 关键行为

- 首次部署无历史值时从 0 开始；此后 Leader 切换沿用 Status 中的累计模拟时间。
- 倍速变化只影响后续步长，不重置已推进的冷启动进度（语义不变）。
- 进程暂停不按墙钟差值补算，避免恢复时制造流量尖峰（语义不变）。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| api/v1 | SimulatorInstanceStatus 增加 `simulationElapsedMs`（Simulator Leader 写） |
| simulator | 首次读取后恢复、每 Tick 持久化累计模拟时间 |
| 控制器/Backend | 无代码变化（Status 新增字段为只读消费） |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `go test ./simulator/...`、`go vet ./simulator/... ./api/...` 通过；新增 Leader 恢复与持久化测试。
- 停止线：本轮只解耦冷启动进度；引擎队列/随机序列随 Leader 重建、逐副本年龄分布留待后续单独评估（见 issue-07 其余方向）。
