# 实现细节

## 1. 变更前状态

- 长时运行靠人工值守：用户盯着 Agent 跑 20 多分钟，无法在无人时段持续施压与采集。
- 无统一问题档案：发现的问题散落在对话里，下一个 Agent 无法接手。
- 无自动化入口：Codex 桌面调度能力未配置，无法 00:00 自动开工。

## 2. 实现

- `hack/night-run/keepalive.mjs`：HTTP 健康探针 + traffic/overview/metrics 检查 + kubectl Pod 汇总；`--once` 单次（退出码 0/1），`--loop` 常驻；健康探针不可达时自动 `make cluster-open` 恢复端口转发；httpGet 网络层错误自动重试 3 次（port-forward 连接复用偶发失败）。
- `hack/night-run/snapshot.mjs`：并行采集 6 个 metricId（controller.errorRate 等）+ overview（时钟/配置/实例）+ traffic + resources + ready 探针明细；写入 `.runtime/night-run/<本地日期>/snapshots/<UTC时间>.json`；`--summary` 输出人读摘要。
- `hack/night-run/phase_a_prompt.md` / `phase_b_prompt.md`：四槽位（窗口/目标/红线/输出）可复用提示词模板。
- `hack/night-run/problems.template.md`：问题档案模板，Phase A 实例化到 `.runtime/night-run/2026-08-17/problems.md`。
- 自动化 TOML：`$CODEX_HOME/automations/<uuid>/automation.toml`，`kind = "cron"`、`status = "ACTIVE"`、`target = { type = "project", project_id = "c6e05b5a-b28f-40da-911f-5f4c20863353" }`、`cwds = ["\\\\wsl.localhost\\Ubuntu\\root\\hello-k8s-ai"]`；RRULE 按本地时区（Asia/Shanghai）。

## 3. 关键决策

- 两段式而非单条自动化：Phase A 只采集不推码（避免夜间无人审核的代码进入仓库），Phase B 才动代码。
- 非运行日空跑：prompt 首步判断日期与窗口，避免每天浪费 Token。
- 问题档案不入库：原始上下文可能含环境敏感信息；摘要进 change-history。
- 快照用 Backend API 而非直连 Prometheus：单入口、带认证/幂等约定，与前端同一事实源。