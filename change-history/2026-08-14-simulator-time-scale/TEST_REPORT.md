# 测试报告

测试日期：2026-08-14

## 1. 当前交付环境结果

| 检查 | 结果 | 说明 |
| --- | --- | --- |
| 限定变更 `git diff --check` | PASS | 排除用户原有 `PROJECT_OVERVIEW_NEW.md` 后无空白或 patch 格式问题。 |
| Go/TypeScript/TSX 语法树解析 | PASS | 变更涉及的 35 个源码文件均无语法错误。 |
| Frontend oxlint | PASS | `oxlint src verification` 无问题。 |
| YAML 解析 | PASS | `config/` 与 `dashboard/deploy/` YAML 均可解析。 |
| Kustomize 引用静态检查 | PASS | 11 个 Kustomization 的本地 resource/patch 引用均存在；不等同于真实渲染。 |
| Shell 语法 | PASS | `setup.sh`、`hack/local-cluster.sh`、`hack/cleanup-obsolete.sh` 通过 `bash -n`。 |
| Markdown 完整性 | PASS | 59 个 Markdown 无失效相对链接或未闭合代码块。 |
| Frontend 完整 typecheck/build/state | BLOCKED | 附件中的 `node_modules` 不完整，TypeScript 报告 20 个直接模块缺失；本次改动的 6 个受编译覆盖文件没有直接诊断，但不可把该结果冒充完整构建通过。 |
| Go fmt/vet/unit/race/lint | NOT RUN | 当前容器没有 Go/gofmt/golangci-lint。 |
| manifests/generate/Kustomize | NOT RUN | 当前容器没有 controller-gen、kubectl 或独立 Kustomize。 |
| Kind E2E | NOT RUN | 当前容器没有 Docker、kubectl 和 Kind。 |

未运行项必须由 CI 或目标机补齐；本记录不复用旧日志冒充本次结果。

## 2. 新增单元与契约测试

### Controller

- Clock 缺失时创建 `default`/1x；
- 5x 同步只改变 Instance timeScale，保留 replicas、QPS、annotation 和其他 writer 的 Status；
- Clock Status 报告 applied/synchronized/total/Ready；
- 超范围输入报告 InvalidRate 且不修改 Instance。

### Simulator

- 同一进程先以 2x 推进 2 秒，再运行时改为 8x推进到累计 10 秒；
- 冷启动按累计模拟时长结束，`status.score` 恢复 effectiveScore；
- `status.observedAt` 保持注入的真实墙钟，证明倍速没有污染新鲜度；
- 零值/超范围防御性钳制和 duration 饱和累计。

### Backend

- Cache 读取 desired/applied/sync counts/version；
- generation 落后的旧 Ready 不会被误报 converged；
- Clock 缺失回退 1x但不可写假 Ready；
- Clock DTO 的 server/actual/logical time 不随 Simulator rate 改变；
- Gateway 创建/更新 singleton、处理与 Controller 的并发创建、拒绝 0/21、拒绝旧 resourceVersion；
- Prometheus `simulator.timeScale` 查询模板固定且过滤安全。

### Frontend

状态验证覆盖：能力未开放时拒绝提交；能力开放时实际发送 PATCH，包含 rate、resourceVersion、dryRun=false 和 `Idempotency-Key`；receipt 更新目标版本并进入等待收敛状态。

## 3. E2E 用例设计

新增 Kind 用例：

1. 创建 Clock、Model、节点 Policy 和带固定放置计划的 SimulatorInstance；
2. 等待 Simulator Pod Ready，记录 Pod 名和 UID；
3. 把 Clock 从 1x Patch 到 10x；
4. 等待 Clock desired/applied=10、同步数等于总数、Ready=True；
5. 确认目标 Instance timeScale=10；
6. 读取 Pod `/metrics`，断言 time_scale=10、simulation_step_seconds=50；
7. 再读 Pod UID，确认没有 rollout/restart。

同步计数不写死为 1，测试允许套件中存在其他 Instance，但要求 synced=total 且至少一个实例。

## 4. CI/目标机必须执行

```bash
make manifests generate YEAR=2026
make fmt-check
make test
make test-backend
make test-e2e-compile
make test-frontend
make verify-deploy
make lint
make test-e2e
```

涉及生成物时，生成后还要确认无未提交差异：

```bash
git diff --exit-code -- api/v1 config/crd config/rbac PROJECT
```

完整 Docker Desktop 部署验收：

```bash
make cluster-up
```

## 5. 重点回归观察

- Clock Status Patch 是否自触发高频 reconcile；
- Instance 每 5 秒 Status 变化是否误触发全量 Clock 扇出；
- rate 变化是否修改 Deployment template 或 Pod UID；
- 高倍速下 Tick duration、实体/虚拟 Queue 和进程资源是否可接受；
- Controller cooldown/freshness 是否保持真实时间；
- Frontend 在 pending/converging/historical 状态是否阻止重复命令；
- Backend cache 在 Spec/Status 短暂错位时是否始终报告 `converged=false`。
