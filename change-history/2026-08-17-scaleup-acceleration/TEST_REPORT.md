# 测试报告

## 单元测试（internal/controller）

- `go test ./internal/controller/ -count=1`：全部通过。
- 新增 `TestDecideAtBatchesScaleUpByPressureGap` 覆盖：
  - 队列缺口批量（350 queue / 阈值 150 / 单副本并发 16 → 一次 +10）；
  - TTFT-only 保持 +1；
  - 单节点放不下整批 → 减半回退（GPU 4 → +2）；
  - 引导期低于地板 → 一次补到地板但不超过批次上限（0→10）；
  - 无容量 → `no_feasible_placement`。
- 既有 `TestDecideAtScaleUpCooldownAndScaleDownFloor`、`TestDecideAtSupportsScaleToZeroAndMaximum` 等全部通过（批量=1 时行为不变）。

## 静态检查

- `make lint`：0 issues（重构 `scaleUpDecision` 后 gocyclo 通过）。
- `make test`：全包通过（含 vet + cover）。
- `npm run check`（frontend）：通过。

## 真机验证（docker-desktop 集群）

- 部署：`kubectl kustomize config/dev | kubectl apply -f -` + rollout restart，controller Pod Running，`SIMULATOR_IMAGE=hello-k8s-ai-simulator:dev` 在 env 中。
- 压测：400 QPS（Backend API + Idempotency-Key）→ 副本 16→18→20（批量生效），节点容量上限拦截为 `no_feasible_placement`，Ready=True。
- 恢复：QPS 回 35，Orchestrator Ready=True Reconciled。
