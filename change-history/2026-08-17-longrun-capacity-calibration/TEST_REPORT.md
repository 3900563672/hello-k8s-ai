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

## 未验证 / 待出结论（18:00）

- 650 QPS 峰值是否触发队列与批量扩容、是否撞 200 副本天花板。
- 18:00 自动恢复 35qps 与 summary.md 生成。
- 缩容滞回结论复核。
