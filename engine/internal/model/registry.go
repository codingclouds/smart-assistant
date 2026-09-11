package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// StoredModel 存储在数据库中的模型配置。
type StoredModel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	ApiFormat string `json:"api_format"`
	APIKey    string `json:"-"` // 不返回给前端
	Enabled   bool   `json:"enabled"`
	IsDefault bool   `json:"is_default"`
}

// Registry 管理模型配置的注册与路由。
type Registry struct {
	db                *sql.DB
	openAIProvider    *OpenAIProvider
	anthropicProvider *AnthropicProvider
}

func NewRegistry(db *sql.DB) *Registry {
	return &Registry{
		db:                db,
		openAIProvider:    NewOpenAIProvider(),
		anthropicProvider: NewAnthropicProvider(),
	}
}

// NewProvider 根据 api_format 返回对应的 Provider。
func (r *Registry) NewProvider(apiFormat string) Provider {
	switch apiFormat {
	case "anthropic":
		return r.anthropicProvider
	default:
		return r.openAIProvider
	}
}

// GetConfig 获取指定模型的配置。
func (r *Registry) GetConfig(modelName string) (*ModelConfig, error) {
	row := r.db.QueryRow(
		`SELECT base_url, api_format, config FROM models WHERE name = ? AND enabled = 1 LIMIT 1`,
		modelName,
	)
	var baseURL, apiFormat, configJSON string
	if err := row.Scan(&baseURL, &apiFormat, &configJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("model %q not found, please configure it in settings", modelName)
		}
		return nil, err
	}

	var config ModelConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("invalid model config JSON for %q (raw=%q): %w", modelName, configJSON, err)
	}
	// base_url 和 api_format 存储在 models 表的独立列中，不在 config JSON 里
	if config.BaseURL == "" {
		config.BaseURL = baseURL
	}
	if config.ApiFormat == "" {
		config.ApiFormat = apiFormat
	}
	return &config, nil
}

