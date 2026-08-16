# 变更归档

本目录保存需要长期追踪的代码变更记录。它不替代 `docs/` 中的当前架构说明：`docs/` 描述现在如何运行，本目录说明某次修改为什么发生、改了什么、如何测试和回滚。

| 日期 | 主题 | 级别 | 入口 |
| --- | --- | --- | --- |
| 2026-08-14 | Simulator 时间倍速控制链路 | P1 | [查看记录](2026-08-14-simulator-time-scale/README.md) |
| 2026-08-14 | Model 能力基准分生产路径修复 | P0 | [查看记录](2026-08-14-model-absolute-score-production-path/README.md) |
| 2026-08-14 | Orchestrator 放置修复的 CI 收敛 | P0 follow-up | [查看记录](2026-08-14-orchestrator-placement-ci-follow-up/README.md) |
| 2026-08-14 | Orchestrator 选点执行契约修复 | P0 | [查看记录](2026-08-14-orchestrator-placement/README.md) |
| 2026-08-16 | 文档体系分层重构 | P1 | [查看记录](2026-08-16-docs-hierarchy/README.md) |
| 2026-08-16 | 分层文档维护边界与同步协议 | P1 | [查看记录](2026-08-16-docs-layered-ownership/README.md) |
| 2026-08-16 | 数据库生命周期自动化与当前态持久化读路径（Phase 1-3） | P1 | [查看记录](2026-08-16-database-lifecycle/README.md) |
| 2026-08-16 | 本地完整栈本机验证与启动速度优化 | P2 | [查看记录](2026-08-16-local-startup-optimization/README.md) |
| 2026-08-16 | CI 加速与工作流细化（轮询节奏 / 归档详略） | P1 | [查看记录](2026-08-16-ci-acceleration-and-workflow/README.md) |
| 2026-08-16 | 可观测性收敛到 Dashboard 单入口（Prometheus / Jaeger / Grafana） | P1 | [查看记录](2026-08-16-observability-single-entry/README.md) |
| 2026-08-16 | 修复 Grafana 运行中内存打满导致探针失败与组件意外停止 | P1 | [查看记录](2026-08-16-grafana-memory-stability/README.md) |
| 2026-08-16 | 前端策略管理打通配置到真实工作负载的完整闭环 | P1 | [查看记录](2026-08-16-frontend-policy-closed-loop/README.md) |

> 详略规范：每条目四件套齐全（README 精简总览 + IMPLEMENTATION_DETAILS / TEST_REPORT / MIGRATION_AND_ROLLBACK 完整细节），见 `docs/agents/SYNC.md`。
