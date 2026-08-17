# 长跑工具与可观测性（2026-08-17 修复）

> 日期：2026-08-17 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-17 --until 超时多跑一轮：轮间 sleep 不裁剪到截止时间（已修复）
- 现象：`--until 18:00` 实际 18:14 才恢复流量并退出，18:14:49 还短时 patch 650qps（7 秒后恢复 35）。
- 原因：旧版 do-while 循环里 `shouldStop()` 只在每轮结束后检查，轮间 sleep 固定 INTERVAL（900s），截止前最后一轮后仍睡满一轮。
- 解决：`msUntilStop()` 计算剩余时间，`sleep = min(轮次间隔补足, 剩余)`；循环顶部 `round > 0` 时提前判停；`remaining == 0` 直接 break（day-watch.mjs 已修）。
- 验证：`--until 19:34` 测试运行 19:34:07 整点停止（含恢复流量 + summary），无多余轮次。

### 2026-08-17 30 分钟快照错过峰值强度：summary 会严重低估峰值指标
- 现象：4 小时长跑 summary 显示 queue max=135，实际峰值 ~2491（PG `resource_events` 5s 序列）。
- 原因：快照每 2 轮（30 分钟）一次，恰好落在峰值起始 2 秒处；15 分钟轮次也覆盖不到 15 分钟峰值的中间段。
- 解决：day-watch 每轮轻量采样 6 个指标 + 进入峰值时预约「峰值中点」补采样（summary「轮内指标」节）；精确序列始终以 PG `resource_events`（5s）为准。
- 验证：测试剧本触发峰值中点采样并落盘；正式 run 已重生成 summary（快照局限已注明）。

### 2026-08-17 rounds 目录跨 run 复用：summary 混入历史轮次（已修复）
- 现象：正式 run 的 summary「总轮数 29」，混入 13:21-13:29 测试轮次；陈旧扩缩容事件（05:12:53Z）入表。
- 原因：rounds/ 按日期目录复用，多轮 run（测试/正式）文件混在一起，旧版 buildSummary 全量统计。
- 解决：启动写 `meta.json`（startIso/endIso/args），summary 只统计 `[startIso, endIso]` 窗口；扩缩容事件按事件时间过滤；新增 `--resummarize` 重生成模式（无 meta.json 会拒绝，需手工补）。
- 验证：正式 run 补 meta 后重生成：20 轮、事件 2 条、快照 10 个。

### 2026-08-17 PromQL 空集：零错误时 ratio 类指标塌成空（errorRate 恒为 null，已修复）
- 现象：controller.errorRate / simulator.errorRate 在快照与指标 API 里恒为 null，即使系统零错误。
- 原因：`sum(rate(x{outcome="error"}[5m]))` 在没有任何 error 系列时返回空集，分子空 → 整个 ratio 表达式无结果（Prometheus 经典空集问题）。
- 解决：分子分母都加 `or on() vector(0)`（`config/observability/prometheus.yaml` recording rule + `dashboard/backend/internal/providers/prometheus/client.go` simulator.errorRate 查询）。
- 验证：部署后两个指标均返回 1 条 series、值 0。
- 备注：以后写「错误比例 / 占比」类 PromQL 都要带空集保护，否则零错误时面板显示「无数据」而非 0。
