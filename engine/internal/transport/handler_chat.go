package transport

import (
	"log"

	"github.com/smart-assistant/engine/internal/agent"
)

// ==================== 流式聊天处理 ====================

// handleChatSend 处理流式对话请求（chat.send）。
// 这是唯一返回多帧流式响应的方法：meta → token* → done/error。
func handleChatSend(c *Conn, id string, params []byte) {
	var req agent.ChatRequest
	if !parseParams(c, id, params, &req) {
		return
	}

	if req.Prompt == "" {
		c.sendError(id, -32001, "prompt is required")
		return
	}

	log.Printf("[ws] chat.send START id=%s prompt=%s", id, req.Prompt)
	ch, err := c.router.AgentLoop.HandleChatWS(c.ctx, c.UserID, req)
	if err != nil {
		log.Printf("[ws] chat.send HandleChatWS ERROR id=%s err=%v", id, err)
		c.sendError(id, -32001, err.Error())
		return
	}

	// 从 channel 读取流事件并逐帧发送
	for evt := range ch {
		log.Printf("[ws] chat.send STREAM id=%s type=%s data=%v", id, evt.Type, evt.Data)
		c.Stream(id, evt.Type, evt.Data)
	}
	log.Printf("[ws] chat.send DONE id=%s (channel closed)", id)
}

// handleChatCancel 取消正在执行的对话。
func handleChatCancel(c *Conn, id string, params []byte) {
	var req struct {
		RunID string `json:"run_id"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}

	if err := c.router.AgentLoop.HandleCancel(req.RunID); err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, map[string]string{"status": "cancelled"})
}

// handleChatConfirm 处理用户对 ask_user 确认请求的响应。
// 前端用户在确认卡片上操作后，通过此方法将响应传递回阻塞中的 Agent Loop。
func handleChatConfirm(c *Conn, id string, params []byte) {
	var req agent.ConfirmResponse
	if !parseParams(c, id, params, &req) {
		return
	}

	if req.CallID == "" {
		c.sendError(id, -32001, "call_id is required")
		return
	}

	log.Printf("[ws] chat.confirm call_id=%s response=%s", req.CallID, req.Response)
	if agent.ResolveConfirm(req.CallID, req) {
		c.sendResult(id, map[string]string{"status": "confirmed"})
	} else {
		c.sendError(id, -32001, "unknown or expired confirmation call_id: "+req.CallID)
	}
}