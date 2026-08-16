# 升级、迁移与回滚

## 1. 升级前备份

本次新增 CRD 并扩展 SimulatorInstance Spec。部署前保存相关对象：

```bash
kubectl get crd simulationclocks.platform.study.com \
  simulatorinstances.platform.study.com -o yaml \
  > /tmp/time-scale-crds-before.yaml
kubectl get simulatorinstances.platform.study.com -o yaml \
  > /tmp/simulatorinstances-before-time-scale.yaml
kubectl get simulationclocks.platform.study.com -o yaml \
  > /tmp/simulationclocks-before-time-scale.yaml 2>/dev/null || true
```

## 2. 推荐升级顺序

项目的一键部署继续使用：

```bash
make cluster-up
```

自定义发布应保持：

1. 应用新 CRD 与 RBAC；
2. 部署新 Controller Manager，等待 `SimulationClock/default` 和 Instance 字段收敛；
3. 部署新 Simulator 镜像；
4. 部署 Backend 与 Frontend；
5. 运行 Clock、Instance、Metrics 和 API 验收。

新 `timeScale` 有 CRD 默认值 1；Clock Controller 也会修补所有现存 Instance。升级阶段旧 Simulator 会忽略未知字段并继续 1x，新 Simulator 能读取字段，因此滚动发布不会要求同时停机。

## 3. 升级验收

```bash
kubectl get simulationclock/default \
  -o custom-columns='NAME:.metadata.name,DESIRED:.spec.rate,APPLIED:.status.appliedRate,SYNCED:.status.synchronizedInstances,TOTAL:.status.totalInstances,READY:.status.conditions[?(@.type=="Ready")].status'

kubectl get simulatorinstances \
  -o custom-columns='NAME:.metadata.name,RATE:.spec.timeScale'
```

再通过 Backend 查看：

```bash
curl -sS http://localhost:8080/api/v1/clock
```

验收条件：

1. desired=applied；
2. synchronizedInstances=totalInstances；
3. observedGeneration 等于 metadata.generation，Ready=True；
4. 全部 Instance timeScale 等于 desired；
5. Simulator 指标 time_scale 等于 desired，step_seconds 等于真实 Tick × desired；
6. 调整倍速前后 Simulator Pod UID 不变。

## 4. 兼容行为

- Clock 不存在：Controller 自动创建 1x；Backend 也可通过专用 API 创建用户选择值。
- 旧 Instance 缺 timeScale：API default 和 Controller 都收敛到当前值。
- Spec 刚更新、旧 Ready 仍为 True：Backend 因 generation 不一致报告 `converged=false`。
- 部分实例 Patch 失败：Clock Ready=False，Controller 重试；成功实例不回滚。
- Simulator 遇到旧对象零值或异常超范围值：运行时防御性钳制到 1..20，但 CRD/Controller/Backend 正常路径会更早拒绝。

## 5. 安全回滚

先把系统恢复到 1x并等待收敛：

```bash
kubectl patch simulationclock/default --type=merge \
  --patch '{"spec":{"rate":1}}'
kubectl wait --for='jsonpath={.status.conditions[?(@.type=="Ready")].status}=True' \
  simulationclock/default --timeout=120s
```

确认所有 Instance 为 1 后，再按旧版本顺序回滚 Simulator、Controller、Backend 和 Frontend。数据库没有 schema 变化，无需数据库回滚。

旧 Controller 不认识 SimulationClock；如果保留新 CRD/对象，它只会成为未消费配置，不影响旧 Simulator。只有在确认不会再快速恢复新版本并已经保存备份后，才考虑单独删除该 CRD。不要先删除 SimulatorInstance CRD，也不要在倍速仍大于 1 时直接回滚，避免用户误以为旧进程仍在加速。

## 6. 回滚后的检查

- 旧 Simulator 继续每 5 秒以 1x 更新 observedAt；
- Deployment/Pod 数量和 UID 不因回滚前的 rate 修改额外变化；
- Traffic、Performance、Orchestrator 继续使用真实时间；
- Backend/Frontend 不再显示倍速入口时，不应遗留可点击但无效的控件；
- 保存升级前和回滚后对象/指标证据，直到确认控制环稳定。
