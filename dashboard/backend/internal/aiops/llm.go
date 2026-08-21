package aiops

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

// LLM 是分析 worker 依赖的模型接口，便于单元测试注入假模型。
type LLM interface {
	// CompleteJSON 调用模型并返回纯 JSON 文本（响应格式强制 json_object）与 token 用量。
	// temperature 为 0 时使用服务端默认（各层推荐值见 prompts.Definition）。
	CompleteJSON(ctx context.Context, system, user string, maxTokens int, temperature float64) (Completion, error)
	// StreamComplete 流式调用模型（stream=true），每个增量回调 onDelta；无增量时也要调用一次 onDelta("")。
	// onUsage 在流结束后回调 token 用量（prompt/completion，服务端返回 usage 时；无 usage 时回调零值）。
	StreamComplete(ctx context.Context, system, user string, maxTokens int, temperature float64, onDelta func(string), onUsage func(TokenUsage)) error
}

// OpenAI 是 OpenAI 兼容 chat completions 客户端（M0 Provider 抽象）。
// 预算由 worker 层统计调用次数，客户端只负责单次调用的可靠性与超时。
// 配置可运行时更新（#110 阶段四：面板配置 key/model/baseURL），读操作走 mu 保护。
type OpenAI struct {
	mu        sync.RWMutex
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

// UpdateConfig 运行时更新配置（#110 阶段四）；空字段保持不变。
func (client *OpenAI) UpdateConfig(baseURL, apiKey, model string) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if strings.TrimSpace(baseURL) != "" {
		client.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
	if strings.TrimSpace(apiKey) != "" {
		client.apiKey = strings.TrimSpace(apiKey)
	}
	if strings.TrimSpace(model) != "" {
		client.model = strings.TrimSpace(model)
	}
}

// Snapshot 返回当前配置快照（apiKey 只含是否配置，不回显明文）。
func (client *OpenAI) Snapshot() (baseURL, model string, keyConfigured bool) {
	client.mu.RLock()
	defer client.mu.RUnlock()
	return client.baseURL, client.model, client.apiKey != ""
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

// TokenUsage 是单次模型调用的 token 用量（OpenAI usage 字段，审计 #110 阶段四）。
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
}

// Completion 是 CompleteJSON 的返回（#112 token 预算）：模型输出文本 + token 用量。
type Completion struct {
	Content string
	Usage   TokenUsage
}

type chatStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatRequest struct {
	Model          string             `json:"model"`
	Messages       []chatMessage      `json:"messages"`
	MaxTokens      int                `json:"max_tokens"`
	Temperature    *float64           `json:"temperature,omitempty"`
	Stream         bool               `json:"stream,omitempty"`
	StreamOptions  *chatStreamOptions `json:"stream_options,omitempty"`
	ResponseFormat *responseFormat    `json:"response_format,omitempty"`
}

type chatChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type chatStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// CompleteJSON 调用 chat completions，最多重试 2 次（429/5xx/网络错误），指数退避。
func (client *OpenAI) CompleteJSON(ctx context.Context, system, user string, maxTokens int, temperature float64) (Completion, error) {
	client.mu.RLock()
	model, baseURL := client.model, client.baseURL
	client.mu.RUnlock()
	payload, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens:      maxTokens,
		Temperature:    optionalTemperature(temperature),
		ResponseFormat: &responseFormat{Type: "json_object"},
	})
	if err != nil {
		return Completion{}, fmt.Errorf("marshal chat request: %w", err)
	}
	endpoint := baseURL + "/chat/completions"
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Completion{}, ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * 500 * time.Millisecond):
			}
		}
		completion, statusCode, err := client.do(ctx, endpoint, payload)
		if err == nil {
			return completion, nil
		}
		lastErr = err
		// 4xx（429 除外）为确定性错误，重试无意义。
		if statusCode >= 400 && statusCode < 500 && statusCode != http.StatusTooManyRequests {
			return Completion{}, fmt.Errorf("LLM call failed: %w", err)
		}
		client.logger.Warn("AIOps LLM call failed, retrying", "attempt", attempt, "status", statusCode, "error", err)
	}
	return Completion{}, fmt.Errorf("LLM call failed after retries: %w", lastErr)
}

// StreamComplete 流式调用 chat completions；错误语义与 CompleteJSON 一致（无重试，流一旦开始不重试）。
// include_usage=true 让兼容服务端在最后一个 chunk 返回 usage，流结束后回调 onUsage（无 usage 时回调零值）。
func (client *OpenAI) StreamComplete(ctx context.Context, system, user string, maxTokens int, temperature float64, onDelta func(string), onUsage func(TokenUsage)) error {
	client.mu.RLock()
	model, baseURL, apiKey := client.model, client.baseURL, client.apiKey
	client.mu.RUnlock()
	payload, err := json.Marshal(chatRequest{
		Model:         model,
		Messages:      []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}},
		MaxTokens:     maxTokens,
		Temperature:   optionalTemperature(temperature),
		Stream:        true,
		StreamOptions: &chatStreamOptions{IncludeUsage: true},
	})
	if err != nil {
		return fmt.Errorf("marshal stream chat request: %w", err)
	}
	endpoint := baseURL + "/chat/completions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build stream LLM request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	response, err := client.client.Do(request)
	if err != nil {
		return fmt.Errorf("send stream LLM request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		return fmt.Errorf("LLM stream HTTP %d: %s", response.StatusCode, truncate(string(body), 200))
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	emitted := false
	usage := TokenUsage{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk chatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			client.logger.Warn("AIOps stream chunk decode failed", "error", err)
			continue
		}
		if chunk.Error != nil {
			return fmt.Errorf("LLM stream error: %s", chunk.Error.Message)
		}
		if chunk.Usage != nil {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
		}
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta.Content
			onDelta(delta)
			emitted = true
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read LLM stream: %w", err)
	}
	if !emitted {
		onDelta("")
	}
	if onUsage != nil {
		onUsage(usage)
	}
	return nil
}

func (client *OpenAI) do(ctx context.Context, endpoint string, payload []byte) (Completion, int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Completion{}, 0, fmt.Errorf("build LLM request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	client.mu.RLock()
	apiKey := client.apiKey
	client.mu.RUnlock()
	request.Header.Set("Authorization", "Bearer "+apiKey)

	response, err := client.client.Do(request)
	if err != nil {
		return Completion{}, 0, fmt.Errorf("send LLM request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return Completion{}, 0, fmt.Errorf("read LLM response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return Completion{}, response.StatusCode, fmt.Errorf("LLM HTTP %d: %s", response.StatusCode, truncate(string(body), 200))
	}
	var decoded chatResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return Completion{}, response.StatusCode, fmt.Errorf("decode LLM response: %w", err)
	}
	if decoded.Error != nil {
		return Completion{}, response.StatusCode, fmt.Errorf("LLM error: %s", decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 || decoded.Choices[0].Message.Content == "" {
		return Completion{}, response.StatusCode, fmt.Errorf("LLM response has no content")
	}
	completion := Completion{Content: decoded.Choices[0].Message.Content}
	if decoded.Usage != nil {
		completion.Usage = TokenUsage{PromptTokens: decoded.Usage.PromptTokens, CompletionTokens: decoded.Usage.CompletionTokens}
	}
	return completion, response.StatusCode, nil
}

// optionalTemperature 返回可选的 temperature 指针；0 表示不设置（服务端默认）。
func optionalTemperature(temperature float64) *float64 {
	if temperature == 0 {
		return nil
	}
	return &temperature
}

func truncate(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
