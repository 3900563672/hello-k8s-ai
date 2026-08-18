# IMPLEMENTATION_DETAILS：Docker bind 失效恢复

## 改动前状态

- `hack/kind/kind-5node.yaml`：5 个节点全部 extraMounts `hostPath: /var/lib/hello-k8s-ai-pv -> containerPath: /var/lib/hello-k8s-ai-pv`，注释声称"docker_data.vhdx 内"（错误假设）。
- 节点容器挂载：`/var` 为 named volume（每节点独立，/dev/sde 1007G）；`/var/lib/hello-k8s-ai-pv` 为 tmpfs（bind 失效 fallback）。
- 3 个 PVC（postgres worker3 / prometheus worker / jaeger worker4）目录空 root:root 755。

## 实施步骤（2026-08-18 晚，实测）

1. **确认持久层**：`docker inspect <node> --format '{{range .Mounts}}{{if eq .Destination "/var"}}{{.Name}}{{end}}{{end}}'` → 5 个节点 5 个独立 named volume；`df -h /var` 显示 /dev/sde。
2. **umount tmpfs**（4 个剩余节点；worker3 已在实验中完成）：
   ```bash
   for n in hello-k8s-ai-dev-control-plane hello-k8s-ai-dev-worker hello-k8s-ai-dev-worker2 hello-k8s-ai-dev-worker4; do
     docker exec "$n" umount /var/lib/hello-k8s-ai-pv
   done
   ```
   验证：`docker exec <node> mount | grep hello-k8s-ai-pv` → NO_MOUNT。
3. **目录所有权**（local-path 目录名 `pvc-<uuid>_<ns>_<pvc>`）：
   - worker3 postgres：`chown -R 70:70 <dir> && chmod 700`（postgres:17-alpine uid 70，fsGroup=70）
   - worker prometheus：`chown -R 65534:65534 <dir> && chmod 700`（nobody，fsGroup=65534）
   - worker4 jaeger：`mkdir -p <dir> && chmod 777`（fsGroup=0，非 root；PVC 挂在 /tmp）
4. **重建 Pod**：删除 4 个故障 Pod（3 个 CrashLoopBackOff + 1 个 Unknown backend）→ kubelet 以 DirectoryOrCreate 重新挂载，目录已存在则保留所有权。
5. **数据恢复**：`nohup bash hack/kind/restore-data.sh > /var/tmp/restore-data.log 2>&1 &`。
   - 阶段 1：`kubectl exec statefulset/... psql < dashboard.sql`（2.5G，最耗时）
   - 阶段 2/3：scale deployment 0 → busybox helper pod + `kubectl cp` + tar 解包 → 回滚到 1（prometheus / jaeger）

## 关键决策

- **不重建集群**：数据在备份里且服务可恢复，重建 + 恢复耗时长且有额外风险。
- **umount 而非重新 bind**：bind 源已重置且 Docker Desktop bind 机制失效（两次最小实验证实），umount 让路径落到持久 /var volume。
- **先备份后动手**：数据还在容器视图/旧挂载时可导出时先导出（12:04 已备份完整）。
