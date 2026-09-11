package transport

// ==================== 模型管理处理 ====================

// handleModelsList 列出所有模型配置。
func handleModelsList(c *Conn, id string, params []byte) {
	models, err := c.router.ModelRegistry.ListAll()
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, models)
}

// handleModelsCreate 创建新的模型配置。
func handleModelsCreate(c *Conn, id string, params []byte) {
	var req struct {
		Name      string `json:"name"`
		BaseURL   string `json:"base_url"`
		ApiFormat string `json:"api_format"`
		APIKey    string `json:"api_key"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}

	model, err := c.router.ModelRegistry.Create(req.Name, req.BaseURL, req.ApiFormat, req.APIKey)
	if err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, model)
}

// handleModelsUpdate 更新模型配置。
func handleModelsUpdate(c *Conn, id string, params []byte) {
	var req struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		BaseURL   string `json:"base_url"`
		ApiFormat string `json:"api_format"`
		APIKey    string `json:"api_key"`
		Enabled   *bool  `json:"enabled"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}

	if err := c.router.ModelRegistry.Update(req.ID, req.Name, req.BaseURL, req.ApiFormat, req.APIKey, req.Enabled); err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, map[string]string{"status": "updated"})
}

// handleModelsSetDefault 设置默认模型。
func handleModelsSetDefault(c *Conn, id string, params []byte) {
	var req struct {
		ID string `json:"id"`
	}
	if !parseParams(c, id, params, &req) {
		return
	}

	if err := c.router.ModelRegistry.SetDefault(req.ID); err != nil {
		c.sendError(id, -32001, err.Error())
		return
	}
	c.sendResult(id, map[string]string{"status": "ok"})
}