// Package transport 提供 WebSocket 传输层，将现有 Service 方法映射为 WebSocket JSON-RPC 调用。
package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ==================== 消息类型 ====================

// Request 客户端请求帧。
type Request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Response 非流式响应帧。
type Response struct {
	ID     string       `json:"id"`
	Result interface{}  `json:"result,omitempty"`
	Error  *RPCError    `json:"error,omitempty"`
}

// StreamFrame 流式推送帧（chat.send 专用）。
type StreamFrame struct {
	ID   string      `json:"id"`
	Type string      `json:"type"`          // "meta" | "token" | "done" | "error"
	Data interface{} `json:"data"`
}

// RPCError JSON-RPC 风格错误。
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ==================== 连接上下文 ====================

// contextKey 用于在 context 中传递字段。
type contextKey string

const (
	ctxUserID   contextKey = "user_id"
	ctxUsername contextKey = "username"
)

// ==================== WebSocket 连接 ====================

// Conn 包装一个 WebSocket 连接，提供线程安全的读/写和认证状态。
type Conn struct {
	id          string
	ws          *websocket.Conn
	writeMu     sync.Mutex
	UserID      string
	Username    string
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	router      *Router // 关联的路由器，供 handler 访问 service
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 本地进程间通信，允许所有来源
	},
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// HandleUpgrade 处理 HTTP → WebSocket 升级并启动连接循环。
func HandleUpgrade(w http.ResponseWriter, r *http.Request, router *Router) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	conn := &Conn{
		id:     uuid.New().String(),
		ws:     ws,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
		router: router,
	}

	log.Printf("ws connect: %s", conn.id)
	go conn.readLoop()
	<-conn.done
	log.Printf("ws disconnect: %s", conn.id)
}

// Close 关闭 WebSocket 连接。
func (c *Conn) Close() {
	log.Printf("[ws] Close called conn=%s", c.id)
	c.cancel()
	c.ws.Close()
}

// WriteJSON 线程安全地写一条 JSON 帧。
func (c *Conn) WriteJSON(v interface{}) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteJSON(v)
}

// readLoop 读取请求帧并分发给 router。
func (c *Conn) readLoop() {
	defer func() {
		c.cancel()
		c.ws.Close()
		close(c.done)
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		_, raw, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("ws read error: %v", err)
			}
			return
		}

		var req Request
		if err := json.Unmarshal(raw, &req); err != nil {
			log.Printf("[ws] readLoop PARSE ERROR raw=%s err=%v", string(raw), err)
			c.WriteJSON(Response{ID: "", Error: &RPCError{Code: -32700, Message: "parse error"}})
			continue
		}

		log.Printf("[ws] readLoop RECV id=%s method=%s", req.ID, req.Method)
		go c.router.Dispatch(c, req)
	}
}

// newResponse 构造成功响应。
func newResponse(id string, result interface{}) Response {
	return Response{ID: id, Result: result}
}

// newError 构造错误响应。
func newError(id string, code int, msg string) Response {
	return Response{ID: id, Error: &RPCError{Code: code, Message: msg}}
}

// sendError 便捷方法：发送错误帧。
func (c *Conn) sendError(id string, code int, msg string) {
	c.WriteJSON(newError(id, code, msg))
}

// sendResult 便捷方法：发送成功结果帧。
func (c *Conn) sendResult(id string, result interface{}) {
	c.WriteJSON(newResponse(id, result))
}

// Stream 发送流式帧。
func (c *Conn) Stream(id, eventType string, data interface{}) {
	err := c.WriteJSON(StreamFrame{ID: id, Type: eventType, Data: data})
	if err != nil {
		log.Printf("[ws] Stream WRITE ERROR id=%s type=%s err=%v", id, eventType, err)
	}
}

// newErr 创建 RPCError。
func newErr(code int, msg string) *RPCError {
	return &RPCError{Code: code, Message: msg}
}

// errf 创建格式化 RPCError。
func errf(code int, format string, args ...interface{}) *RPCError {
	return &RPCError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// parseParams 解析 JSON params 到目标结构体，失败时自动发送错误帧。
func parseParams[T any](c *Conn, id string, data json.RawMessage, target *T) bool {
	if err := json.Unmarshal(data, target); err != nil {
		c.sendError(id, -32700, "invalid params")
		return false
	}
	return true
}

func init() {
	// 设置 WebSocket ping/pong 超时
	_ = time.Second
}