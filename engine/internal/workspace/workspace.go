package workspace

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/smart-assistant/engine/internal/auth"
)

// Workspace 表示一个独立的工作空间。
type Workspace struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Config    string `json:"config"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Service 工作空间管理。
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// GetByUser 返回指定用户的所有工作空间。
func (s *Service) GetByUser(userID string) ([]Workspace, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, name, config, created_at, updated_at
		 FROM workspaces WHERE user_id = ? ORDER BY created_at ASC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []Workspace
	for rows.Next() {
		var ws Workspace
		var config sql.NullString
		if err := rows.Scan(&ws.ID, &ws.UserID, &ws.Name, &config, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			continue
		}
		if config.Valid {
			ws.Config = config.String
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

// Create 创建新工作空间并返回。
func (s *Service) Create(name, userID string) (*Workspace, error) {
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT INTO workspaces (id, user_id, name) VALUES (?, ?, ?)`,
		id, userID, name,
	)
	if err != nil {
		return nil, err
	}

	return &Workspace{ID: id, UserID: userID, Name: name}, nil
}

// HandleList 返回当前用户的所有工作空间。
func (s *Service) HandleList(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	rows, err := s.db.Query(
		`SELECT id, user_id, name, config, created_at, updated_at
		 FROM workspaces WHERE user_id = ? ORDER BY created_at ASC`, userID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var workspaces []Workspace
	for rows.Next() {
		var ws Workspace
		var config sql.NullString
		if err := rows.Scan(&ws.ID, &ws.UserID, &ws.Name, &config, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			continue
		}
		if config.Valid {
			ws.Config = config.String
		}
		workspaces = append(workspaces, ws)
	}

	if workspaces == nil {
		workspaces = []Workspace{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workspaces)
}

// HandleCreate 创建新工作空间。
func (s *Service) HandleCreate(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	ws, err := s.Create(req.Name, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ws)
}