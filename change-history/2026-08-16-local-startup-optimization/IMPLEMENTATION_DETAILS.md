# 实现修改明细

- 变更日期：2026-08-16
- 关联问题：Refs #11
- 关联提交：`1f7b987`（并行构建与跳过重复拉取）、`2885f1e`（端口转发存活检查）、`f801e3d`（本机验证与文档归档）

## 1. 改动前状态

- `bash setup.sh` 一键启动完整栈耗时 6 分 1 秒，其中四个项目镜像**串行**构建约 5 分钟为主。
- 第三方运行镜像（postgres / prometheus / jaeger / otel / grafana）每次启动都重新 `docker pull`，即使本地已存在。
- `hack/local-cluster.sh` 在 Windows 侧操作后丢失执行位（100644），`setup.sh` 直接 `Permission denied`。
- 端口转发存活检查只看 `ps` 与 PID 文件：进程死亡但 PID 文件残留时，`cluster-open` 误判"已在运行"，端口无监听也不重建。

## 2. 修改内容

### 2.1 四个项目镜像并行构建

- `hack/local-cluster.sh` 的 `build_project_images()` 把 manager / simulator / backend / frontend 四个 `docker build` 放入后台并行执行（每个 `&` 收集 PID，最后 `wait` 所有 PID）。
- 并行构建只影响本地启动脚本，不改变镜像内容、构建参数与部署清单。
- 构建日志会交错输出；判断失败以退出码与最终镜像存在为准，不能按日志顺序读。

### 2.2 跳过重复拉取

- `pull_runtime_images()` 只对**非项目**运行镜像（数组下标 4 起）做 `docker image inspect` 检查，本地已存在则跳过 `docker pull`；项目镜像始终由 `build_project_images()` 重新构建。
- 拉取失败仍走 `retry 3`，语义不变。

### 2.3 修复脚本执行位

- 恢复 `hack/local-cluster.sh`（及相关脚本）为 100755 并提交 mode 变化，`setup.sh` 不再 `Permission denied`。

### 2.4 端口转发存活检查

- `start_port_forward()` 的存活判断从"只看进程参数"改为"进程参数匹配 **且** `/dev/tcp/127.0.0.1/<local_port>` 真实可连"。
- 端口不通即视为失效：删除残留 PID 文件并重建转发；重建后继续等待日志出现 `Forwarding from`。
- 该检查对所有转发端口（8080 dashboard、5432 数据库等）统一生效。

## 3. 兼容性与边界

- 低配机器（<4 CPU）并行构建可能收益变小，未做针对性调优。
- 多副本 Backend 行为不在本次验证范围（本机为单副本）。
- 启动脚本最终结论以"全部端口可用 + `verify_data_flow` 完整验收通过"为准。

## 4. 验证方式

- 本机 docker-desktop 集群实测：基线 6m01s → 优化后 3m38s（详见 [TEST_REPORT.md](TEST_REPORT.md)）。
- 完整验收 `verify_data_flow` 10 项全部通过；重启后历史数据继续累积、迁移幂等。
