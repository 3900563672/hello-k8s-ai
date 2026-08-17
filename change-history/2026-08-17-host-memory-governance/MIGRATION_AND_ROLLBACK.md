# 迁移与回滚：宿主内存治理与稳定性校准

## 数据迁移

- 无数据迁移。集群数据（CRD/CR/PVC/PostgreSQL）全程保留；`wsl --shutdown` 只回收内存，不删发行版文件系统。
- 模拟器负载清理是"缩容/停止"不是删除：CR 保留（`spec.replicas=0`），Deployment 保留（replicas=0），PVC 数据不变。

## 回滚

| 变更 | 回滚方式 |
| --- | --- |
| `.wslconfig`（memory=12GB + autoMemoryReclaim） | 删除或改回，然后 `wsl --shutdown` 生效 |
| Docker AI 关闭 | `settings-store.json` 恢复 `EnableDockerAI=true`/`InferenceCanUseGPUVariant=true`（备份 `settings-store.json.memguard.bak`）后重启 Docker Desktop |
| Jaeger 资源（1Gi/GOMEMLIMIT） | 从 git 还原 `config/observability/jaeger.yaml` 后 `kubectl apply` + scale 0→1 |
| CR `spec.replicas=0` | 恢复 `kubectl patch simulatorinstance tenant-core-model-lite --type=merge -p '{"spec":{"replicas":200}}'`（注意：当前 replicas=0 是唯一能保持负载停止的状态，恢复前确认内存余量） |

## 恢复步骤（后续开发）

1. 确认 `vmmemWSL ≤ 12GB`、Windows 空闲内存 ≥ 5GB。
2. 模拟器负载按需恢复：patch CR `spec.replicas` 到目标值（不要用 200 起步，先小后大）。
3. 节点数缩减完成前，负载预算 ≈ 30-50 Pod（见 RESILIENCE 3.5）。
