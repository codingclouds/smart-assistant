package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/smart-assistant/engine/internal/auth"
	"github.com/smart-assistant/engine/internal/model"
	"github.com/smart-assistant/engine/internal/session"
	"github.com/smart-assistant/engine/internal/tool"
	"github.com/smart-assistant/engine/internal/trace"
	"github.com/smart-assistant/engine/internal/workspace"
)

// Loop 是 Agent 核心对话循环：prompt→model→tools→response。
// v1 支持单 Agent 流式对话及取消（HITL 基础版）。
type Loop struct {
	modelRegistry    *model.Registry
	sessionService   *session.Service
	traceService     *trace.Service
	workspaceService *workspace.Service
	toolRegistry     *tool.Registry
}

// LoopConfig Agent 循环初始化配置。
type LoopConfig struct {
	ModelRegistry    *model.Registry
	SessionService   *session.Service
	TraceService     *trace.Service
	WorkspaceService *workspace.Service
	ToolRegistry     *tool.Registry
}

// 存储活跃的执行上下文，支持取消操作。
var (
	activeRuns   = make(map[string]context.CancelFunc)
	activeRunsMu sync.Mutex
)

func NewLoop(cfg LoopConfig) *Loop {
	return &Loop{
		modelRegistry:    cfg.ModelRegistry,
		sessionService:   cfg.SessionService,
		traceService:     cfg.TraceService,
		workspaceService: cfg.WorkspaceService,
		toolRegistry:     cfg.ToolRegistry,
	}
}

// ChatRequest 前端发送的对话请求。
type ChatRequest struct {
	Prompt      string `json:"prompt"`
	SessionID   string `json:"session_id"`
	WorkspaceID string `json:"workspace_id"`
	ModelName   string `json:"model_name"`
	Title       string `json:"title"`
}

// ChatEvent 对话流事件。
type ChatEvent struct {
	Type string      // "meta" | "token" | "thinking" | "tool_call" | "tool_result" | "done" | "error"
	Data interface{}
}

// 工具结果大小上限（字符数），超出部分截断并附加提示。
// 参考 Hermes enforce_turn_budget：防止大结果撑爆上下文窗口。
const maxToolResultChars = 50000

// 迭代预算告警（参考 Hermes _maybe_inject_iteration_budget_warning）。
const iterationBudgetWarning = "[SYSTEM NOTICE — 已达到最大工具调用次数] 这是最后一次工具调用机会。" +
	"请在本次调用后基于已有信息完成用户任务，不要再发起新的工具调用。直接给出最终回答。"

// truncateToolResult 截断过大的工具结果，防止上下文窗口爆炸。
func truncateToolResult(output string) string {
	if len(output) <= maxToolResultChars {
		return output
	}
	half := maxToolResultChars / 2
	return output[:half] +
		fmt.Sprintf("\n\n... [结果过长，已截断。完整输出共 %d 字符，此处仅显示前 %d 和后 %d 字符] ...\n\n",
			len(output), half, half) +
		output[len(output)-half:]
}

// formatToolError 将工具错误格式化为结构化 JSON（参考 Hermes 错误分类模式）。
// 结构化格式帮助模型更好地理解失败原因并决定后续策略。
func formatToolError(toolName, errorMsg, output string) string {
	errJSON := map[string]interface{}{
		"error":       errorMsg,
		"tool":        toolName,
		"status":      "failed",
		"output":      truncateToolResult(output),
		"suggestion":  "请基于已有知识继续回答，不要再次调用该工具。若无法回答，诚实地告知用户并建议替代方案。",
	}
	jsonBytes, _ := json.Marshal(errJSON)
	return string(jsonBytes)
}

