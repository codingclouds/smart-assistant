package transport

import "github.com/smart-assistant/engine/internal/session"

// ==================== 会话处理 ====================

// handleSessionsList 列出当前用户的会话。
func handleSessionsList(c *Conn, id string, params []byte) {
	sessions, err := c.router.SessionSvc.ListByUser(c.UserID)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	if sessions == nil {
		sessions = []session.Session{}
	}
	c.sendResult(id, sessions)
}

// handleSessionsGet 获取指定会话的消息列表。
func handleSessionsGet(c *Conn, id string, params []byte) {
	var req struct {
		ID string `json:"id"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}

	msgs, err := c.router.SessionSvc.GetMessages(req.ID)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, msgs)
}

// handleSessionsDelete 删除指定会话。
func handleSessionsDelete(c *Conn, id string, params []byte) {
	var req struct {
		ID string `json:"id"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}

	if err := c.router.SessionSvc.DeleteSession(req.ID); err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, map[string]string{"status": "deleted"})
}