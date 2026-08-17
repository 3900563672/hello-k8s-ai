# Prometheus 容器告警三个坑：container_id label、Pod 名随重启变化、模拟器扩容噪音

> 日期：2026-08-18 ｜ 触发者：本地 Agent ｜ 相关：change-history/2026-08-18-alert-drill-fixes/（原 KNOWN_PITFALLS 顶部条目，拆分时单独归档）

### 2026-08-18 Prometheus 容器告警三个坑：container_id label、Pod 名随重启变化、模拟器扩容噪音
- 现象（降级演练实触发验证）：
  1. 重启告警 `changes(container_start_time_seconds[10m]) > 0` 永远不触发——该指标带 container_id label，容器重启后是全新序列，changes 数不出"消失+出现"；
  2. 改为 offset 差值法后若聚合键含 pod（`max by (namespace, pod, container)`）仍不触发——Pod 重启后 Pod 名变化，当前侧与 offset 侧的 label 集不匹配，相减恒空；
  3. 模拟器实例（每实例一 Pod，Orchestrator 动态扩缩）新 Pod 启动会让 start_time 变化，重启告警误报；
  4. 内存告警 `clamp_min(container_spec_memory_limit_bytes, 1)` 对无 limit 容器（limit=0 被 clamp 成 1）比例爆表假阳性。
- 原因：cAdvisor 指标语义 + 滚动/扩容生命周期；聚合键必须选"重启前后不变的 label"。
- 解决（提交 5fa4da6/771962d）：
  1. 内存告警分子加 `and on (namespace, pod, container) (limit > 0)` 过滤无 limit 容器；
  2. 重启告警最终式：`(max by (namespace, container) (container_start_time_seconds{container!="", container!="simulator", ...}) - max by (namespace, container) (... offset 10m)) > 60`——按 (namespace, container) 聚合（稳定）、排除 simulator、差值 >60s；
  3. 模拟器容器补 resources（requests 50m/64Mi，limits 500m/256Mi），实测均值 17MB。
- 验证：promtool check rules SUCCESS（9 rules）；重启 backend/prometheus 后告警 firing（排除 simulator 后）；模拟器扩容不误报；内存表达式 matches=0（无假阳性）。
- 备注：`for: 10m` 的内存告警完整触发验证耗时过长，本次用表达式实时查询验证正确性；`for: 1m` 的重启告警做了完整 firing 验证。
