package transport

// ==================== 记忆处理 ====================

// handleMemoryList 列出当前工作空间的记忆片段。
func handleMemoryList(c *Conn, id string, params []byte) {
	var req struct {
		WorkspaceID string `json:"workspace_id"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}
	if req.WorkspaceID == "" {
		c.sendError(id, -32001, "workspace_id is required")
		return
	}

	fragments, err := c.router.MemorySvc.ListByWorkspace(req.WorkspaceID)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, fragments)
}

// handleMemoryToggle 切换记忆片段的启用/禁用状态。
func handleMemoryToggle(c *Conn, id string, params []byte) {
	var req struct {
		MemoryID string `json:"memory_id"`
		Enabled  bool   `json:"enabled"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}
	if req.MemoryID == "" {
		c.sendError(id, -32001, "memory_id is required")
		return
	}

	fragment, err := c.router.MemorySvc.ToggleEnabled(req.MemoryID, req.Enabled)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, fragment)
}

// handleMemoryCreate 创建一条新的记忆片段。
func handleMemoryCreate(c *Conn, id string, params []byte) {
	var req struct {
		WorkspaceID     string `json:"workspace_id"`
		Content         string `json:"content"`
		SourceSessionID string `json:"source_session_id"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}
	if req.WorkspaceID == "" || req.Content == "" {
		c.sendError(id, -32001, "workspace_id and content are required")
		return
	}

	fragment, err := c.router.MemorySvc.CreateMemory(req.WorkspaceID, req.Content, req.SourceSessionID)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, fragment)
}