# 2026-08-13 部署整理与一键启动交付（原根目录 CHANGELOG.md）

> 变更日期：2026-08-13 ｜ 关联问题：无 ｜ 变更级别：P1
> 变更范围：部署、文档
> 数据库变化：PostgreSQL 改为部署时生成随机密码，使用 `standard` StorageClass 与 10Gi PVC；重复部署保留 Secret/PVC

## 背景与决策

- 原日常部署会在旁边另建 Kind 集群，与 Docker Desktop 内置 Kubernetes 并存混乱；决定固定复用当前 `docker-desktop` Context，不创建、不重置、不删除集群。
- 部署步骤分散在多个手工命令中，改为一键 `bash setup.sh`：清理废弃内容后一条命令完成预检、镜像准备、部署、演示数据和链路验收。

## 实现摘要

- 新增 `setup.sh`：清理已确认废弃内容，构建 Controller / Simulator / Dashboard Backend / Frontend 四个镜像，预拉取 PostgreSQL / Prometheus / OTel Collector / Jaeger / Grafana 并导入全部 Node 的 containerd，修复旧 `controller:latest` 导致的 `ImagePullBackOff`。
- 按目标集群实际 Worker Node 动态创建 WorkerNode、TenantNodePolicy 与 ModelNodePolicy，不再写死节点名。
- 统一 Controller / Simulator / Backend 的集群与环境标识为 `docker-desktop`。
- 新增 `make cluster-status` / `cluster-open` / `cluster-urls` / `cluster-down`；停止命令只缩容工作负载，不删除集群、CRD、CR 或 PVC。
- 新增覆盖旧项目后的显式清理脚本（`hack/cleanup-obsolete.sh`），处理旧 Frontend `DOCS/`、`docs/yaml/`、`config/kind/`、重复锁文件、缓存、覆盖率和旧 PDF。

## 测试与验证

- `setup.sh`、`hack/local-cluster.sh`、`hack/cleanup-obsolete.sh` 通过 `bash -n`。
- `config/dev`、`config/demo`、`dashboard/deploy` 均成功渲染。
- Frontend 通过 oxlint、TypeScript、Vite 生产构建和状态模型校验。
- 使用伪 Context 验证安全闸门：当前 Context 不是 `docker-desktop` 时，在任何集群写入前停止并生成诊断。
- 当前交付环境没有 Docker、kubectl、Go 和目标集群，实际镜像构建与运行验收需用户执行 `bash setup.sh`；脚本不把未验证状态报告为成功。

## 迁移与回滚

- 回滚方式：改用旧部署流程（另建 Kind + `make deploy`）前，先确认不需要保留当前 `docker-desktop` 上的数据卷与 PVC。
- 本条目由根目录 `CHANGELOG.md` 归档而来，2026-08-18 文档体系重构时删除原文件；后续变更统一归档在 `change-history/`。
