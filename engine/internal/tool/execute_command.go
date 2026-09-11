package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/smart-assistant/engine/internal/workspace"
)

// ExecuteCommandTool 在工作空间环境中执行 shell 命令的工具。
// 命令通过 bash -c 运行，有 30 秒超时。危险命令模式会被拒绝。
type ExecuteCommandTool struct{}

// NewExecuteCommandTool 创建一个新的 ExecuteCommandTool 实例。
func NewExecuteCommandTool() Tool {
	return &ExecuteCommandTool{}
}

func (t *ExecuteCommandTool) Definition() Definition {
	return Definition{
		Name:        "execute_command",
		Description: "在工作空间环境中执行 shell 命令。命令在 30 秒超时内通过 bash -c 运行。用于文件操作、运行脚本或系统任务。危险命令（如 rm -rf /、sudo）会被拒绝。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "要执行的 shell 命令。将通过 bash -c 运行。使用标准 shell 语法。",
				},
				"working_dir": map[string]interface{}{
					"type":        "string",
					"description": "可选的工作目录，相对于工作空间根目录。默认为工作空间根目录。",
				},
			},
			"required": []string{"command"},
		},
	}
}

// dangerousPatterns 包含被拒绝执行的命令模式。
// v1 使用 deny-list 方式；未来可加入沙箱执行。
var dangerousPatterns = []string{
	"rm -rf /",
	"rm -rf ~",
	"rm -rf /*",
	"sudo ",
	"mkfs.",
	"dd if=",
	"> /dev/sda",
	"chmod 777 /",
	"chown -R /",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
	"init 0",
	"init 6",
	"systemctl stop",
	"docker rm",
	"docker rmi",
	"kill -9",
}

// execResult JSON 结构体，由 Execute 返回，供 engine 层解析 file_info。
type execResult struct {
	ExitCode     int         `json:"exit_code"`
	Stdout       string      `json:"stdout"`
	Stderr       string      `json:"stderr"`
	TextOutput   string      `json:"text_output"`
	FilesCreated []fileEntry `json:"files_created"`
}

// fileEntry 描述命令执行后在工作空间中检测到的新文件。
type fileEntry struct {
	FilePath    string `json:"file_path"`
	FileName    string `json:"file_name"`
	ByteCount   int    `json:"byte_count"`
	WorkspaceID string `json:"workspace_id"`
}

