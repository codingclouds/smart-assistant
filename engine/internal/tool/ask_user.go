package tool

import (
	"context"
	"fmt"
	"strings"
)

// AskUserTool 是一个交互式工具：当模型需要用户确认或补充信息时调用。
// 此工具的 Execute 方法由 Agent Loop 特殊处理（不在此处实际执行），
// Loop 会检测到此工具调用后暂停输出、展示确认卡片、等待用户响应。
type AskUserTool struct{}

// NewAskUserTool 创建 ask_user 交互式确认工具。
func NewAskUserTool() Tool {
	return &AskUserTool{}
}

// Definition 返回 ask_user 工具的定义。
func (t *AskUserTool) Definition() Definition {
	return Definition{
		Name: "ask_user",
		Description: `当需要用户确认操作、提供额外信息或做出选择时使用此工具。
适用场景：
- 在执行可能有风险的操作前请求用户确认
- 需要用户在多个选项中选择
- 需要用户补充关键信息
- 用户要求"让我确认一下"或类似交互时
调用此工具后系统会暂停并展示确认表单，等待用户响应后继续。`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"question": map[string]interface{}{
					"type":        "string",
					"description": "向用户提出的问题或确认内容",
				},
				"options": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "可选的预定义选项列表（如 [\"确认\", \"取消\"]），不填则表示自由文本输入",
				},
				"context": map[string]interface{}{
					"type":        "string",
					"description": "帮助用户理解当前上下文的额外说明",
				},
			},
			"required": []string{"question"},
		},
	}
}

// Execute ask_user 的实际执行由 Agent Loop 特殊处理。
// 此方法仅作为兜底：如果被 Registry 直接调用，返回提示信息。
func (t *AskUserTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	question, _ := args["question"].(string)
	if strings.TrimSpace(question) == "" {
		question = "需要用户确认"
	}
	return fmt.Sprintf("[需要用户确认] %s\n请在前端界面中响应此确认请求。", question), nil
}

var _ Tool = (*AskUserTool)(nil)