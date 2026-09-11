package session

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/smart-assistant/engine/internal/auth"
)

// Session 表示一次对话会话。
type Session struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Message 表示一条对话消息。
type Message struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	Role       string `json:"role"`
	Content    string `json:"content"`
	TokenCount int    `json:"token_count"`
	Blocks     string `json:"blocks"`
	CreatedAt  string `json:"created_at"`
}

// TokenStat Token 使用统计行。
type TokenStat struct {
	ModelName   string `json:"model_name"`
	TotalInput  int    `json:"total_input"`
	TotalOutput int    `json:"total_output"`
	TotalTokens int    `json:"total_tokens"`
}

// Service 会话与消息管理。
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// CreateSession 创建新会话。若 title 为空则使用默认标题。
func (s *Service) CreateSession(workspaceID, title string) (*Session, error) {
	if title == "" {
		title = "新对话"
	}
	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, workspace_id, title) VALUES (?, ?, ?)`,
		id, workspaceID, title,
	)
	if err != nil {
		return nil, err
	}
	return s.GetSession(id)
}

// GetSession 获取指定会话。
func (s *Service) GetSession(id string) (*Session, error) {
	var sess Session
	err := s.db.QueryRow(
		`SELECT id, workspace_id, title, created_at, updated_at FROM sessions WHERE id = ?`, id,
	).Scan(&sess.ID, &sess.WorkspaceID, &sess.Title, &sess.CreatedAt, &sess.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// AddMessage 追加消息到会话。blocks 参数是工具调用/文件产物等结构化数据的 JSON 数组。
func (s *Service) AddMessage(sessionID, role, content string, tokenCount int, blocks string) error {
	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT INTO messages (id, session_id, role, content, token_count, blocks) VALUES (?, ?, ?, ?, ?, ?)`,
		id, sessionID, role, content, tokenCount, blocks,
	)
	if err != nil {
		return err
	}

	_, _ = s.db.Exec(`UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, sessionID)
	return nil
}

// GetMessages 获取会话消息列表。
func (s *Service) GetMessages(sessionID string) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, token_count, blocks, created_at
		 FROM messages WHERE session_id = ? ORDER BY created_at ASC`, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.TokenCount, &m.Blocks, &m.CreatedAt); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// ListByUser 返回用户所有会话。
func (s *Service) ListByUser(userID string) ([]Session, error) {
	rows, err := s.db.Query(
		`SELECT s.id, s.workspace_id, s.title, s.created_at, s.updated_at
		 FROM sessions s
		 JOIN workspaces w ON s.workspace_id = w.id
		 WHERE w.user_id = ?
		 ORDER BY s.updated_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.WorkspaceID, &sess.Title, &sess.CreatedAt, &sess.UpdatedAt); err != nil {
			continue
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// DeleteSession 删除会话及其消息。
func (s *Service) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE session_id = ?`, id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// TokenStatsByUser 返回用户的 Token 使用统计。
func (s *Service) TokenStatsByUser(userID string) ([]TokenStat, error) {
	rows, err := s.db.Query(
		`SELECT ts.model_name, SUM(ts.input_tokens) as total_input, SUM(ts.output_tokens) as total_output
		 FROM token_stats ts
		 JOIN workspaces w ON ts.workspace_id = w.id
		 WHERE w.user_id = ?
		 GROUP BY ts.model_name
		 ORDER BY total_input DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []TokenStat
	for rows.Next() {
		var s TokenStat
		if err := rows.Scan(&s.ModelName, &s.TotalInput, &s.TotalOutput); err != nil {
			continue
		}
		s.TotalTokens = s.TotalInput + s.TotalOutput
		stats = append(stats, s)
	}
	return stats, nil
}

// HandleList 返回当前用户所有会话列表。
func (s *Service) HandleList(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	sessions, err := s.ListByUser(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if sessions == nil {
		sessions = []Session{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// HandleGet 返回指定会话的消息历史。
func (s *Service) HandleGet(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	msgs, err := s.GetMessages(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if msgs == nil {
		msgs = []Message{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

// HandleDelete 删除指定会话。
func (s *Service) HandleDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if err := s.DeleteSession(sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// HandleTokenStats 返回 Token 统计。
func (s *Service) HandleTokenStats(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	stats, err := s.TokenStatsByUser(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if stats == nil {
		stats = []TokenStat{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}