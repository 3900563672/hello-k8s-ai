# 宿主内存治理与稳定性校准（WSL2 内存爆满根因修复）

- 变更日期：2026-08-17（Asia/Shanghai 22:00~23:30；UTC 14:00~15:30）
- 关联问题：[#29](https://github.com/3900563672/hello-k8s-ai/issues/29)（design: 稳定性工程——资源校准与告警自愈、运行前体检、工具链自检）
- 变更级别：P0 宿主环境稳定性
- 变更范围：`config/observability/jaeger.yaml`、`docs/agents/KNOWN_PITFALLS.md`、`docs/agents/RESILIENCE.md`、`docs/agents/WORKFLOW.md`、宿主 `.wslconfig`（不入库）、Docker Desktop `settings-store.json`（不入库，备份 `.memguard.bak`）
- CRD 变化：无 ｜ 数据库变化：无

## 1. 为什么做

2026-08-17 晚宿主物理内存被占爆（31.4GB 只剩 0.4GB），C 盘 pagefile 自动扩到 38GB（C 盘被吃 20-30GB），WSL 内 kubectl 持续超时（0x8007274c），开发无法继续。根因是四层叠加：WSL2 默认占 50% 内存且永不归还；Docker Desktop 内置 K8s 开了 10 个节点容器（VM 内 8-10GB）；长跑测试遗留 SimulatorInstance（200 副本）从未清理（5-8GB）；Jaeger limit 512Mi 小于 badger 默认缓存导致反复 OOM。

## 2. 完成结果

1. **宿主治理**：新建 `%USERPROFILE%\.wslconfig`（`memory=12GB` + `autoMemoryReclaim=gradual`，VM 上限 15.7→12GB 且空闲页自动归还）；关闭 Docker AI（`EnableDockerAI=false`/`InferenceCanUseGPUVariant=false`）；关闭豆包与 Edge 非必要进程。
2. **负载清零**：`make cluster-down` + 长跑 CR `spec.replicas` 归零 + 两个模拟器 Deployment 手动缩 0；Pod 数 208→5，仅剩系统组件。
3. **Jaeger 止血**：limit 512Mi→1Gi、request 128Mi→256Mi、新增 `GOMEMLIMIT=805306368`（768Mi，Go runtime 不接受 Mi 后缀，踩坑见 KNOWN_PITFALLS）；按 scale 0→1 流程重启，0 重启稳定 Ready。
4. **防复发沉淀**：KNOWN_PITFALLS 新增"宿主内存治理"主题 5 条坑；RESILIENCE 新增 3.5 节内存预算与治理（31.4GB 机器、10 节点开销实测、负载预算公式、长跑后强制清理）；WORKFLOW 新增 4.2.1 长时运行结束必须清理。
5. **待办（不阻塞）**：K8s 节点数 10→4~5（需先备份评估，改节点数可能重置内置 K8s）；`SimulatorInstance replicas=0` 应成为合法暂停态（当前会触发校验报错，见 KNOWN_PITFALLS）；pagefile 固定 16GB（需重启电脑）。

## 3. 影响文件

| 文件 | 变更 |
| --- | --- |
| `config/observability/jaeger.yaml` | 内存 request/limit 校准 + GOMEMLIMIT（字节数）+ 注释说明 badger 缓存开销 |
| `docs/agents/KNOWN_PITFALLS.md` | 新增"宿主内存治理"主题（根因链/GOMEMLIMIT/replicas=0/cluster-down 复活/.wslconfig 覆盖） |
| `docs/agents/RESILIENCE.md` | 新增 3.5 内存预算与治理（实测数据 + 公式 + 强制清理） |
| `docs/agents/WORKFLOW.md` | 新增 4.2.1 长时运行结束必须清理 |


## 5. 后续更正（同日第二次提交）

- **`replicas=0` 不是"暂停"态**：实测删除负载时把 CR 归零后，Orchestrator 看到流量（qps=35）会从 0 自动扩容，负载复活。`replicas=0` 是合法最小值（新实例骨架），不是停止方式。
- **停止实例的正确姿势**：删除 `TenantModelPolicy`（Deny 时自动删除 SimulatorInstance 并清理 Deployment）；只删 SimulatorInstance 会被策略立即重建。
- **附带修复**：`replicas=0` 与持久化放置计划失配时的一致性校验死锁已修复（清除历史计划注解 + 暂停实例不参与资源预留 + 新增测试 `TestSimulatorInstancePauseClearsPlacementPlan`），见提交 `simulatorinstance_placement.go` / `orchestrator_data.go`。
## 4. 验证

- 空闲内存 0.4GB → 9.9GB（wsl --shutdown 回收后）；vmmemWSL 峰值 15.7GB → 12GB 上限。
- Pod 208 → 5（`kubectl get pods -n hello-k8s-ai-system` 仅 controller/jaeger/otel/grafana/prometheus）。
- Jaeger 0 重启 Ready；otel-collector 对 Jaeger 的 connection refused 随 Jaeger Ready 消退。
- 详见 TEST_REPORT.md。
