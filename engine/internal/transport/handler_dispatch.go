package transport

import "github.com/smart-assistant/engine/internal/agent"

// ==================== 多 Agent 调度处理 ====================

// handleExpertsList 返回内置专家 Agent 池列表。
func handleExpertsList(c *Conn, id string, params []byte) {
	experts := agent.ExpertPool()
	// 只返回前端需要的字段，不暴露完整 SystemPrompt
	type expertBrief struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Emoji       string   `json:"emoji"`
		Skills      []string `json:"skills"`
		ModelName   string   `json:"model_name"`
	}
	briefs := make([]expertBrief, 0, len(experts))
	for _, e := range experts {
		briefs = append(briefs, expertBrief{
			ID:          e.ID,
			Name:        e.Name,
			Description: e.Description,
			Emoji:       e.Emoji,
			Skills:      e.Skills,
			ModelName:   e.ModelName,
		})
	}
	c.sendResult(id, briefs)
}

// handleDispatchRun 执行多 Agent 调度，流式返回每个专家的执行进度。
func handleDispatchRun(c *Conn, id string, params []byte) {
	var req agent.DispatchRequest
	if !parseParams(c, id, params, &req) {
		return
	}

	if req.Prompt == "" {
		c.sendError(id, -32001, "prompt is required")
		return
	}

	if len(req.ExpertIDs) == 0 {
		c.sendError(id, -32001, "at least one expert is required")
		return
	}

	ch, err := c.router.AgentLoop.ExecuteStream(c.ctx, c.UserID, req)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}

	// 从 channel 读取流事件并逐帧发送
	for evt := range ch {
		c.Stream(id, evt.Type, evt.Data)
	}
}