package aiops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

// LLM 是分析 worker 依赖的模型接口，便于单元测试注入假模型。
type LLM interface {
	// CompleteJSON 调用模型并返回纯 JSON 文本（响应格式强制 json_object）。
	CompleteJSON(ctx context.Context, system, user string, maxTokens int) (string, error)
}

// OpenAI 是 OpenAI 兼容 chat completions 客户端（M0 Provider 抽象）。
// 预算由 worker 层统计调用次数，客户端只负责单次调用的可靠性与超时。
type OpenAI struct {
	baseURL   string
	apiKey    string
	model     string
	timeout   time.Duration
	maxTokens int
	client    *http.Client
	logger    *slog.Logger
}

// NewOpenAI 构造 OpenAI 兼容客户端；baseURL 指向 /v1 根路径（如 https://api.openai.com/v1）。
func NewOpenAI(cfg config.AIOpsConfig, logger *slog.Logger) *OpenAI {
	return &OpenAI{
		baseURL:   cfg.OpenAIBaseURL,
		apiKey:    cfg.OpenAIAPIKey,
		model:     cfg.Model,
		timeout:   cfg.Timeout,
		maxTokens: cfg.MaxTokensPerCall,
		client:    &http.Client{Timeout: cfg.Timeout},
		logger:    logger,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	MaxTokens      int             `json:"max_tokens"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// CompleteJSON 调用 chat completions，最多重试 2 次（429/5xx/网络错误），指数退避。
func (client *OpenAI) CompleteJSON(ctx context.Context, system, user string, maxTokens int) (string, error) {
	payload, err := json.Marshal(chatRequest{
		Model: client.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens:      maxTokens,
		ResponseFormat: &responseFormat{Type: "json_object"},
	})
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}
	endpoint := client.baseURL + "/chat/completions"
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * 500 * time.Millisecond):
			}
		}
		response, statusCode, err := client.do(ctx, endpoint, payload)
		if err == nil {
			return response, nil
		}
		lastErr = err
		// 4xx（429 除外）为确定性错误，重试无意义。
		if statusCode >= 400 && statusCode < 500 && statusCode != http.StatusTooManyRequests {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}
		client.logger.Warn("AIOps LLM call failed, retrying", "attempt", attempt, "status", statusCode, "error", err)
	}
	return "", fmt.Errorf("LLM call failed after retries: %w", lastErr)
}

func (client *OpenAI) do(ctx context.Context, endpoint string, payload []byte) (string, int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", 0, fmt.Errorf("build LLM request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+client.apiKey)

	response, err := client.client.Do(request)
	if err != nil {
		return "", 0, fmt.Errorf("send LLM request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return "", 0, fmt.Errorf("read LLM response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return "", response.StatusCode, fmt.Errorf("LLM HTTP %d: %s", response.StatusCode, truncate(string(body), 200))
	}
	var decoded chatResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", response.StatusCode, fmt.Errorf("decode LLM response: %w", err)
	}
	if decoded.Error != nil {
		return "", response.StatusCode, fmt.Errorf("LLM error: %s", decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 || decoded.Choices[0].Message.Content == "" {
		return "", response.StatusCode, fmt.Errorf("LLM response has no content")
	}
	return decoded.Choices[0].Message.Content, response.StatusCode, nil
}

func truncate(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
