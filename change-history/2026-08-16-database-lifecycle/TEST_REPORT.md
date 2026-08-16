# 测试报告

- 变更日期：2026-08-16
- 验证环境：WSL Ubuntu（无 Go；Docker 可用）

## 实际执行的验证

| 验证项 | 命令 | 结果 |
| --- | --- | --- |
| 编译 | `docker run --rm -v $PWD:/app -w /app golang:1.26 go build ./...` | 通过（BUILD_OK） |
| 静态检查 | 同上 `go vet ./...` | 通过 |
| Backend 全量测试 | 同上 `go test ./...`（TEST_DATABASE_URL 指向 docker 起的 postgres:17-alpine） | 全部包 ok |
| 生命周期集成测试 | `go test ./internal/store/ -run TestPostgresLifecycle -v` | PASS：3 个迁移自动应用、快照/当前态写入、重启连接后迁移幂等且历史数据仍在、states 幂等 upsert（2 行） |
| 部署清单 | 检查 backend.yaml initContainer 与 postgresql.yaml | initContainer 已加入，PostgreSQL StatefulSet + PVC 保持 |

## 验证环境说明

- 本机 WSL 无 Go：全部编译与测试在 `golang:1.26` 容器内完成（版本与 Dockerfile 一致）。
- PostgreSQL 用 `postgres:17-alpine` 容器（端口 55432），测试后已删除。
- Kubernetes 侧未部署：initContainer 行为未在真实集群验证，等待 CI / 用户 `make cluster-up`。

## 未验证范围

- 真实 docker-desktop 集群的完整链路（迁移自动应用日志、健康接口 database 详情）。
- 重试逻辑未自动化测试（逻辑为循环 + 退避，简单；CI 编译与单元测试覆盖）。
- 根 Go module（Controller/Simulator）未改动，不需要重跑。

## CI 最终结果（2026-08-16）

| Workflow | 结果 |
| --- | --- |
| 代码检查（fmt-check / vet / controller 测试 / 生成文件校验） | 通过 |
| 源码与部署验证（含 Backend 测试 `go test -race ./...`、Frontend 检查、清单生成） | 通过 |
| E2E 测试 | 通过 |

- 首次推送后 `make fmt-check` 曾报三个 store 文件未格式化，已由 `fix: 修正 store 包 gofmt 格式 Refs #12`（52ded71）修正，重新推送后三个 workflow 全部通过。

## 结论

Phase 1 + Phase 2 代码通过编译、vet 与全部测试；数据库生命周期与当前态入库行为有集成测试实证；CI 三个 workflow 最终全部通过。