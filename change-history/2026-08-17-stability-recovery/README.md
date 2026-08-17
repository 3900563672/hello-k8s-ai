# 变更总览：稳定性恢复顺序——运行前体检、工具链自检与 Prometheus 内存/重启告警

> 日期：2026-08-17 ｜ 级别：P1 ｜ 对应 Issue：[#29 稳定性工程重大更新](https://github.com/3900563672/hello-k8s-ai/issues/29)

## 为什么做

Issue #29 五类根因中，"环境层脆弱且不自愈"与"工具链自身没被测试"此前没有制度化防线：

- 一键启动 / 长跑启动前没有统一健康检查：节点 NotReady、PVC 未 Bound、端口冲突、残留负载、宿主内存不足都是"跑起来才爆"；
- 工具链自身没被测试：`snapshot.mjs` 漏定义 `sleep` 都能上线跑，脚本语法问题运行到才暴露；
- 可观测性慢半拍：Prometheus 只抓业务指标，看不到容器内存水位与重启，内存打满 / OOMKilled 只能事后人查。

## 改成什么

1. **运行前体检 `hack/preflight.sh`（新增）**：一键启动与长跑启动前的统一体检，8 组 19 项检查；FAIL 项中止启动，WARN 项提示。已接入 `run_up` 与 `start-longrun.sh`。
2. **工具链自检 `make selfcheck`（新增，已并入 `make verify`）**：全部 `*.sh` 语法（`bash -n`）+ `hack/*.mjs` 语法（`node --check`）+ 三套清单渲染（config/dev、config/demo、dashboard/deploy）。
3. **Prometheus 内存/重启告警**：新增 cAdvisor 抓取（经 API Server proxy，RBAC 加 `nodes/proxy`），新增 `HelloK8sAIContainerMemoryHigh`（>85% limit 持续 10 分钟）与 `HelloK8sAIContainerRestarted`（10 分钟内 `container_start_time_seconds` 变化）两条告警。
4. **Prometheus Deployment 改 `Recreate` 策略**：单副本 + RWO PVC 滚动更新会新旧 Pod 抢 TSDB 锁 CrashLoop（实测 `lock DB directory: resource temporarily unavailable`）。

## 关键行为

- 长跑脚本强制 `PREFLIGHT_REQUIRE_GUARD=1`：sleep-guard 未开启直接 FAIL、不启动。
- 体检不通过时 `run_up` 与长跑都拒绝启动，先修 FAIL 项再继续；WARN 项（worker6 cordon、端口未监听等）只提示。
- cAdvisor 走 `kubernetes.default.svc:443` + `/api/v1/nodes/${1}/proxy/metrics/cadvisor`，10 节点全部抓取 `up=1`。
- Prometheus 配置变更后直接 `rollout restart` 即可（Recreate 先删后建，TSDB 数据在 PVC 上不丢）。
