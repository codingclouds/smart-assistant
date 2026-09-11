package store

import (
	"database/sql"
	"fmt"
)

// migrate 执行 Schema 迁移。生产环境应使用版本化迁移工具。
func migrate(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS users (
		    id TEXT PRIMARY KEY,
		    username TEXT UNIQUE NOT NULL,
		    password_hash TEXT NOT NULL,
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS workspaces (
		    id TEXT PRIMARY KEY,
		    user_id TEXT NOT NULL REFERENCES users(id),
		    name TEXT NOT NULL,
		    config TEXT,
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS sessions (
		    id TEXT PRIMARY KEY,
		    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
		    title TEXT,
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS messages (
		    id TEXT PRIMARY KEY,
		    session_id TEXT NOT NULL REFERENCES sessions(id),
		    role TEXT NOT NULL,
		    content TEXT NOT NULL,
		    token_count INTEGER DEFAULT 0,
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS memories (
		    id TEXT PRIMARY KEY,
		    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
		    content TEXT NOT NULL,
		    source_session_id TEXT REFERENCES sessions(id),
		    enabled INTEGER DEFAULT 1,
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS skills (
		    id TEXT PRIMARY KEY,
		    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
		    name TEXT NOT NULL,
		    version TEXT NOT NULL,
		    enabled INTEGER DEFAULT 1,
		    installed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS traces (
		    id TEXT PRIMARY KEY,
		    session_id TEXT NOT NULL REFERENCES sessions(id),
		    agent_id TEXT NOT NULL,
		    event_type TEXT NOT NULL,
		    payload TEXT NOT NULL,
		    duration_ms INTEGER,
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS token_stats (
		    id TEXT PRIMARY KEY,
		    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
		    session_id TEXT REFERENCES sessions(id),
		    model_name TEXT NOT NULL,
		    input_tokens INTEGER DEFAULT 0,
		    output_tokens INTEGER DEFAULT 0,
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS models (
		    id TEXT PRIMARY KEY,
		    name TEXT NOT NULL,
		    base_url TEXT NOT NULL DEFAULT '',
		    api_format TEXT NOT NULL DEFAULT 'openai',
		    config TEXT NOT NULL DEFAULT '{}',
		    enabled INTEGER DEFAULT 1,
		    is_default INTEGER DEFAULT 0,
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_sessions_workspace ON sessions(workspace_id);
		CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
		CREATE INDEX IF NOT EXISTS idx_memories_workspace ON memories(workspace_id);
		CREATE INDEX IF NOT EXISTS idx_skills_workspace ON skills(workspace_id);
		CREATE INDEX IF NOT EXISTS idx_traces_session ON traces(session_id);
		CREATE INDEX IF NOT EXISTS idx_token_stats_workspace ON token_stats(workspace_id);
		`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}

	// 兼容老数据库：补 api_format 列（SQLite 不支持 IF NOT EXISTS for ALTER TABLE，忽略错误）
	db.Exec(`ALTER TABLE models ADD COLUMN api_format TEXT NOT NULL DEFAULT 'openai'`)

	// 兼容老数据库：补 is_default 列
	db.Exec(`ALTER TABLE models ADD COLUMN is_default INTEGER DEFAULT 0`)

	// 为 messages 表补 blocks 列（存储工具调用、文件产物等结构化数据）
	db.Exec(`ALTER TABLE messages ADD COLUMN blocks TEXT NOT NULL DEFAULT '[]'`)

	return nil
}