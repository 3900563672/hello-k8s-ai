# 实现细节

## 1. 容量公式推导（RESILIENCE.md 第 3 节底稿）

- 模拟器是单 Leader 引擎：总 QPS 均摊到 `availableReplicas`，每副本并发槽 = 模型 `maxConcurrency`。
- 单副本服务时长（model-lite，prompt 500 / output 200 token 固定）：`prefillBaseMs(50) + prefillPerTokenUs(500)×0.5 + decodePerTokenMs(20)×200 = 4300ms`。
- 单副本容量 = `maxConcurrency ÷ 平均服务时长` = 16 ÷ 4.3 ≈ **3.7 qps/副本**。
- 所需副本 = `QPS × 平均服务时长 ÷ maxConcurrency`：650 QPS × 4.3 ÷ 16 ≈ **175 副本**。
- 节点天花板 = `min(⌊gpu/gpuUnits⌋, ⌊maxConcurrency/模型maxConcurrency⌋)`；当前 2 节点 × 1600 并发 ÷ 16 = **200 副本**，峰值 650 需 ~176 副本，留有余量。

## 2. TTFT 判读与缩容滞回

- Simulator 的 TTFT 指标只在请求排队（每副本负载 ρ→1）时上升；低负载时恒等于服务基线（model-lite ≈ 320ms）。
- Orchestrator 缩容同时看 TTFT 与 queue：TTFT 基线 320ms > 缩容下阈值 300ms，导致队列排空后 `needDown` 被 TTFT 挡住 → 峰值副本数保持不回缩。这是 2026-08-17 观察结论，策略未改。

## 3. 长跑剧本与产物（day-watch.mjs v2，前序提交已实现）

- 命令行契约：`--until HH:MM`（到点自动恢复 `--final-qps` 并退出）、`--baseline-qps / --peak-qps / --peak-minutes / --cycle-minutes`、`--interval` 轮询秒数。
- 产物统一落 `.runtime/longrun/YYYY-MM-DD/`：`rounds/`（每轮 JSON）+ `snapshots/`（kubeSnapshot 含节点用量/扩缩事件）+ `summary.md`。
- 轮次间隔按上一轮耗时补足防漂移；preflight 校验 18080 端口与 sleep-guard。
- 14:17 起每 60 分钟一轮：45 分钟基线 300 + 15 分钟峰值 650；18:00 恢复 35qps 并生成 summary.md。

## 4. 本次知识沉淀内容

- RESILIENCE.md：第 3 节「容量校准公式 + 剧本设计规则」，第 4 节验收清单改为执行中状态。
- KNOWN_PITFALLS.md：新增「压测与长跑剧本设计」小节（剧本无效反例、TTFT 判读、容量天花板）。
- change-history/README.md：补登 2026-08-17 前 4 条遗漏归档 + 本条。
