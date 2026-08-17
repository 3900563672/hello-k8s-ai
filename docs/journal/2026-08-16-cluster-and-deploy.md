# 集群操作与部署

> 日期：2026-08-16 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-17 WSL 内访问 localhost:8080 与 Windows dllhost 转发冲突（脚本必须走 18080）
- 现象：day-watch 10:03 起 GET traffic 全部失败（`read ECONNRESET`/`fetch failed`），keepalive 每轮却 ok:true；WSL 内 curl 8080 时好时坏（首个 200、后续全 000）；Windows 侧访问 localhost:8080 始终 200。
- 原因：Windows 侧 `dllhost.exe` 监听 `127.0.0.1:8080`（WSL2 localhost 转发宿主）；WSL 内 kubectl port-forward 也监听 8080，同端口冲突，WSL 内连接被抢占/重置。keepalive 每轮是独立新进程，首个连接成功即报 ok（假阳性）。
- 解决：WSL 内脚本一律走 `18080`（`local-cluster.sh` 新增 `dashboard-internal` 转发 18080:80）；8080 保留给 Windows 浏览器。day-watch 默认 `--base-url http://localhost:18080` 并透传给 keepalive/snapshot。
- 验证：18080 连续 4 次 200；PATCH 50→35 全链路成功；Windows 8080 保持 200。
- 备注：kubectl port-forward 日志“Handling connection”增加但连接仍失败 = 端口冲突特征；先查 Windows `netstat -ano | findstr 8080`。


### 2026-08-17 kubectl 输出超过 1MB 时 spawnSync/execFileSync 报 ENOBUFS（Pod 多时 keepalive pods 检查必现）
- 现象：副本扩到 141 后 `keepalive.mjs --once` 的 pods 检查失败 `spawnSync kubectl ENOBUFS`，其余检查全绿（假阴性）。
- 原因：`execFileSync`/`spawnSync` 默认 maxBuffer=1MB；`kubectl get pods -o json` 在 100+ 模拟器 Pod 时 JSON 远超 1MB。
- 解决：所有 kubectl 子进程调用加 `maxBuffer: 32 * 1024 * 1024`（keepalive.mjs runKubectl、day-watch.mjs kubeSnapshot）。
- 验证：141 Pod 时 keepalive 全绿（simulatorPods=141 running=141 ready=141）。
- 备注：副本少时不会触发，容易被漏测；长时测试扩到 100+ 副本后首次暴露。


### 2026-08-17 批量扩容已上线：扩容会停在"节点容量上限"，不是 maxReplicas 的问题
- 现象：400 QPS 压测下副本 16→18→20 后停止，队列 2 分钟冲到 7 万、TTFT 小时级；Orchestrator Ready=True 但不再扩。
- 原因：单副本吞吐 = maxConcurrency ÷ 平均服务时长（model-lite 约 3.7 qps）；400 QPS 需 ≈108 副本，而 2 个 WorkerNode（各 maxConcurrency=160）÷ 模型 16 = 全租户最多 20 副本，扩容到节点容量即返回 `no_feasible_placement`（正常容量不足，不是错误）。`maxReplicas=0` 只解除策略上限，节点配置才是真实天花板。
- 解决：测试前把 WorkerNode `spec.maxConcurrency`（和 gpu）调大，例如目标 N 副本 × 模型 maxConcurrency × 节点数；前端"配置详解-模拟条件下怎么填"有换算公式。
- 验证：调大节点后副本应随批量扩容（每批 1..10，冷却 60s 一批）持续增长。
- 备注：压测后队列清空需要时间（排水速度 = 副本×单副本吞吐）；TTFT 平均值会带峰值尾巴，看 queue 回落为准。

### 2026-08-17 压测走 Backend API 需要 Idempotency-Key 头
- 现象：`curl -X PATCH .../traffic` 返回 `MISSING_IDEMPOTENCY_KEY`。
- 原因：Backend 写接口要求命令幂等键。
- 解决：加 `-H 'Idempotency-Key: <任意唯一值>'`；day-watch.mjs 内部已处理，手工压测脚本要带上。
- 验证：带键后返回 `state: accepted`，Tenant.spec.qps 已更新。


