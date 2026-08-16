# 已知坑位清单

> 维护层：agents ｜ 最后同步：2026-08-16 ｜ 对应变更：change-history/2026-08-16-docs-layered-ownership/
> 记录格式：现象 → 原因 → 解决 → 验证 → 日期。新坑按日期倒序追加到对应主题。
> 这是"踩坑即记录"的流水账，不替代 `docs/` 的正式说明。
> 领域层面的"已知易误判点"（来自原 AI_CONTEXT 第 8 节）见本文最后一节。

## 命令与终端（WSL / Windows 宿主）

### 2026-08-16 PowerShell 直传 wsl.exe 引号被拆
- 现象：`wsl.exe -d Ubuntu -- bash -lc "..."` 内含引号或括号时 bash 报 `syntax error`。
- 原因：Windows 到 wsl.exe 的原生参数传递会重排引号，内部双引号被拆开。
- 解决：脚本内容 base64 编码后 `echo <b64> | base64 -d > /tmp/x.sh && bash /tmp/x.sh`；含中文或引号的脚本一律走 base64。
- 验证：本次文档体系交付全程使用该方法无失败。
- 备注：wsl 启动时的 "localhost 代理未镜像" 是环境噪音，可忽略。

### 2026-08-16 git commit 中文提交信息丢失
- 现象：PowerShell 传参提交后，GitHub 上提交标题只剩英文前缀。
- 原因：Windows → WSL 参数编码吞掉非 ASCII 字符。
- 解决：`echo "提交信息" | base64 -w0 > /tmp/msg.b64 && git commit -F <(base64 -d /tmp/msg.b64)`。
- 验证：`dc5308e`、`89916cc` 中文完整。

## Go 构建与 CRD 生成

### 2026-08-16 gofmt 对齐问题被 lint 拦下
- 现象：`internal/controller/simulationclock_controller_test.go` 等 3 个文件报 "File is not properly formatted (gofmt)"。
- 原因：手工编辑导致对齐与 gofmt 不一致；lint 使用自定义 golangci-lint（`.custom-gcl.yml`）。
- 解决：`make fmt`；lint 前先 `make golangci-lint` 编译带自定义插件的二进制。
- 验证：修复后 `make lint` 通过。

### CRD 修改后必须重新生成
- 现象：只改 `api/v1/*_types.go` 会导致清单与生成结果不一致。
- 解决：`make manifests generate YEAR=2026`；`config/crd/bases/*.yaml`、`config/rbac/role.yaml`、`zz_generated.deepcopy.go` 只由生成器维护，不手改。
- 验证：CI "源码与部署验证" 会核对生成一致性。

## GitHub API 与 gh

### 2026-08-16 gh 偶发 TLS handshake timeout / EOF
- 现象：批量创建 label、编辑 issue 时部分请求失败。
- 解决：失败项重试即可；批量脚本要容忍失败，最后统一核对清单。
- 验证：重试后 17 个 label、5 个 issue 标签全部就位。

### gh 权限不足不等于无数据
- 现象：Projects v2 字段查询报权限错误；SSH keys 列表 404。
- 原因：token scopes 缺 `read:project`、`admin:public_key`。
- 解决：先查 `totalCount` 判断"有没有"；缺 scope 需 `gh auth refresh` 或重新授权，不要根据报错断定无数据。
- 验证：`projectsV2 totalCount=0` 确认真无 Projects。

## YAML 与模板

### 2026-08-16 issue form description 中 `bug: ` 冒号被 YAML 解析
- 现象：`yaml.safe_load` 报 "mapping values are not allowed here"。
- 原因：plain scalar 里 `bug: ` 被当成嵌套 mapping。
- 解决：description 整行加双引号包裹。
- 验证：3 个 issue 模板 YAML 校验通过。

## 数据库与容器验证

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

### 迁移规则（本次确立）
- 新表/结构变更只追加 `migrations/NNN_*.sql`，不修改已应用的迁移（`schema_migrations` 已记录）。
- 迁移必须幂等（`IF NOT EXISTS` / `ON CONFLICT`），Backend 启动自动应用。
- 验证：`TestPostgresLifecycle` 覆盖"迁移幂等 + 重启后历史仍在"。

### 2026-08-16 一键启动脚本的两个坑
- `hack/local-cluster.sh` 可能丢失执行位（Windows 侧操作后 100644）：`setup.sh` 报 `Permission denied` 时先 `chmod +x hack/*.sh` 并提交 mode 变化。
- 端口转发存活检查只看 ps 会误判：进程死亡但 PID 文件残留时 `cluster-open` 不会重建转发（8080 无监听但日志说"已在运行"）。修复后检查包含 `/dev/tcp` 端口探测；遇到 8080 无响应先看 `.runtime/port-forward-*.pid` 与 `ps aux | grep port-forward`。
- 并行构建四个镜像后，构建日志会交错输出；判断失败以退出码与最终镜像存在为准，不要按日志顺序读。

## 领域已知易误判点（原 AI_CONTEXT 第 8 节）

- Traffic Overlay 是本地草稿；页面有真实数据不等于场景已写回控制平面。
- `TenantRuntime.status.instanceCount` 的实现含义是可用 Replica 总数。
- `Model.spec.absoluteScore` 是用户/Backend 提供的必填能力基准；旧 `status.absoluteScore` 仅用于滚动升级兼容，不应再写入。
- TenantNodePolicy、ModelNodePolicy 的 Status 当前没有 writer；空 Conditions 不等于失败。
- Backend watch ReplicaSet 并记录事件，但 Workloads DTO 当前未直接展示 ReplicaSet。
- 数据库 `clock_state` 仍未驱动运行时。`SimulationClock/default` 只控制 Simulator 引擎倍速；Backend server/actual/logical time、Controller cooldown/freshness、Lease 和采集周期继续使用真实 UTC。
- 配置批次会先 dry-run 全部对象，再顺序写入；跨对象写入并非数据库式原子事务。
- SSE 是非持久通知流，慢客户端可能丢事件；30 秒轮询是安全网。