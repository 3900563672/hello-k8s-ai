# 变更总览：降级演练缺陷修复——告警表达式三修 + 模拟器容器资源限制

> 日期：2026-08-18 ｜ 级别：P1 ｜ 对应 Issue：[#30 稳定性验收（已关闭）](https://github.com/3900563672/hello-k8s-ai/issues/30)

## 为什么做

- 降级演练第一次真实触发告警，把 8/17 提交的告警规则打出两个缺陷：内存告警对无 limit 容器假阳性；重启告警因 `container_start_time_seconds` 带容器 ID label 永远不会触发。
- 顺带发现模拟器容器没有资源 limit（宿主 OOM 风险，实测均值 17MB 不致命）。
- 用户选择方案 B：一次性修完（告警 + 资源限制 + 重建镜像部署 + 实触发验证）。

## 改成什么

1. **内存告警**（`config/observability/prometheus.yaml`）：分子加 `and on (namespace, pod, container) (limit > 0)`，无 limit 容器不再参与（原来 clamp 成 1 导致比例爆表）。
2. **重启告警**：最终式为 `(max by (namespace, container) (start_time) - max by (namespace, container) (start_time offset 10m)) > 60`——聚合键选重启前后不变的 (namespace, container)，排除 simulator 容器（扩缩容是正常行为）。
3. **模拟器容器资源限制**（`internal/controller/simulatorinstance_controller.go`）：requests 50m/64Mi，limits 500m/256Mi。

## 关键行为

- 重启告警只报稳定组件（backend/frontend/grafana/jaeger/prometheus/otel/controller/postgresql）异常重启，不报模拟器扩容；
- 内存告警只统计有 limit 的容器；
- 模拟器 Deployment 由新 controller 生成时自带 resources，滚动更新生效。

## 验证（实触发）

- promtool check rules SUCCESS（9 rules）；
- 重启 backend/prometheus 后 `HelloK8sAIContainerRestarted` firing（2 条）；
- 模拟器扩容不误报；内存表达式 matches=0（无假阳性）；
- CI 三 job 全绿（代码检查 36s / 部署验证 4m24s / E2E 5m23s）。