# 实现修改明细

## 文件清单

### 新增

| 路径 | 说明 |
| --- | --- |
| `dashboard/backend/internal/store/migrations/003_resource_states.sql` | `resource_states` 表：kind/namespace/name 唯一，payload JSONB，按对象与时间建索引 |
| `dashboard/backend/internal/store/postgres_integration_test.go` | `TestPostgresLifecycle`：自动迁移 → 写入快照/当前态 → 重启连接 → 历史仍在、迁移幂等（`TEST_DATABASE_URL` 跳过机制） |

### 修改

| 路径 | 说明 |
| --- | --- |
| `dashboard/backend/internal/store/store.go` | Store 接口新增 `Status` / `UpsertResourceStates` / `ListResourceStates`；新增 `StoreStatus`、`ResourceStateRecord`；Disabled 同步实现 |
| `dashboard/backend/internal/store/postgres.go` | 三个新方法实现：Status（四表计数）、UpsertResourceStates（pgx batch + ON CONFLICT）、ListResourceStates（kind/namespace 过滤） |
| `dashboard/backend/internal/config/config.go` | `DatabaseConfig` 新增 `StartupRetries` / `StartupBackoff`（env `DATABASE_STARTUP_RETRIES` / `DATABASE_STARTUP_BACKOFF`） |
| `dashboard/backend/internal/app/app.go` | openDatabase 改为退避重试循环 + "PostgreSQL ready; history is persistent" 启动日志；persistSnapshot 新增 `resourceStateRecords` 拆解与 upsert |
| `dashboard/backend/internal/api/handlers_read.go` | 健康接口 `checks.database`：available / migrationsApplied / resourceEvents / resourceSnapshots / resourceStates |
| `dashboard/deploy/backend.yaml` | 新增 initContainer `wait-for-postgresql`（pg_isready 轮询） |
| `docs/agents/KNOWN_PITFALLS.md` | 追加：WSL 无 Go 容器编译、PostgreSQL 集成测试方法、迁移规则 |
| `docs/agents/PRINCIPLES.md` | 新增"数据库 / Schema 修改"规范 |
| `docs/agents/README.md` | 阅读决策表新增"数据库 / Schema 变更"行 |

### 修改（Phase 3）

| 路径 | 说明 |
| --- | --- |
| `dashboard/backend/internal/api/handlers_read.go` | 新增 `handleResourceStates`（`GET /api/v1/resources`）；`snapshotFor` 无 `at` 时数据库优先重建当前态（`currentSnapshotFromStore` / `currentSnapshotFromRecords`），失败回退实时；`capabilities` 中 PostgreSQL role 更新为 `persistent-current-and-history` |
| `dashboard/backend/internal/api/server.go` | 注册 `GET /api/v1/resources` 路由 |
| `dashboard/backend/internal/store/store.go` | `ResourceStateRecord` 补充 camelCase JSON tag（供 HTTP 输出） |

### 新增（Phase 3）

| 路径 | 说明 |
| --- | --- |
| `dashboard/backend/internal/api/resource_states_test.go` | `currentSnapshotFromRecords` 重建测试（全 kind / 空记录 / 损坏 payload 跳过）+ `/resources` handler 测试（过滤透传 / 503 / limit 校验） |

## 设计要点（Phase 3）

- 读路径切换发生在 Backend：`/configuration`、`/overview`、`/traffic` 响应结构不变，前端零改动、无感降级。
- 数据库优先的边界：仅"最新当前态"（无 `at` 参数）切换；历史回放（`at` 参数）原本就走数据库快照，保持不变。
- `asOf` 取 `resource_states` 最新 `captured_at`（数据时间），不伪装成请求时间。
- `GET /api/v1/resources` 与历史回放接口一致：存储不可用时返回 503 problem，不返回伪造数据。

## 设计要点

- 生命周期"自动"：迁移随 Backend 启动自动应用，部署用 initContainer 保证 DB 先就绪，进程内重试兜底，用户无需任何手工建表步骤。
- 生命周期"可感知"：启动日志与健康接口给出迁移数、数据量、是否恢复历史，自动执行看得见。
- 当前态入库：以聚合快照为唯一拆解源（不新开 informer），每 30s upsert，幂等；写失败只记日志（遥测不阻断控制面）。
- 兼容性：003 为纯新增表，旧库重启自动应用；Store 接口新增方法均有 Disabled 兜底。
- 验证可复现：集成测试用 `TEST_DATABASE_URL` 指向专用库，docker 一条命令可复现。