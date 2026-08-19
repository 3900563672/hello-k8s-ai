# 白天长时运行收尾 + Kind 迁移数据备份（#50 前置）

> 日期：2026-08-18

## 为什么做

- 潮汐窗口（10:15–12:00 CST）用 day-watch 无人值守长跑，验证 #48 峰值相位修复后的真实表现，并为 Kind 底座切换提供迁移前基线数据。
- #50（Kind 底座）代码已合入，实际切换前必须完成三套数据（PostgreSQL / Prometheus / Jaeger）的完整备份，避免切换过程丢历史。

## 改成什么

1. **长时运行（day-watch）**：10:15–12:00 连续 11 轮，35qps 基线 + 3 次 50qps 峰值（10:35 / 11:05 / 11:35，各 10 分钟）；keepalive 与 snapshot 0 失败；controller/simulator errorRate 全程 0。
   - 峰值 1（10:35）：13→19 副本扩容 6 个，扩缩容事件正常（18→19 @02:43:18Z）。
   - 峰值 2（11:05）被 #48 验证的 `--once` 截断约 6 分钟（流量提前回 35），如实记录。
   - 峰值 3（11:35）：完整 10 分钟 50qps，副本保持 19，queue 峰值 4、TTFT 峰值 626ms。
   - 12:00 收尾：qps 恢复 35，副本 19（缩容滞回属预期，见 RESILIENCE.md 3.4）。
   - 产物：`.runtime/longrun/2026-08-18/`（不入库）、`/tmp/scale-sample-2026-08-18.csv`、`/tmp/longrun-final-2026-08-18.txt`。
2. **迁移前备份（#50 前置）**：`bash hack/kind/backup-data.sh` 产出 `/var/tmp/hello-k8s-ai-backup-20260818-120414/`：
   - `dashboard.sql` 2.68GB（532,499 行，pg_dump --clean）
   - `prometheus.tar.gz` 276MB（TSDB + WAL，含 cAdvisor 抓取）
   - `jaeger.tar.gz` 413MB（badger）
   - tar 完整性已验证（`tar tzf` 全量列出）；恢复脚本配套 `restore-data.sh` 已同步修复。
3. **备份脚本时序修复**（提交 13c74aa / 4f043a4）：helper Pod 的 `kubectl wait Ready` 只保证容器启动、不保证 tar 完成，首版备份复制出半成品包（几 MB 级）。修复：tar 完成后 `touch /out/done`（解包侧 `/in/done`），脚本轮询完成标志再拷贝，超时 300s 报错退出。
4. **切面存储设计 issue**（#51）：数据切面快照作为第一公民的设计沉淀（生命周期 / 混合采样 / 分层存储 / 容量测算），挂 Project Review `To do` 等用户放行。

## 关键行为

- 备份/恢复期间对应 Deployment 缩到 0（Prometheus / Jaeger），结束后自动恢复；PostgreSQL 热备不缩容。
- 长时运行数据口径：轮次粒度可能错过峰值中段真实强度，精确序列以 PG `resource_events`（5s）为准。
- 峰值 2 截断是验证动作的有意副作用，不是 day-watch 缺陷；#48 的 `--strict-phase` 已保证错位组合启动即拒绝。

## 验证

- day-watch 11 轮：keepalive 0 失败、snapshot 0 失败、errorRate=0（summary.md 与 rounds/ 完整记录）。
- 备份产物：三件套齐全且 `tar tzf` 完整；`make selfcheck` 通过；CI（代码检查 / 源码与部署验证 / E2E / 文档检查）全绿。
- 未验证：Kind 底座上的恢复演练（等用户关闭 Docker Desktop 内置 Kubernetes 后执行 `make cluster-up` + `restore-data.sh`）。

## 回滚

- 长时运行：`make cluster-down` 停止负载；数据保留在 PG/PVC。
- 备份修复：`git revert 13c74aa 4f043a4` 即回到首版脚本（存在半成品包风险，不建议）。
- 数据迁移：如恢复后校验失败，用备份目录重新执行 `restore-data.sh`（幂等，PG 侧 `--clean` 会先 DROP 再重建）。
