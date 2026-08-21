# 后端覆盖率门禁落地（#142/#143）：5 核心包达标 + CI 硬 gate

> 日期：2026-08-21 ｜ 关联：issue #142（覆盖率基线/门禁）、#143（目标）；前端部分同批并入 PR #144

## 为什么做

- 用户要求给后端测试覆盖率设定门禁与目标，CI 不达标即红，防止覆盖率回退；前端 vitest 体系同批补齐（见 PR #144 历史提交）。
- 此前仅 `make test` 能跑，无覆盖率硬约束，5 个核心包（controller/aiops/api/kubernetes/segment）长期低于合理水位。

## 改成什么

1. 新增 `hack/coverage-check.py`（门禁脚本，含包-阈值表、store 的 DB 门控）与 `hack/cover-gaps.py`（覆盖率缺口分析辅助）；`Makefile` 新增 `coverage` target。
2. `.github/workflows/test.yml` 新增 `coverage` job：postgres:17-alpine service + `TEST_DATABASE_URL`，跑主 module 与 dashboard module 覆盖率检查；前端 job 追加 `npm run test:coverage`（vitest v8 coverage + 防回退阈值）。
3. 后端补测（全部纯 fake client / 无 DB 集成依赖）：
   - `internal/controller`：Reconcile 分支、orchestrator 决策输入、映射函数、条件更新、finalizer 清理等（61.2% ≥ 60%）。
   - `dashboard/backend/internal/aiops`：聚合器窗口、worker 生命周期、LLM 配置（83.9% ≥ 80%）。
   - `dashboard/backend/internal/api`：handler 分支、配额、chat/command、buffered 通道（51.0% ≥ 50%）。
   - `dashboard/backend/internal/kubernetes`：typed cache 构造/排序/accessors（44.8% ≥ 40%）。
   - `dashboard/backend/internal/segment`：错误注入与分位数边界（93.3% ≥ 80%）。
4. `store` 包集成测试补齐（`postgres_gap_test.go`、`postgres_aiops_test.go` 扩充），并把 `ListAIOpsCommands` 的 `LIMIT` 改为参数绑定（消除注入面）；CI 有 postgres 时按阈值 40% 硬校验。

## 关键行为

- 未设置 `TEST_DATABASE_URL` 时 store 显示 `SKIP-DB`（警告不红），与本机无 Docker 场景兼容；CI 始终为硬 gate。
- 覆盖率基线以 2026-08-21 实测为准，目标抬升跟踪 #143。

## 验证

- 本机 `python3 hack/coverage-check.py`：15 包 gate，核心包全 OK，store SKIP-DB。
- `go test ./internal/controller/`、dashboard module `go vet`/无 DB 测试全绿；前端 vitest 174 用例全绿（29.42/23.54/30.29/30.09 高于阈值 24/19/21/24）。

## 回滚

- git revert 本批提交；删除 `hack/coverage-check.py`/`hack/cover-gaps.py`、`Makefile` coverage target、test.yml coverage job 与前端 `test:coverage` 步骤。
