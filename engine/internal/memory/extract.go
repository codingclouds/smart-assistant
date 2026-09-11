package memory

// extract.go — 长期记忆摘要提取（v1: 接口骨架，M3 完整实现）。

import (
	"database/sql"
)

// Extractor 从对话历史中提取关键信息生成记忆片段。
type Extractor struct {
	db *sql.DB
}

func NewExtractor(db *sql.DB) *Extractor {
	return &Extractor{db: db}
}

// ExtractKeyPoints 从消息列表中提取关键记忆点。
// v1: 保留接口，M3 实现基于 LLM 的摘要提取。
func (e *Extractor) ExtractKeyPoints(sessionID string) ([]string, error) {
	return nil, nil
}