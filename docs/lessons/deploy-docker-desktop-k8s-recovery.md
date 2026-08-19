# 强杀 Docker Desktop 后内置 K8s 不自动恢复，恢复顺序固定

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-command-and-terminal.md 与 2026-08-18-host-toolchain.md ｜ 适用对象：本地 Agent
> 触发条件（Use when）：Docker Desktop 或整机重启后恢复内置 K8s；kubectl 报 nodes is forbidden / i/o timeout / 8080 无监听时

## 现象

强杀 Docker Desktop 后内置 Kubernetes 节点容器不自动恢复（`docker ps -a` 只剩 3 个容器）；整机重启后 kubectl 一度 `nodes is forbidden`、controller-manager `dial tcp 10.96.0.1:443 i/o timeout`、8080 无监听。

## 根因

内置 K8s 节点容器需在 Settings 手动重新启用；节点容器逐个拉起约 40-60 秒（期间 API 可连但 RBAC/controller 未就绪）；port-forward 是前台进程，重启即丢。

## 可复用规则

- 不要强杀 Docker Desktop；确需重启时让用户从 GUI 操作。
- 整机重启恢复顺序：启动 Docker Desktop → 等 `docker version` server 非空 → 等 `kubectl get nodes` 全 Ready → `kubectl -n hello-k8s-ai-system rollout restart deploy/hello-k8s-ai-controller-manager` → `make cluster-open` → `/api/v1/health/ready` 验收。
- 等待期间 API 可连但 RBAC 未就绪属于正常过渡，不要反复删节点。

## 验证方法

恢复后 10 worker 全 Ready、controller/backend/frontend/可观测组件全 1/1、ready API 200、PostgreSQL 历史完整。
