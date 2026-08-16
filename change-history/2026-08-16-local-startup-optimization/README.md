# 本地完整栈本机验证与启动速度优化

- 变更日期：2026-08-16
- 关联问题：Refs #11（优化一键启动流程性能）
- 变更级别：P2 本地开发体验
- 变更范围：hack/local-cluster.sh（启动脚本）、仓库文件执行位
- CRD 变化：无
- 数据库变化：无（复用 Phase 1-3 的迁移与持久化）

## 1. 完成结果

在用户本机 docker-desktop 集群完成完整栈真实验证（此前只依赖 CI），并优化一键启动耗时：

- 基线：`bash setup.sh` 全程 6 分 1 秒（镜像构建串行约 5 分钟为主）。
- 优化后：3 分 38 秒（提速约 40%）。

## 2. 本机验证结果（真实 docker-desktop 集群）

| 验证项 | 结果 |
| --- | --- |
| initContainer `wait-for-postgresql` | Completed / exit 0（真实集群行为，此前唯一未验证项） |
| 迁移自动应用 | 启动日志 3 个迁移依次应用；健康接口 `migrationsApplied=3` |
| 重启历史恢复 | `historyRecovered=true`；事件 26.6 万+、快照 2000+、当前态 110+ 持续累积 |
| 读路径数据库优先 | `/api/v1/configuration` 的 `asOf` 与数据库 `captured_at` 一致，比 `serverTime` 晚一个快照周期内 |
| `/api/v1/resources` | count=110，按 kind 完整分布（Model/WorkerNode/Tenant/策略/实例/Pod/Deployment/Node） |
| 时间线 | `/api/v1/replay` 基于数据库事件正常返回 |
| 完整验收 | `verify_data_flow` 10 项全部通过（含 Prometheus 指标、Jaeger、Frontend） |

## 3. 优化内容

- `hack/local-cluster.sh`：四个项目镜像并行构建（原串行），本机 24 CPU 下构建阶段显著缩短。
- 第三方运行镜像本地已存在时跳过 `docker pull`。
- 修复 `hack/local-cluster.sh` 丢失执行位（100644 → 100755），`setup.sh` 此前直接 `Permission denied`。
- 修复端口转发存活检查：原来只看 PID/ps，进程死亡但 PID 文件残留时误判"已在运行"；现在增加 `/dev/tcp` 端口探测，残留自动清理并重建。

## 4. 资料入口

- [测试与验证记录](TEST_REPORT.md)
