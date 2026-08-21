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

func (llm fakeCommandLLM) CompleteJSON(_ context.Context, _, _ string, _ int, _ float64) (Completion, error) {
	return Completion{Content: llm.content}, llm.err
}

func (llm fakeCommandLLM) StreamComplete(_ context.Context, _ string, _ string, _ int, _ float64, onDelta func(string), _ func(TokenUsage)) error {
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
				ModelIDs:        []string{"preset-model-002"},
				TenantIDs:       []string{"preset-tenant-001"},
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
		{name: "潮汐流量通过", intent: &CommandIntent{
			TargetTenant: "preset-tenant-001",
			Traffic:      &TrafficIntent{Shape: string(TrafficShapeTidal), PeakQPS: intPointer(50), PeriodMinutes: intPointer(30)},
			Rate:         intPointer(20),
		}, wantErr: false},
		{name: "固定 QPS 超上限由归一化钳制（#134）", intent: &CommandIntent{
			TargetTenant: "core",
			Traffic:      &TrafficIntent{QPS: intPointer(MaxTrafficQPS + 1)},
		}, wantErr: false},
		{name: "峰值 QPS 超上限由归一化钳制（#134）", intent: &CommandIntent{
			TargetTenant: "core",
			Traffic:      &TrafficIntent{Shape: string(TrafficShapeTidal), PeakQPS: intPointer(MaxTrafficQPS + 1)},
		}, wantErr: false},
		{name: "潮汐缺 peakQps 由归一化补默认（#134）", intent: &CommandIntent{
			TargetTenant: "core",
			Traffic:      &TrafficIntent{Shape: string(TrafficShapeTidal)},
		}, wantErr: false},
		{name: "非法形状拒绝", intent: &CommandIntent{
			TargetTenant: "core",
			Traffic:      &TrafficIntent{Shape: "moon", PeakQPS: intPointer(20)},
		}, wantErr: true},
		{name: "写流量缺租户拒绝", intent: &CommandIntent{
			Traffic: &TrafficIntent{QPS: intPointer(20)},
		}, wantErr: true},
		{name: "倍速超上限由归一化钳制（#134）", intent: &CommandIntent{
			Rate: intPointer(MaxSimulationRate + 1),
		}, wantErr: false},
		{name: "倍速为零拒绝", intent: &CommandIntent{
			Rate: intPointer(0),
		}, wantErr: true},
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

func TestNormalizeTrafficIntent(t *testing.T) {
	tests := []struct {
		name         string
		intent       *CommandIntent
		wantPeak     int
		wantRate     int
		wantDuration int
		wantReason   string
		wantCurve    bool
	}{
		{name: "无流量返回 nil", intent: &CommandIntent{}, wantCurve: false},
		{name: "稳态缺省补默认 20/倍速 1", intent: &CommandIntent{
			TargetTenant: "core", Traffic: &TrafficIntent{},
		}, wantPeak: 20, wantRate: 1, wantReason: "defaulted", wantCurve: true},
		{name: "超上限钳制 500→200", intent: &CommandIntent{
			TargetTenant: "core", Traffic: &TrafficIntent{Shape: string(TrafficShapeTidal), PeakQPS: intPointer(500)},
		}, wantPeak: MaxTrafficQPS, wantRate: 1, wantReason: "clamped-to-max", wantCurve: true},
		{name: "非稳态缺时长补一个周期", intent: &CommandIntent{
			TargetTenant: "core", Traffic: &TrafficIntent{Shape: string(TrafficShapeTidal), PeakQPS: intPointer(50)},
		}, wantPeak: 50, wantRate: 1, wantDuration: DefaultTidalPeriod, wantCurve: true},
		{name: "倍速超上限钳制 500→100", intent: &CommandIntent{
			TargetTenant: "core", Rate: intPointer(500),
			Traffic: &TrafficIntent{Shape: string(TrafficShapeTidal), PeakQPS: intPointer(50)},
		}, wantPeak: 50, wantRate: MaxSimulationRate, wantReason: "clamped-to-max", wantCurve: true},
		{name: "2 小时潮汐曲线覆盖全程", intent: &CommandIntent{
			TargetTenant: "preset-tenant-001", DurationMinutes: 120,
			Traffic: &TrafficIntent{Shape: string(TrafficShapeTidal), PeakQPS: intPointer(50), PeriodMinutes: intPointer(30)},
			Rate:    intPointer(20),
		}, wantPeak: 50, wantRate: 20, wantDuration: 120, wantCurve: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applied, err := NormalizeTrafficIntent(tt.intent)
			if err != nil {
				t.Fatalf("NormalizeTrafficIntent() error = %v", err)
			}
			if !tt.wantCurve {
				if applied != nil {
					t.Fatalf("NormalizeTrafficIntent() = %v, want nil", applied)
				}
				return
			}
			if applied == nil {
				t.Fatal("NormalizeTrafficIntent() = nil, want applied")
			}
			expectedEnd := tt.intent.DurationMinutes * 60
			if expectedEnd <= 0 {
				expectedEnd = DefaultTidalPeriod * 60 // 未给时长时曲线预览至少一个周期
			}
			if got := applied.Curve[len(applied.Curve)-1].X; got != expectedEnd {
				t.Errorf("curve 终点 = %d 秒, want %d 秒（覆盖全程）", got, expectedEnd)
			}
			if tt.wantReason != "" {
				found := false
				for _, value := range applied.Values {
					if value.Reason == tt.wantReason {
						found = true
					}
				}
				if !found {
					t.Errorf("applied.Values 缺少 reason=%q：%+v", tt.wantReason, applied.Values)
				}
			}
		})
	}
}

