# CI 加速与工作流细化

- 变更日期：2026-08-16
- 关联问题：无（用户直接要求的工程基建优化）
- 变更级别：P1 工程基建
- 变更范围：`.github/workflows/`（4 个 workflow）、`Makefile`、`docs/agents/`（工作流与归档规范）、`change-history`
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

GitHub Actions 全量测试耗时从"最慢 job 约 5-6 分钟且全部重复跑"降下来，手段分为三层：

- **减少触发面**：docs / change-history / `*.md` 的改动不再触发 lint、单元测试、E2E 三个重型 workflow，只跑新增的"文档检查"（`make docs-check`）。
- **消除重复与编译开销**：Controller job 从"非 race 全包 + race 单包"合并为一次 race 全包；lint 改用官方预编译二进制 + 现场编译自定义插件（logcheck），并用 actions/cache 缓存 `bin/`；E2E 的 kind 改用官方预编译二进制，不再每次 `go install`。
- **镜像构建缓存**：Makefile 新增 `DOCKER_BUILD_CACHE` 注入点，CI 使用 BuildKit `type=gha` 缓存；`docker-build-local` 改为四个镜像并行构建。

同时按用户要求细化了两条协作约定：

- **CI 轮询节奏**：Agent 等待 CI 时每 30 秒轮询一次，不 sleep 到固定大间隔（预期 3-6 分钟，超 10 分钟才停下排查），写入 `docs/agents/WORKFLOW.md` 与 `docs/agents/SYNC.md`。
- **变更归档详略**：change-history 条目必须四件套齐全（README 精简总览 + IMPLEMENTATION_DETAILS / TEST_REPORT / MIGRATION_AND_ROLLBACK 完整细节），禁止只写一行结论；并补强了此前过简的 `2026-08-16-local-startup-optimization` 条目。

## 2. 关键行为

- 代码改动 → 跑 lint / 单元测试 / E2E / 部署验证四个 workflow（docs-only 时只跑文档检查）。
- lint 语义不变：`.custom-gcl.yml` 的 logcheck 插件仍会编译进二进制，缓存命中时跳过编译、未命中时"预编译 + `golangci-lint custom`"现场编译。
- 镜像构建缓存：`--cache-from/--cache-to=type=gha` 由 CI 注入，本地 `make docker-build-local` 行为不变（`DOCKER_BUILD_CACHE` 默认为空）。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| .github/workflows/lint.yml | paths-ignore、actions/cache、预编译 + custom 插件编译 |
| .github/workflows/test.yml | paths-ignore、Controller job 合并为一次 race、deployment job 启用 gha 缓存 |
| .github/workflows/test-e2e.yml | paths-ignore、kind 预编译二进制、gha 缓存 |
| .github/workflows/docs.yml | 新增：docs-only 改动只跑 `make docs-check` |
| Makefile | 新增 `DOCKER_BUILD_CACHE`；`docker-build-local` 并行构建 |
| docs/agents/ | WORKFLOW / SYNC / README / KNOWN_PITFALLS 增加 CI 轮询节奏与归档详略规范 |
| change-history | 新增本条目；补强 local-startup-optimization 条目 |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 剩余未验证

- gha 镜像缓存在多次运行间的实际收益（本次为首个冷缓存运行，需后续运行对比）。
- actions/cache 对 lint 的命中效果（首次未命中，需第二次运行验证）。
- 多分支同时跑时 gha 缓存 key 的分支隔离行为。
