# 升级与回滚

## 1. 迁移

- CRD 升级为 additive：Status 新增可选字段，旧实例无需改动即可运行。
- 旧实例升级后，下一个 Leader 首次接管时没有 `simulationElapsedMs`，冷启动进度从 0 重新累计一次，之后不再受 reporter 生命周期影响。

## 2. 回滚

- `git revert` 本提交，或手动移除 CRD 中 `simulationElapsedMs` 字段后重新应用。
- 回滚后恢复旧行为：Leader 切换会重置冷启动曲线（已知限制，不影响数据）。

## 3. 风险与注意

- Status 写入量不变（Simulator 本来就每 Tick Patch Status），仅多一个整数字段。
- `simulationElapsedMs` 只由 Simulator Leader 写；Controller/Backend 不得回写（见 FIELD_OWNERSHIP）。
