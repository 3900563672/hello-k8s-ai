# 扩容停在"节点容量上限"不是 maxReplicas 问题

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-cluster-and-deploy.md ｜ 适用对象：本地 Agent

## 现象

高 QPS 下副本数停在某个值不再增长，看起来像 maxReplicas 限制。

## 根因

Orchestrator 按 WorkerNode 可用容量调度；节点容量打满后停止扩容（10 worker 的本地集群约 100 副本量级）。这是容量边界，不是配置 bug。

## 可复用规则

- 扩容不涨先查 WorkerNode 容量与 `Orchestrator.status` 的动作/原因，再怀疑代码。
- `maxReplicas=0` 表示无限制（模拟器无网关，接受任意 QPS，扩到容量上限为止）；真实上限是节点容量。

## 验证方法

压测时观察 `Orchestrator.status.reason` 与 WorkerNode usage；容量打满时原因应明确指向容量而非 maxReplicas。
