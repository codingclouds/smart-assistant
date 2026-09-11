package transport

// ==================== 技能管理处理 ====================

// handleSkillsList 列出已安装的技能。
func handleSkillsList(c *Conn, id string, params []byte) {
	skills, err := c.router.SkillSvc.ListAll()
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, skills)
}

// handleSkillsInstall 安装一个技能。
func handleSkillsInstall(c *Conn, id string, params []byte) {
	var req struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}

	skill, err := c.router.SkillSvc.Install(req.Name, req.Version)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, skill)
}