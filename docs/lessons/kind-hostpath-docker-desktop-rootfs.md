# Kind hostPath 放 Docker Desktop 根文件系统：WSL 重启后 bind 失效 → tmpfs 覆盖 → PVC "丢数据"

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-18-docker-bind-pvc-loss.md ｜ 适用对象：本地 Agent / 远程 AI
> 状态：已根治（2026-08-19 移除 extraMounts，见 change-history/2026-08-19-kind-extra-mounts-removed/）；下方恢复路径仅用于旧集群历史故障。
> 触发条件（Use when）：kind 集群 PVC 异常 / CrashLoopBackOff + Permission denied / 节点挂载变 tmpfs / 数据恢复时

## 现象

- `wsl --shutdown` / Docker Desktop 重启后，dev 集群 postgres / jaeger / prometheus 全部 `CrashLoopBackOff`，日志 `Permission denied`（`mkdir ... Permission denied`）。
- 节点容器内 `/var/lib/hello-k8s-ai-pv` 挂载显示为 `tmpfs`（5.9G），PVC 目录空 `root:root 755`。

## 根因（两层）

1. **持久化假设错误**：`hack/kind/kind-5node.yaml` 的 extraMounts 把**宿主** `/var/lib/hello-k8s-ai-pv` bind 进节点容器。在 Docker Desktop 场景，该 hostPath 位于 **Docker Desktop VM 根文件系统**（重置即清空），**不是** vhdx 持久盘。注释"docker_data.vhdx 内"是错误假设。
2. **bind 失效的 fallback**：bind 源重置为空后，Docker 在容器内落成 **tmpfs** 覆盖挂载点；local-path provisioner / kubelet 在 tmpfs 上重建空目录（root:root 755）→ 非 root 容器（postgres uid 70 / prometheus nobody 65534 / jaeger 非 root）写不进去。

## 恢复路径（已验证，幂等）

1. **确认持久层**：节点容器 `/var` 是 named volume（`docker inspect <node> --format '{{range .Mounts}}{{if eq .Destination "/var"}}{{.Name}}{{end}}{{end}}'`，`/dev/sde` 1007G，vhdx 上）。每节点独立 volume。
2. **umount tmpfs**（5 节点逐个）：

   ```bash
   docker exec <node> umount /var/lib/hello-k8s-ai-pv
   ```

   umount 后该路径露出 `/var` volume 底层目录（空但可写、持久）。
3. **修目录所有权**（按容器 uid，local-path 目录名 `pvc-<uuid>_<ns>_<pvc>`）：
   - postgres（postgres:17-alpine，fsGroup=70）：`chown -R 70:70 <dir> && chmod 700`
   - prometheus（nobody，fsGroup=65534）：`chown -R 65534:65534 <dir> && chmod 700`
   - jaeger（fsGroup=0，非 root）：`mkdir -p <dir> && chmod 777`
4. **重建 Pod**：删除 CrashLoopBackOff 的 Pod（kubelet 以 DirectoryOrCreate 重新挂载，目录已存在则保留所有权）。
5. **恢复数据**：`bash hack/kind/restore-data.sh`（SQL psql 导入 + tar 经 busybox helper pod 解包；2.5G SQL 需后台跑）。

## 可复用规则

- **Docker Desktop 下，kind extraMounts 的 hostPath 一律视为临时**；要持久数据，让路径自然落在节点容器 `/var` named volume 上（已实施：2026-08-19 起 kind-5node.yaml 不再带 extraMounts）或挂真 volume。
- **bind 失效 ≠ 数据丢失**：先看节点容器挂载表（tmpfs？），umount 露出底层；备份目录 `/var/tmp/hello-k8s-ai-backup-*/` 是兜底。
- **恢复前先备份**：数据还在容器视图里时先导出（pg_dump + tar），再动挂载。
- **节点容器重启后 tmpfs 会回来**（旧集群）：需要重新 umount；新集群已无 extraMounts，不再出现该问题。
- Docker Desktop 恢复路径：杀光 Docker 进程 + `wsl --shutdown` + 重启 Docker Desktop（IPC 超时/E_UNEXPECTED 时有效）；不要反复只点重启按钮。
