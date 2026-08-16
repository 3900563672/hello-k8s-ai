# 升级与回滚

## 1. 迁移

- 无 CRD、数据库变化；索引由 Controller 启动时注册。
- Pod NodeName 索引为空值不索引，未调度 Pod 不会出现在查询结果中（与旧行为一致：未调度 Pod 本就不统计）。

## 2. 回滚

- `git revert` 本提交即可恢复全量广播与全量遍历。

## 3. 风险与注意

- 若 Pod 的 NodeName 在事件映射时为空（未调度），仍广播全部节点，保证调度后节点用量最终一致。
- fake client 测试必须注册索引，否则 List + MatchingFields 会报错；真实集群由 SetupWithManager 注册。
