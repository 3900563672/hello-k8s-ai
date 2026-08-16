# AI_CONTEXT - 文档分层入口

> 本文件曾是"给任何新 AI 的交接上下文"。文档已按读者分层（2026-08-16），原内容已迁移，请按身份选择入口。

| 你是谁 | 入口 |
| --- | --- |
| 能操作本机仓库的 Agent（Codex、Claude Code） | 先读根目录 `AGENTS.md`，再读 [docs/agents/README.md](agents/README.md) |
| 只在自己工作区工作、收打包内容的远程 AI | [docs/remote-ai/README.md](remote-ai/README.md)，包内先读 `CONTEXT_PACK.md` |
| 人类读者 | 根目录 [README.md](../README.md)，或 [docs/INDEX.md](INDEX.md) |

## 内容去向

- 架构约束、字段所有权速查、Controller 名称、修改规范 → [docs/agents/PRINCIPLES.md](agents/PRINCIPLES.md)
- 已知易误判点 → [docs/agents/KNOWN_PITFALLS.md](agents/KNOWN_PITFALLS.md)
- 视觉验证与监控面板链路 → [docs/agents/UI_VERIFICATION.md](agents/UI_VERIFICATION.md)（Agent 专用：一键截图/读面板、面板现状与已知问题清单）
- 当前状态快照与上下文包 → `make context-pack` 生成（模板 `hack/context-pack-template.md`），输出 `.runtime/context-pack/`
- 历史验证基线 → `change-history/` 对应条目
- 完整字段所有权 → [docs/kubernetes/FIELD_OWNERSHIP.md](kubernetes/FIELD_OWNERSHIP.md)

## 基线速览（2026-08-14，详细以源码为准）

- 11 个 Cluster-scoped CRD；7 个 Reconciler；Simulator 1x..20x 倍速、Lease 选主、5 秒真实 Tick。
- Backend：informer cache、幂等写、PostgreSQL 快照/事件/审计、Prometheus/Jaeger provider、REST/SSE。
- Frontend：`/config`、`/traffic`、`/trace`，真实 Backend 查询。
- 部署：复用 `docker-desktop`，`bash setup.sh` 一键完成预检、构建、导入镜像、部署与验收。
- 未实现：pause/Seek/确定性回放、生产认证授权、PostgreSQL/Prometheus/Jaeger HA 与备份、真实推理数据面。