func (t *ExecuteCommandTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 提取参数
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("command is required and must be a non-empty string")
	}

	// 获取 workspace ID
	workspaceID, found := workspace.WorkspaceIDFromContext(ctx)
	if !found || workspaceID == "" {
		return "", fmt.Errorf("workspace context not available: cannot determine execution environment")
	}

	workspaceRoot := workspace.FilesDir(workspaceID)

	// 安全检查
	if err := checkCommandSafety(command); err != nil {
		return "", err
	}

	// 计算工作目录
	workingDir := workspaceRoot
	if wd, ok := args["working_dir"].(string); ok && wd != "" {
		resolved, err := resolveSafePath(workspaceRoot, wd)
		if err != nil {
			return "", fmt.Errorf("invalid working directory: %w", err)
		}
		workingDir = resolved
	}

	// 确保工作目录存在
	if err := ensureDir(workingDir); err != nil {
		return "", fmt.Errorf("cannot access working directory: %w", err)
	}

	// 创建带超时的执行 context
	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 执行前快照：记录工作空间中所有文件及其大小和修改时间，用于检测命令是否创建/修改了文件
	beforeSnap, beforeErr := snapshotFiles(workspaceRoot)
	log.Printf("[exec-cmd] BEFORE snapshot: root=%s workingDir=%s count=%d err=%v", workspaceRoot, workingDir, len(beforeSnap), beforeErr)
	for k, v := range beforeSnap {
		log.Printf("[exec-cmd] BEFORE: %s (%d bytes, mtime=%s)", k, v.Size, v.ModTime.Format(time.RFC3339))
	}

	// 构建命令
	cmd := exec.CommandContext(execCtx, "bash", "-c", command)
	cmd.Dir = workingDir

	// 捕获 stdout 和 stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// 执行命令
	runErr := cmd.Run()

	// 检查超时
	if execCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after 30 seconds: %s", command)
	}

	// 收集退出码
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("command execution failed: %w", runErr)
		}
	}

	// 格式化文本输出（给人阅读）
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	var textSb strings.Builder
	textSb.WriteString(fmt.Sprintf("Exit Code: %d\n", exitCode))
	if stdoutStr != "" {
		textSb.WriteString("STDOUT:\n")
		textSb.WriteString(stdoutStr)
		if !strings.HasSuffix(stdoutStr, "\n") {
			textSb.WriteString("\n")
		}
	}
	if stderrStr != "" {
		textSb.WriteString("STDERR:\n")
		textSb.WriteString(stderrStr)
		if !strings.HasSuffix(stderrStr, "\n") {
			textSb.WriteString("\n")
		}
	}
	if stdoutStr == "" && stderrStr == "" {
		textSb.WriteString("(no output)\n")
	}

	// 执行后快照：检测新创建或修改的文件
	result := execResult{
		ExitCode:   exitCode,
		Stdout:     stdoutStr,
		Stderr:     stderrStr,
		TextOutput: textSb.String(),
	}

	afterSnap, afterErr := snapshotFiles(workspaceRoot)
	log.Printf("[exec-cmd] AFTER snapshot: count=%d err=%v", len(afterSnap), afterErr)
	for k, v := range afterSnap {
		log.Printf("[exec-cmd] AFTER: %s (%d bytes, mtime=%s)", k, v.Size, v.ModTime.Format(time.RFC3339))
	}
	for path, info := range afterSnap {
		beforeInfo, existed := beforeSnap[path]
		if !existed {
			// 全新文件 — 之前不存在
			log.Printf("[exec-cmd] NEW FILE DETECTED: %s (%d bytes, mtime=%s)", path, info.Size, info.ModTime.Format(time.RFC3339))
			result.FilesCreated = append(result.FilesCreated, fileEntry{
				FilePath:    path,
				FileName:    filepath.Base(path),
				ByteCount:   int(info.Size),
				WorkspaceID: workspaceID,
			})
		} else if !beforeInfo.ModTime.Equal(info.ModTime) {
			// 文件已存在但被重新生成（修改时间不同）—— 仍视为本次执行产物
			log.Printf("[exec-cmd] MODIFIED FILE DETECTED: %s was %d bytes (mtime=%s), now %d bytes (mtime=%s)",
				path, beforeInfo.Size, beforeInfo.ModTime.Format(time.RFC3339),
				info.Size, info.ModTime.Format(time.RFC3339))
			result.FilesCreated = append(result.FilesCreated, fileEntry{
				FilePath:    path,
				FileName:    filepath.Base(path),
				ByteCount:   int(info.Size),
				WorkspaceID: workspaceID,
			})
		}
	}
	if len(result.FilesCreated) == 0 && len(afterSnap) > 0 {
		log.Printf("[exec-cmd] NO new/modified files detected. before=%d after=%d", len(beforeSnap), len(afterSnap))
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// checkCommandSafety 检查命令是否匹配已知的危险模式。
func checkCommandSafety(command string) error {
	lower := strings.ToLower(strings.TrimSpace(command))
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lower, pattern) {
			return fmt.Errorf("dangerous command pattern detected: %q contains %q. This command is blocked for system safety.", command, pattern)
		}
	}
	return nil
}

// ensureDir 确保目录存在，不存在则创建。
func ensureDir(path string) error {
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return nil
		}
		return fmt.Errorf("path exists but is not a directory: %s", path)
	}
	return os.MkdirAll(path, 0755)
}

// fileSnapInfo 记录文件快照时的元数据，用于检测命令执行后的文件变化。
// 同时跟踪文件大小和修改时间，避免仅凭路径+大小未能发现同内容覆盖的情况。
type fileSnapInfo struct {
	Size    int64
	ModTime time.Time
}

// snapshotFiles 遍历工作空间根目录并返回相对路径 → 文件元数据的映射。
// 仅遍历普通文件，跳过目录和符号链接。用于命令执行前后的文件变化检测。
func snapshotFiles(root string) (map[string]fileSnapInfo, error) {
	snap := make(map[string]fileSnapInfo)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}
		if info.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		snap[rel] = fileSnapInfo{Size: info.Size(), ModTime: info.ModTime()}
		return nil
	})
	return snap, err
}

// 编译期检查 Tool 接口实现。
var _ Tool = (*ExecuteCommandTool)(nil)