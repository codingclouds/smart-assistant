package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/smart-assistant/engine/internal/workspace"
)

// WriteFileTool 将内容写入工作空间文件系统的工具。
// 文件路径相对于 workspace root（~/.smart-assistant/workspaces/{id}/files/）。
// 自动创建不存在的父级目录。
type WriteFileTool struct{}

// NewWriteFileTool 创建一个新的 WriteFileTool 实例。
func NewWriteFileTool() Tool {
	return &WriteFileTool{}
}

func (t *WriteFileTool) Definition() Definition {
	return Definition{
		Name:        "write_file",
		Description: "将内容写入工作空间中的文件。自动创建不存在的父级目录。用于保存文档、代码文件、报告或任何生成的内容。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "要写入的文件路径，相对于工作空间根目录（例如 'docs/report.md'、'src/main.go'）。不得包含 '..' 路径穿越或绝对路径。",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "要写入文件的完整文本内容。",
				},
			},
			"required": []string{"file_path", "content"},
		},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 提取参数
	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		return "", fmt.Errorf("file_path is required and must be a non-empty string")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required and must be a string")
	}

	// 检查是否为二进制格式文件 → 拒绝并引导 AI 使用脚本方式
	if err := checkBinaryFormat(filePath, content); err != nil {
		return "", err
	}

	// 获取 workspace ID
	workspaceID, found := workspace.WorkspaceIDFromContext(ctx)
	if !found || workspaceID == "" {
		return "", fmt.Errorf("workspace context not available: cannot determine file storage location")
	}

	// 计算安全路径
	workspaceRoot := workspace.FilesDir(workspaceID)
	fullPath, err := resolveSafePath(workspaceRoot, filePath)
	if err != nil {
		return "", err
	}

	// 确保父目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// 检查文件是否已存在 — 若存在且未带 __confirm_response，返回 confirm_required 信号
	// loop 层检测到此信号后统一发送 confirm_request，用户选择后带 __confirm_response 重新执行
	confirmResponse, hasConfirm := args["__confirm_response"].(string)
	if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
		if hasConfirm {
			// 用户已确认：根据选择决定操作
			switch confirmResponse {
			case "覆盖":
				// 继续执行写入（fall through）
			case "备份后覆盖":
				// 将旧文件重命名为 .bak 备份（若 .bak 已存在则先移除）
				backupPath := fullPath + ".bak"
				os.Remove(backupPath) // 忽略错误（可能不存在）
				if err := os.Rename(fullPath, backupPath); err != nil {
					return "", fmt.Errorf("failed to backup file %s: %w", filePath, err)
				}
				// 继续执行写入
			case "保留两者":
				// 新文件用带编号的名称写入（如 hello.docx → hello_1.docx）
				newPath := generateAltPath(fullPath)
				fullPath = newPath
				// 从 fullPath 反推新的相对路径用于返回
				relPath, _ := filepath.Rel(workspaceRoot, newPath)
				filePath = relPath
				// 确保新路径的父目录存在
				dir := filepath.Dir(fullPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
				}
				// 继续执行写入
			default:
				// 用户选择了「跳过」或其他未定义选项，取消写入
				result := map[string]interface{}{
					"status":     "cancelled",
					"file_path":  filePath,
					"file_name":  filepath.Base(filePath),
					"message":    fmt.Sprintf("用户选择「%s」，跳过写入 %s", confirmResponse, filePath),
				}
				resultJSON, _ := json.Marshal(result)
				return string(resultJSON), nil
			}
		} else {
			// 文件已存在，返回确认请求 — 选项由工具根据场景动态决定
			result := map[string]interface{}{
				"status": "confirm_required",
				"confirm": map[string]interface{}{
					"question": fmt.Sprintf("文件「%s」已存在（%d bytes, %s），如何处理？",
						filePath, info.Size(), info.ModTime().Format(time.RFC3339)),
					"options": []string{"覆盖", "备份后覆盖", "保留两者", "跳过"},
					"context": fmt.Sprintf("目标路径: %s\n已有大小: %d bytes\n修改时间: %s\n\n覆盖 — 直接替换旧文件\n备份后覆盖 — 旧文件重命名为 .bak 备份后写入新文件\n保留两者 — 新文件用带编号的名称写入（如 hello_1.docx）\n跳过 — 保持旧文件不变",
						filePath, info.Size(), info.ModTime().Format(time.RFC3339)),
				},
			}
			resultJSON, _ := json.Marshal(result)
			return string(resultJSON), nil
		}
	}

	// 写入文件
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	// 计算写入的字节数，返回结构化 JSON 方便前端渲染文件卡片
	byteCount := len([]byte(content))
	result := map[string]interface{}{
		"status":       "success",
		"file_path":    filePath,
		"file_name":    filepath.Base(filePath),
		"byte_count":   byteCount,
		"workspace_id": workspaceID,
		"message":      fmt.Sprintf("Successfully wrote %d bytes to %s", byteCount, filePath),
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// resolveSafePath 将用户提供的相对路径解析为 workspace 根目录内的安全绝对路径。
// 拒绝绝对路径和路径穿越攻击（如 ../）。
func resolveSafePath(workspaceRoot, filePath string) (string, error) {
	// 清理输入路径（去除多余斜杠、解析 . 等）
	cleanPath := filepath.Clean(filePath)

	// 拒绝绝对路径 — 所有路径必须相对于 workspace
	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("absolute paths are not allowed, use a relative path within the workspace: %s", filePath)
	}

	// 连接 workspace 根目录
	fullPath := filepath.Join(workspaceRoot, cleanPath)
	// 再次清理以解析任何 .. 组件
	fullPath = filepath.Clean(fullPath)

	// 确保结果在 workspace 根目录内
	sep := string(filepath.Separator)
	if !strings.HasPrefix(fullPath, workspaceRoot+sep) && fullPath != workspaceRoot {
		return "", fmt.Errorf("path traversal detected: %q resolves outside workspace", filePath)
	}

	return fullPath, nil
}

