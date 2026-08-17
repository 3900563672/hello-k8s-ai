# 扩容加速（批量扩容）+ 稳定性矩阵 + 容量指南

- 变更日期：2026-08-17（Asia/Shanghai 12:40~13:10；UTC 2026-08-17 04:40~05:10）
- 关联问题：无（用户直接指令；不改 CRD/API 契约，按 WORKFLOW 属行为优化，直接开发未建 issue）
- 变更级别：P2 稳定性与容量能力
- 变更范围：Controller（Orchestrator 决策与执行）、Frontend（配置详解）、Agent 文档
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

- **批量扩容**：Orchestrator 扩容决策从"每次只扩 1 副本"改为"按队列缺口批量补副本"。队列缺口 ÷ 单副本并发容量向上取整，单次最多 10 副本；引导期（低于副本地板）一次补到地板；单节点放不下整批时逐级减半回退到最小 +1。冷却（默认 60s）仍是批次间隔。
- **落地端**：`persistScalePlan` 一次写入整批副本（`addNodePlacements`），放置计划、幂等注解、ScalingRecord 语义不变。
- **稳定性矩阵**：新增 `docs/agents/RESILIENCE.md`，覆盖 K8s 各组件挂掉后的系统行为与"长时运行验收清单"（清单暂未执行）。
- **容量指南**：GuidePage"模拟条件下怎么填"新增副本吞吐换算、所需副本估算、节点承载上限、无限流量天花板、扩容节奏五条详解。

## 2. 实测结果（2026-08-17 04:42Z 压测）

- 400 QPS 压测：副本 16→18→20（每轮 +2，按当时队列缺口换算），被节点容量（2 节点 × 160 并发 ÷ 16 = 20 副本）正确拦截为 `no_feasible_placement`，Orchestrator Ready=True。
- 队列 2 分钟冲到 7 万、TTFT 小时级：目标容量（≈3.7 qps/副本 × 20 = 74 qps）远小于负载（400 qps）的数学结果，非调度 bug。
- 已恢复 35 QPS；批量扩容代码已用 `kubectl kustomize config/dev | kubectl apply -f -` 部署并运行。

## 3. 影响文件

| 文件 | 变更 |
| --- | --- |
| `internal/controller/orchestrator_decision.go` | `scaleUpDecision` 抽取 + `scaleUpDelta`/`maxScaleUpBatch`/`clampInt` 批量增量 |
| `internal/controller/orchestrator_scoring.go` | `findBestPlacement` 支持 extraReplicas 整批容量检查 |
| `internal/controller/placement_plan.go` | `addNodePlacements` 批量加副本 |
| `internal/controller/orchestrator_executor.go` | scale-up 一次写整批 |
| `internal/controller/refactor_test.go` | 批量扩容/回退/引导期/容量不足用例 |
| `dashboard/frontend/my-app/src/components/features/guide/GuidePage.tsx` | 容量详解 5 条 |
| `docs/agents/RESILIENCE.md` | 新增稳定性矩阵 |
| `docs/agents/KNOWN_PITFALLS.md` | 节点容量天花板 + Idempotency-Key 坑位 |

## 4. 未验证 / 风险

- 大并发 + 大节点容量下的批量扩容尚未验证（14:00 长时测试覆盖）。
- 缩容仍为逐副本（120s 冷却），大规模缩容较慢。
- 4 小时长时验收清单未执行。
