package aiops

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops/prompts"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// llmOnlyService 构造只依赖 LLM 的 Service（schema 兜底链测试用）。
func llmOnlyService(llm LLM) *Service {
	return NewService(config.AIOpsConfig{MaxTokensPerCall: 500}, nil, llm, testLogger())
}

// TestValidateScores 校验 L2 分数契约：范围与枚举。
func TestValidateScores(t *testing.T) {
	valid := model.AIOpsScores{Goal: 80, Stability: 90, Efficiency: 70, Anomaly: 60, Overall: 75, Verdict: "success", Reason: "ok"}
	if err := validateScores(valid); err != nil {
		t.Fatalf("valid scores rejected: %v", err)
	}
	bad := valid
	bad.Overall = 120
	if err := validateScores(bad); err == nil {
		t.Fatalf("overall=120 should fail")
	}
	bad = valid
	bad.Verdict = "weird"
	if err := validateScores(bad); err == nil {
		t.Fatalf("verdict=weird should fail")
	}
	bad = valid
	bad.Reason = strings.Repeat("长", 121)
	if err := validateScores(bad); err == nil {
		t.Fatalf("reason over 120 should fail")
	}
}

// TestValidateEntityResults 校验 L1 输出契约：非空、枚举与长度。
func TestValidateEntityResults(t *testing.T) {
	valid := []l1EntityResult{{
		EntityKind: "Pod", EntityName: "pod-1", Classification: "problem",
		Phenomenon: "现象", Conclusion: "结论", IssueFlag: true,
	}}
	if err := validateEntityResults(valid); err != nil {
		t.Fatalf("valid L1 rejected: %v", err)
	}
	if err := validateEntityResults(nil); err == nil {
		t.Fatalf("empty L1 should fail")
	}
	bad := []l1EntityResult{{EntityKind: "Service", EntityName: "s", Classification: "healthy"}}
	if err := validateEntityResults(bad); err == nil {
		t.Fatalf("entityKind=Service should fail")
	}
	bad = []l1EntityResult{{EntityKind: "Pod", EntityName: "p", Classification: "unknown"}}
	if err := validateEntityResults(bad); err == nil {
		t.Fatalf("classification=unknown should fail")
	}
	bad = []l1EntityResult{{EntityKind: "Pod", EntityName: "p", Classification: "healthy", Phenomenon: strings.Repeat("现", 81)}}
	if err := validateEntityResults(bad); err == nil {
		t.Fatalf("phenomenon over 80 should fail")
	}
}

// TestValidateWindowAggregation 校验 L3/L4 聚合契约。
func TestValidateWindowAggregation(t *testing.T) {
	valid := windowAggregation{Overall: 75, Trend: "stable", CommonIssues: []string{"a"}, Situation: "态势", Recommendation: "建议"}
	if err := validateWindowAggregation(valid); err != nil {
		t.Fatalf("valid aggregation rejected: %v", err)
	}
	bad := valid
	bad.Trend = "unknown"
	if err := validateWindowAggregation(bad); err == nil {
		t.Fatalf("trend=unknown should fail")
	}
	bad = valid
	bad.CommonIssues = []string{"a", "b", "c", "d"}
	if err := validateWindowAggregation(bad); err == nil {
		t.Fatalf("4 issues should fail")
	}
	bad = valid
	bad.Overall = -1
	if err := validateWindowAggregation(bad); err == nil {
		t.Fatalf("overall=-1 should fail")
	}
}

// TestValidateAlertInterpretation 校验警戒解读契约。
func TestValidateAlertInterpretation(t *testing.T) {
	valid := alertInterpretation{Summary: "摘要", Analysis: "分析", Suggestion: "建议"}
	if err := validateAlertInterpretation(valid); err != nil {
		t.Fatalf("valid alert rejected: %v", err)
	}
	bad := valid
	bad.Summary = strings.Repeat("长", 61)
	if err := validateAlertInterpretation(bad); err == nil {
		t.Fatalf("summary over 60 should fail")
	}
	bad = valid
	bad.Summary = ""
	if err := validateAlertInterpretation(bad); err == nil {
		t.Fatalf("empty summary should fail")
	}
}

