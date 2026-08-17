# 测试报告：宿主内存治理与稳定性校准

> 验证环境：Windows 11 + WSL2（Ubuntu）+ Docker Desktop 29.7.2 内置 K8s v1.36.1，2026-08-17 UTC 14:00~15:30。

## 宿主内存

```text
治理前：Total 31.4GB | Free 0.4GB | Used 31GB | Commit 68.7/68.7GB（打满）| pagefile 分配 38GB
wsl --shutdown 后：Free 9.9GB（vmmemWSL 9.6GB→0.5GB）
配置生效后（memory=12GB）：vmmemWSL 峰值 11.7GB（≤12GB 上限）
```

## 集群负载

```text
治理前：hello-k8s-ai-system 208 Pod（模拟器 200 + 系统 8）
make cluster-down + CR replicas=0 + 手动缩 0 后：5 Pod（controller/jaeger/otel-collector/grafana/prometheus）
验证命令：kubectl get pods -n hello-k8s-ai-system --no-headers | wc -l  → 5
```

## Jaeger 修复验证

```bash
kubectl get deploy -n hello-k8s-ai-system hello-k8s-ai-jaeger -o json | jq '.spec.template.spec.containers[0].resources'
# {"limits":{"cpu":"500m","memory":"1Gi"},"requests":{"cpu":"50m","memory":"256Mi"}}
kubectl get pods -n hello-k8s-ai-system -l app.kubernetes.io/name=jaeger
# hello-k8s-ai-jaeger-7d8fc867f9-zwtvc  1/1  Running  0  13s（0 重启）
```

- 首次尝试 `GOMEMLIMIT=768Mi` 崩溃：`fatal error: malformed GOMEMLIMIT`（Go runtime 只接受字节数）；改为 `805306368` 后 Ready（坑已沉淀）。
- otel-collector 日志中 `dial tcp ...4317: connect: connection refused` 随 Jaeger Ready 消退。

## 未验证项

- 长时运行（≥4h）下新配置的稳定性：未跑，等节点数缩减与用户确认后进行。
- 节点数 10→4~5 的数据风险：未执行（待备份评估）。
- `SimulatorInstance replicas=0` 校验修复：未修（已记录待办）。
