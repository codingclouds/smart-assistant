package skill

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// Skill 表示一个已安装的 Skill。
type Skill struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Enabled     bool   `json:"enabled"`
	InstalledAt string `json:"installed_at"`
}

// MarketSkill 表示市场中可用的 Skill。
type MarketSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`
}

// Service Skill 管理。
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// ListAll 返回所有已安装的 Skill。
func (s *Service) ListAll() ([]Skill, error) {
	rows, err := s.db.Query(
		`SELECT id, workspace_id, name, version, enabled, installed_at
		 FROM skills ORDER BY installed_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var sk Skill
		if err := rows.Scan(&sk.ID, &sk.WorkspaceID, &sk.Name, &sk.Version, &sk.Enabled, &sk.InstalledAt); err != nil {
			continue
		}
		skills = append(skills, sk)
	}
	return skills, nil
}

// Install 安装一个 Skill 并返回。
func (s *Service) Install(name, version string) (*Skill, error) {
	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT INTO skills (id, workspace_id, name, version, enabled) VALUES (?, ?, ?, ?, ?)`,
		id, "default", name, version, 1,
	)
	if err != nil {
		return nil, err
	}
	return &Skill{ID: id, Name: name, Version: version, Enabled: true}, nil
}

// HandleList 返回已安装的 Skill 列表。
func (s *Service) HandleList(w http.ResponseWriter, r *http.Request) {
	skills, err := s.ListAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if skills == nil {
		skills = []Skill{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(skills)
}

// HandleInstall 安装一个 Skill。
func (s *Service) HandleInstall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	sk, err := s.Install(req.Name, req.Version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sk)
}