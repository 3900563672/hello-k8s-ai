# 实现修改明细

- 变更日期：2026-08-16
- 关联提交：`599c7f9`（chore: 加速 CI）；工作流与归档规范随本条目一并提交

## 1. 改动前状态（基线）

- push 触发 9 个 job 全部并行：E2E ≈5m07s、deployment/镜像 ≈4m33s、controller ≈2m12s、backend ≈36s、frontend ≈20s、lint ≈2m12s。
- lint 每次 `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` 冷编译，再 `golangci-lint custom` 编译 logcheck 插件。
- E2E 每次 `go install sigs.k8s.io/kind@v0.32.0` 冷编译。
- 镜像构建无任何缓存，四个镜像串行构建。
- docs-only 提交也会触发全部重型 workflow。

## 2. 修改内容

### 2.1 触发面（三个 workflow + 新增 docs workflow）

- `lint.yml` / `test.yml` / `test-e2e.yml` 的 push 与 pull_request 增加 `paths-ignore`：`docs/**`、`change-history/**`、`*.md`。
- 新增 `docs.yml`：`paths` 命中 `docs/**`、`change-history/**`、`*.md` 时只跑 `make docs-check`（python3 hack/check-docs.py 校验 Markdown 链接）。

### 2.2 lint：预编译 + 自定义插件 + 缓存

- 官方 release 没有 v2.12.2 的 with-plugins 预编译资产，且 Makefile 的 `golangci-lint` 目标以文件存在性跳过安装——直接放普通二进制会导致 logcheck 插件静默丢失（lint 语义变化）。
- 解决：CI 用 curl 下载官方 `golangci-lint-2.12.2-linux-amd64.tar.gz` 到 `bin/golangci-lint-v2.12.2`，软链 `bin/golangci-lint` 后执行 `golangci-lint custom --destination bin --name golangci-lint-custom` 并把产物 `mv` 回 `bin/golangci-lint`，与本地 `make lint` 行为完全一致。
- `actions/cache`（v4.3.0）缓存整个 `bin/`，key 含 `runner.os` + `.custom-gcl.yml` 哈希 + `v2.12.2`；命中时跳过下载与编译。

### 2.3 E2E：kind 预编译二进制 + gha 缓存

- kind 改为 curl 官方 release 预编译二进制到 `$HOME/.local/bin` 并加入 `GITHUB_PATH`。
- 新增 `docker buildx create --use --name ci-builder`（`|| true` 容忍已存在），让 E2E 内部 `make docker-build*` 吃到 `DOCKER_BUILD_CACHE`。

### 2.4 deployment job + Makefile

- `Makefile` 新增 `DOCKER_BUILD_CACHE ?=`：`docker-build-manager` / `docker-build-simulator` / backend / frontend 构建统一追加该参数；本地默认空，行为不变。
- `docker-build-local` 从串行依赖改为 `@$(MAKE) -j2 docker-build-manager docker-build-simulator & ... & wait` 四路并行，保留每个镜像的 ENTRYPOINT 校验。
- deployment job 先 `docker buildx create --use`，再注入 `DOCKER_BUILD_CACHE=--cache-from=type=gha --cache-to=type=gha,mode=max` 跑 `make docker-build-local`。

### 2.5 Controller job 合并

- 原"`make test`（vet + 非 race 全包）+ race 单包 controller"合并为：`make fmt-check` + `go vet ./...` + `go test -race $(go list ./... | grep -v /e2e) -count=1`，一次 race 覆盖全包，避免重复跑 Controller 测试。

### 2.6 Agent 工作流与归档规范（docs/agents/）

- `WORKFLOW.md`：新增"4.1 CI 轮询节奏"（每 30 秒轮询、预期 3-6 分钟、超 10 分钟排查、失败取 `--log-failed`）；归档节增加详略规范。
- `SYNC.md`：新增"7. CI 轮询节奏"；归档条目增加详略规范（四件套 + 完整记录背景/实现/验证/回滚/风险）。
- `README.md`：阅读决策表新增"CI / 工作流 / 变更归档"行。
- `KNOWN_PITFALLS.md`：新增"GitHub Actions 与 CI"小节，记录 golangci-lint 插件编译、kind 预编译、buildx gha 缓存、CI 轮询四个坑。

## 3. 关键设计取舍

- **不改变 lint 语义**：保留 `.custom-gcl.yml` 与 logcheck 插件，只是把"go install 编译"换成"预编译二进制 + custom 编译"，产物与本地一致。
- **不改本地行为**：`DOCKER_BUILD_CACHE` 默认空、`docker buildx create` 只在 CI 执行，本地 `make docker-build-local` 与 `make lint` 语义不变。
- **冷启动友好**：即使缓存全部未命中，lint 也比原来的 go install 快；镜像构建第一次跑仍会拉全量依赖，后续靠 gha 缓存加速。

## 4. 验证方式

- 本地 golang 容器完整复现 lint 步骤（下载 → custom 编译 → `golangci-lint run`）：0 issues。
- 四个 workflow YAML 通过 `yaml.safe_load` 校验；Makefile 目标通过 `make -n` 校验。
- 推送 `599c7f9` 后观察真实 CI run 各 job 耗时与结论（见 [TEST_REPORT.md](TEST_REPORT.md)）。
