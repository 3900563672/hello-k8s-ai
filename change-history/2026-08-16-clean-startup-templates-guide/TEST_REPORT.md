# 测试报告

## 1. 前端

- 命令：`cd dashboard/frontend/my-app && npm ci && npm run check`
- 环境：WSL Ubuntu（node v22.22.1 / npm 9.2.0）
- 结果：`npm ci` 成功（156 packages, 0 vulnerabilities）；`npm run check` 全绿——lint 0 警告 0 错误、`tsc -b && vite build` 成功（GuidePage 独立 chunk）、`verify:state` 通过。

## 2. Backend（Go）

- `go build ./...`、`go vet ./internal/app/`：通过。
- `go test ./...`：除 `internal/api` 的 `TestGrafanaProxyPreservesSubPathAndForwards`、`TestGrafanaProxyRootPath` 外全部通过。
- 这两个失败确认与本改动无关（stash 本次 `app.go` 改动后同样失败）：本机 WSL/Docker Desktop 环境对 `httptest.NewServer` 的 127.0.0.1 监听存在约 300ms 的 accept 延迟（独立复现：延迟 300ms 后 dial 与代理均成功）；GitHub Actions CI 环境无此问题。
- 其余包：`internal/clock`、`internal/kubernetes`、`internal/providers/*`、`internal/readmodel`、`internal/store` 全部 ok。

## 3. 脚本与清单

- `bash -n hack/local-cluster.sh`：通过。
- `kubectl --context docker-desktop apply --dry-run=client -k config/observability`：渲染通过。

## 4. 集群假数据清理（本机执行）

- 删除顺序：orchestrator-sample → tenantmodelpolicy-sample → 等 SimulatorInstance 消失 → tenant/model/派生 CR → 动态策略与 WorkerNode → 孤儿 Lease。
- 结果：10 类业务 CR 数量为 0；`resource_snapshots` / `audit_log` / `trace_index` / `command_idempotency` / `resource_states` 计数为 0（`resource_events` 在清理后有少量系统 Lease/Node 心跳事件，属真实遥测）。

## 5. 一键启动（干净模式）

- 命令：`bash setup.sh up`（`DEMO_ENABLED` 默认 false），部署日志 `.runtime/up-20260816T125419Z.log`
- 结果：全部步骤通过——10 节点 Ready、四镜像并行构建、镜像导入、Controller/可观测性/PostgreSQL/Backend/Frontend rollout 完成；8 个工作负载 Pod 全部 Running。
- 干净环境断言（脚本内置，均通过）：
  - 业务 CR 为空：10 类业务 CR 数量为 0，仅保留系统对象 `simulationclock/default`。
  - 无历史快照：`/api/v1/replay` 响应不含 `snapshot-`。
- 数据库最终计数：`resource_snapshots=0`、`resource_states=0`、`audit_log=0`、`trace_index=0`、`command_idempotency=0`；`resource_events` 为持续的真实系统遥测（Lease/Node/Pod 心跳与部署事件，非假数据）。
- 残留清理：首次验收发现 33 条快照与 55 条状态记录（2026-08-16 12:03–12:19Z，旧版本后端在 Docker 中断前写入，无跳过逻辑）；TRUNCATE 两张表后观察 2 分钟，确认新后端不再写入。
- 端口与页面：8080 端口转发存活；`http://localhost:8080`、`http://localhost:8080/guide`、`http://localhost:8080/grafana` 均 HTTP 200（WSL 与 Windows 双侧验证）。