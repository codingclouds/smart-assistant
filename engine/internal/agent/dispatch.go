package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/smart-assistant/engine/internal/model"
	"github.com/smart-assistant/engine/internal/trace"
)

// Dispatch 多 Agent 任务拆分与调度。
// v1: 串行执行 — 每个专家独立处理同一 prompt，结果串行流式返回。
// v2: 并行、条件分支、DAG。

// AgentTask 一个子 Agent 任务。
type AgentTask struct {
	ID           string `json:"id"`
	AgentName    string `json:"agent_name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
	Input        string `json:"input"`
}

// AgentResult 子 Agent 执行结果。
type AgentResult struct {
	TaskID    string `json:"task_id"`
	ExpertID  string `json:"expert_id"`
	ExpertName string `json:"expert_name"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	Elapsed   int    `json:"elapsed_ms"`
}

// DispatchResult 编排执行总结果。
type DispatchResult struct {
	Output     string        `json:"output"`
	SubResults []AgentResult `json:"sub_results"`
	TotalTime  int           `json:"total_time_ms"`
}

// DispatchRequest 多 Agent 调度请求。
type DispatchRequest struct {
	Prompt      string   `json:"prompt"`
	SessionID   string   `json:"session_id"`
	WorkspaceID string   `json:"workspace_id"`
	ExpertIDs   []string `json:"expert_ids"`
}

