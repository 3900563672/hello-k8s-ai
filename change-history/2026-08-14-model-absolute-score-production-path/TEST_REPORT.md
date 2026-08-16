# 测试报告

测试日期：2026-08-14

## 1. 验证结论

| 检查 | 结果 | 说明 |
| --- | --- | --- |
| 缺失常量文件复核 | PASS | `constants.go` 与误删前版本内容一致；根模块恢复编译。 |
| `make manifests generate` | PASS | controller-gen v0.21.0 成功生成 CRD、RBAC 和 DeepCopy。 |
| 根 Go module 单元/集成测试 | PASS | 全部非 E2E 包使用 `-count=1` 通过。 |
| Dashboard Backend 测试 | PASS | 全部包使用 `-count=1` 通过，含新增 Command Gateway 用例。 |
| 根与 Backend `go vet` | PASS | 无 vet 错误。 |
| Go 入口二进制 | PASS | Manager、Simulator 和 Dashboard Backend 均可单独构建。 |
| E2E 编译 | PASS | `go test -tags=e2e ./test/e2e -run '^$' -count=1` 通过。 |
| Shell 语法 | PASS | `setup.sh`、`hack/local-cluster.sh`、`hack/cleanup-obsolete.sh` 通过 `bash -n`。 |
| Kustomize 渲染 | PASS | `config/dev`、`config/demo`、`dashboard/deploy` 均成功构建。 |
| 生成文件一致性 | PASS | 再次生成后 Model CRD、Manager RBAC、DeepCopy 哈希不变。 |
| `go mod tidy` | PASS | 根与 Backend module 均无 go.mod/go.sum 差异。 |
| 限定变更 `git diff --check` | PASS | 本次覆盖包文件无尾随空格或 patch 格式问题。 |
| golangci-lint v2.12.2 | PASS | 首轮复杂度问题抽取函数后复测为 `0 issues`。 |
| Controller race detector | PASS | `go test -race ./internal/controller -count=1` 通过。 |
| Backend race detector | PASS | `go test -race ./... -count=1` 在 Backend module 全部通过。 |
| Frontend 全量源码语法解析 | PASS | esbuild v0.25.12 解析 `src/` 与 `verification/` 下全部 86 个 TypeScript/TSX 文件。 |
| Frontend `npm run check` | NOT RUN | 离线缓存缺少 `zustand@5.0.14`，工作容器不允许访问 npm registry；未伪造 typecheck/build 结果。 |
| 实际 Kind E2E | NOT RUN | 当前工作容器没有可用 Docker/kubectl/kind；只完成 E2E 编译，不伪造集群结果。 |
| GitHub Actions YAML | PASS | 三个现有 workflow 均可解析，E2E 和构建任务具有明确超时与清理路径。 |

Frontend 完整检查由 `test.yml` 的独立 Frontend job 在依赖可安装的 Runner 中执行 `npm ci && npm run check`。

## 2. Controller 覆盖

新增测试验证：

1. Spec 分数优先于旧 Status；
2. 旧对象缺 Spec 时仍能读取 Status；
3. 两处都缺失时返回 0；
4. 只有缺分数模型时 Decision reason 为 `model_absolute_score_missing`；
5. 同时存在有分数模型但容量不足时仍为 `no_feasible_placement`；
6. 从 TenantModelPolicy 收集 Model 的真实路径使用 Spec，不只是测试评分函数手工塞值；
7. Model 分数修改后，即使当前副本已占满节点余量，existing instance 的 effectiveScore 仍能刷新。

## 3. Backend 覆盖

新增 `commands_test.go` 验证：

- `absoluteScore` 在 Model Spec 白名单内；
- 缺失字段被 Command Gateway 拒绝；
- `status` 不能混入 Spec 绕过所有权。

最终的数值下限由生成 CRD 的 `minimum: 1` 保证。

## 4. E2E 用例设计

新增用例不依赖部署脚本的 Status patch，按公开入口验证：

1. 用 kubectl 创建缺少 `spec.absoluteScore` 的 Model，期望 API 拒绝；
2. 创建分数为 137 的有效 Model，读取 Spec 确认值未丢失；
3. 创建 WorkerNode、Tenant、三类 Allow Policy 和 Orchestrator；
4. 等待 TenantModelPolicy Controller 创建休眠 SimulatorInstance；
5. 等待 Orchestrator 使用 Spec 分数首次扩到 1，并写 `effectiveScore=137`；
6. 等待 Simulator Pod 在选定节点 Ready。

这覆盖了：

```text
公开 Model API -> CRD 校验 -> Policy -> Orchestrator -> SimulatorInstance -> Deployment/Pod
```

## 5. 自动化补强

`make verify` 现在统一执行 Go 格式、Controller、Backend、E2E 编译、Frontend、Kustomize 和 lint 检查。GitHub Actions 还会：

- 在两个 Go module 中执行 `go mod tidy` 并拒绝未提交的依赖变化；
- 对 Controller 和 Backend 执行 race detector；
- 重新生成 CRD/RBAC/DeepCopy 并检查差异；
- 构建并校验 Controller、Simulator、Backend 和 Frontend 镜像；
- 使用固定 Kind v0.32.0 和 Kubernetes v1.36.1 节点镜像执行真实 E2E；
- 无论 E2E 结果如何都尝试删除独立测试集群。

## 6. 仍需 CI/目标集群执行

源码中的实际 E2E 需要 Docker、kubectl、Kind 和可拉取的测试基础镜像：

```bash
make test-e2e
```

E2E 已在失败路径中输出 Controller 日志、Kubernetes Events、Metrics Pod 日志和 Pod 描述。当前交付环境无法运行 Docker/Kind，因此实际运行结果以更新后的 CI 或目标机命令输出为准。
