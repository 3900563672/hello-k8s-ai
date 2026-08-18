# MIGRATION_AND_ROLLBACK：Docker bind 失效恢复

## 迁移

- 无代码/配置迁移；纯运维恢复动作（umount + chown + 删 Pod + 数据导入）。
- 数据恢复依赖备份 `/var/tmp/hello-k8s-ai-backup-20260818-120414/`（恢复前勿删；建议恢复验证通过后再归档）。

## 回滚

- 恢复动作本身无回滚需求；若恢复失败，可重跑 `hack/kind/restore-data.sh`（幂等：psql 重复导入用 ON_ERROR_STOP=0；tar 解包前清空目标目录）。
- 节点容器重启后 tmpfs 复现时的恢复 SOP（已沉淀到 lesson）：
  1. 5 节点 umount tmpfs
  2. 修 3 个目录所有权（70:70 / 65534:65534 / 777）
  3. 删故障 Pod
  4. 如需恢复数据：跑 restore-data.sh

## 风险与待办

- **高**：节点容器重启（docker restart / Docker Desktop 重启）后 tmpfs 复现，数据面再次故障——长期修复：`kind-5node.yaml` 删除 extraMounts（让路径落 /var 持久 volume），需重建集群；或 cluster-up 增加 bind 生效检测。
- **中**：Docker Desktop bind 机制整体失效原因未彻底定位（可能与 WSL 网络栈残留有关），建议整机重启后复测。
- 以上两项挂 issue 候选，不在本条目内实施。
