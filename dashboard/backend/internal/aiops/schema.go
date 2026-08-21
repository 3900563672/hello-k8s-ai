package aiops

import (
	"context"
	"fmt"
	"strings"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops/prompts"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// 本文件实现 #112 阶段 A：输出 Schema 运行时校验 + 兜底链。
// 每层输出定义 Go struct + 手写 validate；解析/校验失败重试一次，仍失败由调用方规则兜底，
// 失败原因统一记日志（含提示词版本与哈希），不直接 failed 浪费整轮。

func runeLen(text string) int {
	return len([]rune(text))
}

// validateEntityResult 校验 L1 单实体结果：枚举与长度约束（与提示词契约一致）。
func validateEntityResult(result l1EntityResult) error {
	if strings.TrimSpace(result.EntityName) == "" {
		return fmt.Errorf("entityName 为空")
	}
	switch result.EntityKind {
	case "Pod", "Node", "Tenant":
	default:
		return fmt.Errorf("entityKind %q 不在枚举内", result.EntityKind)
	}
	switch result.Classification {
	case string(model.AIOpsHealthy), string(model.AIOpsSuspect), string(model.AIOpsProblem):
	default:
		return fmt.Errorf("classification %q 不在枚举内", result.Classification)
	}
	if runeLen(result.Phenomenon) > 80 {
		return fmt.Errorf("phenomenon 超过 80 字")
	}
	if runeLen(result.Conclusion) > 80 {
		return fmt.Errorf("conclusion 超过 80 字")
	}
	return nil
}

// validateEntityResults 校验 L1 整批输出：非空 + 逐元素校验。
func validateEntityResults(results []l1EntityResult) error {
	if len(results) == 0 {
		return fmt.Errorf("L1 结果为空数组")
	}
	for i, result := range results {
		if err := validateEntityResult(result); err != nil {
			return fmt.Errorf("第 %d 个实体: %w", i, err)
		}
	}
	return nil
}

// validateScores 校验 L2 分数输出：0-100 范围与 verdict 枚举。
func validateScores(scores model.AIOpsScores) error {
	for name, value := range map[string]int{
		"goal":       scores.Goal,
		"stability":  scores.Stability,
		"efficiency": scores.Efficiency,
		"anomaly":    scores.Anomaly,
		"overall":    scores.Overall,
	} {
		if value < 0 || value > 100 {
			return fmt.Errorf("%s=%d 超出 0-100", name, value)
		}
	}
	switch scores.Verdict {
	case "success", "attention", "problem":
	default:
		return fmt.Errorf("verdict %q 不在枚举内", scores.Verdict)
	}
	if runeLen(scores.Reason) > 120 {
		return fmt.Errorf("reason 超过 120 字")
	}
	return nil
}

// validateWindowAggregation 校验 L3/L4 聚合输出：分数范围、趋势枚举与长度约束。
func validateWindowAggregation(aggregation windowAggregation) error {
	if aggregation.Overall < 0 || aggregation.Overall > 100 {
		return fmt.Errorf("overall=%d 超出 0-100", aggregation.Overall)
	}
	switch aggregation.Trend {
	case "improving", "stable", "degrading":
	default:
		return fmt.Errorf("trend %q 不在枚举内", aggregation.Trend)
	}
	if len(aggregation.CommonIssues) > 3 {
		return fmt.Errorf("commonIssues 超过 3 条")
	}
	for _, issue := range aggregation.CommonIssues {
		if runeLen(issue) > 60 {
			return fmt.Errorf("commonIssue 超过 60 字")
		}
	}
	if runeLen(aggregation.Situation) > 150 {
		return fmt.Errorf("situation 超过 150 字")
	}
	if runeLen(aggregation.Recommendation) > 150 {
		return fmt.Errorf("recommendation 超过 150 字")
	}
	return nil
}

// validateAlertInterpretation 校验警戒解读输出：非空与长度约束。
func validateAlertInterpretation(interpretation alertInterpretation) error {
	if runeLen(interpretation.Summary) == 0 || runeLen(interpretation.Summary) > 60 {
		return fmt.Errorf("summary 为空或超过 60 字")
	}
	if runeLen(interpretation.Analysis) > 100 {
		return fmt.Errorf("analysis 超过 100 字")
	}
	if runeLen(interpretation.Suggestion) > 80 {
		return fmt.Errorf("suggestion 超过 80 字")
	}
	return nil
}

// callStructured 调用 LLM 并做运行时 schema 校验（#112）：解析/校验失败重试一次。
// 返回（结果, token 用量, 是否通过, 失败原因）；失败原因供调用方日志与规则兜底。
func callStructured[T any](ctx context.Context, service *Service, definition prompts.Definition, user string,
	decode func(string) (T, error), validate func(T) error) (T, TokenUsage, bool, string) {
	var zero T
	attempt := func() (T, TokenUsage, error) {
		prompt, err := definition.Render(nil)
		if err != nil {
			return zero, TokenUsage{}, fmt.Errorf("render prompt: %w", err)
		}
		completion, err := service.llm.CompleteJSON(ctx, prompt.System, user, service.config.MaxTokensPerCall, prompt.Temperature)
		if err != nil {
			return zero, TokenUsage{}, err
		}
		parsed, err := decode(completion.Content)
		if err != nil {
			return zero, completion.Usage, err
		}
		if err := validate(parsed); err != nil {
			return zero, completion.Usage, err
		}
		return parsed, completion.Usage, nil
	}
	value, usage, err := attempt()
	if err == nil {
		prompt, _ := definition.Render(nil)
		service.recordTokenUsage(prompt, usage)
		return value, usage, true, ""
	}
	reason := err.Error()
	service.logger.Warn("AIOps schema validation failed, retrying once",
		"layer", definition.ID, "version", definition.Version, "error", err)
	value, usage, err = attempt()
	if err == nil {
		return value, usage, true, "retry_recovered: " + reason
	}
	return zero, usage, false, reason + "; retry failed: " + err.Error()
}

// recordTokenUsage 记录一次 LLM 调用的 token 用量与提示词版本/哈希（#112 预算校准与归因）。
func (service *Service) recordTokenUsage(prompt prompts.Prompt, usage TokenUsage) {
	service.logger.Info("AIOps LLM call usage",
		"layer", prompt.ID, "promptId", prompt.ID, "promptVersion", prompt.Version, "promptHash", prompt.Hash,
		"promptTokens", usage.PromptTokens, "completionTokens", usage.CompletionTokens)
}
