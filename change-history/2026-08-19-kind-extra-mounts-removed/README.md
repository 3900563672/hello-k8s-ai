# Kind 集群移除 extraMounts：PVC 数据改存节点 /var 数据卷（Fixes #54）

> 日期：2026-08-19 ｜ 关联：docs/lessons/kind-hostpath-docker-desktop-rootfs.md、change-history/2026-08-18-docker-bind-failure-recovery/

## 为什么做

- #54：`hack/kind/kind-5node.yaml` 的 extraMounts 把宿主 `/var/lib/hello-k8s-ai-pv` bind 进节点容器，但该 hostPath 在 Docker Desktop 场景位于 VM 根文件系统（非持久），WSL/Docker 重启后 bind 失效 fallback tmpfs，PVC 数据面故障（2026-08-18 事故）。
- 本次把 issue 中"重建集群时根治"的方案落实为代码 + 文档改动。

## 改成什么

1. `hack/kind/kind-5node.yaml`：删除 5 个节点的 extraMounts 块；数据自然落在节点容器 `/var` named volume（Docker 数据盘 vhdx，重启持久），local-path provisioner 仍在 `/var/lib/hello-k8s-ai-pv` 创建 PVC 目录。
2. 修正"删除集群重建后自动挂回"的错误表述（README / AGENTS.md / Makefile / cluster-down.sh）：重建后不会自动挂回，需按 `hack/kind/backup-data.sh` + `restore-data.sh` 显式恢复。
3. `hack/doctor.sh` 的 tmpfs 残留检测保留，对旧集群仍有诊断价值。

## 关键行为

- 新集群在任何重启（WSL / Docker Desktop / 节点容器）后都不再产生 tmpfs 覆盖，无需手工 umount。
- 删除集群不丢数据（`/var` 数据卷仍在），但重建集群挂载的是新数据卷，必须按恢复流程显式恢复。

## 验证

- 本次为代码/文档根治，未重建集群（避免中断环境）；实际效果在下次 kind 重建时体现。
- `make docs-sync` / `make docs-check` / `make docs-sync-check` 通过。

## 回滚

- git revert 本提交即可恢复 extraMounts 配置（只影响下次重建的集群形态，不动当前集群）。
