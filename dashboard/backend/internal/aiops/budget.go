package aiops

import (
	"log/slog"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// 本文件实现 #112 阶段 B：每层输入预算表与截断策略。
// 截断优先级统一为：分数 > 结论 > 现象 > 事件（先裁过程型，再裁结论型，分数/结论保留）。
// 裁剪动作必须记日志，便于校准预算。

// 每层输入预算（rune 数；中文 1 rune ≈ 1 token 的保守估计，英文按 4 字符 ≈ 1 token 折算后留余量）。
const (
	budgetL1FactRunes    = 200  // 单实体事实上限（extractEntities 已保证紧凑字段）
	budgetL2Summaries    = 4000 // L2 输入摘要区预算
	budgetL3Children     = 24   // L3 输入子级总结数上限（窗口内切面数）
	budgetL4Children     = 96   // L4 输入窗口数上限（当日 L3 窗口）
	budgetChatContextRun = 6000 // 对话单轮上下文预算（与 chatContextTarget 对齐）
)

// truncateSummaries 按优先级裁剪 L2 输入摘要（#112）：
// 第一步丢弃现象（过程型）；仍超预算时对结论（结论型）逐条截断，分数与分类永不裁。
// 返回裁剪后的摘要与是否发生裁剪。
func truncateSummaries(summaries []model.AIOpsEntitySummary, budget int, logger *slog.Logger) ([]model.AIOpsEntitySummary, bool) {
	total := func(items []model.AIOpsEntitySummary) int {
		sum := 0
		for _, item := range items {
			sum += runeLen(item.Phenomenon) + runeLen(item.Conclusion) + 8
		}
		return sum
	}
	if total(summaries) <= budget {
		return summaries, false
	}
	trimmed := make([]model.AIOpsEntitySummary, len(summaries))
	copy(trimmed, summaries)
	droppedPhenomenon := 0
	for i := range trimmed {
		if runeLen(trimmed[i].Phenomenon) > 0 {
			trimmed[i].Phenomenon = ""
			droppedPhenomenon++
		}
	}
	if total(trimmed) <= budget {
		logger.Warn("AIOps L2 context truncated: phenomenon dropped",
			"entities", len(trimmed), "dropped", droppedPhenomenon)
		return trimmed, true
	}
	// 仍超预算：逐条截断结论（保留前 40 字），现象已全丢。
	const conclusionCap = 40
	truncated := 0
	for i := range trimmed {
		if runeLen(trimmed[i].Conclusion) > conclusionCap {
			trimmed[i].Conclusion = string([]rune(trimmed[i].Conclusion)[:conclusionCap]) + "…"
			truncated++
		}
	}
	logger.Warn("AIOps L2 context truncated: phenomenon dropped, conclusions capped",
		"entities", len(trimmed), "dropped", droppedPhenomenon, "capped", truncated)
	return trimmed, true
}

// truncateChatContext 对组装好的对话上下文做预算截断（#112）。
// 返回（文本, 是否裁剪）；调用方负责记录裁剪日志。
func truncateChatContext(text string, budget int) (string, bool) {
	if runeLen(text) <= budget {
		return text, false
	}
	runes := []rune(text)
	return string(runes[:budget]) + "…（上下文已裁剪）", true
}

// summarizeBudget 汇总各层预算（供日志与调试）。
func summarizeBudget() map[string]int {
	return map[string]int{
		"L1_fact":    budgetL1FactRunes,
		"L2_input":   budgetL2Summaries,
		"L3_input":   budgetL3Children,
		"L4_input":   budgetL4Children,
		"chat_input": budgetChatContextRun,
	}
}

// trimChildLists 按窗口数上限裁剪 L3/L4 输入（保留最近 N 个）。
func trimChildLists[T any](items []T, budget int) []T {
	if len(items) <= budget {
		return items
	}
	return items[len(items)-budget:]
}
