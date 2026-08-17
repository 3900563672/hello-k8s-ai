# GitHub Actions 与 CI

> 日期：2026-08-16 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-16 golangci-lint 预置普通二进制会跳过自定义插件编译
- 现象：CI 直接把官方 golangci-lint 二进制放到 `bin/` 后，`make lint` 不再编译 `.custom-gcl.yml` 里的 logcheck 插件，lint 语义悄悄变化。
- 原因：Makefile 的 `golangci-lint` 目标以文件存在性判断是否安装；v2.12.2 官方 release 没有 with-plugins 预编译资产。
- 解决：CI 中先用官方二进制执行 `golangci-lint custom --destination bin --name golangci-lint-custom` 再 `mv` 覆盖 `bin/golangci-lint`，与本地 `make lint` 一致；`bin/` 用 actions/cache 缓存（key 含 `.custom-gcl.yml` 哈希）。
- 验证：golang 容器内完整复现 CI 步骤，`golangci-lint run` 0 issues。

### 2026-08-16 CI 里 go install kind 每次都要编译
- 现象：E2E workflow 用 `go install sigs.k8s.io/kind@v0.32.0`，每次冷编译浪费约 1 分钟。
- 解决：curl 官方 release 预编译二进制（`kind-linux-amd64`）到 `$HOME/.local/bin` 并加入 `GITHUB_PATH`。
- 验证：`kind version` 通过，E2E 不再有 go install 编译阶段。

### 2026-08-16 BuildKit gha 缓存要先 docker buildx create
- 现象：`--cache-from/--cache-to=type=gha` 在 builder 未初始化时不生效或报错。
- 解决：工作流先 `docker buildx create --use --name ci-builder`（`|| true` 容忍已存在）；Makefile 通过 `DOCKER_BUILD_CACHE` 变量注入缓存参数，本地默认空不受影响。
- 验证：镜像构建 job 启用 gha 缓存；注意 `|| true` 会掩盖创建失败，出错时先查 builder。

### 2026-08-16 等待 CI 不要长 sleep
- 现象：Agent 等 CI 用固定长 sleep，用户看到"全部跑完了但还在等"。
- 解决：每 30 秒轮询一次（`gh run list` / `gh run view --json jobs`），预期 3-6 分钟，超过 10 分钟无结论再停下排查；失败取 `gh run view <run-id> --log-failed`。
- 验证：本次交付全程 30 秒轮询，无空等。

### 2026-08-16 墙钟由最慢 job 决定，优化墙钟以下的 job 不改变总耗时
- 现象：lint / controller / verify-deploy 各自提速明显，但整次 push 墙钟仍约 5 分 20 秒。
- 原因：三个 workflow 并行，墙钟 = 最慢 job = E2E（5m22s）；其余 job 本来就在墙钟内跑完。
- 解决：先量各 job 耗时找出瓶颈（`gh run view --json jobs`），再决定优化对象；本次结论是 E2E 的 Go 编译（约 3 分钟）为硬成本。
- 验证：lint 从 2m13s 降到 24s，墙钟不变；E2E 内部并行化后墙钟仍不变。

### 2026-08-16 E2E 并行构建 Go 镜像无墙钟收益（CPU 密集）
- 现象：BeforeSuite 并行构建 manager/simulator 镜像（两个 goroutine），构建阶段仍 2m55s，与串行相同。
- 原因：Go 编译是 CPU 密集，4 vCPU runner 上并行两个编译 = 总 CPU 时间不变。
- 解决：并行仍保留（收益在"与 CertManager 安装重叠"），但不要指望并行编译本身省时间；要省只能复用镜像产物（架构级改动，暂不做）。
- 验证：f111704 实测构建阶段 2m55s（串行基线 1m46s + 1m09s）。

### 2026-08-16 gha 镜像缓存对 Go 编译层无效
- 现象：`--cache-from/--cache-to=type=gha` 参数正确传入、builder 用 docker-container，但层输出几乎没有 `CACHED`（4 个镜像仅 3 层）。
- 原因：Dockerfile 里 `COPY . .` 之后是 `go build`；源码每次提交都变，编译层必然重跑。gha 能缓存的只有 base 镜像与 `go mod download` 层，收益约 1 分钟内。
- 解决：保留 cache 参数（对依赖层有少量收益）；不要把镜像构建提速押在 gha 缓存上。
- 备注：actions/cache 的命中匹配除 key 外还包含由 path 派生的 version；修改 cache path 会导致旧缓存 miss（属正常失效，不要误判为 bug）。

### 2026-08-16 E2E 测试阶段约 1 分钟是就绪轮询
- 现象：E2E 测试执行约 1m17s，其中约 24 秒的 spec 里有 11 次每秒一次的轮询。
- 原因：spec 等待资源就绪（deployment/pod/端点），每次轮询间隔 1 秒，属于稳定逻辑。
- 解决：不要为省时间调小轮询间隔（有偶发失败风险）；如确需优化，应改为"按事件等待"（需要较大重构）。
- 验证：多次运行该 spec 稳定约 24 秒。

### 2026-08-16 E2E 偶发 make undeploy 静默挂起直到套件超时（runner 环境 flake）
- 现象：4ab7ec9 的 E2E 全部用例通过后，`make undeploy` 静默挂起约 6 分钟直到 ginkgo 10 分钟超时；`gh run rerun --failed` 重跑同一 commit 直接全绿。
- 原因：`kubectl delete -k config/default` 在 GitHub runner 上偶发不返回（疑似 kind API 或 docker 环境瞬时故障）；同一代码在其他多次运行中 undeploy 均 15 秒内完成。
- 解决：先重跑确认（`gh run rerun <run-id> --failed`）；若同一 commit 复现再排查，看 `gh run view <run-id> --log` 超时前最后一步与挂起命令。
- 验证：31942621277 重跑 success；369c158 等历史运行 undeploy 正常。
