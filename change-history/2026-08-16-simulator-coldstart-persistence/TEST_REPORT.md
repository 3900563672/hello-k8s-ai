# 测试报告

## 1. 执行的验证与真实结果

| 命令 | 结果 |
| --- | --- |
| `go vet ./simulator/... ./api/...` | 通过 |
| `go test ./simulator/... -count=1` | 通过（含新增测试） |
| `make manifests generate YEAR=2026` | 通过，CRD/DeepCopy 差异符合预期 |
| 新增 `TestSimulatorRestoresElapsedFromStatusOnLeaderStart` | 通过：Status 中 12345ms 被恢复并推进一个 tick，且写回 Status |

## 2. 新增测试覆盖

- 新 Leader 从 `status.simulationElapsedMs` 恢复累计模拟时间，而不是从 0 重新冷启动。
- 恢复后每 Tick 继续推进并把新值持久化回 Status。
- 既有 `TestSimulatorStatusPatchPreservesControllerOwnedFields` 仍通过：新增字段不覆盖 Phase/Conditions。

## 3. 未验证项

- 真实集群多 Pod Leader 切换行为：本机无可用集群，由 CI E2E / 本地部署验证（Deployment 重启后冷启动曲线不回退）。
- 旧版本 Status 无 `simulationElapsedMs` 的存量实例：升级后首次 Leader 接管从 0 开始（一次性语义重置，符合预期）。
