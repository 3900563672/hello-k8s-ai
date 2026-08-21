package aiops

import (
	"context"
	"errors"
	"testing"
)

// fakeCommandLLM 返回固定内容或错误，用于 ParseCommand 测试。
type fakeCommandLLM struct {
	content string
	err     error
}

func (llm fakeCommandLLM) CompleteJSON(_ context.Context, _, _ string, _ int) (string, error) {
	return llm.content, llm.err
}

func (llm fakeCommandLLM) StreamComplete(_ context.Context, _ string, _ string, _ int, onDelta func(string), _ func(TokenUsage)) error {
	if llm.err != nil {
		return llm.err
	}
	onDelta(llm.content)
	return nil
}

func TestValidateCommandIntent(t *testing.T) {
	tests := []struct {
		name    string
		intent  *CommandIntent
		wantErr bool
	}{
		{name: "空意图拒绝", intent: nil, wantErr: true},
		{name: "目录内模板通过", intent: &CommandIntent{
			TemplateSelection: TemplateSelections{
				ModelIDs:        []string{"preset-model-standard"},
				TenantIDs:       []string{"preset-tenant-core"},
				TrafficIDs:      []string{"preset-traffic-spike"},
				OrchestratorIDs: []string{"preset-orchestrator-elastic"},
			},
		}, wantErr: false},
		{name: "编造模板 id 拒绝", intent: &CommandIntent{
			TemplateSelection: TemplateSelections{ModelIDs: []string{"preset-model-hacked"}},
		}, wantErr: true},
		{name: "空节点名拒绝", intent: &CommandIntent{
			TemplateSelection: TemplateSelections{NodeNames: []string{" "}},
		}, wantErr: true},
		{name: "负 QPS 拒绝", intent: &CommandIntent{
			Traffic: &TrafficIntent{QPS: intPointer(-1)},
		}, wantErr: true},
		{name: "负倍速拒绝", intent: &CommandIntent{
			Rate: intPointer(-1),
		}, wantErr: true},
		{name: "合法自由流量通过", intent: &CommandIntent{
			TargetTenant: "core",
			Traffic:      &TrafficIntent{QPS: intPointer(50)},
			Rate:         intPointer(4),
		}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommandIntent(tt.intent)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCommandIntent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseCommand(t *testing.T) {
	ctx := context.Background()
	t.Run("解析成功并落库", func(t *testing.T) {
		llm := fakeCommandLLM{content: `{"sceneTimeAnchor":"美国时间 09:00","durationMinutes":120,"sceneType":"突发流量高峰","targetTenant":"core","templateSelection":{"modelIds":["preset-model-standard"],"tenantIds":["preset-tenant-core"],"trafficIds":["preset-traffic-spike"]},"rate":4}`}
		intent, err := ParseCommand(ctx, "美国时间 9 点开始，持续 2 小时，突发流量高峰", llm, 2048)
		if err != nil {
			t.Fatalf("ParseCommand() error = %v", err)
		}
		if intent.SceneType != "突发流量高峰" || intent.DurationMinutes != 120 {
			t.Fatalf("unexpected intent: %+v", intent)
		}
		if len(intent.TemplateSelection.ModelIDs) != 1 {
			t.Fatalf("model selection missing: %+v", intent.TemplateSelection)
		}
	})
	t.Run("LLM 失败返回错误", func(t *testing.T) {
		_, err := ParseCommand(ctx, "anything", fakeCommandLLM{err: errors.New("upstream down")}, 2048)
		if err == nil {
			t.Fatal("expected error when LLM fails")
		}
	})
	t.Run("编造模板 id 被拒绝", func(t *testing.T) {
		llm := fakeCommandLLM{content: `{"templateSelection":{"modelIds":["preset-model-hacked"]}}`}
		_, err := ParseCommand(ctx, "越权", llm, 2048)
		if err == nil {
			t.Fatal("expected rejection for fabricated template id")
		}
	})
	t.Run("非法 JSON 被拒绝", func(t *testing.T) {
		llm := fakeCommandLLM{content: `not json`}
		_, err := ParseCommand(ctx, "坏输出", llm, 2048)
		if err == nil {
			t.Fatal("expected error for non-JSON output")
		}
	})
}

func intPointer(value int) *int {
	return &value
}
