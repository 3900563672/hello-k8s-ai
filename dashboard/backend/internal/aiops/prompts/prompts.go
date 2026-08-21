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
// ID + 版本 + 渲染函数；模板随代码走 git 可回滚。
type Definition struct {
	ID      string
	Version string
	name    string
	raw     string
}

// Prompt 是渲染完成的系统提示词快照（版本与哈希随调用日志记录，便于归因）。
type Prompt struct {
	ID      string
	Version string
	System  string
	Hash    string
}

// 各层提示词定义。版本号随内容演进：修改模板必须升级版本。
var (
	L1Entity            = mustLoad("l1-entity", "1.0.0", "templates/l1_entity.md")
	L2Scores            = mustLoad("l2-scores", "1.0.0", "templates/l2_scores.md")
	L3Window            = mustLoad("l3-window", "1.0.0", "templates/l3_window.md")
	L4Day               = mustLoad("l4-day", "1.0.0", "templates/l4_day.md")
	AlertInterpretation = mustLoad("alert-interpretation", "1.0.0", "templates/alert_interpretation.md")
	CommandIntent       = mustLoad("command-intent", "1.0.0", "templates/command_intent.md")
	ChatAssistant       = mustLoad("chat-assistant", "1.0.0", "templates/chat_assistant.md")
)

// All 返回全部已注册提示词定义（供文档与检查使用）。
func All() []Definition {
	return []Definition{L1Entity, L2Scores, L3Window, L4Day, AlertInterpretation, CommandIntent, ChatAssistant}
}

func mustLoad(id, version, path string) Definition {
	raw, err := templateFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("prompts: load %s: %v", path, err))
	}
	return Definition{ID: id, Version: version, name: path, raw: string(raw)}
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
		ID:      d.ID,
		Version: d.Version,
		System:  system,
		Hash:    hex.EncodeToString(sum[:8]),
	}, nil
}
