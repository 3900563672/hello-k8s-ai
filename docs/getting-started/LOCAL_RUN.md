# 本地运行

## 1. 完整系统

覆盖项目文件后只执行：

```bash
bash setup.sh
```

`setup.sh` 先运行安全的旧文件清理，再调用 `make cluster-up`。部署全程使用当前 `docker-desktop` Context，不创建新集群。

完整流程包括：

1. 检查 Docker、kubectl、Context、API Server、Node、架构和 `standard` StorageClass。
2. 预拉取五个第三方运行镜像并构建四个项目镜像。
3. 将九个运行镜像导入全部 Kubernetes Node 的 containerd。
4. 应用 `config/dev` 与 `dashboard/deploy`。
5. 自动生成随机 PostgreSQL 密码；重部署时保留原 Secret。
6. 按真实 Worker Node 动态创建 WorkerNode 和 Allow Policy。
7. 写入 Model 演示分数并等待 Controller 创建 Simulator Deployment。
8. 验证 Simulator Status、Prometheus 指标、Jaeger Trace、PostgreSQL snapshot、Backend 和 Frontend。
9. 后台启动四个端口转发。

任何一步失败都会停止；主要日志在 `.runtime/up-*.log`，聚合诊断在 `.runtime/last-failure.log`。

## 2. 访问与状态

```bash
make cluster-status
make cluster-urls
```

| 组件 | 地址 |
| --- | --- |
| Dashboard | `http://localhost:8080` |
| Grafana | `http://localhost:3000` |
| Prometheus | `http://localhost:9090` |
| Jaeger | `http://localhost:16686` |

如果关闭终端或 Docker Desktop 后端口转发中断：

```bash
make cluster-open
```

## 3. 重复执行

`make cluster-up` 可以重复执行：

- CRD、清单和演示 CR 使用声明式 apply。
- 项目镜像重新构建后会再次导入所有 Node，并主动 restart 相关 Deployment。
- 已存在的 PostgreSQL Secret 和 PVC 会保留，避免密码变化导致旧数据不可访问。
- 旧 `orchestratorconfigs.platform.study.com` CRD 只提示，不自动删除。

## 4. 停止但保留数据

```bash
make cluster-down
```

该命令停止端口转发并把项目工作负载缩到 0。它不会删除：

- `docker-desktop` 集群或任何 Node；
- 项目 CRD 与 CR；
- PostgreSQL Secret 与 PVC；
- 旁边的 `minikserve-demo`。

再次执行 `make cluster-up` 即可恢复。

## 5. 单独调试 Controller

只有源码开发才需要本机 Go：

```bash
make install
export SIMULATOR_NAMESPACE=hello-k8s-ai-system
export SIMULATOR_IMAGE=hello-k8s-ai-simulator:dev
make run
```

本地进程与集群内 Manager 不应同时处理同一对象。调试前先把集群内 Manager 缩到 0，结束后恢复一键部署。

## 6. 单独调试 Frontend

```bash
cd dashboard/frontend/my-app
npm ci
npm run dev
```

Vite 默认把 `/api` 代理到 `localhost:8080`，因此需先保证 Backend 的端口转发已启动。
