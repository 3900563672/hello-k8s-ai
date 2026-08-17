# 测试报告

## 静态检查

- `make docs-check`：通过（全部 Markdown 链接校验）。
- `make context-pack`：上下文包重新生成（`.runtime/context-pack/`，不入库）。
- 前序代码提交（`e76650d` 批量扩容、`f3ae6a5/5cbe880` 长跑脚本 v2）CI 全绿；本次仅文档变更，CI 只跑文档检查。

## 真机验证（docker-desktop 集群，2026-08-17 13:29 CST 起）

- `day-watch.mjs`（PID 272949）运行中：13:29 轮把 `SimulatorInstance.spec.traffic.qps` 200→300 patch 成功（attempt 1）；keepalive 全绿（health/live、health/ready、traffic 300qps、overview clock=running rate=20、simulator.errorRate 正常）。
- 基线快照：`tenant-core-model-lite=141/141`；节点用量 desktop-worker 1136/1600、desktop-worker2 1120/1600（并发容量顶 1600）。
- 最近扩缩事件：ScaleUp 140→141（05:12Z，批量扩容生效后逐批收敛）。
- 300 QPS 稳态：queue≈0、TTFT≈320ms。

## 结论（18:00 后已出）

- 650 QPS 峰值触发批量扩容 141→200（10 批、60s/批，+10×4/+5×2/+2×4），撞 200 副本天花板（节点 1600/1600 并发满载）。
- queue 峰值 ~2491、TTFT 峰值 ~678s（第一峰值前 12 分钟），14:24 到顶后 ~6 分钟排空，14:30 恢复 TTFT 320ms。
- 后续 3 个峰值在 200 副本下吞吐可扛（queue 0-20）但 TTFT ~1s（ρ≈0.88），200 是吞吐天花板、延迟舒适区约需 350 副本。
- 缩容滞回确认：峰值后副本保持 200 不回缩（预期）。
- 18:14 恢复 35qps（旧版 --until 缺陷多跑一轮，已修复）；详细结论与工具修复见 change-history/2026-08-17-longrun-tooling-fixes/。
