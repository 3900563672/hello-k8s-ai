# 变更总览：AIOps 部署工具化（一键启用脚本 + 干净环境断言兼容模板）

> 日期：2026-08-21 ｜ 级别：P2

## 为什么做

- #136：`make cluster-up` 不会持久化 AIOps 配置，每次重新部署后 AIOPS_ENABLED 回到 false、模型/base URL 回到 OpenAI 默认值，手工 patch 易错且不可复现。
- 部署收尾还发现：`verify_clean_state` 的「业务 CR 为空」断言与 #131 模板预置（10 模型/10 租户/10 节点 preset-*）冲突，`make cluster-up` 最后一步稳定报错退出码 1，后续 port-forward 步骤被跳过。

## 改成什么

1. 新增 `hack/aiops-enable.sh`：一键启用 AIOps 并接入 DeepSeek——Key 来源优先环境变量 `AIOPS_OPENAI_API_KEY`、其次 `.runtime/aiops.env`（不落盘到仓库）；写入 Deployment env（AIOPS_ENABLED=true、base=`https://api.deepseek.com/v1`、model=deepseek-v4-flash）；`rollout status` 等待滚动完成；最后经 `/api/v1/aiops/settings` 验证 enabled/keyConfigured。
2. `hack/local-cluster.sh`：`verify_clean_state` 豁免 `preset-*` 预置模板 CR（#131 设计内行为），其余业务 CR 残留仍按失败处理；日志文案同步更新。

## 关键行为

- `bash hack/aiops-enable.sh` 幂等：重复执行直接覆盖 env 并等待 rollout，无副作用；未找到 Key 时明确报错退出，不产生半启用状态。
- 配额保护不受影响：日配额（300 次/200 万 token）由部署级 env 默认值生效，脚本不修改。

## 验证

- `bash -n` 语法检查通过。
- `make cluster-up` 完整重跑：镜像缓存复用，干净环境断言通过（preset-* 豁免），部署退出码 0。
- `bash hack/aiops-enable.sh` 实测：enabled=true / keyConfigured=true / model=deepseek-v4-flash / base=`https://api.deepseek.com/v1`。
- 命令链路实测：`POST /api/v1/aiops/commands` 解析「2 小时潮汐流量，峰值 50，倍速 20」→ duration=120min、tidal、peakQps=50、rate=20、曲线 25 点正弦采样、墙钟 360s。

## 回滚

- 未合并：git reset --hard HEAD~1。
- 运行中：`kubectl -n hello-k8s-ai-system set env deployment/hello-k8s-ai-dashboard-backend AIOPS_ENABLED=false` 并删除 hack/aiops-enable.sh 与 verify_clean_state 豁免行即可。
