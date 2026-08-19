# 升级与回滚

## 升级

- Schema：Backend 启动自动应用 `003_resource_states.sql`（幂等），无需人工建表或迁移命令。
- 部署：backend.yaml 新增 initContainer 后重新 apply 即可（`make cluster-up` 或 `kubectl apply -k dashboard/deploy`）。
- 配置：新增环境变量均有默认值（重试 6 次 × 5s），无需改动现有 Secret/配置。

## 回滚

| 内容 | 回滚方式 |
| --- | --- |
| 代码 | 还原本次提交涉及的 Go 文件即可；`resource_states` 表保留无害 |
| Schema | 如需彻底移除：`DROP TABLE IF EXISTS resource_states; DELETE FROM schema_migrations WHERE version='migrations/003_resource_states.sql';` |
| 部署 | 移除 backend.yaml 的 initContainer 段 |

## 风险

- `resource_states` 每 30s 全量 upsert：资源量级小（租户/模型/节点数），写入量可忽略；若未来资源量增大，可改为按变更增量写入。
- 健康接口新增 database 详情：只增不改，前端现有字段不受影响。
- 重试窗口（默认 6×5s + 连接超时）与 initContainer 配合；若 PostgreSQL 恢复超过约 2 分钟，进程仍会按 DATABASE_REQUIRED 退出并由 K8s 重启。
