package workspace

import (
	"context"
	"os"
	"path/filepath"
)

type contextKey string

// CtxKeyWorkspaceID 用于在 context 中传递 workspace ID。
// 工具通过 workspace.WorkspaceIDFromContext(ctx) 获取它来计算文件操作根目录。
const CtxKeyWorkspaceID contextKey = "workspaceID"

// WorkspaceIDFromContext 从 context 中提取 workspace ID。
// 如果 context 中没有设置，返回空字符串和 false。
func WorkspaceIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(CtxKeyWorkspaceID).(string)
	return id, ok
}

// FilesDir 返回指定 workspace 的文件系统根目录。
// 路径：~/.smart-assistant/workspaces/{workspaceID}/files/
func FilesDir(workspaceID string) string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".smart-assistant", "workspaces", workspaceID, "files")
}