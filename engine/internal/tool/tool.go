// Package tool 提供 Agent 工具系统的核心接口与注册表。
// 工具是 Agent 与外部世界交互的能力单元（搜索、文件操作、代码执行等）。
package tool

import (
	"context"
	"fmt"
	"log"
)

// Definition 工具定义（发送给模型用于 function calling）。
type Definition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// Call 表示模型发起的一次工具调用。
type Call struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// Result 工具执行结果。
type Result struct {
	CallID string `json:"call_id"`
	Name   string `json:"name"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// Tool 工具接口。每个工具提供定义描述和执行能力。
type Tool interface {
	Definition() Definition
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

// Registry 工具注册表，管理所有可用工具。
type Registry struct {
	tools map[string]Tool
}

// NewRegistry 创建新的工具注册表。
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register 注册一个工具。同名工具后注册的覆盖先注册的。
func (r *Registry) Register(t Tool) {
	name := t.Definition().Name
	r.tools[name] = t
	log.Printf("[tool] registered: %s", name)
}

// Get 按名称获取工具。
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Definitions 返回所有已注册工具的定义列表（供模型选择）。
func (r *Registry) Definitions() []Definition {
	defs := make([]Definition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

// Execute 执行一个工具调用，返回执行结果。
func (r *Registry) Execute(ctx context.Context, call Call) Result {
	t, ok := r.Get(call.Name)
	if !ok {
		return Result{
			CallID: call.ID,
			Name:   call.Name,
			Error:  fmt.Sprintf("tool %q not found", call.Name),
		}
	}

	output, err := t.Execute(ctx, call.Arguments)
	result := Result{
		CallID: call.ID,
		Name:   call.Name,
		Output: output,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}