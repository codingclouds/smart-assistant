package transport

import "log"

// handleToolsList 返回当前注册的所有工具定义及其能力描述。
func handleToolsList(c *Conn, id string, _ []byte) {
	log.Printf("[ws] tools.list")
	tools := c.router.AgentLoop.GetToolDefs()
	c.sendResult(id, tools)
}