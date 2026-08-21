package aiops

import "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"

// 本文件是 AIOps 上下文组装器（#112 阶段 C 收口）。
// 所有层级的模型输入都必须经过这里的组装函数：只读结论型数据（快照差量 / 子级摘要 / 窗口总结 /
// 警戒 / 已完成分数），不读原始 resource_events 全量与 Prometheus 原始序列；预算裁剪在此统一执行。
// 新增输入路径必须走本文件，禁止在调用点自行拼接（防止绕过记忆边界）。

// assembleEntityFacts 组装 L1 输入：起点/终点快照差量 → 实体事实（紧凑、全字符串）。
// 内部委托 extractEntities（保留原函数名便于测试与前端稳定排序）。
func assembleEntityFacts(start, end *model.CurrentSnapshot) []entityFact {
	return extractEntities(start, end)
}

// assembleL2Input 组装 L2 输入：硬指标 + L1 摘要（只读摘要），摘要区超预算按
// 「分数 > 结论 > 现象 > 事件」裁剪。返回（输入文本, 是否裁剪, 错误）。
func (service *Service) assembleL2Input(hard hardMetrics, summaries []model.AIOpsEntitySummary) (string, bool, error) {
	return l2UserPrompt(hard, summaries, budgetL2Summaries, service.logger)
}

// assembleAggregationInput 组装 L3/L4 输入：子级摘要数组，超窗口数预算只保留最近 N 个。
// 返回裁剪后的 children 与是否裁剪（裁剪记日志由调用方完成）。
func assembleAggregationInput(children any, budget int) (any, bool) {
	switch items := children.(type) {
	case []l3Child:
		if trimmed := trimChildLists(items, budget); len(trimmed) != len(items) {
			return trimmed, true
		}
	case []l4Child:
		if trimmed := trimChildLists(items, budget); len(trimmed) != len(items) {
			return trimmed, true
		}
	}
	return children, false
}
