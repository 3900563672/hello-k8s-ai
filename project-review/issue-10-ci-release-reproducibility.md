# 1. 问题标题

CI 与交付基线无法证明完整系统可重复发布

## 2. 当前状态描述

仓库包含三个 GitHub Actions：根 Go 模块 lint、根 Go 模块单元测试和 Kind E2E。根 `Makefile` 的 `make test` 运行根模块包，能够覆盖 Controller、Simulator 和 Envtest，但 `dashboard/backend` 是独立 Go module，不会被根模块的 `go list ./...` 纳入。现有 CI 没有进入该目录执行 Backend 测试。

Frontend `package.json` 已提供 `lint`、`build`、`typecheck`、`verify:state` 和组合的 `check`，但 GitHub Actions 没有安装 Node 依赖或运行这些脚本。Frontend 源码中没有常规单元/组件/E2E 测试文件，只有一个状态验证脚本。

`test/e2e/e2e_test.go` 主要验证 Operator SDK 默认链路：安装 CRD、启动 Controller、访问受保护 metrics。文件末尾明确保留 TODO，尚未断言项目 CR 创建后 Controller 状态收敛。测试不部署 Simulator、Backend、PostgreSQL、Prometheus、OpenTelemetry、Jaeger 或 Frontend，也不验证完整数据流。

E2E 工作流从 `kind.sigs.k8s.io/dl/latest` 下载 Kind，版本不固定；测试和单元工作流都先执行 `go mod tidy`，但没有检查该命令是否改变 go.mod/go.sum。CI 也没有渲染全部 Kustomize、构建四个镜像、执行一键部署脚本或验证镜像入口。

当前归档进一步暴露了交付基线问题：Git `HEAD` 是 `c51f586`，但有 9 个 tracked 文件显示已修改；归档还包含 `.idea/`、`.runtime/`、PID/日志以及一个嵌套的 `hello-k8s-ai.tar.gz`。这些内容大多被 Git 忽略，却进入了源码包。因而“当前可运行版本”无法仅通过 commit ID 重建，交付包也混入了环境状态。

归档内 `.runtime/up-20260813T061633Z.log` 是有价值的全链路成功证据，但它由本地脚本产生，不是 CI 中的可重复发布门禁。

## 3. 问题定位

项目各子系统已有不少测试，问题不是完全没有测试，而是测试边界与发布边界不一致。CI 通过只能说明根 Go 模块和 scaffold E2E 通过，不能说明 Backend、Frontend、数据库迁移、四镜像或跨组件数据链路可以工作。

使用 latest 工具版本会让同一提交在不同日期得到不同 E2E 环境。CI 中执行会修改依赖文件的命令却不检查差异，也可能掩盖未提交的 module 变化。

源码归档不是干净 Git 基线，会使后续开发者无法判断问题来自提交、未提交修改还是本地运行残留。嵌套压缩包和大量日志还增加传输体积与误覆盖风险。

## 4. 影响范围

- Controller/Simulator：根模块有测试，但核心 CR 收敛没有进入真实 Kind E2E。
- Backend：已有 8 个测试文件，但当前 CI 不执行独立 module。
- Frontend：构建和静态检查有脚本，当前 CI 不执行，也缺少用户流程测试。
- 数据库：migration 只在运行时执行，缺少 CI 中的真实 PostgreSQL 升级/回滚兼容验证。
- Kubernetes/部署：Kustomize、RBAC、四镜像和本地全栈脚本没有统一门禁。
- 多人协作：非干净源码包无法准确绑定 commit 和测试结果。
- 运维：本地成功日志不能替代每次变更的自动化验收。

本次环境没有 Go、Docker、kubectl 和 kustomize，因此未独立运行这些测试；`setup.sh` 和 `hack/*.sh` 的 Shell 语法检查通过。工具缺失不是项目失败，但也意味着本次只能确认门禁结构，而不能为当前归档重新签发测试结论。

## 5. 根本原因分析

项目由 Operator 根模块逐步扩展为 Controller、Simulator、独立 Backend、Frontend 和可观测性全栈，但 CI 仍以最初的 Operator 工程边界为中心。子系统增长速度超过了统一交付流水线的演进。

本地一键脚本承担了镜像构建、节点导入、部署、样例初始化和链路验收，功能上已经接近系统测试；但它绑定 docker-desktop 和本地环境，尚未被拆成可固定版本、可在 CI 重复运行的发布门禁。

## 6. 修改方向建议

- 建立一张“组件—检查—产物”矩阵，确保根 Go、Backend、Frontend、Kustomize 和四个镜像都有明确 CI job。
- 扩展 E2E 到最小业务闭环：创建核心 CR，等待 SimulatorInstance/Deployment/Status 收敛，并至少验证 Backend 聚合和一个指标/Trace 路径。
- 固定 Kind、Kubernetes 和构建工具版本；依赖整理命令若在 CI 运行，应在之后断言工作树没有变化。
- 将数据库 migration 放入真实 PostgreSQL 的自动化验证，覆盖空库初始化和已有 schema 升级。
- 保留当前本地一键脚本作为开发者入口，同时提取可复用的无交互验收步骤供 CI 使用；不要求把本地脚本强行改成生产部署器。
- 规定交付包只来自可追溯提交或明确记录的构建输入，排除 `.git`、`.idea`、`.runtime`、PID、日志和嵌套归档。
- 在清理当前归档前先确认 9 个 tracked 修改的业务内容并提交或回退；本次审查不替开发者决定这些修改的去留。

## 7. 优先级

优先级：P1

建议在多人并行开发、版本发布或把当前归档作为长期基线前处理。它不会阻止现有本地演示，但会显著放大后续回归和交付风险。
