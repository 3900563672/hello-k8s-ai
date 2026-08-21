package prompts

import (
	"strings"
	"testing"
)

// TestAllDefinitions 验证全部模板可加载且渲染哈希稳定（同一模板两次渲染哈希一致）。
func TestAllDefinitions(t *testing.T) {
	definitions := All()
	if len(definitions) != 7 {
		t.Fatalf("expected 7 definitions, got %d", len(definitions))
	}
	for _, definition := range definitions {
		if definition.ID == "" || definition.Version == "" {
			t.Fatalf("definition %+v missing id or version", definition)
		}
		if !strings.Contains(definition.raw, "你") {
			t.Fatalf("definition %s raw content looks empty", definition.ID)
		}
		first, err := definition.Render(nil)
		if err != nil {
			t.Fatalf("render %s: %v", definition.ID, err)
		}
		second, err := definition.Render(nil)
		if err != nil {
			t.Fatalf("render %s again: %v", definition.ID, err)
		}
		if first.Hash != second.Hash {
			t.Fatalf("hash unstable for %s: %s vs %s", definition.ID, first.Hash, second.Hash)
		}
		if first.Version != definition.Version {
			t.Fatalf("version mismatch for %s", definition.ID)
		}
	}
}

// TestCommandIntentRender 验证命令模板支持目录注入（{{ .Catalog }} 占位符）。
func TestCommandIntentRender(t *testing.T) {
	prompt, err := CommandIntent.Render(map[string]any{"Catalog": "- tpl-a（示例）：测试"})
	if err != nil {
		t.Fatalf("render command intent: %v", err)
	}
	if !strings.Contains(prompt.System, "- tpl-a（示例）：测试") {
		t.Fatalf("catalog not injected: %s", prompt.System)
	}
	if prompt.Hash == "" || prompt.Version == "" {
		t.Fatalf("prompt metadata missing: %+v", prompt)
	}
}