// TestCallStructuredRetryRecovers 验证兜底链：首次解析失败 → 重试一次成功。
func TestCallStructuredRetryRecovers(t *testing.T) {
	llm := newFakeLLM([]string{"not-json", `{"goal":80,"stability":90,"efficiency":70,"anomaly":60,"overall":75,"verdict":"success","reason":"ok"}`}, nil)
	service := llmOnlyService(llm)
	result, _, ok, reason := callStructured(context.Background(), service, prompts.L2Scores, "user",
		func(content string) (model.AIOpsScores, error) {
			var scores model.AIOpsScores
			err := json.Unmarshal([]byte(content), &scores)
			return scores, err
		}, validateScores)
	if !ok {
		t.Fatalf("expected recovered result, got reason=%s", reason)
	}
	if result.Overall != 75 {
		t.Fatalf("overall = %d, want 75", result.Overall)
	}
	if !strings.Contains(reason, "retry_recovered") {
		t.Fatalf("reason should mention retry_recovered, got %s", reason)
	}
	if llm.callCount() != 2 {
		t.Fatalf("expected 2 calls, got %d", llm.callCount())
	}
}

// TestCallStructuredFallsBack 验证兜底链：两次都失败 → ok=false，原因可见。
func TestCallStructuredFallsBack(t *testing.T) {
	llm := newFakeLLM([]string{"bad", "worse"}, nil)
	service := llmOnlyService(llm)
	_, _, ok, reason := callStructured(context.Background(), service, prompts.L2Scores, "user",
		func(content string) (model.AIOpsScores, error) {
			var scores model.AIOpsScores
			err := json.Unmarshal([]byte(content), &scores)
			return scores, err
		}, validateScores)
	if ok {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(reason, "retry failed") {
		t.Fatalf("reason should mention retry failed, got %s", reason)
	}
}

// TestTruncateSummaries 验证 L2 输入裁剪：先丢现象，再截结论。
func TestTruncateSummaries(t *testing.T) {
	summaries := []model.AIOpsEntitySummary{
		{Phenomenon: strings.Repeat("现", 50), Conclusion: strings.Repeat("结", 50), Classification: "problem"},
		{Phenomenon: strings.Repeat("现", 50), Conclusion: strings.Repeat("结", 50), Classification: "healthy"},
	}
	trimmed, truncated := truncateSummaries(summaries, 100, testLogger())
	if !truncated {
		t.Fatalf("expected truncation")
	}
	if len(trimmed) != 2 {
		t.Fatalf("entity count changed: %d", len(trimmed))
	}
	for _, item := range trimmed {
		if item.Phenomenon != "" {
			t.Fatalf("phenomenon should be dropped, got %q", item.Phenomenon)
		}
	}
	if trimmed[0].Classification != "problem" {
		t.Fatalf("classification should be preserved")
	}
	trimmed, truncated = truncateSummaries(summaries, 50, testLogger())
	if !truncated || trimmed[0].Conclusion == summaries[0].Conclusion {
		t.Fatalf("conclusion should be capped under tiny budget")
	}
}

// TestTruncateChatContext 验证对话上下文预算截断。
func TestTruncateChatContext(t *testing.T) {
	short := "短"
	if text, truncated := truncateChatContext(short, 10); truncated || text != short {
		t.Fatalf("short context should not truncate")
	}
	long := strings.Repeat("长", 100)
	text, truncated := truncateChatContext(long, 50)
	if !truncated {
		t.Fatalf("long context should truncate")
	}
	if len([]rune(text)) > 60 {
		t.Fatalf("truncated text too long: %d", len([]rune(text)))
	}
	if !strings.Contains(text, "已裁剪") {
		t.Fatalf("truncation marker missing")
	}
}

// TestTrimChildLists 验证 L3/L4 输入窗口数上限（保留最近 N 个）。
func TestTrimChildLists(t *testing.T) {
	items := []l3Child{{AnalysisID: "a"}, {AnalysisID: "b"}, {AnalysisID: "c"}}
	trimmed := trimChildLists(items, 2)
	if len(trimmed) != 2 || trimmed[1].AnalysisID != "c" {
		t.Fatalf("should keep latest 2, got %+v", trimmed)
	}
	if len(trimChildLists(items, 10)) != 3 {
		t.Fatalf("within budget should keep all")
	}
}
