# 升级与回滚

- 变更日期：2026-08-16
- 关联提交：`599c7f9`

## 升级方式

- 无数据库迁移、无 CRD 变化、无 Schema 变化：推送代码后由 GitHub Actions 自动生效。
- 本地开发者无感：`DOCKER_BUILD_CACHE` 默认为空，`make lint`、`make docker-build-local` 语义不变。

## 回滚方式

- `git revert 599c7f9`（chore: 加速 CI）即可回到旧 workflow 与旧 Makefile 行为；随后 `git revert` 文档提交恢复工作流规范。
- 回滚不影响集群内工作负载与已部署数据。

## 风险与注意事项

- **触发面变化**：docs-only 提交不再触发 lint / 单元测试 / E2E。若文档改动伴随代码改动，代码 workflow 会照常触发；纯文档改动依赖 `docs.yml` 的链接检查兜底。
- **缓存键漂移**：actions/cache 与 gha 缓存的 key 含版本与配置文件哈希，升级 golangci-lint / 修改 `.custom-gcl.yml` 会自动失效重建；`version: latest` 的 logcheck 插件在缓存命中时保持首次编译版本，需要更新插件时改动 `.custom-gcl.yml` 即可。
- **buildx 初始化被掩盖**：`docker buildx create --use --name ci-builder || true` 失败时静默继续，若镜像构建报缓存相关错误，先检查 builder 是否创建成功。
- **并行构建输出交错**：`docker-build-local` 四路并行日志交错，失败以退出码与镜像存在为准。
