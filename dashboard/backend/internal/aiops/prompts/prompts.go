package prompts

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*.md
var templateFS embed.FS

// Definition 是一层提示词模板的不可变定义（#112 阶段 A）：
// ID + 版本 + 渲染函数 + 推荐温度；模板随代码走 git 可回滚。
// Temperature 为 0 时表示不设置（服务端默认）；分析类 0.1（可复现优先），对话类 0.5（自然度优先）。
type Definition struct {
	ID          string
	Version     string
	Temperature float64
	name        string
	raw         string
}

// Prompt 是渲染完成的系统提示词快照（版本/哈希/温度随调用日志记录，便于归因）。
type Prompt struct {
	ID          string
	Version     string
	System      string
	Hash        string
	Temperature float64
}

// 各层提示词定义。版本号随内容演进：修改模板必须升级版本。
// 温度：#112 期望「分析类 0-0.2（可复现优先），对话类 0.3-0.7（自然度优先）」。
var (
	L1Entity            = mustLoad("l1-entity", "1.0.0", 0.1, "templates/l1_entity.md")
	L2Scores            = mustLoad("l2-scores", "1.0.0", 0.1, "templates/l2_scores.md")
	L3Window            = mustLoad("l3-window", "1.0.0", 0.1, "templates/l3_window.md")
	L4Day               = mustLoad("l4-day", "1.0.0", 0.1, "templates/l4_day.md")
	AlertInterpretation = mustLoad("alert-interpretation", "1.0.0", 0.1, "templates/alert_interpretation.md")
	CommandIntent       = mustLoad("command-intent", "1.0.0", 0.1, "templates/command_intent.md")
	ChatAssistant       = mustLoad("chat-assistant", "1.0.0", 0.5, "templates/chat_assistant.md")
)

// All 返回全部已注册提示词定义（供文档与检查使用）。
func All() []Definition {
	return []Definition{L1Entity, L2Scores, L3Window, L4Day, AlertInterpretation, CommandIntent, ChatAssistant}
}

func mustLoad(id, version string, temperature float64, path string) Definition {
	raw, err := templateFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("prompts: load %s: %v", path, err))
	}
	return Definition{ID: id, Version: version, Temperature: temperature, name: path, raw: string(raw)}
}

// Render 渲染模板并返回带版本/哈希的提示词快照；data 为 nil 时直接使用原文。
func (d Definition) Render(data any) (Prompt, error) {
	system := d.raw
	if data != nil {
		tpl, err := template.New(d.ID).Parse(d.raw)
		if err != nil {
			return Prompt{}, fmt.Errorf("prompts: parse %s: %w", d.name, err)
		}
		var buffer strings.Builder
		if err := tpl.Execute(&buffer, data); err != nil {
			return Prompt{}, fmt.Errorf("prompts: render %s: %w", d.name, err)
		}
		system = buffer.String()
	}
	sum := sha256.Sum256([]byte(system))
	return Prompt{
		ID:          d.ID,
		Version:     d.Version,
		System:      system,
		Hash:        hex.EncodeToString(sum[:8]),
		Temperature: d.Temperature,
	}, nil
}
