# Kind 底座迁移完成：部署闭环 + 数据恢复 + 工具链修复（#50）

> 日期：2026-08-18

## 为什么做

- 用户关闭 Docker Desktop 内置 Kubernetes 后，开发底座切换到 Kind 多节点集群 `hello-k8s-ai-dev`（1 control-plane + 4 worker），需要让 `make cluster-up` 在全新集群上完整跑通（部署 + 验收 + 数据恢复），并把 docker-desktop 时代的历史数据迁移过来。

## 改成什么

1. **preflight 首次部署放行**（`hack/preflight.sh`）：空 namespace 且 CRD 未安装时降级为 WARN（首次部署正常态）；CRD 已存在但工作负载缺失仍 FAIL。
2. **Jaeger 滚动更新死锁自动化**（`hack/local-cluster.sh`）：`restart_and_wait_deployment` 识别 `platform.study.com/restart-procedure: scale-to-zero` 注解，单副本 + RWO PVC 的 Deployment（Jaeger badger）自动走“缩 0 → 扩 1”，不再因新旧 Pod 抢目录锁而 rollout 卡死。
3. **SimulationClock 收敛检查适配干净环境**（`hack/local-cluster.sh`）：无 SimulationClock CR 时跳过收敛断言（docker-desktop 集群的 `default` 是历史残留，全新集群没有）。
4. **Dashboard 镜像拉取策略**（`dashboard/deploy/backend.yaml`、`frontend.yaml`）：`:dev` 镜像 `imagePullPolicy` 从 Always 改为 IfNotPresent。Kind 节点容器内无法访问宿主代理，Always 拉取必然失败（控制面镜像一直是 IfNotPresent，只有 Dashboard 两处遗漏）。
5. **恢复脚本修复**（`hack/kind/restore-data.sh`、`backup-data.sh`）：
   - 辅助 Pod 镜像 `busybox` 显式 `IfNotPresent`（无 tag 默认 Always，Kind 节点拉取失败）；并加入 `RUNTIME_IMAGES` 随 `make cluster-up` 导入节点。
   - 解包时序：容器不再边等边解包（`kubectl cp` 流式写入期间读到半成品 → exit 1），改为容器保持存活、cp 完成后 `kubectl exec` 同步解包。
   - 解包前清空目标目录：tar 覆盖式合并会与新建集群初始化数据混合，导致 Prometheus TSDB `segments are not sequential`、Jaeger badger 错乱。
6. **Kind 底座配套文件**：`hack/kind/local-path-provisioner.yaml` 改用 Kind 自带 provisioner/helper 镜像（`docker.io/kindest/...`，节点已有，无需拉取）；`cluster-up.sh` / `cluster-down.sh` 恢复执行位。

## 关键行为

- 全新集群首次 `make cluster-up`：preflight 0 FAIL（空 namespace 为 WARN）→ 构建导入镜像 → 控制面/Dashboard/PostgreSQL 全部 rollout → 验收（无 SimulationClock 时跳过收敛检查）。
- Jaeger 升级/重启始终安全：脚本按注解自动缩 0 再扩 1，PVC 数据不丢。
- 数据迁移链路：`backup-data.sh`（旧底座）→ `make cluster-up`（新底座）→ `restore-data.sh`（三套数据恢复），恢复后 `make preflight` 全绿。

## 验证

- `make cluster-up` 完整执行：控制面 5 组件 + Dashboard Backend/Frontend + PostgreSQL 全部 1/1；数据迁移后 Prometheus（644 指标）/ Jaeger（3 服务）/ PostgreSQL（resource_events 52 万 + snapshots 3670）/ Replay（500 条历史事件）数据完整。
- `make preflight`：16 通过 / 0 失败 / 3 警告（端口未监听与内存水位为预期 WARN）。
- `make docs-check`：通过。
- 未验证：`make cluster-up` 在干净 Kind 集群上第二次幂等重跑（本次为修复后单次完整执行）；E2E 独立 Kind 集群不受影响。

## 回滚

- 脚本修复：`git revert` 对应 commit 即可；`hack/kind/restore-data.sh` 旧版存在半成品解包与数据混合风险，不建议回退。
- 数据：备份在 `/var/tmp/hello-k8s-ai-backup-20260818-120414/`（不入库），可重新执行 `restore-data.sh`（幂等）。
- 底座：`make kind-down` 删除 Kind 集群（PVC 数据保留）。
