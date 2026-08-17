# 容量校准公式确立 + 14:00-18:00 长跑验证（执行中）

- 变更日期：2026-08-17（Asia/Shanghai 13:00~13:30；UTC 05:00~05:30）
- 关联问题：无（用户指令：把已有成果沉淀为知识，防止后续 AI 重犯同样的错）
- 变更级别：P1 长时运行能力与容量方法论
- 变更范围：`docs/agents/RESILIENCE.md`、`docs/agents/KNOWN_PITFALLS.md`、`change-history/README.md`（前序提交已含 day-watch v2 与批量扩容）
- CRD 变化：无 ｜ 数据库变化：无

## 1. 完成结果

- **容量校准公式（写剧本前必须先算）**：单副本容量 ≈ `maxConcurrency ÷ 平均服务时长`；平均服务时长 = `prefillBaseMs + prefillPerTokenUs×0.5 + decodePerTokenMs×200`（prompt 500 / output 200 固定）。model-lite：50 + 250 + 4000 = 4300ms → 单副本 ≈ **3.7 qps**。
- **TTFT 判读规则**：TTFT 只在排队时上升，低负载恒等于服务基线（≈320ms）；`TTFT=320ms 不代表没负载`，扩容判定以 queue 为主、TTFT 为辅。
- **剧本设计规则**：峰值 QPS ÷ 当前副本数 > 单副本容量，剧本才会产生队列与扩容；反例 200/350qps @ 141 副本（≈0.5×容量）队列恒为 0，整个剧本是无效负载。
- **长跑执行中**：`day-watch.mjs --until 18:00 --baseline-qps 300 --peak-qps 650 --peak-minutes 15 --cycle-minutes 60 --final-qps 35`（PID 272949，13:29 CST 启动），产物 `.runtime/longrun/2026-08-17/`，结论以 18:00 summary.md 为准。

## 2. 实测校准数据点（2026-08-17，rate=20，model-lite）

| 场景 | 负载/副本 | 结果 |
| --- | --- | --- |
| 400 QPS @ 20 副本 | 20/副本 = 5.4×容量 | 队列 2 分钟冲到 7 万、TTFT 小时级（数学结果，非调度 bug） |
| 300 QPS @ 141 副本 | 2.1/副本 = 0.57×容量 | 队列 0、TTFT 320ms 稳定基线 |
| 650 QPS @ 141 副本 | 4.6/副本 = 1.25×容量 | 应触发队列与批量扩容（14:17 峰值验证中） |

## 3. 影响文件

| 文件 | 变更 |
| --- | --- |
| `docs/agents/RESILIENCE.md` | 第 3 节改为容量校准公式 + 剧本设计规则；第 4 节验收清单改为执行中状态 |
| `docs/agents/KNOWN_PITFALLS.md` | 新增「压测与长跑剧本设计」小节（3 条坑位） |
| `change-history/README.md` | 补登 2026-08-17 全部 5 条归档（前 4 条此前漏登，属文档漂移修复） |

## 4. 结论与后续（2026-08-17 18:00 后）

- 650 QPS 峰值实测触发批量扩容 141→200，queue 峰值 ~2491、TTFT 峰值 ~678s，恢复后 TTFT 320ms；缩容滞回确认（副本保持 200）。
- 长跑暴露的 4 个工具层问题（--until 超时 / 快照粒度 / summary 口径 / errorRate 空集）已在 change-history/2026-08-17-longrun-tooling-fixes/ 修复并沉淀。
