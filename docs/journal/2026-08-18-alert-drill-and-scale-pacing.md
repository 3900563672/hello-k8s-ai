# 告警演练与扩容参数化踩坑：空向量规则、WSL 回环路由、放置计划不变量、ConfigMap 热载

> 日期：2026-08-18 ｜ 触发者：本地 Agent ｜ 相关：change-history/2026-08-18-issue31-issue32/

### 2026-08-18 告警演练与扩容参数化踩坑
- 现象 1（LeaderMissing 永远不触发）：全部模拟器 Pod 删除后，`sum by (simulator_instance) (hello_k8s_ai_simulator_leader) == 0` 的查询返回**空向量**而不是 0——Prometheus 对 Pod 级抓取目标在 SD 移除后立即标记 stale，表达式为空 → `== 0` 不匹配 → 告警无法触发。8 分钟实测确认。
- 解决 1：改为 `(absent(sum by (simulator_instance)(leader)) == 1 and sum(container_memory_working_set_bytes{namespace="hello-k8s-ai-system",pod=~"simulator-.*"}) > 0) or (sum by (simulator_instance)(leader) == 0)`。容器指标是 cAdvisor 序列，Pod 死后残留 5 分钟，正好覆盖 `for: 1m` 且 clean 集群不误报。修复后实测 pending→firing→resolved 完整闭环。
- 现象 2（grafana 代理测试本地必失败）：`go test ./internal/api/` 的 httptest 回环连接报 connection refused，CI 同版本通过。根因：本机 WSL 路由表 127 有 `127.0.0.1 via 169.254.73.152 dev loopback0`（Docker Desktop/WSL localhost 转发层，伴随 "localhost 代理未镜像到 WSL" 警告），同进程新建回环监听会被转发层拒绝；kubectl port-forward 等既有监听正常。
- 处理 2：不改系统路由（可能破坏 Docker Desktop 网络）；CI 验证为准，journal 留痕。复现脚本：`/tmp/proxytest/main.go`（httptest + http.Get + raw dial 对比）。
- 现象 3（placement plan 不变量）：演练恢复时手工把 `spec.replicas` 拉回 80，但 orchestrator 上次写入的 `platform.study.com/node-placements` 注解只有 11 → 控制器报 "has 80 replicas but its node placement plan contains 11" 死循环。orchestrator 的 executor 是**原子写** replicas + plan 注解的，手工改 replicas 会破坏不变量。
- 解决 3：把 `spec.replicas` 对齐到注解 plan 的副本总数（11）→ 校验通过 → orchestrator 自然继续扩缩（11→12→…）。
- 现象 4（node Deployment 名称带哈希）：`simulator-tenant-core-model-lite-node-<plan-hash>` 的名称随放置计划变化，脚本写死旧名会 NotFound；演练脚本需 `kubectl get deploy -l platform.study.com/instance=<name>` 动态发现，主/副两个 Deployment 都要操作。
- 现象 5（Prometheus ConfigMap 不热载）：configmap 挂载是符号链接原子替换，Prometheus 的 fsnotify 不触发规则重载；需 `curl -X POST http://localhost:19090/-/reload`（已启用 --web.enable-lifecycle），或 rollout restart。
- 备注：演练用"停 controller + 双 Deployment 缩 0"复现 Leader 缺失最干净（约 2 分钟停机）；`maxScaleUpBatch` 默认 10、0=默认，前端预置模板弹性 20 / 保守 2。
