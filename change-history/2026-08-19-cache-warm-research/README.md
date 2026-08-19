# WSL 缓存预热对照实验：瓶颈定位到 seccomp 通知链（#72）

> 日期：2026-08-19 ｜ 关联：Documents/26_churn_744_6400_analysis.md、23/26 号文档

## 为什么做

- 26 号文档提出待验证假设：128 并发下积压串行点究竟是 guest 侧 seccomp 通知处理链（GnsPortTracker::Run 单线程队列），还是 Windows 侧 ConsommeNetworking 排他锁。
- 通过"预热固定端口到缓存（100% 命中，不经 Windows 锁）"与"全新端口（走完整 Windows 路径）"对照，可判定主瓶颈所在。

## 改成什么

- 新增探针 `research/exp/cache_warm.go`（go run，无需管理员权限）：
  - phase1 预热 100 个固定端口（30000-30099）填充 m_allocatedPorts 缓存（60s TTL）。
  - phase2 128 并发重复 bind 这 100 个端口（缓存命中，不经 Windows 排他锁）。
  - phase3 128 并发 bind 128 个未预热端口（30100-30227，走完整 Windows 路径）。
- 探针产出：p50/p95/p99/max/avg/吞吐/失败端口，可直接复现。

## 实验结果

- phase1 预热 100 端口：1.09s（≈100×10.9ms，注册链 10.4ms 基线一致）。
- phase2 缓存命中：p50=1.32s / p95=1.36s，吞吐 95 bind/s，512/512 成功。
- phase3 未缓存对照：p50=1.39s / p95=1.42s，吞吐 91 bind/s，512/512 成功。
- **结论**：缓存命中与未缓存路径几乎无差，且均 ≈ 128×10.4ms → 瓶颈确认在 guest 侧 seccomp 通知处理链（GnsPortTracker::Run 单线程队列），Windows 排他锁不是主瓶颈，26 号假设证实。

## 验证

- 健康态（200 轮直方图归档后）执行，固定端口不受 fresh-port 退化窗口影响。
- 原始日志：Desktop/WSL/logs/cache-warm-072.txt；结果已回写 26 号文档（Desktop 与仓库 Documents 镜像同步）。

## 回滚

- 纯研究探针与文档，无运行时影响；删除 `research/exp/cache_warm.go` 及 26 号文档"验证结果"小节即可。
