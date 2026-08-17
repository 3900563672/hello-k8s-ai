# cluster-down 后 kubectl apply 会复活负载；replicas=0 不是停止态

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-17-host-memory-governance.md ｜ 适用对象：本地 Agent

## 现象

`make cluster-down` 后再次 `kubectl apply` 全量清单，controller 复活并按 CR spec 重建模拟器负载；长跑后想"停掉"只把 replicas 置 0，Orchestrator 仍按流量扩容。

## 根因

cluster-down 只缩 Deployment 不删 CR；CR 是声明式事实源，controller 恢复后必然收敛到 CR 描述的状态；`replicas=0` 对 Orchestrator 不是停止语义。

## 可复用规则

- 长时运行结束必须：① `make cluster-down`；② 删除长跑 `TenantModelPolicy`（自动删除 SimulatorInstance 与模拟器 Deployment）；③ 验证只剩系统组件；④ 确认内存回落。
- 停止模拟器的正确操作是删 `TenantModelPolicy`，不是把副本数改 0。

## 验证方法

删除 CR 后 `kubectl get pods -n hello-k8s-ai-system` 只剩系统组件（≤8 个），模拟器 Deployment 消失。
