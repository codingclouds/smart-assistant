package transport

// ==================== Trace 处理 ====================

// handleTracesGet 获取指定会话的执行追踪日志。
func handleTracesGet(c *Conn, id string, params []byte) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}

	events, err := c.router.TraceSvc.GetBySession(req.SessionID)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, events)
}