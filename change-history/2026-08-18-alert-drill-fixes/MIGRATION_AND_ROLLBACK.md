# 迁移与回滚：降级演练缺陷修复

## 迁移说明

- 无 CRD/数据库变更。
- 模拟器容器 resources 由 controller 在生成 Deployment 时注入，存量模拟器实例在 controller 升级后由 Reconcile 自动更新模板（滚动替换 Pod）。

## 回滚

- 控制器镜像：重新 `make docker-build` 旧代码并 `rollout restart`；
- 告警规则：`git checkout 1129d9c -- config/observability/prometheus.yaml` 后 apply + restart prometheus（注意用 `-n hello-k8s-ai-system`）。

## 风险

- 重启告警的 offset 10m 窗口在 Prometheus 刚启动（TSDB 加载期间）可能短暂无数据，for: 1m 会重置计时，属正常现象；
- 模拟器实例当前在跑（tenant-core-model-lite，Orchestrator 自动扩缩），如需清负载删除对应 TenantModelPolicy。