// ExecuteStream 串行执行多 Agent 调度，通过 channel 流式返回每个专家的执行进度。
//
// 流事件类型：
//   - "meta": 调度元信息（run_id, session_id, experts）
//   - "expert_start": 某个专家开始执行 {expert_id, expert_name}
//   - "token": 某个专家的流式输出 {expert_id, content}
//   - "expert_done": 某个专家执行完成 {expert_id, expert_name, output, elapsed_ms}
//   - "done": 全部调度完成 {output, sub_results, total_time_ms}
//   - "error": 出错
func (l *Loop) ExecuteStream(ctx context.Context, userID string, req DispatchRequest) (<-chan ChatEvent, error) {
	if len(req.ExpertIDs) == 0 {
		return nil, fmt.Errorf("at least one expert is required")
	}

	// 获取或创建 session
	sess, err := l.getOrCreateSession(userID, req.SessionID, req.WorkspaceID, "")
	if err != nil {
		return nil, fmt.Errorf("session error: %w", err)
	}

	// 查找所有请求的专家
	pool := ExpertPool()
	expertMap := make(map[string]Expert)
	for _, e := range pool {
		expertMap[e.ID] = e
	}

	var experts []Expert
	for _, eid := range req.ExpertIDs {
		e, ok := expertMap[eid]
		if !ok {
			return nil, fmt.Errorf("unknown expert: %s", eid)
		}
		experts = append(experts, e)
	}

	ch := make(chan ChatEvent, 64)

	go func() {
		defer close(ch)

		startTime := time.Now()

		// 保存用户消息
		l.sessionService.AddMessage(sess.ID, "user", req.Prompt, approximateTokens(req.Prompt), "")

		// 发送 meta 事件
		expertSummaries := make([]map[string]string, 0, len(experts))
		for _, e := range experts {
			expertSummaries = append(expertSummaries, map[string]string{
				"id":   e.ID,
				"name": e.Name,
				"emoji": e.Emoji,
			})
		}
		ch <- ChatEvent{
			Type: "meta",
			Data: map[string]interface{}{
				"session_id": sess.ID,
				"experts":    expertSummaries,
			},
		}

		var subResults []AgentResult
		var combinedOutput strings.Builder

		// 串行执行每个专家
		for i, expert := range experts {
			expertStart := time.Now()

			// 发送 expert_start 事件
			ch <- ChatEvent{
				Type: "expert_start",
				Data: map[string]string{
					"expert_id":   expert.ID,
					"expert_name": expert.Name,
					"emoji":       expert.Emoji,
					"index":       fmt.Sprintf("%d/%d", i+1, len(experts)),
				},
			}

			// 构造该专家的模型消息：系统提示 + 用户 prompt
			modelCfg, err := l.modelRegistry.GetConfig(expert.ModelName)
			if err != nil {
				ch <- ChatEvent{
					Type: "error",
					Data: fmt.Sprintf("model config for %s: %v", expert.ID, err),
				}
				subResults = append(subResults, AgentResult{
					TaskID:     fmt.Sprintf("task-%d", i),
					ExpertID:   expert.ID,
					ExpertName: expert.Name,
					Error:      err.Error(),
					Elapsed:    int(time.Since(expertStart).Milliseconds()),
				})
				continue
			}

			modelMsgs := []model.Message{
				{Role: "system", Content: expert.SystemPrompt},
				{Role: "user", Content: req.Prompt},
			}

			provider := l.modelRegistry.NewProvider(modelCfg.ApiFormat)
			chunks, err := provider.StreamChat(ctx, expert.ModelName, modelMsgs, *modelCfg)
			if err != nil {
				l.traceService.Record(sess.ID, expert.ID, trace.EventError, map[string]string{
					"error": err.Error(),
				}, int(time.Since(expertStart).Milliseconds()))

				ch <- ChatEvent{
					Type: "error",
					Data: fmt.Sprintf("%s model error: %v", expert.Name, err),
				}
				subResults = append(subResults, AgentResult{
					TaskID:     fmt.Sprintf("task-%d", i),
					ExpertID:   expert.ID,
					ExpertName: expert.Name,
					Error:      err.Error(),
					Elapsed:    int(time.Since(expertStart).Milliseconds()),
				})
				continue
			}

			// 流式读取该专家的输出
			var expertOutput strings.Builder
			expertDone := false
			for chunk := range chunks {
				if chunk.Error != "" {
					l.traceService.Record(sess.ID, expert.ID, trace.EventError, map[string]string{
						"error": chunk.Error,
					}, int(time.Since(expertStart).Milliseconds()))

					ch <- ChatEvent{
						Type: "error",
						Data: fmt.Sprintf("%s: %s", expert.Name, chunk.Error),
					}
					subResults = append(subResults, AgentResult{
						TaskID:     fmt.Sprintf("task-%d", i),
						ExpertID:   expert.ID,
						ExpertName: expert.Name,
						Output:     expertOutput.String(),
						Error:      chunk.Error,
						Elapsed:    int(time.Since(expertStart).Milliseconds()),
					})
					expertDone = true
					break
				}

				if chunk.Done {
					elapsed := int(time.Since(expertStart).Milliseconds())
					output := expertOutput.String()

					l.traceService.Record(sess.ID, expert.ID, trace.EventResponse, map[string]interface{}{
						"model":   expert.ModelName,
						"content": output[:min(len(output), 200)],
						"elapsed": elapsed,
					}, elapsed)

					subResults = append(subResults, AgentResult{
						TaskID:     fmt.Sprintf("task-%d", i),
						ExpertID:   expert.ID,
						ExpertName: expert.Name,
						Output:     output,
						Elapsed:    elapsed,
					})

					combinedOutput.WriteString(fmt.Sprintf("## %s %s\n\n%s\n\n", expert.Emoji, expert.Name, output))

					ch <- ChatEvent{
						Type: "expert_done",
						Data: map[string]interface{}{
							"expert_id":   expert.ID,
							"expert_name": expert.Name,
							"output":      output,
							"elapsed_ms":  elapsed,
						},
					}
					expertDone = true
					break
				}

				expertOutput.WriteString(chunk.Content)

				// 流式 token，标注来自哪个专家
				ch <- ChatEvent{
					Type: "token",
					Data: map[string]string{
						"expert_id": expert.ID,
						"content":   chunk.Content,
					},
				}
			}

			// 如果 channel 关闭但没有收到 done（模型异常）
			if !expertDone {
				subResults = append(subResults, AgentResult{
					TaskID:     fmt.Sprintf("task-%d", i),
					ExpertID:   expert.ID,
					ExpertName: expert.Name,
					Error:      "model stream ended unexpectedly",
					Elapsed:    int(time.Since(expertStart).Milliseconds()),
				})
			}
		}

		// 保存合并后的助手消息
		finalText := combinedOutput.String()
		l.sessionService.AddMessage(sess.ID, "assistant", finalText, approximateTokens(finalText), "")

		// 发送总 done 事件
		totalElapsed := int(time.Since(startTime).Milliseconds())
		ch <- ChatEvent{
			Type: "done",
			Data: map[string]interface{}{
				"session_id":   sess.ID,
				"output":       finalText,
				"sub_results":  subResults,
				"total_time_ms": totalElapsed,
			},
		}
	}()

	return ch, nil
}