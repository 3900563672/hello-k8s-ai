# 测试报告

## 1. 执行的验证与真实结果

| 命令 | 结果 |
| --- | --- |
| `gofmt -w` 新增/修改文件 | 通过 |
| `go vet ./...`（dashboard/backend） | 通过 |
| `go test ./internal/api/ -run "Auth\|Actor" -v` | 6 个用例全部通过 |
| `go test ./... -skip "Grafana"`（dashboard/backend） | 全部 ok |

覆盖场景：

- 无 token / 错误 token / 非 Bearer scheme → 401；
- 正确 token → 通过；只读 GET → 不要求认证；
- 非生产环境无 token 匿名写 → 通过；生产环境无 token 写 → 503；
- CORS OPTIONS 预检 → 不要求认证；
- `X-Remote-User`：认证 + 信任开关开启 → 采用；认证但开关关闭 → 忽略；匿名 → 忽略（不可伪造）。

## 2. 未验证项

- `TestGrafanaProxyPreservesSubPathAndForwards` 与 `TestGrafanaProxyRootPath` 在本机环境失败（backend httptest 连接 502），已在 `origin/main`（`e52364c`）独立 worktree 复现，属于环境既有失败，与本次改动无关。
- 真实集群中启用 `ADMIN_TOKEN` 后的端到端写链路，依赖本地部署/CI E2E 验证。
- `make lint` / CI 全量在推送后由 GitHub Actions 执行。
