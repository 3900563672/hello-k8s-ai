# 实现细节

## 1. --until 精确停止（day-watch.mjs）

- 旧逻辑：do-while 循环每轮结束后才查 `shouldStop()`，轮间 sleep 固定 `INTERVAL`（900s）→ 18:00 前最后一轮 17:59:50 后仍睡满一轮，18:14 多跑一轮并短时 patch 650qps。
- 新逻辑：
  - `msUntilStop()`：`--until`（今日本地 HH:MM）或 `--hours`（启动后 N 小时）的剩余毫秒；无截止返回 `Infinity`。
  - `sleep = max(5000, min(轮次间隔补足, 剩余时间))`；`remaining === 0` 直接 break。
  - 循环顶部 `round > 0 && shouldStop()` 提前判停（第一轮无论如何先跑，保持原有语义）。
- 验证：`--until 19:34` 测试运行 19:34:07 停止（含恢复流量 + summary），无多余轮次。

## 2. 每轮指标 + 峰值中点采样

- `fetchMetrics()`：GET `/api/v1/metrics?metricId=<id>&step=300s` × 6（controller.errorRate、simulator.errorRate、simulator.ttft、simulator.queue、simulator.qps、simulator.tickLatency），取最后一个有限值；写入 `record.metrics`，stdout 摘要行带 metrics 字段。
- `scheduleMidPeakSample()`：模块级 `prevTarget` 记忆上一轮档位，检测到「基线→峰值」相位切换时 `setTimeout(PEAK_MINUTES/2 分钟)` 再采样一次，落 `metric-samples/mid-peak-round-<n>-<ts>.json` 并进内存列表。
- summary「轮内指标」节：聚合每轮 + 峰值中点采样（按 ts 去重，内存与落盘双路径只计一次），输出 min/max/last + 点数；注明快照粒度局限、以 PG `resource_events` 为准。

## 3. meta.json 与 summary 口径

- 启动写 `meta.json`（runDir/startIso/pid/args/startedAt），`finish()` 补 `endIso`。
- `buildSummary` 只统计 `[startIso, endIso]` 窗口内的 rounds/snapshots/metric-samples；扩缩容事件按事件时间（`scaling.time`）过滤，避免把历史 run 的 `lastScaling` 计入。
- `--resummarize <dir>`：只读不写流量，重生成 summary.md（结束时间取 `meta.endIso`；无 meta.json 报错退出）。
- `--run-dir <dir>`：覆盖产物目录（测试隔离，避免污染正式 run 目录）。

## 4. PromQL 空集保护（errorRate）

- 根因：`sum(rate(x{outcome="error"}[5m]))` 在没有任何 error 系列时返回空集，分子空 → 整个 ratio 表达式无结果 → 后端 `series: []` → 快照 null。系统「零错误」反而显示「无数据」。
- 修复（分子分母都加 `or on() vector(0)`）：
  - `config/observability/prometheus.yaml`：`hello_k8s_ai:controller_reconcile_error_ratio:rate5m` recording rule。
  - `dashboard/backend/internal/providers/prometheus/client.go`：`simulator.errorRate` 查询。
- 验证：部署后 `controller.errorRate` / `simulator.errorRate` 均返回 1 条 series、值 0。
- 备注：以后写「错误比例 / 占比」类 PromQL 都要带空集保护，否则零错误时面板显示「无数据」而非 0。
