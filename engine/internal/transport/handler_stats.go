package transport

// ==================== 统计与心跳处理 ====================

// handleStatsTokens 获取当前用户的 Token 使用统计。
func handleStatsTokens(c *Conn, id string, params []byte) {
	stats, err := c.router.SessionSvc.TokenStatsByUser(c.UserID)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, stats)
}

// handleSystemPing 心跳检测。
func handleSystemPing(c *Conn, id string, params []byte) {
	c.sendResult(id, "pong")
}