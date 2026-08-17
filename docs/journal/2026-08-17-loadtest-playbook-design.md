# 压测与长跑剧本设计（2026-08-17 容量校准确立）

> 日期：2026-08-17 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-17 剧本无效：峰值 QPS ÷ 当前副本数 < 单副本容量（队列恒为 0）
- 现象：200/350 QPS 剧本在 141 副本下 queue 始终为 0、TTFT 恒等于基线，扩容一次都不触发，看似"稳定"实为无效负载。
- 原因：单副本容量 ≈ `maxConcurrency ÷ 平均服务时长`（model-lite ≈ 3.7 qps）；141 副本 × 3.7 ≈ 520 qps 总容量，剧本峰值远低于容量，请求永远不排队。
- 解决：写剧本前先算容量公式与所需副本：`平均服务时长 = prefillBaseMs + prefillPerTokenUs×0.5 + decodePerTokenMs×200`（prompt 500 / output 200 固定）；`所需副本 ≈ QPS × 平均服务时长 ÷ maxConcurrency`；保证 `峰值 QPS ÷ 当前副本数 > 单副本容量` 才会产生队列与扩容。
- 验证：650 QPS @ 141 副本（4.6/副本 = 1.25×容量）实测触发批量扩容 141→200（queue 峰值 ~2491、TTFT 峰值 ~678s，到顶后 ~6 分钟排空）；300 QPS @ 141 副本（0.57×容量）实测 queue 0。
- 备注：400 QPS @ 20 副本（5.4×容量）队列 2 分钟冲到 7 万、TTFT 小时级是数学结果，不是调度 bug；压测前先按公式放大 WorkerNode 容量（见 hack/night-run/README.md）。

### 2026-08-17 TTFT 只在排队时上升：TTFT=320ms 不代表没负载
- 现象：低负载下 TTFT 恒等于服务基线（model-lite ≈ 320ms），用 TTFT 判断是否扩容会误判"无压力"。
- 原因：TTFT 指标只在请求排队（每副本负载 ρ→1）后上升。
- 解决：判断负载以 queue 为主、TTFT 为辅；TTFT 变化只作为"已经排队"的旁证。
- 验证：300 QPS @ 141 副本 queue=0、TTFT 320ms；400 QPS @ 20 副本 queue 7 万、TTFT 小时级。

### 2026-08-17 缩容滞回：TTFT 基线高于缩容下阈值时峰值副本保持不回缩（预期）
- 现象：队列排空后副本数不回落到基线规模。
- 原因：Orchestrator 缩容同时看 TTFT 与 queue，model-lite TTFT 基线 320ms > 缩容下阈值 300ms，`needDown` 被 TTFT 挡住。
- 解决：这是观察到的预期行为（滞回），长跑结束后副本保持峰值规模不算故障；要恢复需改缩容阈值策略（本期未改）。
- 验证：2026-08-17 14:00-18:00 长跑观察中，结论以 summary.md 为准。
