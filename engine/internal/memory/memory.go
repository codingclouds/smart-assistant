package memory

import (
	"database/sql"

	"github.com/google/uuid"
)

// MemoryFragment 一条记忆片段。
type MemoryFragment struct {
	ID              string `json:"id"`
	WorkspaceID     string `json:"workspace_id"`
	Content         string `json:"content"`
	SourceSessionID string `json:"source_session_id"`
	Enabled         bool   `json:"enabled"`
	CreatedAt       string `json:"created_at"`
}

// Service 长期记忆管理。v2: 支持基本 CRUD 操作。
type Service struct {
	db *sql.DB
}

// NewService 创建带 DB 连接的记忆服务。
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ListByWorkspace 获取工作空间的所有记忆片段。
func (s *Service) ListByWorkspace(workspaceID string) ([]MemoryFragment, error) {
	rows, err := s.db.Query(
		`SELECT id, workspace_id, content, source_session_id, enabled, created_at
		 FROM memories WHERE workspace_id = ? ORDER BY created_at DESC`, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fragments []MemoryFragment
	for rows.Next() {
		var m MemoryFragment
		var enabled int
		var srcID sql.NullString
		if err := rows.Scan(&m.ID, &m.WorkspaceID, &m.Content, &srcID, &enabled, &m.CreatedAt); err != nil {
			continue
		}
		m.Enabled = enabled == 1
		if srcID.Valid {
			m.SourceSessionID = srcID.String
		}
		fragments = append(fragments, m)
	}
	if fragments == nil {
		fragments = []MemoryFragment{}
	}
	return fragments, nil
}

// ToggleEnabled 切换记忆片段的启用状态。
func (s *Service) ToggleEnabled(id string, enabled bool) (*MemoryFragment, error) {
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.db.Exec(`UPDATE memories SET enabled = ? WHERE id = ?`, val, id)
	if err != nil {
		return nil, err
	}

	var m MemoryFragment
	var e int
	var srcID sql.NullString
	err = s.db.QueryRow(
		`SELECT id, workspace_id, content, source_session_id, enabled, created_at
		 FROM memories WHERE id = ?`, id,
	).Scan(&m.ID, &m.WorkspaceID, &m.Content, &srcID, &e, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	m.Enabled = e == 1
	if srcID.Valid {
		m.SourceSessionID = srcID.String
	}
	return &m, nil
}

// CreateMemory 创建一条新的记忆片段。
func (s *Service) CreateMemory(workspaceID, content, sourceSessionID string) (*MemoryFragment, error) {
	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT INTO memories (id, workspace_id, content, source_session_id, enabled) VALUES (?, ?, ?, ?, 1)`,
		id, workspaceID, content, sourceSessionID,
	)
	if err != nil {
		return nil, err
	}
	return s.ToggleEnabled(id, true)
}

// Extract 从会话历史中提取记忆片段。
// v2: 在 CreateMemory 基础上提供语义接口。
func (s *Service) Extract(sessionID string, messages []interface{}) ([]MemoryFragment, error) {
	return nil, nil
}

// Inject 返回应注入到当前上下文的记忆片段列表。
// v2: 在 ListByWorkspace 基础上过滤已启用项。
func (s *Service) Inject(workspaceID string) ([]MemoryFragment, error) {
	rows, err := s.db.Query(
		`SELECT id, workspace_id, content, source_session_id, enabled, created_at
		 FROM memories WHERE workspace_id = ? AND enabled = 1 ORDER BY created_at DESC`, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fragments []MemoryFragment
	for rows.Next() {
		var m MemoryFragment
		var enabled int
		var srcID sql.NullString
		if err := rows.Scan(&m.ID, &m.WorkspaceID, &m.Content, &srcID, &enabled, &m.CreatedAt); err != nil {
			continue
		}
		m.Enabled = enabled == 1
		if srcID.Valid {
			m.SourceSessionID = srcID.String
		}
		fragments = append(fragments, m)
	}
	if fragments == nil {
		fragments = []MemoryFragment{}
	}
	return fragments, nil
}