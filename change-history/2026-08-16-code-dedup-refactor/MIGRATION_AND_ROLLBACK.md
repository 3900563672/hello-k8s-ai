# 升级与回滚

## 1. 迁移

- 无迁移需求：本次不涉及 CRD、数据库、配置文件或部署清单。
- 新增 `internal/k8sutil` 与 `dashboard/backend/internal/providers/httputil` 均为纯内部代码包，不影响运行期外部契约。

## 2. 回滚

- `git revert` 本提交即可完整回滚；无数据或 Schema 需要回退。
- 回滚后恢复为重构前的重复实现，行为与现在一致。

## 3. 风险与注意

- 风险极低：所有改动都经过 gofmt / vet / 单测 / 前端构建验证。
- 唯一语义细节：`PatchStatusWithRetry` 在对象无变化时不发 API（幂等优化）；Simulator 每 Tick 的 ObservedAt 都会推进，实际写入频率不变。若未来有人依赖「无变化也 Patch 产生 resourceVersion 变化」的行为，需要显式绕过该 helper。
- 新增包名与函数名遵循仓库既有中文注释风格，后续可继续向 `internal/k8sutil` 收敛其他 Kubernetes 对象操作工具。
