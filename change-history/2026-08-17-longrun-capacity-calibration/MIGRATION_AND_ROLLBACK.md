# 迁移与回滚

## 部署方式

- 本次变更是文档沉淀（RESILIENCE / KNOWN_PITFALLS / change-history 索引），无代码、CRD、数据库变化，不需要部署。
- 长跑脚本继续运行中（PID 272949），不要重启；重启会丢失轮次连续性。

## 停止与恢复

- 18:00 脚本自动恢复 35qps 并退出；提前停止：`kill <pid>` 后手动恢复流量（走 Backend API + `Idempotency-Key` 头，或 kubectl patch `SimulatorInstance.spec.traffic.qps`）。
- 异常恢复：读 `.runtime/longrun/2026-08-17/day-watch.log` 与 `rounds/` 定位，按 `hack/night-run/README.md` 重新启动，`--until 18:00` 会按剩余时间继续。

## 回滚

- 文档回滚：`git revert` 本次提交即可；不影响运行中的长跑。
- 容量结论与剧本规则是方法论沉淀，不涉及系统行为变更，无需回滚。
