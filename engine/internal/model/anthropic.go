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

// AnthropicProvider 实现 Anthropic Messages API 的 Provider。
// API 文档: https://docs.anthropic.com/en/api/messages
type AnthropicProvider struct {
	httpClient *http.Client
	mu         sync.Mutex
	tools      []ToolDefinition
}

func NewAnthropicProvider() *AnthropicProvider {
	return &AnthropicProvider{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// SetTools 设置当前请求可用的工具定义。
func (p *AnthropicProvider) SetTools(tools []ToolDefinition) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tools = tools
}

func (p *AnthropicProvider) getTools() []ToolDefinition {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tools
}

// anthropicRequest Anthropic Messages API 请求体。
type anthropicRequest struct {
	Model     string              `json:"model"`
	Messages  []anthropicMsg      `json:"messages"`
	System    string              `json:"system,omitempty"`
	Stream    bool                `json:"stream"`
	MaxTokens int                 `json:"max_tokens"`
	Tools     []anthropicToolDef  `json:"tools,omitempty"`
}

type anthropicMsg struct {
	Role    string           `json:"role"`
	Content interface{}      `json:"content"` // string | []anthropicContentBlock
}

// anthropicContentBlock 用于构造包含 tool_use 和 tool_result 的消息内容。
type anthropicContentBlock struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"`
}

type anthropicToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// anthropicSSE Anthropic SSE 事件帧。
type anthropicSSE struct {
	Type         string `json:"type"`
	ContentBlock *struct {
		Type string          `json:"type"`
		ID   string          `json:"id"`
		Name string          `json:"name"`
		Text string          `json:"text"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content_block,omitempty"`
	Delta *struct {
		Type         string `json:"type"`
		Text         string `json:"text,omitempty"`
		Thinking     string `json:"thinking,omitempty"`
		PartialJSON  string `json:"partial_json,omitempty"`
		StopReason   string `json:"stop_reason,omitempty"`
		StopSequence string `json:"stop_sequence,omitempty"`
	} `json:"delta,omitempty"`
	Message *struct {
		ID           string         `json:"id"`
		Type         string         `json:"type"`
		Role         string         `json:"role"`
		Content      []contentBlock `json:"content"`
		Model        string         `json:"model"`
		StopReason   string         `json:"stop_reason"`
		StopSequence string         `json:"stop_sequence"`
		Usage        struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message,omitempty"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// pendingToolCall 流式累积中的工具调用。
type pendingToolCall struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func (p *AnthropicProvider) StreamChat(ctx context.Context, modelName string, messages []Message, config ModelConfig) (<-chan StreamChunk, error) {
	// 提取 system 消息（Anthropic API 要求 system 单独提供）
	var systemPrompt string
	var chatMsgs []anthropicMsg
	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt = m.Content
		} else if m.Role == "tool" {
			// tool 消息转换为 user role 的 tool_result content block
			chatMsgs = append(chatMsgs, anthropicMsg{
				Role: "user",
				Content: []anthropicContentBlock{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
				}},
			})
		} else if len(m.ToolCalls) > 0 {
			// assistant 消息带有 tool_use content blocks
			blocks := []anthropicContentBlock{}
			if m.Content != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				var input interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
					input = map[string]interface{}{"raw": tc.Function.Arguments}
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			chatMsgs = append(chatMsgs, anthropicMsg{
				Role:    "assistant",
				Content: blocks,
			})
		} else {
			chatMsgs = append(chatMsgs, anthropicMsg{Role: m.Role, Content: m.Content})
		}
	}

	maxTokens := config.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	// 构造 tools 定义
	var apiTools []anthropicToolDef
	regTools := p.getTools()
	if len(regTools) > 0 {
		apiTools = make([]anthropicToolDef, len(regTools))
		for i, td := range regTools {
			apiTools[i] = anthropicToolDef{
				Name:        td.Name,
				Description: td.Description,
				InputSchema: td.Parameters,
			}
		}
	}

	body := anthropicRequest{
		Model:     modelName,
		Messages:  chatMsgs,
		System:    systemPrompt,
		Stream:    true,
		MaxTokens: maxTokens,
		Tools:     apiTools,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	baseURL := strings.TrimRight(config.BaseURL, "/")
	endpoint := baseURL + "/v1/messages"
	keyPreview := ""
	if len(config.APIKey) > 10 {
		keyPreview = config.APIKey[:10]
	}
	log.Printf("[anthropic] request url=%s model=%s api_key=%s... tools=%d", endpoint, modelName, keyPreview, len(apiTools))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	ch := make(chan StreamChunk, 32)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			log.Printf("[anthropic] API error (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
			ch <- StreamChunk{Error: fmt.Sprintf("API error (HTTP %d): %s", resp.StatusCode, string(bodyBytes))}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		// 流式累积中的工具调用
		pendingCalls := make(map[int]*pendingToolCall)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				ch <- StreamChunk{Error: ctx.Err().Error()}
				return
			default:
			}

			rawLine := scanner.Text()
			line := strings.TrimSpace(rawLine)
			if line == "" {
				continue
			}
			// DashScope 使用 "data:" (无空格)，标准 SSE 使用 "data: " (有空格)，兼容两种格式
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			jsonStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

			var event anthropicSSE
			if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
				log.Printf("[anthropic] unmarshal error: %v", err)
				continue
			}

			switch event.Type {
			case "content_block_start":
				if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
					idx := len(pendingCalls)
					pendingCalls[idx] = &pendingToolCall{
						ID:   event.ContentBlock.ID,
						Name: event.ContentBlock.Name,
					}
					log.Printf("[anthropic] tool_use started: id=%s name=%s", event.ContentBlock.ID, event.ContentBlock.Name)
				}

			case "content_block_delta":
				if event.Delta == nil {
					continue
				}
				switch event.Delta.Type {
				case "text_delta":
					if event.Delta.Text != "" {
						ch <- StreamChunk{Content: event.Delta.Text}
					}
				case "thinking_delta":
					if event.Delta.Thinking != "" {
						log.Printf("[anthropic] thinking_delta: %s", event.Delta.Thinking[:min(80, len(event.Delta.Thinking))])
						ch <- StreamChunk{Thinking: event.Delta.Thinking}
					}
				case "input_json_delta":
					if event.Delta.PartialJSON != "" {
						// 追加到最后一个待处理的 tool_call
						if len(pendingCalls) > 0 {
							// 找到最新创建的 pendingCall
							lastIdx := len(pendingCalls) - 1
							for idx := lastIdx; idx >= 0; idx-- {
								if pc, ok := pendingCalls[idx]; ok {
									pc.Arguments.WriteString(event.Delta.PartialJSON)
									break
								}
							}
						}
					}
				}

			case "message_delta":
				if event.Delta != nil && event.Delta.StopReason != "" {
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

			case "message_stop":
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

			case "error":
				errMsg := "unknown error"
				if event.Error != nil {
					errMsg = event.Error.Message
				}
				ch <- StreamChunk{Error: errMsg}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("[anthropic] scanner error: %v", err)
			select {
			case ch <- StreamChunk{Error: err.Error()}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

func (p *AnthropicProvider) Chat(ctx context.Context, modelName string, messages []Message, config ModelConfig) (string, error) {
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
var _ ToolsProvider = (*AnthropicProvider)(nil)