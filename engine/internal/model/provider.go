package model

import (
	"context"
)

// ToolCallFunction 工具调用的函数描述（OpenAI 格式）。
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolCall 表示模型的一次工具调用（OpenAI 格式，含 type/function 包装）。
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallDelta 流式累积的工具调用 delta（OpenAI 格式）。
type ToolCallDelta struct {
	ID       string           `json:"id"`
	Function ToolCallFunction `json:"function"`
}

// Message 表示一条对话消息。
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// StreamChunk 流式响应的一个片段。
type StreamChunk struct {
	Content   string          `json:"content"`
	Thinking  string          `json:"thinking,omitempty"`
	Done      bool            `json:"done"`
	Error     string          `json:"error,omitempty"`
	ToolCalls []ToolCallDelta `json:"tool_calls,omitempty"`
}

// Provider 抽象多模型厂商的统一调用接口。
type Provider interface {
	// StreamChat 发起流式对话，通过 channel 返回 token 片段。
	StreamChat(ctx context.Context, modelName string, messages []Message, config ModelConfig) (<-chan StreamChunk, error)
	// Chat 同步对话，返回完整响应。
	Chat(ctx context.Context, modelName string, messages []Message, config ModelConfig) (string, error)
}

// ModelConfig 模型配置参数。
type ModelConfig struct {
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url"`
	ApiFormat   string  `json:"api_format"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	TopP        float64 `json:"top_p"`
}

// ToolsProvider 支持工具调用的 Provider 扩展接口。
// 如果 Provider 实现了此接口，引擎会通过 SetTools 传递工具定义。
type ToolsProvider interface {
	Provider
	// SetTools 设置当前请求可用的工具定义。
	SetTools(tools []ToolDefinition)
}

// ToolDefinition 工具定义（发送给模型的 JSON Schema）。
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}