### 2026-08-17 更新 Controller 必须用 config/dev 部署，make deploy(config/default) 会丢掉 SIMULATOR_IMAGE env
- 现象：`make deploy`（kustomize config/default）更新 controller 后，SimulatorInstance Controller 重建模拟器 Deployment，新 Pod 用 `simulator:latest`（本地很老、无 9090 端点的镜像）→ readiness/liveness 探针 connection refused → 29s 优雅退出循环（Exit 0 + Completed，易误判为正常退出）。
- 原因：dev 栈的 `SIMULATOR_IMAGE=hello-k8s-ai-simulator:dev` env 在 `config/dev/manager-observability-patch.yaml` 里，`make deploy` 用 `config/default` 不含该 patch，apply 覆盖 Deployment 后 env 丢失，controller 回落默认 `simulator:latest`；本地 `simulator:latest` 是过期镜像。
- 解决：dev 集群更新 controller 一律 `kubectl kustomize config/dev | kubectl apply -f -`（幂等），之后 rollout restart；不要用 `make deploy`。判断依据：`kubectl get deploy hello-k8s-ai-controller-manager -n hello-k8s-ai-system -o yaml | grep SIMULATOR_IMAGE` 应有值。
- 验证：config/dev 重部署后 controller 重建模拟器 Deployment 模板为 `hello-k8s-ai-simulator:dev`，CrashLoop 的 RS 缩到 0，实例恢复 Running 并继续扩缩（本次实测 REPLICAS 10→12）。
- 备注：`simulator:latest` 默认值仅用于 CI/隔离环境；本机 dev 栈镜像 tag 是 `hello-k8s-ai-simulator:dev`（Makefile SIMULATOR_IMG）。

### 2026-08-17 节点 DNS 故障导致 nginx 启动失败：先验节点再怪代码
- 现象：重新部署 frontend 后新 Pod `CrashLoopBackOff`，日志 `nginx: [emerg] host not found in upstream "hello-k8s-ai-dashboard-backend"`；同镜像旧 Pod（另一节点）一直正常，集群内 nslookup FQDN 也正常。
- 原因：新 Pod 被调度到 `desktop-worker6`，该节点 kindnet 网络故障（kindnet/kube-proxy 均重启过 6 次），Pod 内 DNS 全超时（`busybox nslookup` 实测 `connection timed out`）。
- 解决：`kubectl cordon desktop-worker6` 后 rollout restart，新 Pod 落到正常节点即恢复；根因修复后 `kubectl uncordon desktop-worker6`。
- 验证：cordon 后 frontend rollout 成功、nginx 200；worker6 上 busybox 探针复现 DNS 全超时。
- 备注：`nginx: emerg host not found in upstream` 先查节点与 DNS（`kubectl run` 探针），不要直接改 nginx.conf/重写 upstream。

### 2026-08-17 镜像 tag 相同（:dev）时 kubectl apply 不触发滚动，必须 rollout restart
- 现象：本地重新构建 `hello-k8s-ai-dashboard-backend:dev` 后 `kubectl kustomize config/dev | kubectl apply -f -` 显示 configured，但 Pod 仍是旧镜像内容（新路由 404）。
- 原因：Kubernetes 按镜像 tag 判断 spec 是否变化，`:dev` tag 内容更新不改变 spec。
- 解决：apply 后显式 `kubectl -n hello-k8s-ai-system rollout restart deployment <name>`。
- 验证：restart 后新 Pod 生效（/segment 200）。

### 2026-08-17 make lint 触发 golangci-lint 重下载时 GOSUMDB 校验失败（本机）
- 现象：`make lint` 报 `invalid GOSUMDB: malformed verifier id`，且下载规则失败后把 `bin/golangci-lint` 符号链接删掉。
- 原因：本机 Go 环境 GOSUMDB/代理校验异常，Makefile 按 mtime 判断需要重建工具。
- 解决：直接运行已有二进制 `bin/golangci-lint-v2.12.2 run`（缺失时 `ln -sf bin/golangci-lint-v2.12.2 bin/golangci-lint`）。
- 验证：`bin/golangci-lint run` → `0 issues`。

### 2026-08-17 Prometheus emptyDir 重启即丢历史：段/历史查询出现"区间无数据"先查 Prometheus 存活时间【已修复：PVC 化】
- 现象：段查询 06:00Z-10:00Z 区间 qps/queue/ttft 全部 0 series，只有 errorRate 有 121 个常量 0 点（`or on() vector(0)` 空集保护产生）。
- 原因：Prometheus 数据卷是 emptyDir，11:41Z 部署时重启过，重启前的原始指标全部丢失；errorRate 的空集保护会在无数据时填常量 0 系列，容易误读成"指标为 0"。
- 解决（2026-08-17 同日）：Prometheus 数据卷改 PVC（`hello-k8s-ai-prometheus-data` 20Gi），retention 24h→168h；Jaeger 同步改 badger + PVC。旧 emptyDir 数据不迁移，切换时从空开始属预期。查询"区间无数据"时仍先核对"窗口是否早于组件首次建库时间"与"是否超出 168h 保留窗口"。
- 验证：PVC 化后 scale 0→1 重启，`count(up)` 205 不变、重启前 30 分钟采样仍在（见 change-history/2026-08-17-observability-persistence/TEST_REPORT.md）。
- 备注：历史窗口早于 12:54Z（本次切 PVC 时间）的指标仍缺失，是历史数据损失，不是新问题。



