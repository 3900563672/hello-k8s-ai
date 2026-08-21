# 本地运行

> 维护层：human | last-reviewed：2026-08-21 | 事实源：config/samples/、config/demo/ 等

## 1. 完整系统

覆盖项目文件后只执行：

```bash
bash setup.sh
```

`setup.sh` 先运行安全的旧文件清理，再调用 `make cluster-up`。部署使用固定 Kind 开发集群 `hello-k8s-ai-dev`（`make cluster-up` 幂等创建/复用，默认 context `kind-hello-k8s-ai-dev`），不再使用 Docker Desktop 内置 Kubernetes。

完整流程包括：

1. 检查 Docker、kubectl、Context、API Server、Node、架构和 `standard` StorageClass。
2. 预拉取五个第三方运行镜像并构建四个项目镜像。
3. 将九个运行镜像导入全部 Kubernetes Node 的 containerd。
4. 应用 `config/dev` 与 `dashboard/deploy`。
5. 自动生成随机 PostgreSQL 密码；重部署时保留原 Secret。
6. 默认不写入演示数据，保持干净环境；只有设置 `DEMO_ENABLED=true` 时才按真实 Worker Node 创建 WorkerNode 和 Allow Policy、写入演示 Model 分数并等待 Controller 创建 Simulator Deployment。
7. 验证 Backend API、SimulationClock 与 Frontend 页面；干净模式断言业务 CR 与历史快照为空，演示模式额外验证 Simulator Status、Prometheus 指标、Jaeger Trace 与 PostgreSQL snapshot。
8. 后台启动 Dashboard 端口转发（单入口）。

任何一步失败都会停止；主要日志在 `.runtime/up-*.log`，聚合诊断在 `.runtime/last-failure.log`。

一键启动得到的是干净环境：没有预置的租户、模型、节点与策略，也没有历史切面。用户可在 Dashboard 中用预置模板（模型/租户/节点/编排策略/流量）快速开始，或在「填写指南」（`/guide`）页查看字段含义、默认值与系统常量。

## 2. 访问与状态

```bash
make cluster-status
make cluster-urls
```

| 组件 | 地址 |
| --- | --- |
| Dashboard | `http://localhost:8080` |
| Grafana | `http://localhost:8080/grafana` |
| Prometheus | Dashboard「数据回显」页（Backend 代理 `/api/v1/metrics/query`） |
| Jaeger | Dashboard「数据回显」页（Backend 代理 `/api/v1/traces`） |

如果关闭终端或 Docker Desktop 后端口转发中断：

```bash
make cluster-open
```

WSL / Docker Desktop 重启后（apiserver 不可达、PV tmpfs 遮罩、转发中断、本地后端与前端未起），一条命令自愈并拉起完整联调环境：

```bash
make env-up
```

`env-up` 幂等可反复执行：自愈（apiserver 重启、PV umount）+ port-forward 重建 + 本地后端（密钥从集群 Secret 注入，不入库）+ 前端 vite（代理到本地后端）。

## 3. 重复执行

`make cluster-up` 可以重复执行：

- CRD 与清单使用声明式 apply；默认不写入演示 CR。设置 `DEMO_ENABLED=true` 时会 apply `config/demo` 并强制触发演示编排。
- 项目镜像重新构建后会再次导入所有 Node，并主动 restart 相关 Deployment。
- 已存在的 PostgreSQL Secret 和 PVC 会保留，避免密码变化导致旧数据不可访问。
- 旧 `orchestratorconfigs.platform.study.com` CRD 只提示，不自动删除。

## 4. 停止但保留数据

```bash
make cluster-down
```

该命令停止端口转发并把项目工作负载缩到 0。它不会删除：

- Kind 开发集群 `hello-k8s-ai-dev` 或任何 Node；
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