func TestGenerateTrafficCurve(t *testing.T) {
	curve := GenerateTrafficCurve(TrafficShapeTidal, 50, 30, 120)
	if len(curve) < 10 {
		t.Fatalf("2 小时潮汐曲线点数过少：%d", len(curve))
	}
	if curve[0].X != 0 || curve[len(curve)-1].X != 7200 {
		t.Fatalf("曲线范围应为 0..7200 秒，实际 %d..%d", curve[0].X, curve[len(curve)-1].X)
	}
	for _, point := range curve {
		if point.Y < 1 || point.Y > 50 {
			t.Errorf("曲线点越界 (%ds, %dQPS)，应在 1..50", point.X, point.Y)
		}
	}
	// 潮汐必须有起伏（不是平线，#134 根因 1）
	minValue, maxValue := curve[0].Y, curve[0].Y
	for _, point := range curve[1:] {
		if point.Y < minValue {
			minValue = point.Y
		}
		if point.Y > maxValue {
			maxValue = point.Y
		}
	}
	if minValue == maxValue {
		t.Error("潮汐曲线是平线（min==max），波形生成失败")
	}
	// 长时长自适应粒度：24h+ 用 30 分钟粒度，点数有界
	longCurve := GenerateTrafficCurve(TrafficShapeTidal, 50, 30, 72*60)
	if len(longCurve) > 200 {
		t.Errorf("72 小时曲线点数过多：%d", len(longCurve))
	}
}

func TestParseCommand(t *testing.T) {
	ctx := context.Background()
	t.Run("解析成功并落库", func(t *testing.T) {
		llm := fakeCommandLLM{content: `{"sceneTimeAnchor":"美国时间 09:00","durationMinutes":120,"sceneType":"突发流量高峰","targetTenant":"core","templateSelection":{"modelIds":["preset-model-002"],"tenantIds":["preset-tenant-001"],"trafficIds":["preset-traffic-spike"]},"rate":4}`}
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

func TestTrafficShapeQPS(t *testing.T) {
	// 平稳：恒为峰值
	if got := TrafficShapeQPS(10, TrafficShapeSteady, 50, 30); got != 50 {
		t.Fatalf("steady qps = %d, want 50", got)
	}
	// 潮汐：值域在 [base, peak] 内，且周期 30 分钟整周期回到起点附近
	peak := 60
	base := peak / 5
	for minute := 0.0; minute < 120; minute += 5 {
		got := TrafficShapeQPS(minute, TrafficShapeTidal, peak, 30)
		if got < base || got > peak {
			t.Fatalf("tidal qps at %v = %d, out of range [%d, %d]", minute, got, base, peak)
		}
	}
	start := TrafficShapeQPS(0, TrafficShapeTidal, peak, 30)
	afterPeriod := TrafficShapeQPS(30, TrafficShapeTidal, peak, 30)
	if start != afterPeriod {
		t.Fatalf("tidal not periodic: t0=%d t30=%d", start, afterPeriod)
	}
	// 斜坡：单调不降，到达周期后维持峰值
	prev := -1
	for minute := 0.0; minute <= 30; minute += 5 {
		got := TrafficShapeQPS(minute, TrafficShapeRamp, peak, 30)
		if got < prev {
			t.Fatalf("ramp decreased at %v: %d < %d", minute, got, prev)
		}
		prev = got
	}
	if got := TrafficShapeQPS(31, TrafficShapeRamp, peak, 30); got != peak {
		t.Fatalf("ramp hold = %d, want %d", got, peak)
	}
	// 脉冲：前 80% 低水位，后 20% 峰值
	if got := TrafficShapeQPS(10, TrafficShapeSpike, peak, 30); got != base {
		t.Fatalf("spike low = %d, want %d", got, base)
	}
	if got := TrafficShapeQPS(25, TrafficShapeSpike, peak, 30); got != peak {
		t.Fatalf("spike high = %d, want %d", got, peak)
	}
	// 非法峰值：返回 0
	if got := TrafficShapeQPS(0, TrafficShapeTidal, 0, 30); got != 0 {
		t.Fatalf("zero peak = %d, want 0", got)
	}
}
