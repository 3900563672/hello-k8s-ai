# 升级与回滚

## 升级

1. 拉取新代码后重新执行 `bash setup.sh`（默认干净模式）或 `DEMO_ENABLED=true make cluster-up`（演示模式）。
2. 需要从“旧演示数据”迁移到干净环境时：
   - 必须保持 Controller 在线（finalizer 依赖），按顺序删除：orchestrator-sample → tenantmodelpolicy-sample → 等 simulatorinstance 消失 → tenant/model/派生 CR → 动态策略与 WorkerNode → 孤儿 Lease。
   - 清空数据库业务表：`TRUNCATE resource_snapshots, resource_events, audit_log, trace_index, command_idempotency, resource_states RESTART IDENTITY;`（保留表结构；`resource_events` 会继续记录真实系统事件）。
   - 验证：`kubectl get tenants,models,orchestrators,simulatorinstances,tenantperformances,tenantruntimes,tenantmodelpolicies,tenantnodepolicies,modelnodepolicies,workernodes` 为空。

## 回滚

- 代码回滚：`git revert <commit>` 或 `git reset --hard` 到上一提交；然后重新 `bash setup.sh` 恢复旧镜像与行为。
- 回滚后若想恢复演示数据：`DEMO_ENABLED=true make cluster-up`（`deploy_demo` 仍保留）。
- 数据说明：预置模板仅存在于前端内存（刷新即恢复预置）；已删除的 CR 无法通过回滚恢复，需要重新创建（演示 CR 可通过 `config/demo` apply 重建）。

## 风险

- 空配置快照跳过是行为变化：依赖 `/replay` 持续有快照的外部脚本需要适配（干净模式下无快照是预期）。
- `resource_events` 仍会增长（系统 Lease/Node 心跳），不属于可清理的“假数据”。
- 清理演示 CR 时若 Controller 不在线，删除会挂起（DeletionTimestamp 不消失），恢复 Controller 后自动完成。