### 2026-08-17 Jaeger badger 单副本 + RWO PVC：重启/升级必须先 scale 到 0 再扩回 1
- 现象：Deployment 滚动更新（rollout restart）时新 Pod CrashLoopBackOff，日志 `Cannot acquire directory lock on "/tmp/jaeger/". Another process is using this Badger database`；旧 Pod 一直 Terminating，rollout 卡死。
- 原因：badger 在数据目录写 LOCK 文件；单副本 + RWO PVC 滚动更新会短暂出现新旧两个 Pod 同时挂载同一 PVC，新 Pod 抢不到锁即退出。
- 解决：重启/升级 Jaeger 用 `kubectl scale deploy hello-k8s-ai-jaeger --replicas=0`（等 Pod 清空）→ `--replicas=1`，不要 rollout restart；Prometheus TSDB 同样有目录锁，按同一流程操作。清单已加注解 `platform.study.com/restart-procedure: scale-to-zero`。
- 验证：scale 0→1 后 Jaeger 正常启动，重启前 Trace 仍在（badger 持久化生效）。
- 备注：若已陷入"新旧 RS 抢锁"死锁，直接 scale 0 清空所有 Pod 再扩回即可，无需删除 RS。

### 2026-08-17 Jaeger v2 显式配置后 OTLP receiver 默认只绑 127.0.0.1
- 现象：给 Jaeger 加 config.yaml 后 otel-collector 持续 `connection refused`（`dial tcp ...:4317`），Jaeger 日志显示 OTLP 只监听 `127.0.0.1:4317/4318`。
- 原因：Jaeger v2 带配置文件时 OTLP receiver 默认绑定 localhost；之前无配置时内置默认绑 0.0.0.0，掩盖了差异。
- 解决：配置里显式写 `receivers.otlp.protocols.grpc.endpoint: 0.0.0.0:4317`、`http.endpoint: 0.0.0.0:4318`。
- 验证：改后 Jaeger 监听 `[::]:4317/4318`（含 IPv4 映射），collector 导出自愈，`/api/services` 有数据。

### 2026-08-17 prometheus/client_golang CounterVec 无 label 实例时不出现在 /metrics
- 现象：Backend 加了 `promauto.NewCounterVec`，但 `/metrics` 里找不到该指标（以为没注册）；二进制 strings 又能搜到指标名。
- 原因：CounterVec 的 Gather 只输出"已有 label 组合"的系列；从未 `WithLabelValues(...)` 过就不输出，静默期看不到 0 值。
- 解决：需要"始终可见的 0 值"的计数用普通 `promauto.NewCounter`（无 label）；需要按 kind 拆分的场景要在启动时预建 label 实例或接受"首次发生才可见"。
- 验证：改用普通 Counter 后 `/metrics` 立刻出现两个计数器（0 值）。

### 2026-08-17 desktop-worker6 kindnet 网络持续故障（2026-08-17 13:10Z 复测），保持 cordon
- 现象：cordon 后复测（busybox 调度到 worker6 执行 nslookup）仍 `connection timed out; no servers could be reached`；kindnet 日志持续 `lookup desktop-control-plane: i/o timeout`。
- 原因：worker6 节点网络栈（kindnet/kube-proxy 均重启过 6 次）未自愈；根因未定位，疑似 Docker Desktop 内置 K8s 节点容器重建后的 CNI 混乱（历史上 desktop-worker6/9 曾同 IP）。
- 解决：继续 `kubectl cordon desktop-worker6`；不要重启节点/集群（用户未批准）；后续从 kindnet/kube-proxy 日志与节点容器侧排查。
- 验证：cordon 状态在 `kubectl get nodes` 可见（Ready,SchedulingDisabled），业务 Pod 全部跑在健康节点。

### 2026-08-17 本机 go get 新依赖必须 GOSUMDB=off
- 现象：`go get github.com/prometheus/client_golang@latest` 报 `verifying module: invalid GOSUMDB: malformed verifier id`（GOSUMDB=sum.goproxy.cn 本机异常）。
- 原因：本机 Go 环境 sumdb 校验问题（与 `make lint` 的 GOSUMDB 失败同源）。
- 解决：`export GOSUMDB=off` 后执行 `go get`/`go mod tidy`；go.sum 正常入库，CI 侧校验不受影响。
- 验证：GOSUMDB=off 后依赖拉取成功、`go build ./...` 通过。

