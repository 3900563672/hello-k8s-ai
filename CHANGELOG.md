# 2026-08-13 部署交付说明

> 本文件只记录 2026-08-13 的部署整理，不代表当前全部能力。后续 CRD、Controller 和业务链路变更统一归档在 [`change-history/`](change-history/README.md)。

日期：2026-08-13

## 部署修改

- 删除日常部署中另建 `docker-desktop` Kind 集群的流程，固定复用当前 `docker-desktop` Kubernetes Context。
- 新增 `bash setup.sh`：清理明确废弃内容后，一条命令完成预检、镜像准备、部署、演示数据和链路验收。
- 构建 Controller、Simulator、Dashboard Backend、Dashboard Frontend 四个项目镜像。
- 预拉取 PostgreSQL、Prometheus、OpenTelemetry Collector、Jaeger、Grafana 运行镜像，并导入全部 Kubernetes Node 的 containerd，修复旧 `controller:latest` 导致的 `ImagePullBackOff`。
- 同步修正 `config/manager` 与 `make deploy` 的基础镜像替换入口，避免绕过一键脚本时重新使用旧镜像名。
- 按目标集群实际 Worker Node 动态创建 WorkerNode、TenantNodePolicy 和 ModelNodePolicy，不再写死节点名。
- PostgreSQL 改为部署时生成随机密码，使用 `standard` StorageClass 与 10Gi PVC；重复部署保留 Secret/PVC。
- 统一 Controller、Simulator、Backend 的集群与环境标识为 `docker-desktop`。
- 新增 `make cluster-status`、`cluster-open`、`cluster-urls`、`cluster-down`；停止命令只缩容工作负载，不删除集群、CRD、CR 或 PVC。
- 自动验收 CRD、Controller、Simulator Status、Prometheus Metrics、OpenTelemetry/Jaeger Trace、PostgreSQL snapshot、Backend API 与 Frontend 页面。
- 失败时停止后续步骤并保存 `.runtime/last-failure.log`。

## 文档与清理

- README 与全部部署、运行、验证、集群、排障、配置和白皮书说明改为现有 Docker Desktop 集群流程。
- 新增覆盖旧项目后的显式清理脚本，处理旧 Frontend `DOCS/`、`docs/yaml/`、`config/kind/`、重复锁文件、缓存、覆盖率和旧 PDF。
- 最终压缩包不包含 `.git`、`.idea`、`.audit`、`bin`、`tmp`、`output`、覆盖率产物或重复 PDF。
- 旁边的 `minikserve-demo` 以及未知旧 CRD `orchestratorconfigs.platform.study.com` 均不自动删除。

## 本轮验证

- `setup.sh`、`hack/local-cluster.sh`、`hack/cleanup-obsolete.sh` 通过 `bash -n`。
- Makefile 在没有本机 Go 的环境中可正常解析，新增 target 可见。
- `config/dev`、`config/demo`、`dashboard/deploy` 均成功渲染。
- Frontend 通过 oxlint、TypeScript、Vite 生产构建和状态模型校验。
- 使用伪 Context 验证安全闸门：当前 Context 不是 `docker-desktop` 时，在任何集群写入前停止并生成诊断。
- 当前交付环境没有 Docker、kubectl、Go 和目标集群，因此实际镜像构建与运行验收需由用户执行 `bash setup.sh` 完成；脚本不会把未验证状态报告为成功。

## 未改变的边界

未修改 CRD 设计、Controller 架构、Backend API、Frontend 页面结构或数据库技术方案。当前仍是本地开发/演示部署，不是生产就绪拓扑。
