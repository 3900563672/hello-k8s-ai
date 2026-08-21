# kind 节点 /var/lib/hello-k8s-ai-pv tmpfs 覆盖恢复 SOP（2026-08-20 实测）

## 现象

- Docker Desktop 或节点容器重启后，kubectl 可用但工作负载 CrashLoopBackOff。
- postgres 报 `mkdir: can't create directory '/var/lib/postgresql/data/pgdata': Permission denied`（目录被换成空目录且属主 root）。
- 节点内 `mount | grep hello-k8s-ai-pv` 显示 `none on /var/lib/hello-k8s-ai-pv type tmpfs`。
- **根因**：旧配置创建的集群带 extraMounts（bind /var/lib/hello-k8s-ai-pv → Docker Desktop VM 根文件系统）。
  Docker Desktop 重启后 VM rootfs 重建，bind 失效并 fallback 成 tmpfs，**遮住** named volume 里的真数据。
  当前 kind-5node.yaml 已无 extraMounts，但旧集群的容器挂载保留，需要手工恢复。

## 恢复步骤（5 节点）

1. 确认故障形态：`docker exec <node> mount | grep hello-k8s-ai-pv` → tmpfs = 被覆盖。
2. 执行 umount（露出 named volume 真数据）：
   `docker exec hello-k8s-ai-dev-control-plane umount /var/lib/hello-k8s-ai-pv`（5 个节点都做）。
3. 验证真数据露出：`docker exec <node> ls -la /var/lib/hello-k8s-ai-pv/` 应看到各 PVC 目录及数据时间戳。
4. 删除故障 pod 让 kubelet 重新挂载：postgres(StatefulSet) / prometheus / jaeger 的 pod 各删一次。
5. 等 pod 1/1 Running（约 30s），验证数据：`psql ... select count(*) from resource_snapshots`。

## 数据位置速查（当前集群）

| PVC | 节点 | 数据路径（named volume） |
| --- | --- | --- |
| postgres | hello-k8s-ai-dev-worker3 | pvc-e6f6ddd0-...-postgresql-0（uid 70） |
| prometheus | hello-k8s-ai-dev-worker | pvc-273047fb-...-prometheus-data（nobody） |
| jaeger | hello-k8s-ai-dev-worker4 | pvc-3f0a65f2-...-jaeger-data |

## 根治

- 重建集群（`make kind-down && make cluster-up`）用无 extraMounts 的新配置，PVC 数据按 hack/kind/backup-data.sh + restore-data.sh 显式恢复。
- 在此之前，任何 Docker Desktop / WSL 重启后都按本 SOP 恢复。

## 教训

- "数据目录变空" ≠ 数据丢失：先查 mount，tmpfs/bind 覆盖时 umount 即恢复。
- 旧配置创建的容器挂载不会随 yaml 更新而消失，必须显式 umount 或重建集群。
- 本 SOP 此前缺失（文档只记了现象），现已补齐。

## 2026-08-21 追加：bind 失效机制实测 + 根治完成

### 机制补全（08-21 实测）

- 旧集群节点容器内 `/var/lib/hello-k8s-ai-pv` 的挂载定义是 **bind**，Source 指向 Docker Desktop 的
  WSL bind-mounts（`/run/desktop/mnt/host/wsl/docker-desktop-bind-mounts/Ubuntu/<hash>`，即 WSL Ubuntu
  发行版的同名路径）。`docker inspect <node> .Mounts` 可见。
- Docker Desktop / WSL 重启后：VM 内整个 `/run/desktop/mnt/host/wsl/docker-desktop-bind-mounts/` 注册
  丢失（实测目录不存在）→ bind 失效 → Docker Desktop 静默用 5.9G tmpfs 覆盖挂载点。
- 数据实际双份：WSL Ubuntu `/var/lib/hello-k8s-ai-pv/`（bind 有效期写入，旧）+ 节点容器 `/var`
  named volume（SOP 恢复后写入，**更新，以此为准**）。08-21 恢复验证 PG 60.4 万条事件全在。

### 错误路径（08-21 实测踩坑）

- 症状出现时先 `chown` 空目录 + 删 pod 重新初始化 = 在 tmpfs 遮罩上写新数据（假数据），
  下一次重启还会复发。**必须先查 mount**：`docker exec <node> mount | grep hello-k8s-ai-pv`，
  是 tmpfs 就 umount（见上恢复步骤），不要 chown、不要重建初始化。

### 根治完成（08-21）

1. `hack/kind/backup-data.sh`（KUBE_CONTEXT=kind-hello-k8s-ai-dev）→ /var/tmp/hello-k8s-ai-backup-rootfix/
   （dashboard.sql 2.77GB / prometheus.tar.gz 161MB / jaeger.tar.gz 452MB）。
2. `make kind-down`（旧 /var named volume 保留）。
3. `make cluster-up`（kind-5node.yaml 无 extraMounts 新配置 + 全套部署）。
4. `hack/kind/restore-data.sh`（BACKUP_DIR 指向备份目录）。
5. 验证：PG 行数 / Prometheus count(up) / Jaeger / Grafana。

### 数据位置（根治后新集群）

| PVC | 数据位置 |
| --- | --- |
| postgres / prometheus / jaeger | 节点容器 `/var` named volume（无 bind、无 tmpfs 覆盖） |

### 遗留

- WSL Ubuntu `/var/lib/hello-k8s-ai-pv/`（旧 bind 源）仍有 08-18~08-20 早期数据，保留作冷备；
  新集群不再挂载它，确认稳定后可清理（约 3G）。