### 2026-08-16 Simulator Pod 调度绑定 WorkerNode 名：虚拟节点名无法调度
- 现象：用虚拟节点名（如 node-gpu-1）创建 WorkerNode 并建 TenantNodePolicy 后，SimulatorInstance 副本一直 Pending，`describe` 显示 node selector 匹配不到真实节点。
- 原因：Simulator 物化的 Pod 通过 affinity/nodeSelector 绑定 WorkerNode 名称，虚拟名在真实集群里不存在。
- 解决：WorkerNode 的 name 必须使用集群真实节点名（docker-desktop 为 desktop-worker、desktop-worker2 ...）；建 WorkerNode 前先 `kubectl get nodes` 核对。
- 验证：改为真实节点名后 SimulatorInstance 副本正常调度并 Running。
- 备注：Docker Desktop 内置 K8s 的节点名是 desktop-worker*，与 Kind 测试集群（hello-k8s-ai-test-e2e 的 kind-control-plane/worker）不同，切换环境要重查。

### 2026-08-16 Docker Desktop 重建节点后可能出现重复主机 IP（双节点同 172.18.0.4）
- 现象：rollout 新 Pod 卡 Init:0/1，init 容器 `wait-for-postgresql` 报 `no response`；Pod IP 属于其他节点网段（worker6 上的 Pod 拿到 10.244.2.x，而 worker6 的 CIDR 是 10.244.8.0/24）。
- 原因：此前强杀 Docker Desktop 后内置 K8s 节点容器重建，desktop-worker6 与 desktop-worker9 主机 IP 都是 172.18.0.4（`kubectl get pods -A -o wide` 可见两个节点同 IP），CNI 分配混乱。
- 解决：删除卡住的 Pod 让它重新调度到健康节点；必要时 `kubectl cordon desktop-worker6 desktop-worker9`。不要为此重建集群或重启 Docker。
- 验证：新 Pod 落到 worker9（10.244.2.5）后 rollout 成功；演练数据（模拟器/数据库）全程不受影响。
- 备注：彻底修复需 Docker Desktop → Settings → Kubernetes 重新启用节点容器（会中断全部工作负载），本次未做。

### 2026-08-16 清理演示 CR 必须在 Controller 在线时进行
- 现象：`cluster-down` 后删除 Tenant/Model/Policy 卡在 DeletionTimestamp，对象不消失。
- 原因：tenant-model-policy、simulator-instance-controller、performance-collector、traffic-distribution 四个 finalizer 依赖 Controller 处理。
- 解决：先恢复 Controller（`make cluster-up`）再按顺序删除：orchestrator → tenantmodelpolicy → 等 instance 消失 → tenant/model/派生 CR → 动态策略与 WorkerNode。
- 验证：本次清理 10 类业务 CR 全部归零。
- 备注：`simulationclock/default` 是系统默认对象，删除后控制器会自动重建，不需要也不建议清理。

### 2026-08-16 空配置不再写历史快照（干净环境预期）
- 现象：干净环境（无业务 CR）下 `/replay` 没有 `snapshot-*`。
- 原因：`persistSnapshot` 新增 `snapshotHasBusinessData` 判定，无模型/租户/节点/策略/编排器/实例时跳过写快照。
- 解决：这是预期行为；`resource_events` 仍会记录真实系统 Lease/Node 心跳事件，不属于假数据。
- 验证：`bash setup.sh` 干净模式验收通过。
- 备注：旧版本后端写入的残留快照不会自动消失；发现 `resource_snapshots` 有历史行时可 TRUNCATE 后再验收（脚本断言只检查 `/replay` 响应，不检查库表行数）。`resource_states` 的业务行已由 `PruneResourceStates` 自动清理，无需手工处理。

### 2026-08-16 本机 Go 测试 httptest 回环有约 300ms accept 延迟
- 现象：`TestGrafanaProxyPreservesSubPathAndForwards`、`TestGrafanaProxyRootPath` 在本机 WSL 报 502，CI 正常。
- 原因：本机 WSL/Docker Desktop 环境下 `httptest.NewServer` 的 127.0.0.1 监听刚建立时连接被拒（独立复现：延迟 300ms 后 dial 与反向代理均成功）。
- 解决：本地判断与本次改动无关时，以 CI 结果为准；不要为此改测试。
- 验证：stash 本次改动后同样失败；GitHub Actions 上历史提交均通过。
