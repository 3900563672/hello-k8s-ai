# 2026-08-21 AIOps 流量形状与上限（潮汐/脉冲/斜坡 + 防打爆）

> 日期：2026-08-21 ｜ 关联：docs/aiops/AIOPS_OVERVIEW.md、docs/backend/API_DESIGN.md

## 为什么做

- 实测「帮我模拟 2 个小时的潮汐流量」这类非结构化指令：AI 解析出空壳 intent（无租户/无流量/无倍速），执行后实验空跑；带租户名与倍速时仍不给 QPS——AI 不知道潮汐给多少，要么省略（功能失效）、要么可能给超大值（打爆环境）。
- 控制面 `SetTenantQPS` 只写单值 QPS，无波形能力，潮汐无法表达。

## 改成什么

1. `TrafficIntent` 扩展：`shape`（steady/tidal/spike/ramp）+ `peakQps` + `periodMinutes`（潮汐周期，默认 30 分钟）；`TrafficShapeQPS` 纯函数按模拟时间算波形值。
2. 解析校验上限：单命令峰值 QPS ∈ [1,200]、倍速 ∈ [1,100]、写流量必须指定 targetTenant、tidal/spike/ramp 必须带 peakQps、形状白名单。
3. 执行端波形调度器（`handlers_aiops_commands.go`）：非平稳流量按模拟时间推进（墙钟 = 模拟时长/倍速），每秒写一次租户 QPS，墙钟到点自动归零，不留残留流量；独立 context 不随请求取消。
4. 提示词（`command_intent.md`）：流量必填规则、峰值默认 20、目标租户必选、至少选 1 个模型模板、倍速 1-100。
5. 前端确认面板展示波形（峰值/形状/周期）与「可执行范围」提示条（数据来自 `GET /aiops/limits`）。
6. Traffic 模板库新增小时级潮汐模板 `preset-traffic-tidal-2h`。

## 关键行为

- 2 小时潮汐 + 20 倍速 = 6 分钟墙钟内完成 4 个 30 分钟模拟周期，QPS 在 [峰值/5, 峰值] 间正弦波动。
- 上限在解析阶段拦截，不产生任何写操作。
- 限制可见性：`GET /api/v1/aiops/limits` 暴露全部硬限制（单一事实源），确认面板展示「可执行范围」提示条（峰值 QPS ≤ 200 / 倍速 1-100 / 波形 / 潮汐周期），用户不会再对流量约束感到困惑。
- 长时波形：Traffic 模板库新增 `preset-traffic-tidal-2h`（2 小时潮汐，峰值 50 QPS，30 分钟周期正弦，15 分钟粒度采样），预览图时间轴支持小时级显示。

## 验证

- 后端单测：校验上限用例（超 QPS/超倍速/缺租户/缺峰值/非法形状）+ 波形函数值域与周期性。
- 实测解析：非结构化潮汐指令 → 解析出 targetTenant + traffic(shape=tidal, peakQps) + rate。

## 回滚

- git revert 本提交；波形调度器停止后需手动把租户 QPS 归零（`kubectl patch tenant <name> --type=merge -p '{"spec":{"qps":0}}'`）。
