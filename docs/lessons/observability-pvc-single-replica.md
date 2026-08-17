# 单副本 + RWO PVC 的状态型组件：升级必须先 scale 0 再扩 1

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-cluster-and-deploy.md 与 2026-08-17-observability-and-storage.md ｜ 适用对象：本地 Agent

## 现象

Prometheus / Jaeger（badger/TSDB 单副本 + RWO PVC）滚动更新 CrashLoop：TSDB/badger 目录锁冲突。

## 根因

同一 PVC 被新旧两个 Pod 同时挂载；状态型单副本组件滚动更新会锁冲突。

## 可复用规则

- 这类组件更新用 Recreate 策略，或先 scale 0 等 PVC 释放再扩 1；不用 rollout restart。
- 部署清单与运维手册中明确标注该约束。

## 验证方法

更新后组件 Ready 且数据完整（历史查询无 gap）；`kubectl get pvc` 状态 Bound。
