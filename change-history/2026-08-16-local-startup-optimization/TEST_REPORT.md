# 测试与验证记录

- 验证日期：2026-08-16
- 环境：WSL Ubuntu + Docker Desktop 集群（docker-desktop context，1 控制面 + 9 worker，24 CPU / 16 GiB）

## 执行记录

| 轮次 | 命令 | 总耗时 | 结果 |
| --- | --- | --- | --- |
| 基线 | `bash setup.sh`（首次，镜像缓存命中） | 6m01s | 通过，完整验收 10 项全过 |
| 优化后 | `bash setup.sh`（并行构建 + 跳过已有 pull） | 3m38s | 通过，完整验收 10 项全过 |

## 关键观察

- 构建阶段是主要耗时（串行约 5 分钟）；并行后总耗时降约 40%。
- 第二次部署后历史数据继续累积（resourceEvents 263769 → 266869，resourceSnapshots 2015 → 2029），迁移幂等，无数据丢失。
- 发现并修复：`hack/local-cluster.sh` 执行位丢失导致 `setup.sh` Permission denied；dashboard 端口转发进程死亡后 PID 文件残留导致后续 `cluster-open` 误判（8080 无监听）。
- 未验证：多副本 Backend 行为（本地为单副本）；极端低配机器（<4 CPU）并行构建表现。

## 结论

本机真实集群验证覆盖了 Phase 1-3 在集群侧的行为；启动优化在不改变部署语义的前提下将一键启动从 6 分 1 秒降至 3 分 38 秒。
