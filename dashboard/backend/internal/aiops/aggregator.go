package aiops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops/prompts"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// 本文件实现 M3 时间聚合（#95）：L3 窗口总结 + L4 日总结。
// 与 L1/L2 同一 worker 框架：输入源换成子级摘要（L3 读切面 L2 结果，L4 读 L3 窗口总结），
// 触发为定时器；LLM 失败走规则兜底，不影响调度主流程。

// windowAggregation 是窗口/日聚合的产出（LLM 与规则兜底共用 schema）。
type windowAggregation struct {
	Overall        int      `json:"overall"`
	Trend          string   `json:"trend"`
	CommonIssues   []string `json:"commonIssues"`
	Situation      string   `json:"situation"`
	Recommendation string   `json:"recommendation"`
}

// l3Child 是喂给 L3 模型的一个切面总结（紧凑，只读 L2 结果）。
type l3Child struct {
	AnalysisID string `json:"analysisId"`
	Overall    int    `json:"overall"`
	Verdict    string `json:"verdict,omitempty"`
	Reason     string `json:"reason,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
}

// l4Child 是喂给 L4 模型的一个 L3 窗口总结。
type l4Child struct {
	WindowStart string `json:"windowStart"`
	Overall     int    `json:"overall"`
	Trend       string `json:"trend,omitempty"`
	Situation   string `json:"situation,omitempty"`
}

// runWindowAggregation 聚合最近 windowSlots 个 L3 窗口：
// 已结束且已聚合的窗口跳过，进行中窗口每轮重算（Upsert 幂等）。
func (service *Service) runWindowAggregation(ctx context.Context) error {
	granularity := service.config.WindowGranularity
	now := time.Now().UTC()
	existing, err := service.database.ListAIOpsWindowSummaries(ctx, string(model.AIOpsWindowL3), 1000)
	if err != nil {
		return fmt.Errorf("list existing L3 windows: %w", err)
	}
	existingSet := make(map[string]bool, len(existing))
	for _, summary := range existing {
		existingSet[summary.WindowID] = true
	}
	const windowSlots = 6
	for offset := 0; offset < windowSlots; offset++ {
		windowEnd := now.Add(-time.Duration(offset) * granularity)
		windowStart := windowEnd.Add(-granularity)
		windowID := l3WindowID(windowStart)
		// 已结束窗口且已聚合 → 跳过（数据不再变化）。
		if windowEnd.Before(now.Add(-granularity)) && existingSet[windowID] {
			continue
		}
		analyses, err := service.database.ListAIOpsAnalysesInWindow(ctx, windowStart, windowEnd)
		if err != nil {
			return fmt.Errorf("list analyses in window %s: %w", windowID, err)
		}
		if len(analyses) == 0 {
			continue
		}
		aggregation, err := service.aggregateChildren(ctx, prompts.L3Window, toL3Children(analyses), analyses)
		if err != nil {
			return fmt.Errorf("aggregate L3 window %s: %w", windowID, err)
		}
		summary := model.AIOpsWindowSummary{
			WindowID:    windowID,
			Level:       string(model.AIOpsWindowL3),
			WindowStart: windowStart,
			WindowEnd:   windowEnd,
			Scores:      mustRaw(aggregation),
		}
		if err := service.database.UpsertAIOpsWindowSummary(ctx, summary); err != nil {
			return fmt.Errorf("upsert L3 window %s: %w", windowID, err)
		}
		service.logger.Debug("AIOps L3 window aggregated", "windowId", windowID, "analyses", len(analyses))
	}
	return nil
}

// runDayAggregation 聚合当日 L4 日总结：一天一次（window_id 唯一，已存在跳过）。
func (service *Service) runDayAggregation(ctx context.Context) error {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	windowID := l4WindowID(dayStart)
	if existing, err := service.database.ListAIOpsWindowSummaries(ctx, string(model.AIOpsWindowL4), 1); err != nil {
		return fmt.Errorf("list existing L4: %w", err)
	} else {
		for _, summary := range existing {
			if summary.WindowID == windowID {
				return nil // 今日日总结已产出
			}
		}
	}
	windows, err := service.database.ListAIOpsWindowSummaries(ctx, string(model.AIOpsWindowL3), 100)
	if err != nil {
		return fmt.Errorf("list L3 windows: %w", err)
	}
	children := make([]l4Child, 0, len(windows))
	for _, window := range windows {
		if window.WindowStart.Before(dayStart) || !window.WindowStart.Before(dayEnd) {
			continue
		}
		var aggregation windowAggregation
		if err := json.Unmarshal(window.Scores, &aggregation); err != nil {
			continue
		}
		children = append(children, l4Child{
			WindowStart: window.WindowStart.Format(time.RFC3339),
			Overall:     aggregation.Overall,
			Trend:       aggregation.Trend,
			Situation:   aggregation.Situation,
		})
	}
	if len(children) == 0 {
		return nil
	}
	aggregation, err := service.aggregateChildren(ctx, prompts.L4Day, children, nil)
	if err != nil {
		return fmt.Errorf("aggregate L4 %s: %w", windowID, err)
	}
	summary := model.AIOpsWindowSummary{
		WindowID:    windowID,
		Level:       string(model.AIOpsWindowL4),
		WindowStart: dayStart,
		WindowEnd:   dayEnd,
		Scores:      mustRaw(aggregation),
	}
	if err := service.database.UpsertAIOpsWindowSummary(ctx, summary); err != nil {
		return fmt.Errorf("upsert L4 %s: %w", windowID, err)
	}
	service.logger.Debug("AIOps L4 day aggregated", "windowId", windowID, "windows", len(children))
	return nil
}

// aggregateChildren 用 LLM 聚合子级摘要；LLM 失败用规则兜底（平均分 + 问题抽取）。
func (service *Service) aggregateChildren(ctx context.Context, systemPrompt prompts.Definition, children any, analyses []model.AIOpsAnalysis) (windowAggregation, error) {
	// 输入预算：超过窗口数上限只保留最近 N 个（#112 阶段 B，裁剪记日志）。
	switch items := children.(type) {
	case []l3Child:
		if trimmed := trimChildLists(items, budgetL3Children); len(trimmed) != len(items) {
			service.logger.Warn("AIOps L3 input trimmed by budget", "before", len(items), "after", len(trimmed))
			children = trimmed
		}
	case []l4Child:
		if trimmed := trimChildLists(items, budgetL4Children); len(trimmed) != len(items) {
			service.logger.Warn("AIOps L4 input trimmed by budget", "before", len(items), "after", len(trimmed))
			children = trimmed
		}
	}
	payload, err := json.Marshal(children)
	if err != nil {
		return windowAggregation{}, fmt.Errorf("marshal children: %w", err)
	}
	aggregation, usage, ok, reason := callStructured(ctx, service, systemPrompt, string(payload),
		func(content string) (windowAggregation, error) {
			var aggregation windowAggregation
			err := json.Unmarshal([]byte(content), &aggregation)
			return aggregation, err
		}, validateWindowAggregation)
	if ok {
		service.recordTokenUsage("aggregation", systemPrompt.ID, usage)
		return normalizeWindowAggregation(aggregation), nil
	}
	service.logger.Warn("AIOps window aggregation falling back to rules", "reason", reason)
	return service.ruleWindowAggregation(children, analyses), nil
}

// ruleWindowAggregation 规则兜底：overall 取子级平均，trend 由平均比较，问题从 verdict 抽取。
func (service *Service) ruleWindowAggregation(children any, analyses []model.AIOpsAnalysis) windowAggregation {
	aggregation := windowAggregation{Trend: "stable"}
	switch items := children.(type) {
	case []l3Child:
		total, count := 0, 0
		issues := []string{}
		for _, child := range items {
			total += child.Overall
			count++
			if child.Verdict != "success" && strings.TrimSpace(child.Reason) != "" {
				issues = append(issues, child.Reason)
			}
		}
		if count > 0 {
			aggregation.Overall = total / count
		}
		aggregation.CommonIssues = firstN(issues, 3)
	case []l4Child:
		total, count := 0, 0
		for _, child := range items {
			total += child.Overall
			count++
		}
		if count > 0 {
			aggregation.Overall = total / count
		}
		aggregation.Situation = fmt.Sprintf("当日共 %d 个 L3 窗口，综合分 %d。", count, aggregation.Overall)
	}
	_ = analyses
	if aggregation.Situation == "" {
		aggregation.Situation = "规则兜底：基于窗口内切面分数平均。"
	}
	aggregation.Recommendation = "建议关注连续低分切面与趋势恶化窗口。"
	return aggregation
}

// normalizeWindowAggregation 收敛 LLM 输出字段取值。
func normalizeWindowAggregation(aggregation windowAggregation) windowAggregation {
	switch aggregation.Trend {
	case "improving", "stable", "degrading":
	default:
		aggregation.Trend = "stable"
	}
	if aggregation.Overall < 0 {
		aggregation.Overall = 0
	}
	if aggregation.Overall > 100 {
		aggregation.Overall = 100
	}
	aggregation.CommonIssues = firstN(aggregation.CommonIssues, 3)
	return aggregation
}

// toL3Children 从窗口内切面分析提取 L3 输入（只读 L2 分数与结论）。
func toL3Children(analyses []model.AIOpsAnalysis) []l3Child {
	children := make([]l3Child, 0, len(analyses))
	for _, analysis := range analyses {
		var scores model.AIOpsScores
		if err := json.Unmarshal(analysis.Scores, &scores); err != nil {
			continue
		}
		children = append(children, l3Child{
			AnalysisID: analysis.AnalysisID,
			Overall:    scores.Overall,
			Verdict:    scores.Verdict,
			Reason:     scores.Reason,
			CreatedAt:  analysis.CreatedAt.Format(time.RFC3339),
		})
	}
	return children
}

func l3WindowID(windowStart time.Time) string {
	return "L3-" + windowStart.UTC().Format("2006-01-02T15-04")
}

func l4WindowID(dayStart time.Time) string {
	return "L4-" + dayStart.UTC().Format("2006-01-02")
}

func firstN(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func mustRaw(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}
