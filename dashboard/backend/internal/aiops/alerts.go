package aiops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// 本文件实现 M3 警戒（#95）：对切面分数序列跑规则（连续低分 / 趋势下滑），
// 触发后调用 LLM 生成解读，写入 aiops_alerts（自建表，不进 Prometheus）。
// 幂等：alert_id 由规则 + 触发切面 + 触发窗口派生，重复评估不产生重复告警。

const (
	alertRuleConsecutiveLow = "consecutive-low-score"
	alertRuleTrendDrop      = "trend-drop"
)

// alertInterpretation 是告警的 AI 解读（LLM 或规则兜底）。
type alertInterpretation struct {
	Summary    string `json:"summary"`
	Analysis   string `json:"analysis"`
	Suggestion string `json:"suggestion"`
}

// alertSystemPrompt 警戒解读提示词。
const alertSystemPrompt = `你是 Kubernetes 调度实验的警戒解读器。给定触发条件与相关切面分数，
输出简短解读 JSON：{"summary":"不超过60字的一句话概括","analysis":"不超过100字的分析","suggestion":"不超过80字的建议"}
只输出 JSON。`

// evaluateAlerts 对最近分数序列跑规则；每个 tick 幂等执行。
func (service *Service) evaluateAlerts(ctx context.Context) error {
	sequence, err := service.scoreSequence(ctx, 6)
	if err != nil {
		return fmt.Errorf("load score sequence: %w", err)
	}
	if len(sequence) < service.config.AlertConsecutive {
		return nil
	}
	// 规则 1：连续 N 次 overall 低于阈值。
	if tail := consecutiveLowTail(sequence, service.config.AlertThreshold, service.config.AlertConsecutive); len(tail) > 0 {
		last := tail[len(tail)-1]
		if err := service.emitAlert(ctx, alertRuleConsecutiveLow, "warning", last.analysisID, tail); err != nil {
			return err
		}
	}
	// 规则 2：最近窗口平均分较前一窗口下滑超过 20%。
	if drop := trendDrop(sequence); drop != nil {
		if err := service.emitAlert(ctx, alertRuleTrendDrop, "warning", drop.last.analysisID, drop.items); err != nil {
			return err
		}
	}
	return nil
}

// scorePoint 是分数序列的一个点。
type scorePoint struct {
	at         time.Time
	overall    int
	analysisID string
}

// scoreSequence 取最近 windowSlots 个窗口内已完成切面的 overall 分数序列（时间升序）。
func (service *Service) scoreSequence(ctx context.Context, windowSlots int) ([]scorePoint, error) {
	granularity := service.config.WindowGranularity
	now := time.Now().UTC()
	start := now.Add(-time.Duration(windowSlots) * granularity)
	analyses, err := service.database.ListAIOpsAnalysesInWindow(ctx, start, now)
	if err != nil {
		return nil, err
	}
	sequence := make([]scorePoint, 0, len(analyses))
	for _, analysis := range analyses {
		var scores model.AIOpsScores
		if err := json.Unmarshal(analysis.Scores, &scores); err != nil || scores.Overall == 0 {
			continue
		}
		sequence = append(sequence, scorePoint{
			at: analysis.CreatedAt, overall: scores.Overall, analysisID: analysis.AnalysisID,
		})
	}
	return sequence, nil
}

// consecutiveLowTail 返回序列尾部满足"连续 N 次低于阈值"的最长连续低分段；不满足返回空。
func consecutiveLowTail(sequence []scorePoint, threshold, consecutive int) []scorePoint {
	tail := make([]scorePoint, 0)
	for i := len(sequence) - 1; i >= 0; i-- {
		if sequence[i].overall < threshold {
			tail = append([]scorePoint{sequence[i]}, tail...)
		} else {
			break
		}
	}
	if len(tail) < consecutive {
		return nil
	}
	return tail
}

// trendDrop 比较最近窗口与前一窗口的平均分；下滑超过 20% 返回相关序列，否则 nil。
func trendDrop(sequence []scorePoint) *trendDropResult {
	if len(sequence) < 4 {
		return nil
	}
	mid := len(sequence) / 2
	recent := averageOverall(sequence[mid:])
	previous := averageOverall(sequence[:mid])
	if previous <= 0 {
		return nil
	}
	if recent <= previous*8/10 {
		return &trendDropResult{last: sequence[len(sequence)-1], items: sequence}
	}
	return nil
}

type trendDropResult struct {
	last  scorePoint
	items []scorePoint
}

func averageOverall(sequence []scorePoint) int {
	if len(sequence) == 0 {
		return 0
	}
	total := 0
	for _, point := range sequence {
		total += point.overall
	}
	return total / len(sequence)
}

// emitAlert 生成并写入告警；alert_id 由规则 + 触发切面 + 触发时间窗口派生（幂等）。
func (service *Service) emitAlert(ctx context.Context, rule, severity, analysisID string, sequence []scorePoint) error {
	if analysisID == "" {
		analysisID = "unknown"
	}
	triggerWindow := sequence[len(sequence)-1].at.UTC().Truncate(service.config.WindowGranularity).Format("2006-01-02T15-04")
	alertID := alertID(rule, analysisID, triggerWindow)
	interpretation := alertInterpretation{
		Summary:    fmt.Sprintf("%d 次连续低分（阈值 %d，最近 %d 分）。", len(sequence), service.config.AlertThreshold, sequence[len(sequence)-1].overall),
		Analysis:   buildSequenceSummary(sequence),
		Suggestion: "建议检查对应切面的调度决策与流量配置。",
	}
	// LLM 生成解读；失败用规则文本兜底。
	payload, err := json.Marshal(sequence)
	if err == nil {
		if content, callErr := service.llm.CompleteJSON(ctx, alertSystemPrompt, string(payload), service.config.MaxTokensPerCall); callErr == nil {
			var parsed alertInterpretation
			if parseErr := json.Unmarshal([]byte(content), &parsed); parseErr == nil && parsed.Summary != "" {
				interpretation = parsed
			}
		}
	}
	alert := model.AIOpsAlert{
		AlertID:        alertID,
		Rule:           rule,
		Severity:       severity,
		TriggeredAt:    time.Now().UTC(),
		AnalysisID:     &analysisID,
		Interpretation: mustRaw(interpretation),
	}
	if err := service.database.CreateAIOpsAlert(ctx, alert); err != nil {
		return fmt.Errorf("create alert %s: %w", alertID, err)
	}
	service.logger.Info("AIOps alert triggered", "alertId", alertID, "rule", rule, "analysisId", analysisID)
	return nil
}

func buildSequenceSummary(sequence []scorePoint) string {
	parts := make([]string, 0, len(sequence))
	for _, point := range sequence {
		parts = append(parts, fmt.Sprintf("%d", point.overall))
	}
	return "分数序列：" + strings.Join(parts, " → ")
}

func alertID(rule, analysisID, triggerWindow string) string {
	sum := sha256.Sum256([]byte(rule + "|" + analysisID + "|" + triggerWindow))
	return "alert-" + hex.EncodeToString(sum[:8])
}
