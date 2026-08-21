# 开发环境

> 维护层：human | last-reviewed：2026-08-21 | 事实源：docs/MAP.yaml、源码、change-history/

## 1. 一键部署所需环境

| 工具/能力 | 要求 | 用途 |
| --- | --- | --- |
| Docker Desktop | Linux 容器与 Kubernetes 已启用 | 构建镜像、承载现有多节点集群 |
| Docker CLI | 能连接 Docker Desktop Engine | 拉取、构建、保存并导入镜像 |
| kubectl | 当前 Context 为 `kind-hello-k8s-ai-dev` | Kustomize、部署、等待与 API 代理验收 |
| Bash/Make | WSL 环境自带版本即可 | 执行一键脚本 |
| StorageClass | `standard` | PostgreSQL 10Gi PVC |

本地完整栈部署不要求安装 Go、Node.js、npm、kind 或独立 Kustomize。Controller、Simulator、Backend 和 Frontend 都在 Docker build 阶段编译。

部署前最小确认：

```bash
docker info >/dev/null
kubectl config current-context
kubectl get nodes
kubectl get storageclass standard
```

Context 必须是 `kind-hello-k8s-ai-dev`（`make cluster-up` 幂等创建/复用的固定 Kind 开发集群）。脚本会自动创建/切换 Context，但不会重置或删除集群。

## 2. 开发源码时的额外工具

| 工具 | 仓库约束 | 用途 |
| --- | --- | --- |
| Go | 1.26.x | 根 Controller/Simulator 与 Dashboard Backend 两个 module |
| Node.js | 24 | Frontend 本地开发 |
| npm | 使用 `package-lock.json` | 唯一前端包管理器 |
| Kustomize | Makefile 固定 v5.8.1 | 生成或单独渲染清单 |
| controller-gen | v0.21.0 | CRD/RBAC/DeepCopy 生成 |
| golangci-lint | v2.12.2 | Go 静态检查 |
| kind | 仅自动化 E2E | 独立测试集群 `hello-k8s-ai-test-e2e` |

根 `go.mod` 与 `dashboard/backend/go.mod` 是两个独立 Go module。

```bash
go mod download
(cd dashboard/backend && go mod download)
(cd dashboard/frontend/my-app && npm ci)
```

## 3. 生成文件规则

以下文件由工具生成，不手工修改：

- `config/crd/bases/*.yaml`
- `config/rbac/role.yaml`
- `**/zz_generated.*.go`

修改 `api/v1` 后执行：

```bash
make manifests generate
git diff -- api/v1 config/crd config/rbac
```

## 4. WSL 代理提示

WSL 输出“localhost 代理未镜像”不等于 Docker Desktop Engine 无法拉取镜像。部署脚本使用 Docker daemon 的网络配置并对运行镜像拉取重试三次；如果仍失败，会在构建或拉取阶段立即停止，不会继续创建失败 Pod。

不要把 `registry-mirror:1273` 写进项目清单。它是 Docker Desktop 内部实现细节，本部署通过向每个 Node 的 containerd 导入镜像来避免依赖该地址。

## 5. 本地运行变量

通常无需修改：

```bash
KUBE_CONTEXT=kind-hello-k8s-ai-dev
NAMESPACE=hello-k8s-ai-system
DEMO_MODEL_ABSOLUTE_SCORE=100
```

可在执行时覆盖演示分数：

```bash
make cluster-up DEMO_MODEL_ABSOLUTE_SCORE=200
```

镜像名也可通过 Make 变量覆盖，但本地清单默认使用仓库定义的四个 `:dev` tag。

## 6. 不属于本地部署的内容

- `minikserve-demo` 是旁边的独立 Kind 集群，脚本不会读取、加载镜像或删除它。
- `make cleanup-test-e2e` 只针对固定 E2E 集群名，不能作为日常环境停止命令。
- 当前清单仍是开发/演示拓扑，不具备生产认证、HA、备份和可观测存储持久化。