// ListAll 返回所有已配置的模型。
func (r *Registry) ListAll() ([]StoredModel, error) {
	rows, err := r.db.Query(`SELECT id, name, base_url, api_format, enabled, is_default FROM models`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []StoredModel
	for rows.Next() {
		var m StoredModel
		if err := rows.Scan(&m.ID, &m.Name, &m.BaseURL, &m.ApiFormat, &m.Enabled, &m.IsDefault); err != nil {
			continue
		}
		models = append(models, m)
	}
	return models, nil
}

// Create 创建新的模型配置。
func (r *Registry) Create(name, baseURL, apiFormat, apiKey string) (*StoredModel, error) {
	if apiFormat == "" {
		apiFormat = "openai"
	}
	m := &StoredModel{
		ID:        uuid.NewString(),
		Name:      name,
		BaseURL:   baseURL,
		ApiFormat: apiFormat,
		Enabled:   true,
		IsDefault: false,
	}
	configJSON, _ := json.Marshal(map[string]interface{}{})

	// 检查是否为首个模型，若是则自动设为默认
	var count int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM models`).Scan(&count); err == nil && count == 0 {
		m.IsDefault = true
	}

	_, err := r.db.Exec(
		`INSERT INTO models (id, name, base_url, api_format, config, enabled, is_default) VALUES (?, ?, ?, ?, ?, 1, ?)`,
		m.ID, m.Name, m.BaseURL, m.ApiFormat, string(configJSON), m.IsDefault,
	)
	if err != nil {
		return nil, err
	}

	if apiKey != "" {
		apiConfig, _ := json.Marshal(map[string]string{"api_key": apiKey})
		r.db.Exec(`UPDATE models SET config = ? WHERE id = ?`, string(apiConfig), m.ID)
	}
	return m, nil
}

// Update 更新模型配置。
func (r *Registry) Update(id, name, baseURL, apiFormat, apiKey string, enabled *bool) error {
	// 若未传 enabled，保留当前值，避免 nil 解引用 panic
	enabledVal := false
	if enabled != nil {
		enabledVal = *enabled
	} else {
		row := r.db.QueryRow(`SELECT enabled FROM models WHERE id = ?`, id)
		if err := row.Scan(&enabledVal); err != nil {
			return fmt.Errorf("lookup current enabled: %w", err)
		}
	}

	_, err := r.db.Exec(
		`UPDATE models SET name=?, base_url=?, api_format=?, enabled=? WHERE id=?`,
		name, baseURL, apiFormat, enabledVal, id,
	)
	if err != nil {
		return err
	}

	// 如果传了新的 api_key，写入 config JSON
	if apiKey != "" {
		apiConfig, _ := json.Marshal(map[string]string{"api_key": apiKey})
		r.db.Exec(`UPDATE models SET config = ? WHERE id = ?`, string(apiConfig), id)
	}

	return nil
}

// SetDefault 设置指定模型为默认模型，同时取消其他模型的默认状态。
func (r *Registry) SetDefault(id string) error {
	// 先取消所有默认
	_, err := r.db.Exec(`UPDATE models SET is_default = 0 WHERE is_default = 1`)
	if err != nil {
		return fmt.Errorf("clear defaults: %w", err)
	}
	// 设置新的默认
	result, err := r.db.Exec(`UPDATE models SET is_default = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("set default: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("model not found: %s", id)
	}
	return nil
}

// GetDefault 返回默认模型配置。
func (r *Registry) GetDefault() (*StoredModel, error) {
	row := r.db.QueryRow(
		`SELECT id, name, base_url, api_format, enabled, is_default FROM models WHERE is_default = 1 LIMIT 1`,
	)
	var m StoredModel
	if err := row.Scan(&m.ID, &m.Name, &m.BaseURL, &m.ApiFormat, &m.Enabled, &m.IsDefault); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no default model set")
		}
		return nil, err
	}
	return &m, nil
}

// HandleList 返回已配置的模型列表。
func (r *Registry) HandleList(w http.ResponseWriter, req *http.Request) {
	models, err := r.ListAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if models == nil {
		models = []StoredModel{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

// HandleCreate 创建新的模型配置。
func (r *Registry) HandleCreate(w http.ResponseWriter, req *http.Request) {
	var config struct {
		Name      string `json:"name"`
		BaseURL   string `json:"base_url"`
		ApiFormat string `json:"api_format"`
		APIKey    string `json:"api_key"`
	}
	if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if config.Name == "" || config.BaseURL == "" {
		http.Error(w, "name and base_url are required", http.StatusBadRequest)
		return
	}

	m, err := r.Create(config.Name, config.BaseURL, config.ApiFormat, config.APIKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(m)
}

// HandleUpdate 更新模型配置。
func (r *Registry) HandleUpdate(w http.ResponseWriter, req *http.Request) {
	modelID := chi.URLParam(req, "id")

	var config struct {
		Name      string `json:"name"`
		BaseURL   string `json:"base_url"`
		ApiFormat string `json:"api_format"`
		APIKey    string `json:"api_key"`
		Enabled   bool   `json:"enabled"`
	}
	if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := r.Update(modelID, config.Name, config.BaseURL, config.ApiFormat, config.APIKey, &config.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// 确保不同 API 格式的 Base URL 提示一致
var knownBaseURLHints = map[string]string{
	"openai":    "https://api.openai.com/v1（引擎会自动拼接 /chat/completions）",
	"anthropic": "https://api.anthropic.com（引擎会自动拼接 /v1/messages）",
}

func BaseURLHint(apiFormat string) string {
	if h, ok := knownBaseURLHints[apiFormat]; ok {
		return h
	}
	return "请输入 API 基础地址"
}

// 编译期检查 interface 实现
var _ = fmt.Sprintf("%v", knownBaseURLHints)