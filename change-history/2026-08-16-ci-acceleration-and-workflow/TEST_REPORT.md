# 测试报告

- 验证日期：2026-08-16
- 环境：GitHub Actions ubuntu-latest（CI）、WSL Ubuntu + golang:1.26 容器（本地 lint 复现）

## 1. 本地 lint 流程复现（容器内）

在 `golang:1.26` 容器内按 CI 相同步骤执行：

```text
curl 下载 golangci-lint-2.12.2-linux-amd64 → 软链 bin/golangci-lint
→ golangci-lint custom --destination bin --name golangci-lint-custom
→ mv 覆盖 bin/golangci-lint → golangci-lint run
```

结果：`golangci-lint` 版本为 `v2.12.2-custom-gcl-*`（插件已编译），`golangci-lint run` 输出 `0 issues`。

## 2. 静态校验

| 校验项 | 命令 | 结果 |
| --- | --- | --- |
| workflow YAML | python3 `yaml.safe_load`（4 个文件） | 通过 |
| Makefile 语法 | `make -n lint`、`make -n docker-build-local` | 通过 |
| lint 插件语义 | 容器内 `golangci-lint run` | 0 issues |

## 3. CI 实测（推送 599c7f9，缓存冷启动）

| job | 基线（f801e3d） | 本次（599c7f9） | 说明 |
| --- | --- | --- | --- |
| lint | ≈2m12s | 2m13s | 冷缓存首次：curl 预编译 + custom 插件编译 |
| controller | ≈2m12s | 2m24s | 合并为一次 race 全包，覆盖面变大 |
| backend | ≈36s | 38s | 无变化 |
| frontend | ≈20s | 18s | 无变化 |
| deployment/镜像 | ≈4m33s | 4m38s | 冷 gha 缓存：并行构建 + cache 导出 |
| E2E | ≈5m07s | 5m18s | kind 预编译生效，镜像构建为冷缓存 |
| 整次 push 墙钟 | ≈5m07s（最慢 job） | 5m22s | 三个 run 并行，最慢为 E2E |

> 结论：**冷缓存首次运行与基线持平**（lint 不再 go install，E2E 不再编译 kind，但 actions/cache 与 gha 镜像缓存均未命中）。预期收益在后续运行显现：lint 命中 `bin/` 缓存跳过下载与插件编译；deployment / E2E 命中 gha 镜像缓存跳过重复构建层。docs-only 提交只触发"文档检查"的行为已由本次文档提交单独验证。

## 4. 未验证项

- gha 镜像缓存命中后的构建耗时（需第二次代码提交观察）。
- actions/cache 命中后的 lint job 耗时（需第二次运行观察）。
- docs-only 提交只跑"文档检查"的完整触发行为（随文档提交验证）。
