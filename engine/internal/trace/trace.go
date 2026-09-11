package trace

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// EventType 追踪事件类型。
type EventType string

const (
	EventToolCall EventType = "tool_call"
	EventThought  EventType = "thought"
	EventResponse EventType = "response"
	EventError    EventType = "error"
)

// Event 表示一次执行追踪事件。
type Event struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	AgentID    string    `json:"agent_id"`
	EventType  EventType `json:"event_type"`
	Payload    string    `json:"payload"`
	DurationMs int       `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// Service 执行追踪服务。
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Record 记录一条追踪事件。
func (s *Service) Record(sessionID, agentID string, eventType EventType, payload interface{}, durationMs int) error {
	id := uuid.New().String()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		`INSERT INTO traces (id, session_id, agent_id, event_type, payload, duration_ms) VALUES (?, ?, ?, ?, ?, ?)`,
		id, sessionID, agentID, string(eventType), string(payloadBytes), durationMs,
	)
	return err
}

// GetBySession 返回指定会话的追踪事件。
func (s *Service) GetBySession(sessionID string) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, agent_id, event_type, payload, duration_ms, created_at
		 FROM traces WHERE session_id = ? ORDER BY created_at ASC`, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var evt Event
		var duration sql.NullInt64
		if err := rows.Scan(&evt.ID, &evt.SessionID, &evt.AgentID, &evt.EventType, &evt.Payload, &duration, &evt.CreatedAt); err != nil {
			continue
		}
		if duration.Valid {
			evt.DurationMs = int(duration.Int64)
		}
		events = append(events, evt)
	}
	return events, nil
}

// HandleGet 获取指定会话的追踪日志。
func (s *Service) HandleGet(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")

	events, err := s.GetBySession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if events == nil {
		events = []Event{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}