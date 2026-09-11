package agent

import (
	"fmt"
	"sync"
	"time"
)

// ConfirmRequest 确认请求的数据结构。
type ConfirmRequest struct {
	CallID   string   `json:"call_id"`
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
	Context  string   `json:"context,omitempty"`
}

// ConfirmResponse 用户对确认请求的响应。
type ConfirmResponse struct {
	CallID   string `json:"call_id"`
	Response string `json:"response"`
}

// pendingConfirm 内部挂起的确认请求。
type pendingConfirm struct {
	responseCh chan ConfirmResponse
	expiresAt  time.Time
}

var (
	pendingConfirms   = make(map[string]*pendingConfirm)
	pendingConfirmsMu sync.Mutex
)

// RegisterConfirm 注册一个待处理的确认请求，返回用于接收用户响应的 channel。
// callID 用于关联前端卡片和后端等待点。
func RegisterConfirm(callID string, timeout time.Duration) (<-chan ConfirmResponse, error) {
	pendingConfirmsMu.Lock()
	defer pendingConfirmsMu.Unlock()

	if _, exists := pendingConfirms[callID]; exists {
		return nil, fmt.Errorf("confirm already pending for call_id=%s", callID)
	}

	ch := make(chan ConfirmResponse, 1)
	pendingConfirms[callID] = &pendingConfirm{
		responseCh: ch,
		expiresAt:  time.Now().Add(timeout),
	}
	return ch, nil
}

// ResolveConfirm 用用户响应解决一个挂起的确认请求。
// 返回 true 表示成功解决，false 表示 callID 不存在或已过期。
func ResolveConfirm(callID string, response ConfirmResponse) bool {
	pendingConfirmsMu.Lock()
	pc, exists := pendingConfirms[callID]
	if !exists {
		pendingConfirmsMu.Unlock()
		return false
	}
	delete(pendingConfirms, callID)
	pendingConfirmsMu.Unlock()

	select {
	case pc.responseCh <- response:
		return true
	default:
		return false
	}
}

// CancelConfirm 取消一个挂起的确认请求（例如用户取消或连接断开时）。
func CancelConfirm(callID string) {
	pendingConfirmsMu.Lock()
	pc, exists := pendingConfirms[callID]
	if !exists {
		pendingConfirmsMu.Unlock()
		return
	}
	delete(pendingConfirms, callID)
	pendingConfirmsMu.Unlock()

	select {
	case pc.responseCh <- ConfirmResponse{CallID: callID, Response: "[已取消]"}:
	default:
	}
}

// CleanExpiredConfirms 清理过期的确认请求。
func CleanExpiredConfirms() {
	pendingConfirmsMu.Lock()
	defer pendingConfirmsMu.Unlock()

	now := time.Now()
	for id, pc := range pendingConfirms {
		if now.After(pc.expiresAt) {
			select {
			case pc.responseCh <- ConfirmResponse{CallID: id, Response: "[确认超时]"}:
			default:
			}
			delete(pendingConfirms, id)
		}
	}
}