package memory

// inject.go — 记忆片段选择性注入上下文（v1: 接口骨架，M3 完整实现）。

// Injector 管理记忆片段的上下文注入策略。
type Injector struct {
	enabled bool
}

func NewInjector() *Injector {
	return &Injector{enabled: true}
}

// GetRelevantMemories 获取与当前对话相关的记忆片段。
// v1: 返回空列表，M3 实现基于向量相似度的检索。
func (i *Injector) GetRelevantMemories(workspaceID string, currentPrompt string, limit int) ([]MemoryFragment, error) {
	return nil, nil
}