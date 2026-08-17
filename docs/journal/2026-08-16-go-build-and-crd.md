# Go 构建与 CRD 生成

> 日期：2026-08-16 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-16 gofmt 对齐问题被 lint 拦下
- 现象：`internal/controller/simulationclock_controller_test.go` 等 3 个文件报 "File is not properly formatted (gofmt)"。
- 原因：手工编辑导致对齐与 gofmt 不一致；lint 使用自定义 golangci-lint（`.custom-gcl.yml`）。
- 解决：`make fmt`；lint 前先 `make golangci-lint` 编译带自定义插件的二进制。
- 验证：修复后 `make lint` 通过。

### CRD 修改后必须重新生成
- 现象：只改 `api/v1/*_types.go` 会导致清单与生成结果不一致。
- 解决：`make manifests generate YEAR=2026`；`config/crd/bases/*.yaml`、`config/rbac/role.yaml`、`zz_generated.deepcopy.go` 只由生成器维护，不手改。
- 验证：CI "源码与部署验证" 会核对生成一致性。
