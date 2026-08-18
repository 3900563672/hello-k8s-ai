# 参与贡献

欢迎通过 Issue 与 PR 参与 hello-k8s-ai。项目采用 **Issue 驱动 + 文档同步** 的协作方式：任何改动都要能追溯到问题，任何代码变更都要同步文档。

## 1. 快速导航

| 内容 | 入口 |
| --- | --- |
| 项目总览 | [README.md](README.md)、[PROJECT_OVERVIEW_NEW.md](PROJECT_OVERVIEW_NEW.md) |
| 人类文档索引 | [docs/INDEX.md](docs/INDEX.md) |
| 本地 Agent 指南 | [AGENTS.md](AGENTS.md) + [docs/agents/README.md](docs/agents/README.md) |
| 变更历史 | [change-history/README.md](change-history/README.md) |

## 2. 提 Issue

仓库已配置三类模板：

| 模板 | 用途 | 模板文件 |
| --- | --- | --- |
| Bug | 缺陷报告 | `.github/ISSUE_TEMPLATE/1-bug.yml` |
| Feature | 新功能 | `.github/ISSUE_TEMPLATE/2-feature.yml` |
| Design | 设计方案 | `.github/ISSUE_TEMPLATE/3-design.yml` |

一个好 Issue 包含：**问题描述（现状与影响）→ 期望状态 → 修改方向 → 验收标准**。设计类 Issue 建议先给方案再写代码，避免返工。

## 3. 提交 PR

1. 从 `main` 开分支，命名用 `feat/`、`fix/`、`docs/`、`refactor/` 前缀。
2. 遵守仓库约束：Reconcile 幂等、最小改动、保留字段所有权语义、不手改生成文件（`config/crd/bases/`、`config/rbac/role.yaml`、`zz_generated`）。
3. **同步文档**：改源码必须更新 [docs/MAP.yaml](docs/MAP.yaml) 映射的专题文档（CI 强制拦截，见 [docs/README.md](docs/README.md)）。
4. **验证**：代码改动运行 `make verify`；文档改动运行 `make docs-check` 与 `make docs-sync-check`。
5. **归档**：变更记录追加到 `change-history/YYYY-MM-DD-<slug>/README.md`，并运行 `make docs-sync`。

## 4. 验证命令

| 检查项 | 命令 | 说明 |
| --- | --- | --- |
| 全部静态验证 | `make verify` | Go 格式/测试/lint + Backend + Frontend + 三套 Kustomize 渲染 |
| 生成文件 | `make manifests generate YEAR=2026` | 改 `api/v1` 后执行并核对差异 |
| 文档门禁 | `make docs-check` | 链接、白名单、MAP 门禁、时间戳、归档格式 |
| 派生文档同步 | `make docs-sync-check` | 提交后工作区无派生差异 |

## 5. 协作约定

- **一个逻辑闭环 = 一个 commit**：中间过程本地 squash，提交信息用中文，格式 `类型: 中文摘要`（如 `fix: ...`、`docs: ...`），关联 Issue 加 `Fixes #N`。
- **本地开发环境**：复用 Docker Desktop 已有 Kubernetes（Context `docker-desktop`），`bash setup.sh` 一键部署；不要创建、重置或删除集群。
- **自动化 E2E**：使用独立 Kind 集群 `hello-k8s-ai-test-e2e`，不复用日常开发集群。
- **AI 协作**：本地 Agent 见 [docs/agents/](docs/agents/README.md)，远程 AI 见 [docs/remote-ai/](docs/remote-ai/README.md)；人类如何指挥 AI 见 [docs/getting-started/AI_COLLABORATION.md](docs/getting-started/AI_COLLABORATION.md)。
