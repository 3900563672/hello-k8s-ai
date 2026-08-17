# 容器重启告警三坑：container_id label、聚合键、扩容噪音

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-18-prometheus-alert-three-pitfalls.md ｜ 适用对象：本地 Agent

## 现象

重启告警永远不触发（`changes(container_start_time_seconds[10m]) > 0`）；改为 offset 差值后聚合键含 pod 仍不触发；模拟器扩容时误报重启。

## 根因

1. 指标带 container_id label，容器重启后是全新序列，`changes()` 数不出"消失+出现"；
2. Pod 名随重启变化，聚合键含 pod 时 offset 两侧 label 不匹配恒空；
3. 模拟器扩缩容也是新 Pod 启动，会被当成重启。

## 可复用规则

- 重启检测用 offset 差值法，并**按 `(namespace, container)` 聚合**（去掉 pod 与 container_id）。
- 排除会因扩缩容产生新 Pod 的工作负载：`container!="simulator"`。
- 最终式：`max by (namespace, container) (container_start_time_seconds{...} offset 10m)` 与当前值差 > 阈值。

## 验证方法

人为删除 Pod 触发重启，确认告警 firing；扩容模拟器副本数，确认不误报；`promtool test rules` 固化。
