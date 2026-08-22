package aiops

import (
	"context"
	"testing"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

// TestLimitsDefaults 验证意图执行硬限制与能力开关（#134 前端提示条单一事实源）。
func TestLimitsDefaults(t *testing.T) {
	limits := Limits()
	if limits.MaxTrafficQPS != MaxTrafficQPS {
		t.Fatalf("MaxTrafficQPS = %d, want %d", limits.MaxTrafficQPS, MaxTrafficQPS)
	}
	if limits.MaxSimulationRate != MaxSimulationRate {
		t.Fatalf("MaxSimulationRate = %d, want %d", limits.MaxSimulationRate, MaxSimulationRate)
	}
	if len(limits.TrafficShapes) != 4 {
		t.Fatalf("TrafficShapes = %v, want 4 种形状", limits.TrafficShapes)
	}
	if !limits.UnlimitedDuration || !limits.SupportsStop || !limits.TrafficRequiresTenant {
		t.Fatal("能力开关应为 true（时长自由/可停止/流量需租户）")
	}
	if limits.DefaultRate != 1 || limits.DefaultPeakQPS != DefaultPeakQPS {
		t.Fatalf("默认倍速/峰值错误: rate=%d peak=%d", limits.DefaultRate, limits.DefaultPeakQPS)
	}
}

// TestServiceToggleAndSettings 验证运行时开关与设置掩码（面板写入，服务端内存态）。
func TestServiceToggleAndSettings(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{Model: "deepseek-v4-flash"})
	if !service.Enabled() {
		t.Fatal("初始应开启（部署级默认启用）")
	}
	service.SetEnabled(false)
	if service.Enabled() {
		t.Fatal("SetEnabled(false) 后应关闭")
	}
	service.SetEnabled(true)
	if !service.Enabled() {
		t.Fatal("SetEnabled(true) 后应开启")
	}
	state := service.Settings()
	if !state.Enabled || state.Model != "deepseek-v4-flash" {
		t.Fatalf("Settings 异常: %+v", state)
	}
	service.SetEnabled(false)
	if service.Enabled() {
		t.Fatal("SetEnabled(false) 后应关闭")
	}
}

// TestServiceSettingsKeyConfigured 验证配置 Key 后掩码状态。
func TestServiceSettingsKeyConfigured(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{OpenAIAPIKey: "sk-test"})
	state := service.Settings()
	if !state.KeyConfigured || !state.Configured {
		t.Fatalf("配置 Key 后 KeyConfigured/Configured 应为 true: %+v", state)
	}
}

// TestServiceQuotaStatusDisabled 验证未配置配额时 Enabled=false。
func TestServiceQuotaStatusDisabled(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	status, err := service.QuotaStatus(context.Background())
	if err != nil {
		t.Fatalf("QuotaStatus: %v", err)
	}
	if status.Enabled {
		t.Fatal("无配额配置时 Enabled 应为 false")
	}
}

// TestServiceQuotaStatusEnabled 验证配额用量回显（#134 面板可见）。
func TestServiceQuotaStatusEnabled(t *testing.T) {
	database := newFakeStore(nil)
	database.usageCalls = 12
	database.usageTokens = 3456
	service := NewService(config.AIOpsConfig{DailyMaxCalls: 100, DailyMaxTokens: 2000000}, database, newFakeLLM(nil, nil), testLogger())
	status, err := service.QuotaStatus(context.Background())
	if err != nil {
		t.Fatalf("QuotaStatus: %v", err)
	}
	if !status.Enabled || status.CallsUsed != 12 || status.CallsMax != 100 ||
		status.TokensUsed != 3456 || status.TokensMax != 2000000 {
		t.Fatalf("QuotaStatus 异常: %+v", status)
	}
}

// TestChatAllowedModels 验证模型白名单与默认回退。
func TestChatAllowedModels(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{Model: "deepseek-v4-flash"})
	models := service.ChatAllowedModels()
	if len(models) != 1 || models[0] != "deepseek-v4-flash" {
		t.Fatalf("默认白名单异常: %v", models)
	}
	service.config.ChatModels = []string{"model-a", "model-b"}
	models = service.ChatAllowedModels()
	if len(models) != 2 || models[0] != "model-a" || models[1] != "model-b" {
		t.Fatalf("白名单覆盖异常: %v", models)
	}
}

// TestSummarizeBudgetAndTrim 验证预算汇总结构与子列表裁剪。
func TestSummarizeBudgetAndTrim(t *testing.T) {
	budget := summarizeBudget()
	if len(budget) != 5 {
		t.Fatalf("summarizeBudget 应有 5 项: %v", budget)
	}
	items := trimChildLists([]int{1, 2, 3}, 2)
	if len(items) != 2 || items[0] != 2 || items[1] != 3 {
		t.Fatalf("trimChildLists 应保留最近 N 个: %v", items)
	}
	if got := trimChildLists([]int{1, 2}, 5); len(got) != 2 {
		t.Fatalf("不足预算不应裁剪: %v", got)
	}
}

// TestServiceParseCommand 验证 Service 层意图解析入口（LLM + 目录校验）。
func TestServiceParseCommand(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	service.llm = fakeCommandLLM{content: `{"sceneType":"潮汐流量","targetTenant":"preset-tenant-001","traffic":{"shape":"tidal","peakQps":50,"periodMinutes":30},"rate":20}`}
	intent, err := service.ParseCommand(context.Background(), "2 小时潮汐流量，峰值 50，倍速 20")
	if err != nil {
		t.Fatalf("ParseCommand: %v", err)
	}
	if intent == nil || intent.TargetTenant != "preset-tenant-001" || intent.Rate == nil || *intent.Rate != 20 {
		t.Fatalf("解析结果异常: %+v", intent)
	}
	if intent.Traffic == nil || intent.Traffic.PeakQPS == nil || *intent.Traffic.PeakQPS != 50 {
		t.Fatalf("流量意图异常: %+v", intent.Traffic)
	}
}
