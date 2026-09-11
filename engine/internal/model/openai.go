package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OpenAIProvider 实现 OpenAI-compatible API 的 Provider。
type OpenAIProvider struct {
	httpClient *http.Client
	mu         sync.Mutex
	tools      []ToolDefinition
}

func NewOpenAIProvider() *OpenAIProvider {
	return &OpenAIProvider{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// SetTools 设置当前请求可用的工具定义。
func (p *OpenAIProvider) SetTools(tools []ToolDefinition) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tools = tools
}

func (p *OpenAIProvider) getTools() []ToolDefinition {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tools
}

type openAIRequest struct {
	Model       string             `json:"model"`
	Messages    []Message          `json:"messages"`
	Stream      bool               `json:"stream"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	Tools       []openAIToolDef    `json:"tools,omitempty"`
	ToolChoice  string             `json:"tool_choice,omitempty"`
}

type openAIToolDef struct {
	Type     string                 `json:"type"`
	Function openAIFunctionDef      `json:"function"`
}

type openAIFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type openAIChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// pendingOpenAIToolCall 流式累积中的 OpenAI 工具调用。
type pendingOpenAIToolCall struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func (p *OpenAIProvider) StreamChat(ctx context.Context, modelName string, messages []Message, config ModelConfig) (<-chan StreamChunk, error) {
	// 构造 tools 定义
	var apiTools []openAIToolDef
	regTools := p.getTools()
	if len(regTools) > 0 {
		apiTools = make([]openAIToolDef, len(regTools))
		for i, td := range regTools {
			apiTools[i] = openAIToolDef{
				Type: "function",
				Function: openAIFunctionDef{
					Name:        td.Name,
					Description: td.Description,
					Parameters:  td.Parameters,
				},
			}
		}
	}

	body := openAIRequest{
		Model:       modelName,
		Messages:    messages,
		Stream:      true,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		TopP:        config.TopP,
		Tools:       apiTools,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	baseURL := strings.TrimRight(config.BaseURL, "/")
	endpoint := baseURL + "/v1/chat/completions"
	// 如果 BaseURL 已经包含 /v1，不要重复拼接（例如 https://api.deepseek.com/v1）
	if strings.HasSuffix(baseURL, "/v1") {
		endpoint = baseURL + "/chat/completions"
	}
	log.Printf("[openai] request url=%s model=%s tools=%d", endpoint, modelName, len(apiTools))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	ch := make(chan StreamChunk, 32)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		// 检查 HTTP 状态码：非 200 返回直接作为错误
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			log.Printf("[openai] non-200 response: status=%d body=%s", resp.StatusCode, string(body))
			// 打印请求体（截断至 2000 字符）以便排查
			payloadPreview := string(payload)
			if len(payloadPreview) > 2000 {
				payloadPreview = payloadPreview[:2000] + "...[truncated]"
			}
			log.Printf("[openai] request payload: %s", payloadPreview)
			ch <- StreamChunk{Error: fmt.Sprintf("API error (HTTP %d): %s", resp.StatusCode, string(body))}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		// 流式累积工具调用（按 index 分组）
		pendingCalls := make(map[int]*pendingOpenAIToolCall)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				ch <- StreamChunk{Error: ctx.Err().Error()}
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" || line == "data: [DONE]" {
				continue
			}

			jsonStr := strings.TrimPrefix(line, "data: ")
			var chunk openAIChunk
			if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
				continue // 跳过解析失败的行
			}

			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0]

				// 处理文本内容
				if delta.Delta.Content != "" {
					ch <- StreamChunk{Content: delta.Delta.Content}
				}

				// 处理推理内容（DeepSeek/Qwen reasoning_content）
				if delta.Delta.ReasoningContent != "" {
					preview := delta.Delta.ReasoningContent
					if len(preview) > 80 {
						preview = preview[:80]
					}
					log.Printf("[openai] reasoning_content: %s", preview)
					ch <- StreamChunk{Thinking: delta.Delta.ReasoningContent}
				}

				// 处理工具调用 delta
				for _, tc := range delta.Delta.ToolCalls {
					pc, ok := pendingCalls[tc.Index]
					if !ok {
						pc = &pendingOpenAIToolCall{}
						pendingCalls[tc.Index] = pc
					}
					if tc.ID != "" {
						pc.ID = tc.ID
					}
					if tc.Function.Name != "" {
						pc.Name = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						pc.Arguments.WriteString(tc.Function.Arguments)
					}
				}

				if delta.FinishReason != "" {
					// 发送累积的 tool_calls
					if len(pendingCalls) > 0 {
						var deltas []ToolCallDelta
						for idx := 0; idx < len(pendingCalls); idx++ {
							if pc, ok := pendingCalls[idx]; ok {
								deltas = append(deltas, ToolCallDelta{
									ID: pc.ID,
									Function: ToolCallFunction{
										Name:      pc.Name,
										Arguments: pc.Arguments.String(),
									},
								})
							}
						}
						ch <- StreamChunk{ToolCalls: deltas}
					}
					ch <- StreamChunk{Done: true}
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case ch <- StreamChunk{Error: err.Error()}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

func (p *OpenAIProvider) Chat(ctx context.Context, modelName string, messages []Message, config ModelConfig) (string, error) {
	ch, err := p.StreamChat(ctx, modelName, messages, config)
	if err != nil {
		return "", err
	}

	var fullText strings.Builder
	for chunk := range ch {
		if chunk.Error != "" {
			return fullText.String(), fmt.Errorf("stream error: %s", chunk.Error)
		}
		if chunk.Done {
			break
		}
		fullText.WriteString(chunk.Content)
	}

	return fullText.String(), nil
}

// 编译期检查 ToolsProvider 接口实现。
var _ ToolsProvider = (*OpenAIProvider)(nil)

// jsonBodyReader wraps bytes.Reader to provide io.ReadCloser.
func asReader(v interface{}) (io.Reader, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}