// binaryFormats 列出必须通过脚本生成的二进制/复合文件格式，不能直接用 write_file 写入。
var binaryFormats = map[string]string{
	".docx": "Word 文档",
	".doc":  "Word 文档（旧版）",
	".xlsx": "Excel 表格",
	".xls":  "Excel 表格（旧版）",
	".pptx": "PowerPoint 演示文稿",
	".ppt":  "PowerPoint 演示文稿（旧版）",
	".pdf":  "PDF 文档",
	".odt":  "OpenDocument 文本",
	".ods":  "OpenDocument 表格",
	".odp":  "OpenDocument 演示文稿",
}

// checkBinaryFormat 检查文件扩展名是否为二进制格式。
// 若是，返回错误并引导 AI 使用 write_file + execute_command 脚本方式生成。
func checkBinaryFormat(filePath, content string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	formatName, isBinary := binaryFormats[ext]
	if !isBinary {
		return nil
	}

	// 如果内容是有效的文本（如完整的 Python 脚本），允许通过
	// 这样可以写一个 .docx 生成脚本而不会被误拦截——但脚本不会用 .docx 扩展名，
	// 所以这个分支主要防御二进制内容
	contentBytes := []byte(content)
	if len(contentBytes) > 0 && contentBytes[0] < 32 && contentBytes[0] != '\t' && contentBytes[0] != '\n' && contentBytes[0] != '\r' {
		return fmt.Errorf(
			"write_file 不支持直接写入 %s（%s 是二进制复合格式）。"+
				"请改为：1) 用 write_file 将 Python 生成脚本写入工作空间（如 generate.py）；"+
				"2) 用 execute_command 执行 `python3 generate.py`；"+
				"3) 系统会自动检测生成的文件并以卡片展示。",
			filePath, formatName,
		)
	}
	return nil
}

// generateAltPath 为已存在的文件路径生成带编号的替代路径。
// 例如 hello.docx → hello_1.docx，若 hello_1.docx 也存在则递增到 hello_2.docx。
func generateAltPath(fullPath string) string {
	dir := filepath.Dir(fullPath)
	ext := filepath.Ext(fullPath)
	base := strings.TrimSuffix(filepath.Base(fullPath), ext)

	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	// 兜底：理论上不会到达（1000 个同名文件极端罕见）
	return filepath.Join(dir, fmt.Sprintf("%s_new%s", base, ext))
}

// 编译期检查 Tool 接口实现。
var _ Tool = (*WriteFileTool)(nil)