# Docker bind 失效 → tmpfs 覆盖 → PVC "丢数据"：恢复 + 根因修正

> 日期：2026-08-18 ｜ 关联：docs/journal/2026-08-18-docker-bind-pvc-loss.md、docs/lessons/kind-hostpath-docker-desktop-rootfs.md

## 为什么做

- WSL 研究触发 `wsl --shutdown` → Docker Desktop 重启 → dev 集群 postgres / jaeger / prometheus 全部 CrashLoopBackOff（Permission denied）。
- 根因：`kind-5node.yaml` extraMounts 把宿主 `/var/lib/hello-k8s-ai-pv` bind 进节点容器，但该 hostPath 在 **Docker Desktop VM 根文件系统**（WSL 重启即重置）；bind 失效后 Docker fallback 成 tmpfs 覆盖挂载点，PVC 目录变空 root:root 755，非 root 容器写不进去。
- 用户要求：立刻修复 + 全部沉淀，避免以后 AI 再踩。

## 改成什么

1. **磁盘层修复**：5 个节点容器 `umount /var/lib/hello-k8s-ai-pv`（移除 tmpfs，露出持久 `/var` named volume 底层）→ 数据面不再依赖临时根文件系统。
2. **所有权修复**：postgres 目录 `70:70`（postgres:17-alpine）、prometheus 目录 `65534:65534`（nobody）、jaeger 目录 `777`（fsGroup=0 非 root）→ 3 个 Pod 从 CrashLoopBackOff 恢复 Ready。
3. **数据恢复**：`hack/kind/restore-data.sh` 后台执行（备份 `/var/tmp/hello-k8s-ai-backup-20260818-120414/`：dashboard.sql 2.5G + prometheus.tar.gz 264M + jaeger.tar.gz 395M）。
4. **沉淀**：journal + lesson + 本四件套；`kind-5node.yaml` 的持久化假设错误已记录，长期修复（删 extraMounts / cluster-up 检测）挂 issue 候选。

## 关键行为

- 修复后 `/var/lib/hello-k8s-ai-pv` 位于节点容器持久 `/var` volume（/dev/sde，vhdx），Docker Desktop 重启后数据不再丢。
- **注意**：节点容器重启（docker restart / 集群重建）后 bind 配置还在 → tmpfs 会回来，需要重新 umount（已记入 lesson，长期修复待做）。
- 恢复脚本幂等；SQL 恢复耗时长（2.5G），需后台 + 日志轮询。

## 验证

- 8/8 系统 Pod Ready（controller / backend / frontend / postgres / grafana / jaeger / otel / prometheus）。
- 3 个数据面 Pod 0 重启 Running；postgres 监听 5432 正常。
- 数据恢复进行中（restore-data.sh 日志 `/var/tmp/restore-data.log`）；完成后核对 resource_events 行数 ≈52 万、Grafana/Jaeger 有数据。

## 回滚

- 恢复动作无代码改动（纯运维操作），"回滚"= 重跑故障：节点容器重启后 tmpfs 复现。真正回滚是重建集群前恢复备份。
- 文档回滚：git revert 本条目相关提交。
