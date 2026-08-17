# 长跑工具修复（--until 精确停止 / 每轮指标 / summary 口径）+ 4 小时长跑验收结论

- 变更日期：2026-08-17（Asia/Shanghai 19:20~20:00；UTC 11:20~12:00）
- 关联问题：无（18:00 长跑分析后发现的 4 个工具层问题，用户指令全部修复并沉淀）
- 变更级别：P1 运行能力与可观测性
- 变更范围：`hack/night-run/day-watch.mjs`、`config/observability/prometheus.yaml`、`dashboard/backend/internal/providers/prometheus/client.go`、Agent 文档
- CRD 变化：无 ｜ 数据库变化：无

## 1. 完成结果（4 个问题全修复）

1. **--until 超时多跑一轮**：轮次间隔 sleep 不裁剪到截止时间（计划 18:00 实际 18:14 才停，多跑一轮且 18:14:49 短时 patch 650qps 7 秒）。修复：`msUntilStop()` + sleep 钳制到截止时间 + 循环顶部提前判停。实测 `--until 19:34` 在 19:34:07 整点停止，无多余轮次。
2. **summary 指标低估峰值强度**：旧 summary 只聚合 30 分钟快照（queue max=135 vs 实际 PG 序列峰值 2491）。修复：每轮轻量采样 6 个指标（轮次粒度）+ 进入峰值时预约「峰值中点」补采样；summary 新增「轮内指标」节并注明快照粒度局限；精确序列仍以 PG `resource_events`（5s）为准。
3. **summary 混入历史轮次**：rounds/ 目录跨 run 复用导致总轮数 29（混入 13:21-13:29 测试轮次）、陈旧扩缩容事件入表。修复：启动写 `meta.json`（startIso/endIso/args），summary 只统计窗口内数据；新增 `--resummarize` 重生成模式。重生成后正式 run：20 轮、扩缩容事件 2 条、快照 10 个。
4. **errorRate 恒为 null**：PromQL 空集问题——无 error 系列时 ratio 表达式塌成空。修复：controller recording rule 与 backend `simulator.errorRate` 查询加 `or on() vector(0)` 空集保护。实测两指标从空变为 0。

## 2. 4 小时长跑最终结论（2026-08-17，剧本 300 基线 + 650 峰值）

- 运行 13:29→18:14（18:14 系旧版 --until 缺陷，已修），20 轮 0 keepalive 失败、0 snapshot 失败、无 CrashLoop。
- 第一峰值（14:14-14:29，650qps）：批量扩容 141→200（10 批、60s/批，+10×4/+5×2/+2×4 按队列缺口自适应），撞节点容量天花板（2×1600÷16=200）；queue 峰值 ~2491、TTFT 峰值 ~678s；14:24 到顶后约 6 分钟排空，14:30 恢复 TTFT 320ms 稳态。
- 后续 3 个峰值（15:14/16:14/17:14）：200 副本下吞吐可扛（queue 0-20）但 TTFT 升到 ~1s（ρ≈0.88，偶发 3-4.5s）——200 是吞吐天花板，延迟舒适区约需 350 副本。
- 缩容滞回确认：TTFT 基线 320ms > 缩容下阈值 300ms，峰值后副本保持 200 不回缩（预期，未改策略）。
- 当前：35qps、200 副本、queue 0、TTFT 320ms、Orchestrator Ready。

## 3. 影响文件

| 文件 | 变更 |
| --- | --- |
| `hack/night-run/day-watch.mjs` | 截止钳制、每轮指标、峰值中点采样、meta.json、--run-dir/--resummarize |
| `config/observability/prometheus.yaml` | controller errorRatio recording rule 空集保护 |
| `dashboard/backend/internal/providers/prometheus/client.go` | simulator.errorRate 查询空集保护 |
| `docs/agents/KNOWN_PITFALLS.md` | 新增「长跑工具与可观测性」小节 4 条坑位 |
| `docs/agents/RESILIENCE.md` | 验收清单标记完成、剧本规则补延迟舒适区、风险补 errorRate |
| `hack/night-run/README.md` | day-watch 新参数与行为说明 |
| `change-history/README.md` | 补登本条 |

## 4. 未验证 / 风险

- 峰值中点采样逻辑只在 2 分钟测试剧本验证过触发与落盘，尚未经历真实 4 小时剧本（下个剧本自动生效）。
- errorRate=0 修复于长跑结束后，需下次长跑复核全程为 0。
- `--resummarize` 对无 meta.json 的旧目录会拒绝，需手工补写 meta 后重试。
