# 2026-08-18 Docker bind 失效 → tmpfs 覆盖 → PVC "丢数据"：完整恢复链

> 日期：2026-08-18 ｜ 触发者：WSL 研究（`wsl --shutdown`）→ Docker Desktop 重启 → 集群数据面故障 ｜ 相关：docs/lessons/kind-hostpath-docker-desktop-rootfs.md、change-history/2026-08-18-docker-bind-failure-recovery/

## 事故时间线

1. **触发**：为验证 WSL 回环问题执行 `wsl --shutdown`；用户手动重启 Docker Desktop → 引擎恢复 29.7.2。
2. **API 端口转发坏**：Windows curl `https://127.0.0.1:42839` TLS 失败 → `docker restart hello-k8s-ai-dev-control-plane` → 恢复（HTTP 200，5 节点 Ready）。
3. **三个数据面 Pod CrashLoopBackOff**：postgres / jaeger / prometheus 全部 `Permission denied`。
   - postgres：`mkdir /var/lib/postgresql/data/pgdata: Permission denied`
   - prometheus：`open /prometheus/queries.active: permission denied`
   - jaeger：`mkdir /tmp/jaeger/: permission denied`
4. **根因链**（逐层定位）：
   - `kind-5node.yaml` 用 extraMounts 把**宿主** `/var/lib/hello-k8s-ai-pv` bind 进节点容器（注释声称"docker_data.vhdx 内"——**错误假设**）。
   - 该 hostPath 实际落在 **Docker Desktop VM 根文件系统**（非持久 vhdx），WSL 重启后被重置为空。
   - bind 源失效后 Docker 在容器内 fallback 成 **tmpfs（5.9G）** 覆盖挂载点 → 三个 PVC 目录变空 `root:root 755`。
   - local-path provisioner / kubelet 在 tmpfs 上重建了空目录 → Pod 以非 root 启动 → Permission denied。
5. **最小实验证实**：`docker run -v /var/lib/test-bind` 读不到文件；vhdx 内路径同样失败 → **bind 机制整体失效**（Docker Desktop 重启后）。
6. **Docker Desktop 多次重启失败**（IPC ping 超时 / E_UNEXPECTED）→ 最终修复路径：**杀光所有 Docker 进程 + `wsl --shutdown` + 重启 Docker Desktop** → 引擎就绪 29.7.2，bind 依然失效。
7. **关键突破口**：kind 节点容器内 `/var` 是**持久 docker volume**（`/dev/sde`，vhdx 上）；`docker exec ... umount /var/lib/hello-k8s-ai-pv` 移除 tmpfs 后，露出 volume 底层目录（空但可写）→ **数据落回持久层**。
8. **数据备份**（完整，勿动）：`/var/tmp/hello-k8s-ai-backup-20260818-120414/`：`dashboard.sql` 2.5G（resource_events 52 万 + snapshots 3670）+ `prometheus.tar.gz` 264M + `jaeger.tar.gz` 395M。
9. **恢复**：5 节点 umount tmpfs → 修目录所有权（postgres 70:70、prometheus 65534:65534、jaeger 777）→ 删 4 个故障 Pod（含 Unknown backend）→ 全部 Ready → `hack/kind/restore-data.sh` 后台恢复数据。

## 关键事实（后续勿再踩）

- **kind extraMounts 的 hostPath 在 Docker Desktop 场景指向 VM 根文件系统**，Docker Desktop 重启/WSL 重启后内容重置；`/var`（named volume）才持久（/dev/sde，1007G）。
- 节点容器重启后 bind 配置还在 → tmpfs 会回来 → **需要重新 umount**（本条目修复后，除非重建集群，否则每次 Docker Desktop 重启都要重做 5 节点 umount）。
- **长期修复**：从 `kind-5node.yaml` 删除 extraMounts（让 `/var/lib/hello-k8s-ai-pv` 自然落在 `/var` 持久 volume 上）；或 cluster-up 增加"bind 是否生效"检测（探测文件 + 告警）。重建集群前先导出备份。
- 目录所有权是恢复的关键：local-path 目录被 kubelet 以 root:root 重建后，非 root 容器必然 Permission denied；修复 = 按容器 uid chown（postgres:17-alpine=70，prometheus=nobody 65534，jaeger=fsGroup 0 → 777）。

## 处理

- 恢复服务（完成）：磁盘层 umount → 所有权 → Pod 重建 → 8/8 Ready。
- 数据恢复（进行中）：`restore-data.sh` 后台执行（SQL 2.5G + 2 个 tar），日志 `/var/tmp/restore-data.log`。
- 沉淀：本条目 + `docs/lessons/kind-hostpath-docker-desktop-rootfs.md` + `change-history/2026-08-18-docker-bind-failure-recovery/`。
- 待办（issue 候选）：kind-5node.yaml 长期修复；cluster-up 故障检测；Docker Desktop 重启后自动 umount 脚本。
