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

## 4. 迭代实测（599c7f9 → f111704）

### 4.1 提交与改动

| 提交 | 内容 | 预期 |
| --- | --- | --- |
| 5db8668 | concurrency cancel-in-progress | 频繁推送不排队 |
| 622dc45 | 修复 lint 产物 mtime、镜像改走 buildx | lint 缓存命中、gha 生效 |
| cd18a89 | workflow_dispatch；lint 缓存加分析缓存 + touch | lint 全命中 |
| f111704 | E2E 并行构建 + CertManager 并行；deployment 拆两个 job | E2E / 镜像 job 提速 |

### 4.2 关键发现

- **actions/cache 的 key 匹配包含 path 派生的 version**：622dc45 给 lint 缓存增加 `~/.cache/golangci-lint` 路径后，同 key 旧缓存 miss（正常失效），本次重新保存，下次命中。
- **make 的目录前置坑**：缓存 restore 后 `bin/golangci-lint`（custom 产物）mtime 比 `bin/` 目录旧，make 判定目标过期而重编译；修复：命中缓存后 `touch` 刷新 mtime。
- **lint 全命中后 24-32 秒**：bin/ 缓存（跳过下载与插件编译）+ `~/.cache/golangci-lint` 分析缓存。
- **gha 镜像缓存基本未生效**：层输出几乎无 `CACHED`（4 镜像仅 3 层）；Dockerfile `COPY . .` 后是 `go build`，源码每次提交都变，编译层必然重跑——镜像构建提速不押在 gha 上。
- **E2E 并行构建 Go 镜像无收益**：4 vCPU runner 上并行两个 Go 编译，构建阶段仍 2m55s（CPU 密集，总 CPU 时间不变）。
- **墙钟 = 最慢 job = E2E**：lint/controller/verify-deploy 的提速都在墙钟以下，不改变总耗时。

### 4.3 最终数据（f111704，缓存热）

| job | 基线（f801e3d） | 最终（f111704） | 说明 |
| --- | --- | --- | --- |
| lint | ≈2m12s | **24s** | bin/ + 分析缓存命中 |
| controller | ≈2m12s | 2m31s | race 全包，受并发 VM 波动 |
| backend / frontend | 36s / 20s | 33s / 23s | 无变化 |
| verify-deploy | （并入 deployment） | **45s** | 从 deployment 拆出并行 |
| docker-images | （并入 deployment 4m33s） | **4m05s** | 4 镜像并行构建，Go 编译硬成本 |
| E2E | ≈5m07s | **5m22s** | Go 编译 2m55s + kind 25s + 测试 ~1m |
| 整次 push 墙钟 | ≈5m07s | **5m22s** | 瓶颈 E2E；墙钟以下 job 均大幅提速 |

> 最终结论：墙钟由 E2E 决定，E2E 的 Go 编译（约 3 分钟）是每次代码提交的硬成本。两处独立编译（E2E + docker-images）由 GitHub Actions job 隔离决定，共享产物会引入 job 串行依赖反而更慢。**在现有架构下约 5 分钟是现实下限**；要突破需引入镜像 registry（如 ghcr）预构建，属于架构级改动，暂缓。

## 5. 未验证项

- ghcr 预构建镜像方案（架构级，未实施）。
- E2E 测试就绪轮询改为事件等待（有偶发失败风险，未动）。
- gha 镜像缓存对 base / go mod 层的少量收益（当前未观察到明显命中）。

## 4. 未验证项

- gha 镜像缓存命中后的构建耗时（需第二次代码提交观察）。
- actions/cache 命中后的 lint job 耗时（需第二次运行观察）。
- docs-only 提交只跑"文档检查"的完整触发行为（随文档提交验证）。
