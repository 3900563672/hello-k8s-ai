# 数据库与容器验证

> 日期：2026-08-16 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-16 WSL 无 Go：用 golang 容器编译与测试
- 现象：WSL 里没有 go 命令，无法本地编译/跑测试。
- 解决：`docker run --rm -v $PWD:/app -w /app golang:1.26 go test ./...`；版本与 Dockerfile 保持一致（当前 golang:1.26）。
- 验证：Backend 全量测试在容器内通过。

### 2026-08-16 PostgreSQL 集成测试
- 方法：`docker run -d --name hk8s-pg-test -e POSTGRES_USER=dashboard -e POSTGRES_PASSWORD=dashboard -e POSTGRES_DB=dashboard -p 55432:5432 postgres:17-alpine`，然后 `TEST_DATABASE_URL=postgres://dashboard:dashboard@localhost:55432/dashboard?sslmode=disable go test ./internal/store/ -run TestPostgresLifecycle -v`。
- 注意：测试容器访问宿主端口用 `--network host`（WSL 原生 docker 没有 host.docker.internal）。
- 测试用专用数据库，不要指向真实集群的库。

### 2026-08-16 当前态读路径的降级边界
- `/configuration`、`/overview`、`/traffic` 无 `at` 参数时优先从数据库 `resource_states` 重建当前态；数据库可用但表为空（快照循环未跑过）时同样回退实时聚合，不要误判为故障。
- `asOf` 是数据库最新 `captured_at`（数据时间），可能比墙钟晚最多一个快照周期（默认 30s），不是 bug。
- 修改读路径时保持响应结构与实时路径一致（空集合输出 `[]` 而非 `null`）。

### 2026-08-16 resource_states 只增不改导致已删除资源变幽灵数据
- 现象：删除 Model/WorkerNode/策略等资源后，`/configuration` 仍显示已删除对象；集群 `kubectl get` 已无该对象，数据库 `resource_states` 残留旧行。
- 原因：`UpsertResourceStates` 只有 INSERT ... ON CONFLICT DO UPDATE，没有删除路径；快照循环每 30s 把当前态 upsert 进 `resource_states`，读路径又优先从该表重建当前态，已删除资源永远不会消失。
- 解决：`persistSnapshot` 每次写当前态后调用新增的 `PruneResourceStates`，按本次快照的活跃业务资源集合删除库中不存在的业务行（Model/WorkerNode/Tenant/三种 Policy/Orchestrator/SimulationClock/SimulatorInstance/Performance/Runtime/Traffic）；Node/Deployment/Pod 系统遥测保留。无业务资源时同样清理，保证删除全部资源后读路径为空。
- 验证：真实集群创建 model-prune-test → 快照写入 → API 删除 → 一个快照周期后库中与 `/configuration` 均消失；`TestPostgresLifecycle` 覆盖清理行为。
- 备注：已部署环境的存量幽灵行需要手动 DELETE 一次，新代码只负责增量清理。

### 迁移规则（本次确立）
- 新表/结构变更只追加 `migrations/NNN_*.sql`，不修改已应用的迁移（`schema_migrations` 已记录）。
- 迁移必须幂等（`IF NOT EXISTS` / `ON CONFLICT`），Backend 启动自动应用。
- 验证：`TestPostgresLifecycle` 覆盖"迁移幂等 + 重启后历史仍在"。

### 2026-08-16 一键启动脚本的两个坑
- `hack/local-cluster.sh` 可能丢失执行位（Windows 侧操作后 100644）：`setup.sh` 报 `Permission denied` 时先 `chmod +x hack/*.sh` 并提交 mode 变化。
- 端口转发存活检查只看 ps 会误判：进程死亡但 PID 文件残留时 `cluster-open` 不会重建转发（8080 无监听但日志说"已在运行"）。修复后检查包含 `/dev/tcp` 端口探测；遇到 8080 无响应先看 `.runtime/port-forward-*.pid` 与 `ps aux | grep port-forward`。
- 并行构建四个镜像后，构建日志会交错输出；判断失败以退出码与最终镜像存在为准，不要按日志顺序读。
### 2026-08-16 rollout restart 会杀死 kubectl port-forward 进程
- 现象：`rollout restart` backend/frontend 后，8080 变 000；`ps` 里 port-forward 进程消失，日志结尾是 `lost connection to pod`。
- 原因：kubectl port-forward 在 pod 重建后不会自动恢复连接，进程直接退出；脚本的 pid 文件仍残留旧 PID。
- 解决：部署后检查 `/dev/tcp/127.0.0.1/8080` 与 `ps aux | grep port-forward`；进程不存在就重建（`bash setup.sh open` 或手动 nohup + 更新 pid 文件）。
- 验证：本次部署后手动重建转发，8080/guide 恢复 200。
