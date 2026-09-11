package transport

import "log"

// ==================== 认证处理 ====================

// handleAuthRegister 处理注册请求。
func handleAuthRegister(c *Conn, id string, params []byte) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !parseParams(c, id, params, &req) {
		log.Printf("[auth.register] parse params FAILED for id=%s, raw=%s", id, string(params))
		return
	}

	log.Printf("[auth.register] username=%q pwLen=%d", req.Username, len(req.Password))

	result, err := c.router.AuthSvc.Register(req.Username, req.Password)
	if err != nil {
		log.Printf("[auth.register] FAILED username=%q err=%v", req.Username, err)
		c.sendError(id, -32001, err.Error())
		return
	}

	log.Printf("[auth.register] OK user_id=%s username=%s", result.UserID, result.Username)

	// 认证成功，绑定用户到连接
	c.UserID = result.UserID
	c.Username = result.Username

	c.sendResult(id, result)
}

// handleAuthLogin 处理登录请求。
func handleAuthLogin(c *Conn, id string, params []byte) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !parseParams(c, id, params, &req) {
		log.Printf("[auth.login] parse params FAILED for id=%s, raw=%s", id, string(params))
		return
	}

	log.Printf("[auth.login] username=%q pwLen=%d", req.Username, len(req.Password))

	result, err := c.router.AuthSvc.Login(req.Username, req.Password)
	if err != nil {
		log.Printf("[auth.login] FAILED username=%q err=%v", req.Username, err)
		c.sendError(id, -32001, err.Error())
		return
	}

	log.Printf("[auth.login] OK user_id=%s username=%s", result.UserID, result.Username)

	// 认证成功，绑定用户到连接
	c.UserID = result.UserID
	c.Username = result.Username

	c.sendResult(id, result)
}