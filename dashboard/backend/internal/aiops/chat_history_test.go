package aiops

import (
	"context"
	"testing"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

func TestChatHistory(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	ctx := context.Background()
	service.ChatRecord(ctx, "session-a", "问题一", "回答一", ChatContextRefs{})
	service.ChatRecord(ctx, "session-a", "问题二", "回答二", ChatContextRefs{WindowIDs: []string{"w-1"}})
	service.ChatRecord(ctx, "session-b", "别处问题", "别处回答", ChatContextRefs{})

	messages, err := service.ChatHistory(ctx, "session-a", 50)
	if err != nil {
		t.Fatalf("chat history: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("history len = %d, want 4", len(messages))
	}
	wantRoles := []string{"user", "assistant", "user", "assistant"}
	for i, want := range wantRoles {
		if messages[i].Role != want {
			t.Fatalf("message %d role = %q, want %q", i, messages[i].Role, want)
		}
	}
	if messages[0].Content != "问题一" || messages[3].Content != "回答二" {
		t.Fatalf("unexpected history order: %+v", messages)
	}

	limited, err := service.ChatHistory(ctx, "session-a", 2)
	if err != nil {
		t.Fatalf("chat history with limit: %v", err)
	}
	if len(limited) != 2 || limited[0].Content != "问题二" || limited[1].Content != "回答二" {
		t.Fatalf("limited history = %+v, want last 2 of session-a", limited)
	}

	if _, err := service.ChatHistory(ctx, "", 50); err == nil {
		t.Fatal("empty sessionId should be rejected")
	}

	other, err := service.ChatHistory(ctx, "session-b", 50)
	if err != nil {
		t.Fatalf("chat history of other session: %v", err)
	}
	if len(other) != 2 || other[0].Content != "别处问题" {
		t.Fatalf("other session history = %+v", other)
	}

	empty, err := service.ChatHistory(ctx, "no-such-session", 50)
	if err != nil {
		t.Fatalf("chat history of unknown session: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("unknown session history len = %d, want 0", len(empty))
	}
}