// HandleChatWS 处理 WebSocket 版本的对说请求，通过 channel 返回流事件。
func (l *Loop) HandleChatWS(ctx context.Context, userID string, req ChatRequest) (<-chan ChatEvent, error) {
	if req.ModelName == "" {
		req.ModelName = "default"
	}

	sess, err := l.getOrCreateSession(userID, req.SessionID, req.WorkspaceID, req.Title)
	if err != nil {
		return nil, fmt.Errorf("session error: %w", err)
	}

	ch := make(chan ChatEvent, 32)

	go func() {
		defer close(ch)

		// 创建可取消的 context
		runCtx, cancel := context.WithCancel(ctx)
		runID := uuid.New().String()

		// 注入 workspace ID 到 context，供工具在运行时获取文件操作根目录
		if sess.WorkspaceID != "" {
			runCtx = context.WithValue(runCtx, workspace.CtxKeyWorkspaceID, sess.WorkspaceID)
		}

		activeRunsMu.Lock()
		activeRuns[runID] = cancel
		activeRunsMu.Unlock()

		defer func() {
			activeRunsMu.Lock()
			delete(activeRuns, runID)
			activeRunsMu.Unlock()
			cancel()
		}()

		// 发送 meta 事件
		ch <- ChatEvent{
			Type: "meta",
			Data: map[string]string{
				"run_id":     runID,
				"session_id": sess.ID,
			},
		}

		// 保存用户消息
		l.sessionService.AddMessage(sess.ID, "user", req.Prompt, approximateTokens(req.Prompt), "")
		startTime := time.Now()

		// 获取历史消息
		historyMsgs, _ := l.sessionService.GetMessages(sess.ID)
		modelMsgs := convertToModelMessages(historyMsgs)

		// 注入系统提示词：当前日期时间
		now := time.Now()
		weekdayNames := []string{"日", "一", "二", "三", "四", "五", "六"}
		systemMsg := model.Message{
			Role: "system",
			Content: fmt.Sprintf(
				"当前日期时间是 %s（星期%s）。请在回答中需要参考时间时以此为准。",
				now.Format("2006-01-02 15:04:05"),
				weekdayNames[now.Weekday()],
			),
		}
		modelMsgs = append([]model.Message{systemMsg}, modelMsgs...)

		// 获取模型配置
		modelCfg, err := l.modelRegistry.GetConfig(req.ModelName)
		if err != nil {
			ch <- ChatEvent{Type: "error", Data: fmt.Sprintf("模型配置错误：%v，请在设置中检查模型配置是否正确", err)}
			return
		}

		// 工具循环：最多 5 次迭代
		const maxToolIterations = 5
		var fullText strings.Builder

		for iteration := 0; iteration < maxToolIterations; iteration++ {
			select {
			case <-runCtx.Done():
				ch <- ChatEvent{Type: "error", Data: "cancelled"}
				return
			default:
			}

			// 创建 provider 并设置 tools
			provider := l.modelRegistry.NewProvider(modelCfg.ApiFormat)

			// 获取工具定义并注入 system prompt
			toolDefs := l.getToolDefs()
			systemWithTools := buildSystemPromptWithTools(systemMsg.Content, toolDefs)

			// 更新第一条 system 消息以包含工具描述
			msgsCopy := make([]model.Message, len(modelMsgs))
			copy(msgsCopy, modelMsgs)
			if len(msgsCopy) > 0 && msgsCopy[0].Role == "system" {
				msgsCopy[0].Content = systemWithTools
			}

			// 设置 tools
			if tp, ok := provider.(model.ToolsProvider); ok && len(toolDefs) > 0 {
				tp.SetTools(toolDefs)
			}

			// 迭代预算告警：在最后一次工具调用前注入系统提醒（参考 Hermes _maybe_inject_iteration_budget_warning）。
			if iteration == maxToolIterations-1 {
				msgsCopy = append(msgsCopy, model.Message{
					Role:    "user",
					Content: iterationBudgetWarning,
				})
			}

			chunks, err := provider.StreamChat(runCtx, req.ModelName, msgsCopy, *modelCfg)
			if err != nil {
				l.traceService.Record(sess.ID, "main", trace.EventError, map[string]string{
					"error": err.Error(),
				}, 0)
				ch <- ChatEvent{Type: "error", Data: fmt.Sprintf("model error: %v", err)}
				return
			}

			// 收集本轮响应
			var iterText strings.Builder
			var iterThinking strings.Builder
			var toolCalls []model.ToolCall

			for chunk := range chunks {
				if chunk.Error != "" {
					l.traceService.Record(sess.ID, "main", trace.EventError, map[string]string{
						"error": chunk.Error,
					}, int(time.Since(startTime).Milliseconds()))
					ch <- ChatEvent{Type: "error", Data: chunk.Error}
					return
				}

				if chunk.Content != "" {
					iterText.WriteString(chunk.Content)
					ch <- ChatEvent{Type: "token", Data: chunk.Content}
				}

				// 转发模型推理/思考内容，同时累积（推理模型可能只输出 reasoning_content）
				// 当 content 为空时，也将 thinking 作为 token 发送，确保前端实时显示
				if chunk.Thinking != "" {
					iterThinking.WriteString(chunk.Thinking)
					ch <- ChatEvent{Type: "thinking", Data: chunk.Thinking}
					if chunk.Content == "" {
						ch <- ChatEvent{Type: "token", Data: chunk.Thinking}
					}
				}

				// 收集工具调用
				for _, tc := range chunk.ToolCalls {
					toolCalls = append(toolCalls, model.ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: model.ToolCallFunction{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				}

				if chunk.Done {
					break
				}
			}

			// 构建本轮实际回复文本：优先用 content，若为空则回退到 thinking（推理模型场景）
			iterOutput := iterText.String()
			if iterOutput == "" {
				iterOutput = iterThinking.String()
			}

			// 没有工具调用 → 对话结束
			if len(toolCalls) == 0 {
				fullText.WriteString(iterOutput)
				elapsed := int(time.Since(startTime).Milliseconds())
				finalText := fullText.String()

				l.sessionService.AddMessage(sess.ID, "assistant", finalText, approximateTokens(finalText), "")
				l.traceService.Record(sess.ID, "main", trace.EventResponse, map[string]interface{}{
					"model":      req.ModelName,
					"content":    finalText[:min(len(finalText), 200)],
					"elapsed":    elapsed,
					"iterations": iteration + 1,
				}, elapsed)

				ch <- ChatEvent{
					Type: "done",
					Data: map[string]interface{}{
						"session_id":  sess.ID,
						"elapsed_ms":  elapsed,
						"token_count": approximateTokens(finalText),
					},
				}
				return
			}

			// 有工具调用：发送事件 + 执行 + 追加到消息历史
			fullText.WriteString(iterOutput)

			// 构造 assistant 消息（含文本 + 工具调用）
			assistantMsg := model.Message{
				Role:      "assistant",
				Content:   iterOutput,
				ToolCalls: toolCalls,
			}
			modelMsgs = append(modelMsgs, assistantMsg)

			for _, tc := range toolCalls {
				// 解析参数
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					log.Printf("[agent] tool call args parse error: %v raw=%s", err, tc.Function.Arguments)
					args = map[string]interface{}{"raw": tc.Function.Arguments}
				}

				// === ask_user 特殊处理：人机交互确认流程 ===
				if tc.Function.Name == "ask_user" {
					question, _ := args["question"].(string)
					if question == "" {
						question = "请确认此操作"
					}
					contextStr, _ := args["context"].(string)
					var options []string
					if rawOpts, ok := args["options"]; ok {
						switch opts := rawOpts.(type) {
						case []interface{}:
							for _, o := range opts {
								if s, ok := o.(string); ok {
									options = append(options, s)
								}
							}
						}
					}

					// 发送 tool_call 事件（前端展示确认卡片头部）
					ch <- ChatEvent{
						Type: "tool_call",
						Data: map[string]interface{}{
							"call_id":   tc.ID,
							"name":      tc.Function.Name,
							"arguments": tc.Function.Arguments,
						},
					}

					// 发送 confirm_request 事件（前端展示交互表单）
					ch <- ChatEvent{
						Type: "confirm_request",
						Data: map[string]interface{}{
							"call_id":  tc.ID,
							"question": question,
							"options":  options,
							"context":  contextStr,
						},
					}

					// 等待用户响应（最多 5 分钟）
					respCh, err := RegisterConfirm(tc.ID, 5*time.Minute)
					if err != nil {
						log.Printf("[agent] confirm registration error: %v", err)
						ch <- ChatEvent{Type: "error", Data: fmt.Sprintf("确认流程错误: %v", err)}
						return
					}

					// 阻塞等待用户响应或取消
					var userResponse string
					select {
					case <-runCtx.Done():
						CancelConfirm(tc.ID)
						ch <- ChatEvent{Type: "error", Data: "cancelled"}
						return
					case resp := <-respCh:
						userResponse = resp.Response
					}

					// 将用户响应作为 tool_result 发回模型
					resultOutput := fmt.Sprintf("用户回复: %s", userResponse)
					ch <- ChatEvent{
						Type: "tool_result",
						Data: map[string]interface{}{
							"call_id": tc.ID,
							"name":    tc.Function.Name,
							"result":  resultOutput,
						},
					}

					toolMsg := model.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
						Content:    resultOutput,
					}
					modelMsgs = append(modelMsgs, toolMsg)

					l.traceService.Record(sess.ID, "tool", trace.EventResponse, map[string]interface{}{
						"tool":       tc.Function.Name,
						"call_id":    tc.ID,
						"user_input": userResponse,
					}, int(time.Since(startTime).Milliseconds()))

					continue
				}

				// === 普通工具执行 ===
				// 发送 tool_call 事件
				ch <- ChatEvent{
					Type: "tool_call",
					Data: map[string]interface{}{
						"call_id":   tc.ID,
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				}

				// 每步工具调用前检查中断（参考 Hermes _interrupt_requested 预检）。
				// 比只在迭代层面检查更精细，避免已被取消的 run 仍发出工具调用。
				select {
				case <-runCtx.Done():
					ch <- ChatEvent{Type: "error", Data: "cancelled"}
					return
				default:
				}

				// 执行工具
				result := l.toolRegistry.Execute(runCtx, tool.Call{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: args,
				})

				// 工具结果截断（参考 Hermes enforce_turn_budget）。
				truncatedOutput := truncateToolResult(result.Output)

				// === 通用工具确认协议 ===
				// 任何工具需要人机交互确认时，返回 {"status":"confirm_required","confirm":{"question":"...","options":["A","B"],"context":"..."}}
				// loop 统一检测此信号 → 发送 confirm_request → 等待用户选择 → 带 __confirm_response 重新执行工具
				var confirmCheck struct {
					Status  string `json:"status"`
					Confirm struct {
						Question string   `json:"question"`
						Options  []string `json:"options"`
						Context  string   `json:"context"`
					} `json:"confirm"`
				}
				if json.Unmarshal([]byte(result.Output), &confirmCheck) == nil && confirmCheck.Status == "confirm_required" {
					confirm := confirmCheck.Confirm
					log.Printf("[loop] confirm_required: tool=%s question=%q options=%v", result.Name, confirm.Question, confirm.Options)

					// 发送 confirm_request 事件
					ch <- ChatEvent{
						Type: "confirm_request",
						Data: map[string]interface{}{
							"call_id":  tc.ID,
							"question": confirm.Question,
							"options":  confirm.Options,
							"context":  confirm.Context,
						},
					}

					// 等待用户响应（最多 5 分钟）
					respCh, err := RegisterConfirm(tc.ID, 5*time.Minute)
					if err != nil {
						log.Printf("[agent] confirm registration error: %v", err)
						ch <- ChatEvent{Type: "error", Data: fmt.Sprintf("确认流程错误: %v", err)}
						return
					}

					var userChoice string
					select {
					case <-runCtx.Done():
						CancelConfirm(tc.ID)
						ch <- ChatEvent{Type: "error", Data: "cancelled"}
						return
					case resp := <-respCh:
						userChoice = resp.Response
					}

					log.Printf("[loop] confirm: user chose %q for tool=%s", userChoice, result.Name)

					// 带确认结果重新执行工具（复制 args 避免污染原始参数）
					confirmArgs := make(map[string]interface{}, len(args)+1)
					for k, v := range args {
						confirmArgs[k] = v
					}
					confirmArgs["__confirm_response"] = userChoice
					result = l.toolRegistry.Execute(runCtx, tool.Call{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: confirmArgs,
					})
					truncatedOutput = truncateToolResult(result.Output)
				}

				// 发送 tool_result 事件
				toolResultData := map[string]interface{}{
					"call_id": result.CallID,
					"name":    result.Name,
					"result":  truncatedOutput,
					"error":   result.Error,
				}

				// write_file 成功时，解析 JSON 结果并附加 file_info，方便前端渲染文件卡片
				// 中间脚本（.py/.sh 等）不产出卡片，仅作为普通工具调用展示
				if result.Name == "write_file" && result.Error == "" {
					var fileInfo struct {
						FilePath    string `json:"file_path"`
						FileName    string `json:"file_name"`
						ByteCount   int    `json:"byte_count"`
						WorkspaceID string `json:"workspace_id"`
					}
					if err := json.Unmarshal([]byte(result.Output), &fileInfo); err == nil && fileInfo.FilePath != "" {
						log.Printf("[file-info] write_file: fileName=%s ext=%s show=%v", fileInfo.FileName, strings.ToLower(filepath.Ext(fileInfo.FileName)), shouldShowFileArtifact(fileInfo.FileName))
						if shouldShowFileArtifact(fileInfo.FileName) {
							log.Printf("[file-info] write_file: ATTACHED file_info for %s", fileInfo.FileName)
							toolResultData["file_info"] = map[string]interface{}{
								"file_path":    fileInfo.FilePath,
								"file_name":    fileInfo.FileName,
								"byte_count":   fileInfo.ByteCount,
								"workspace_id": fileInfo.WorkspaceID,
							}
						}
					}
				}

				// execute_command 成功时，解析 JSON 结果中的 files_created 数组（参考 Hermes 工具组合模式）。
				// 当 AI 通过 execute_command 执行脚本生成复杂格式文件（.docx/.xlsx 等）时自动检测。
				if result.Name == "execute_command" && result.Error == "" {
					var execResult struct {
						FilesCreated []struct {
							FilePath    string `json:"file_path"`
							FileName    string `json:"file_name"`
							ByteCount   int    `json:"byte_count"`
							WorkspaceID string `json:"workspace_id"`
						} `json:"files_created"`
					}
					if err := json.Unmarshal([]byte(result.Output), &execResult); err == nil {
						log.Printf("[file-info] execute_command: files_created=%d", len(execResult.FilesCreated))
						// 过滤掉中间脚本产物，只展示最终输出文件的卡片
						artifacts := make([]map[string]interface{}, 0, len(execResult.FilesCreated))
						for _, f := range execResult.FilesCreated {
							show := shouldShowFileArtifact(f.FileName)
							log.Printf("[file-info] execute_command: file=%s ext=%s show=%v", f.FileName, strings.ToLower(filepath.Ext(f.FileName)), show)
							if show {
								artifacts = append(artifacts, map[string]interface{}{
									"file_path":    f.FilePath,
									"file_name":    f.FileName,
									"byte_count":   f.ByteCount,
									"workspace_id": f.WorkspaceID,
								})
							}
						}
						if len(artifacts) == 1 {
							toolResultData["file_info"] = artifacts[0]
						} else if len(artifacts) > 1 {
							toolResultData["file_infos"] = artifacts
						}
					}
				}

				ch <- ChatEvent{
					Type: "tool_result",
					Data: toolResultData,
				}

				// 构造 tool 结果消息
				var resultContent string
				if result.Error != "" {
					// 结构化 JSON 错误格式（参考 Hermes 错误分类模式）。
					resultContent = formatToolError(tc.Function.Name, result.Error, result.Output)
				} else {
					resultContent = truncatedOutput
				}
				toolMsg := model.Message{
					Role:       "tool",
					ToolCallID: result.CallID,
					Name:       result.Name,
					Content:    resultContent,
				}
				modelMsgs = append(modelMsgs, toolMsg)

				// 增量持久化：每步工具执行后立即保存 assistant + tool 消息（参考 Hermes incremental persistence）。
				// 避免回合中途崩溃导致工具结果全部丢失。

				// 构建 blocks JSON：包含 tool_call、tool_result 和 file_artifact（如有）。
				// 前端加载历史时通过 blocks 重建工具调用卡片和文件产物卡片。
				toolBlocks := []map[string]interface{}{
					{
						"type":      "tool_call",
						"call_id":   tc.ID,
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
					{
						"type":    "tool_result",
						"call_id": result.CallID,
						"name":    result.Name,
						"result":  truncatedOutput,
						"error":   result.Error,
					},
				}
				// 追加 file_artifact 块
				if fi, ok := toolResultData["file_info"]; ok {
					if fiMap, ok2 := fi.(map[string]interface{}); ok2 {
						toolBlocks = append(toolBlocks, map[string]interface{}{
							"type":         "file_artifact",
							"file_path":    fiMap["file_path"],
							"file_name":    fiMap["file_name"],
							"byte_count":   fiMap["byte_count"],
							"workspace_id": fiMap["workspace_id"],
							"call_id":      result.CallID,
						})
					}
				}
				if fis, ok := toolResultData["file_infos"]; ok {
					if fisSlice, ok2 := fis.([]map[string]interface{}); ok2 {
						for _, fi := range fisSlice {
							toolBlocks = append(toolBlocks, map[string]interface{}{
								"type":         "file_artifact",
								"file_path":    fi["file_path"],
								"file_name":    fi["file_name"],
								"byte_count":   fi["byte_count"],
								"workspace_id": fi["workspace_id"],
								"call_id":      result.CallID,
							})
						}
					}
				}
				blocksJSON, _ := json.Marshal(toolBlocks)

				l.sessionService.AddMessage(sess.ID, "assistant", iterOutput,
					approximateTokens(iterOutput), "")
				l.sessionService.AddMessage(sess.ID, "tool", resultContent,
					approximateTokens(resultContent), string(blocksJSON))

				l.traceService.Record(sess.ID, "tool", trace.EventResponse, map[string]interface{}{
					"tool":       result.Name,
					"call_id":    result.CallID,
					"has_error":  result.Error != "",
					"output_len": len(result.Output),
				}, int(time.Since(startTime).Milliseconds()))
			}
		}

		// 超过最大迭代次数，强制保存并结束
		finalText := fullText.String()
		l.sessionService.AddMessage(sess.ID, "assistant", finalText, approximateTokens(finalText), "")
		elapsed := int(time.Since(startTime).Milliseconds())
		ch <- ChatEvent{
			Type: "done",
			Data: map[string]interface{}{
				"session_id":  sess.ID,
				"elapsed_ms":  elapsed,
				"token_count": approximateTokens(finalText),
				"warning":     fmt.Sprintf("reached max tool iterations (%d)", maxToolIterations),
			},
		}
	}()

	return ch, nil
}

func (l *Loop) HandleChat(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	if req.ModelName == "" {
		req.ModelName = "default"
	}

	// 获取或创建 session
	sess, err := l.getOrCreateSession(userID, req.SessionID, req.WorkspaceID, req.Title)
	if err != nil {
		http.Error(w, fmt.Sprintf("session error: %v", err), http.StatusInternalServerError)
		return
	}

	// 设置 SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(r.Context())
	runID := uuid.New().String()
	activeRunsMu.Lock()
	activeRuns[runID] = cancel
	activeRunsMu.Unlock()

	defer func() {
		activeRunsMu.Lock()
		delete(activeRuns, runID)
		activeRunsMu.Unlock()
		cancel()
	}()

	// 在第一次 SSE 事件中返回 runID（用于取消操作）
	sendSSE(w, flusher, "meta", map[string]string{
		"run_id":     runID,
		"session_id": sess.ID,
	})

	// 保存用户消息
	l.sessionService.AddMessage(sess.ID, "user", req.Prompt, approximateTokens(req.Prompt), "")

	startTime := time.Now()

	// 获取历史消息并构造 prompt
	historyMsgs, _ := l.sessionService.GetMessages(sess.ID)
	modelMsgs := convertToModelMessages(historyMsgs)

	// 注入系统提示词：当前日期时间
	now := time.Now()
	weekdayNames := []string{"日", "一", "二", "三", "四", "五", "六"}
	systemMsg := model.Message{
		Role: "system",
		Content: fmt.Sprintf(
			"当前日期时间是 %s（星期%s）。请在回答中需要参考时间时以此为准。",
			now.Format("2006-01-02 15:04:05"),
			weekdayNames[now.Weekday()],
		),
	}
	modelMsgs = append([]model.Message{systemMsg}, modelMsgs...)

	// 获取模型配置
	modelCfg, err := l.modelRegistry.GetConfig(req.ModelName)
	if err != nil {
		sendSSEError(w, flusher, fmt.Sprintf("model config error: %v", err))
		return
	}

	// 调用模型
	provider := l.modelRegistry.NewProvider(modelCfg.ApiFormat)
	chunks, err := provider.StreamChat(ctx, req.ModelName, modelMsgs, *modelCfg)
	if err != nil {
		l.traceService.Record(sess.ID, "main", trace.EventError, map[string]string{
			"error": err.Error(),
		}, 0)
		sendSSEError(w, flusher, fmt.Sprintf("model error: %v", err))
		return
	}

	log.Printf("[handlechat] started reading chunks channel")
	// 流式输出
	var fullText strings.Builder
	var thinkingText strings.Builder

	for chunk := range chunks {
		if chunk.Error != "" {
			l.traceService.Record(sess.ID, "main", trace.EventError, map[string]string{
				"error": chunk.Error,
			}, int(time.Since(startTime).Milliseconds()))
			sendSSEError(w, flusher, chunk.Error)
			return
		}

		if chunk.Content != "" {
			fullText.WriteString(chunk.Content)
			sendSSE(w, flusher, "token", chunk.Content)
		}

		// 处理推理/思考内容：同时作为 thinking 事件和 token（推理模型可能只输出 reasoning_content）
		if chunk.Thinking != "" {
			thinkingText.WriteString(chunk.Thinking)
			sendSSE(w, flusher, "thinking", chunk.Thinking)
			if chunk.Content == "" {
				sendSSE(w, flusher, "token", chunk.Thinking)
			}
		}

		if chunk.Done {
			elapsed := int(time.Since(startTime).Milliseconds())

			// 构建最终回复文本：优先用 content，若为空则回退到 thinking
			finalText := fullText.String()
			if finalText == "" {
				finalText = thinkingText.String()
			}

			// 保存助手消息
			l.sessionService.AddMessage(sess.ID, "assistant", finalText, approximateTokens(finalText), "")

			// 记录 trace
			l.traceService.Record(sess.ID, "main", trace.EventResponse, map[string]interface{}{
				"model":   req.ModelName,
				"content": finalText[:min(len(finalText), 200)],
				"elapsed": elapsed,
			}, elapsed)

			// 发送完成事件
			sendSSE(w, flusher, "done", map[string]interface{}{
				"session_id":  sess.ID,
				"elapsed_ms":  elapsed,
				"token_count": approximateTokens(finalText),
			})
			return
		}
	}
}

// HandleCancel 取消正在执行的 Agent 任务。
func (l *Loop) HandleCancel(runID string) error {
	activeRunsMu.Lock()
	cancel, ok := activeRuns[runID]
	activeRunsMu.Unlock()

	if !ok {
		return fmt.Errorf("run %s not found or already completed", runID)
	}

	cancel()
	return nil
}

// getOrCreateSession 获取或创建会话。
func (l *Loop) getOrCreateSession(userID, sessionID, workspaceID, title string) (*session.Session, error) {
	if sessionID != "" {
		return l.sessionService.GetSession(sessionID)
	}

	// 查找用户工作空间列表
	rows, err := l.workspaceService.GetByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("lookup workspace: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no workspace found for user")
	}

	// 验证 workspaceID 是否有效（属于该用户），否则使用第一个
	validWS := false
	for _, ws := range rows {
		if ws.ID == workspaceID {
			validWS = true
			break
		}
	}
	if !validWS {
		workspaceID = rows[0].ID
	}
	if title == "" {
		title = "新对话"
	}
	log.Printf("[session] creating session for user=%s workspace=%s title=%s", userID, workspaceID, title)
	return l.sessionService.CreateSession(workspaceID, title)
}

// getToolDefs 从 toolRegistry 获取模型格式的工具定义。
func (l *Loop) getToolDefs() []model.ToolDefinition {
	if l.toolRegistry == nil {
		return nil
	}
	defs := l.toolRegistry.Definitions()
	result := make([]model.ToolDefinition, len(defs))
	for i, d := range defs {
		result[i] = model.ToolDefinition{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  d.Parameters,
		}
	}
	return result
}

// GetToolDefs 返回当前注册的所有工具定义（公开方法，供 transport 层使用）。
func (l *Loop) GetToolDefs() []model.ToolDefinition {
	return l.getToolDefs()
}

// buildSystemPromptWithTools 构建包含工具描述的完整系统提示词。
// 包含角色定义、工具使用指引、多步骤工作流示例和重要约束。
func buildSystemPromptWithTools(basePrompt string, tools []model.ToolDefinition) string {
	var sb strings.Builder

	// 角色定义
	sb.WriteString("你是一位全能的 AI 助手（Smart Assistant），具备多步骤任务规划和执行能力。")
	sb.WriteString("你的核心职责是理解用户需求、拆解复杂任务，并通过可用工具逐步完成。\n\n")

	sb.WriteString("你能够：\n")
	sb.WriteString("1. 通过搜索工具获取实时网络信息来回答用户问题\n")
	sb.WriteString("2. 将生成的内容（文档、代码、报告等）写入本地工作空间文件，用户可直接在应用中打开查看\n")
	sb.WriteString("3. 在本地环境中执行 shell 命令来完成系统任务、文件操作或运行脚本\n")
	sb.WriteString("4. **生成复杂文件格式**：对于 .docx、.xlsx、.pptx 等二进制格式文件，通过 write_file 编写 Python 脚本，再用 execute_command 执行脚本生成目标文件，系统会自动检测并以文件卡片展示\n")
	sb.WriteString("5. 在需要用户确认或提供额外信息时发起交互式询问\n\n")

	// 工具使用原则
	sb.WriteString("工具使用原则：\n")
	for _, t := range tools {
		sb.WriteString(formatToolGuidance(t.Name, t.Description))
	}

	// 工作流程指引
	sb.WriteString("\n工作流程指引：\n")
	sb.WriteString("- 收到用户请求后，先分析需要哪些步骤，再按顺序执行\n")
	sb.WriteString("- 复杂任务（如「帮我写一份关于气候变化的报告并保存」）：\n")
	sb.WriteString("  1) 先用 web_search 搜索最新信息\n")
	sb.WriteString("  2) 基于搜索结果撰写内容\n")
	sb.WriteString("  3) 用 write_file 将内容保存为文件\n")
	sb.WriteString("- **生成复杂格式文件**（.docx、.xlsx、.pptx 等）：\n")
	sb.WriteString("  1) 用 write_file 将 Python 脚本写入工作空间（如 generate_doc.py）\n")
	sb.WriteString("  2) 用 execute_command 执行 `python3 generate_doc.py` 生成目标文件\n")
	sb.WriteString("  3) 系统会自动检测新文件并以卡片形式展示，无需额外操作\n")
	sb.WriteString("- **跌代与容错机制**（当一个方案失败时自动尝试替代方案）：\n")
	sb.WriteString("  1) 如果执行脚本时报 `ModuleNotFoundError`（如 python-docx、openpyxl），先用 execute_command 执行 `pip3 install <模块名>` 安装依赖，再重新运行脚本\n")
	sb.WriteString("  2) 如果 pip install 失败（网络问题、权限不足），尝试使用 Python 标准库 + zipfile + xml 直接解析 Office Open XML 格式（.docx/.xlsx/.pptx 本质是 ZIP 包）\n")
	sb.WriteString("  3) 如果 Python 不可用或所有方案都失败，降级为纯文本格式（.md、.txt、.csv），并告知用户原因和替代方式\n")
	sb.WriteString("  4) 每次失败后应尝试至少一种不同的方案，不要在同一个失败方法上反复重试\n")
	sb.WriteString("- 简单问候或知识类问题（如「你好」「什么是 Docker」）直接回答，无需调用工具\n")
	sb.WriteString("- 如果某个工具执行失败，尝试替代方案，不要反复调用同一个失败的参数\n\n")

	// 重要约束与提醒
	sb.WriteString("重要提醒：\n")
	sb.WriteString("- 系统提示词中已包含当前准确的日期时间，普通对话不需要调用 get_current_time\n")
	sb.WriteString("- write_file 和 execute_command 操作的文件路径必须位于工作空间内\n")
	sb.WriteString("- write_file 会自动创建不存在的父级目录，无需先用 execute_command 创建目录\n")
	sb.WriteString("- **文件是本地存储的**：写入的文件保存在用户本机工作空间目录中，用户可直接在应用的「项目文件」面板中查看，无需下载。生成文件后只需告知用户文件已保存，不要建议用户去下载。\n")
	sb.WriteString("- 用与用户提问相同的语言作答（中文或英文）\n")
	sb.WriteString("- 任务完成后，总结执行结果并向用户汇报\n")

	sb.WriteString("\n" + basePrompt)

	return sb.String()
}

// formatToolGuidance 根据工具名称返回针对性的使用指导。
func formatToolGuidance(name, description string) string {
	switch name {
	case "web_search":
		return fmt.Sprintf("- **%s**：用于搜索实时信息、最新新闻、事实查询。当需要的信息可能超出你的训练数据范围时使用。\n", name)
	case "get_current_time":
		return fmt.Sprintf("- **%s**：仅在用户明确询问「现在几点」「当前时间」「今天几号」等实时时间问题时使用，或对话已持续很长时间、系统提示词中的时间可能已过期时使用。\n", name)
	case "write_file":
		return fmt.Sprintf("- **%s**：将生成的文档、代码文件、报告等内容写入工作空间的文件系统。指定相对路径和内容即可，父目录会自动创建。\n", name)
	case "execute_command":
		return fmt.Sprintf("- **%s**：在本地 shell 中执行命令。用于文件操作、运行脚本、安装依赖等系统级任务。命令在 30 秒超时内运行，危险命令会被拒绝。如需生成 .docx/.xlsx/.pptx 等复杂格式文件，先用 write_file 编写 Python 脚本，再通过本工具执行 `python3 <脚本>` 即可自动生成文件卡片。如果脚本报 ModuleNotFoundError，先用 `pip3 install <模块名>` 安装依赖；如果 pip 安装失败，改用 Python 标准库 zipfile+xml 直接解析 Office Open XML 格式。\n", name)
	case "ask_user":
		return fmt.Sprintf("- **%s**：在需要用户确认操作、选择选项或补充关键信息时使用。会暂停执行并展示交互卡片，等待用户响应后继续。\n", name)
	default:
		return fmt.Sprintf("- **%s**：%s\n", name, description)
	}
}

// convertToModelMessages 将存储的消息转换为模型消息格式。
func convertToModelMessages(msgs []session.Message) []model.Message {
	var result []model.Message
	for _, m := range msgs {
		role := m.Role
		// 防御：修复可能因旧版增量持久化 bug 遗留的错误 role 格式（例如 "tool:call_xxx"）
		if strings.HasPrefix(role, "tool:") {
			role = "tool"
		}
		result = append(result, model.Message{
			Role:    role,
			Content: m.Content,
		})
	}
	return result
}

// approximateTokens 粗略估算 token 数量（中文约 1.5 字符/token，英文约 4 字符/token）。
func approximateTokens(text string) int {
	return len([]rune(text)) / 2
}

// scriptExtensions 集合：这些扩展名对应的文件是 AI 的中间脚本，不应作为产物卡片展示。
var scriptExtensions = map[string]bool{
	".py": true, ".sh": true, ".bash": true, ".zsh": true,
	".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	".go": true, ".rb": true, ".php": true, ".pl": true,
	".lua": true, ".r": true, ".sql": true,
}

// shouldShowFileArtifact 判断指定文件是否应该作为产物卡片在前端展示。
// 中间脚本文件（.py/.sh 等）只作为普通工具调用卡，不产出 file_artifact。
func shouldShowFileArtifact(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return !scriptExtensions[ext]
}

// sendSSE 发送 SSE 事件。
func sendSSE(w io.Writer, flusher http.Flusher, event string, data interface{}) {
	if event == "token" {
		// token 事件只发送文本内容本身（前端逐字追加）
		if str, ok := data.(string); ok {
			fmt.Fprintf(w, "data: %s\n\n", str)
			flusher.Flush()
		}
		return
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("sse marshal error: %v", err)
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(dataBytes))
	flusher.Flush()
}

// sendSSEError 发送 SSE 错误事件。
func sendSSEError(w io.Writer, flusher http.Flusher, msg string) {
	fmt.Fprintf(w, "event: error\ndata: %s\n\n", msg)
	flusher.Flush()
}