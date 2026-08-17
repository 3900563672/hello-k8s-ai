# Prometheus 内存告警对无 limit 容器假阳性：分子必须过滤

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-17-observability-and-storage.md 与 2026-08-18-prometheus-alert-three-pitfalls.md ｜ 适用对象：本地 Agent

## 现象

内存告警对没有配置 limit 的容器全部触发：limit=0 被 `clamp_min(limit,1)` 钳成 1，使用率比例直接爆表。

## 根因

`container_spec_memory_limit_bytes` 为 0 表示无 limit，不能参与使用率计算。

## 可复用规则

- 内存使用率分子加 `and on (namespace, pod, container) (container_spec_memory_limit_bytes{...} > 0)` 过滤无 limit 容器。
- 新增容器资源上限时同时检查告警表达式是否覆盖（模拟器容器现在有 limit）。

## 验证方法

`promtool check rules` + 表达式实时查询：无 limit 容器不出现在告警结果中。
