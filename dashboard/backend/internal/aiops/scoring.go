package aiops

import (
	"math"
	"strings"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// hardMetrics 是从切面子表算出的确定性硬指标（L2 混合打分：规则先行，LLM 只做判断）。
type hardMetrics struct {
	ErrorRateAvg  float64 `json:"errorRateAvg"`
	ErrorRateMax  float64 `json:"errorRateMax"`
	TTFTP95MaxMS  float64 `json:"ttftP95MaxMs"`
	QPSAvg        float64 `json:"qpsAvg"`
	QPSTarget     float64 `json:"qpsTarget"`
	RestartCount  int     `json:"restartCount"`
	DecisionCount int     `json:"decisionCount"`
	BurstCount    int     `json:"burstCount"`
	ErrorCount    int     `json:"errorCount"`
	GapCount      int     `json:"gapCount"`
	EventTotal    int     `json:"eventTotal"`
	TraceCount    int     `json:"traceCount"`
}

func isErrorRateMetric(name string) bool {
	return name == "simulator.errorRate" || name == "controller.errorRate"
}

// computeHardMetrics 聚合切面事件、指标桶与 Trace 数为硬指标。
func computeHardMetrics(events []model.SegmentEvent, metrics []model.MetricBucket, traces []model.TraceSummary) hardMetrics {
	hard := hardMetrics{}
	for _, event := range events {
		hard.EventTotal++
		switch event.EventType {
		case model.SegmentEventDecision:
			hard.DecisionCount++
		case model.SegmentEventBurst:
			hard.BurstCount++
		case model.SegmentEventError:
			hard.ErrorCount++
		case model.SegmentEventGap:
			hard.GapCount++
		}
	}
	errorSum, errorBuckets, qpsSum, qpsBuckets, ttftP95Max := 0.0, 0, 0.0, 0, 0.0
	for _, bucket := range metrics {
		switch {
		case isErrorRateMetric(bucket.MetricName):
			errorSum += bucket.Avg
			errorBuckets++
			if bucket.Max > hard.ErrorRateMax {
				hard.ErrorRateMax = bucket.Max
			}
		case bucket.MetricName == "simulator.qps":
			qpsSum += bucket.Avg
			qpsBuckets++
		case bucket.MetricName == "simulator.ttft":
			if bucket.P95 > ttftP95Max {
				ttftP95Max = bucket.P95
			}
		}
	}
	if errorBuckets > 0 {
		hard.ErrorRateAvg = errorSum / float64(errorBuckets)
	}
	if qpsBuckets > 0 {
		hard.QPSAvg = qpsSum / float64(qpsBuckets)
	}
	hard.TTFTP95MaxMS = ttftP95Max
	hard.TraceCount = len(traces)
	return hard
}

// fallbackScores 在 LLM 不可用/预算耗尽时给出规则分兜底（保证分析不阻塞）。
func fallbackScores(hard hardMetrics) model.AIOpsScores {
	goal := 80.0
	if hard.QPSTarget > 0 {
		ratio := hard.QPSAvg / hard.QPSTarget
		goal = math.Min(100, 100*ratio)
	}
	stability := 100 - hard.ErrorRateAvg*2000 - float64(hard.RestartCount)*8 - float64(hard.BurstCount)*10
	efficiency := 100 - float64(hard.DecisionCount)*3
	anomaly := 100 - float64(hard.ErrorCount)*20 - float64(hard.GapCount)*10 - float64(hard.RestartCount)*15
	clamp := func(value float64) int {
		if value < 0 {
			return 0
		}
		if value > 100 {
			return 100
		}
		return int(math.Round(value))
	}
	scores := model.AIOpsScores{
		Goal:       clamp(goal),
		Stability:  clamp(stability),
		Efficiency: clamp(efficiency),
		Anomaly:    clamp(anomaly),
	}
	scores.Overall = clamp(0.35*float64(scores.Goal) + 0.30*float64(scores.Stability) +
		0.15*float64(scores.Efficiency) + 0.20*float64(scores.Anomaly))
	switch {
	case scores.Overall >= 80:
		scores.Verdict = "success"
	case scores.Overall >= 60:
		scores.Verdict = "attention"
	default:
		scores.Verdict = "problem"
	}
	scores.Reason = "规则分兜底：" + ruleReason(hard)
	return scores
}

func ruleReason(hard hardMetrics) string {
	var parts []string
	if hard.QPSTarget > 0 && hard.QPSAvg < hard.QPSTarget*0.8 {
		parts = append(parts, "目标 QPS 达成率偏低")
	}
	if hard.ErrorRateAvg > 0.05 {
		parts = append(parts, "错误率超过阈值")
	}
	if hard.ErrorCount > 0 {
		parts = append(parts, "存在错误事件")
	}
	if hard.BurstCount > 0 {
		parts = append(parts, "副本数快速变化")
	}
	if hard.RestartCount > 0 {
		parts = append(parts, "存在容器重启")
	}
	if len(parts) == 0 {
		parts = append(parts, "未见明显异常")
	}
	return strings.Join(parts, "；")
}

// normalizeScores 收敛 LLM 返回的分数（越界钳制、verdict 规范化）。
func normalizeScores(scores model.AIOpsScores) model.AIOpsScores {
	clamp := func(value int) int {
		if value < 0 {
			return 0
		}
		if value > 100 {
			return 100
		}
		return value
	}
	scores.Goal = clamp(scores.Goal)
	scores.Stability = clamp(scores.Stability)
	scores.Efficiency = clamp(scores.Efficiency)
	scores.Anomaly = clamp(scores.Anomaly)
	scores.Overall = clamp(scores.Overall)
	switch scores.Verdict {
	case "success", "attention", "problem":
	default:
		scores.Verdict = "attention"
	}
	return scores
}
