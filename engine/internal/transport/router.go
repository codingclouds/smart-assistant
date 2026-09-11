package transport

import (
	"log"

	"github.com/smart-assistant/engine/internal/agent"
	"github.com/smart-assistant/engine/internal/auth"
	"github.com/smart-assistant/engine/internal/memory"
	"github.com/smart-assistant/engine/internal/model"
	"github.com/smart-assistant/engine/internal/session"
	"github.com/smart-assistant/engine/internal/skill"
	"github.com/smart-assistant/engine/internal/trace"
	"github.com/smart-assistant/engine/internal/workspace"
)

// HandlerFunc WebSocket 方法处理函数签名。
type HandlerFunc func(c *Conn, id string, params []byte)

// Router WebSocket 方法路由器。
// 持有所有 Service 引用，注册 17 个方法，将 JSON-RPC 请求分发到对应 handler。
type Router struct {
	handlers      map[string]HandlerFunc
	AuthSvc       *auth.Service
	SessionSvc    *session.Service
	WorkspaceSvc  *workspace.Service
	ModelRegistry *model.Registry
	TraceSvc      *trace.Service
	SkillSvc      *skill.Service
	MemorySvc     *memory.Service
	AgentLoop     *agent.Loop
}

// NewRouter 创建路由器并注册所有方法。
func NewRouter(
	authSvc *auth.Service,
	sessionSvc *session.Service,
	workspaceSvc *workspace.Service,
	modelRegistry *model.Registry,
	traceSvc *trace.Service,
	skillSvc *skill.Service,
	memorySvc *memory.Service,
	agentLoop *agent.Loop,
) *Router {
	r := &Router{
		handlers:      make(map[string]HandlerFunc),
		AuthSvc:       authSvc,
		SessionSvc:    sessionSvc,
		WorkspaceSvc:  workspaceSvc,
		ModelRegistry: modelRegistry,
		TraceSvc:      traceSvc,
		SkillSvc:      skillSvc,
		MemorySvc:     memorySvc,
		AgentLoop:     agentLoop,
	}

	// 认证方法（连接握手 + 注册/登录）
	r.handlers["auth.register"] = handleAuthRegister
	r.handlers["auth.login"] = handleAuthLogin

	// 对话方法（流式）
	r.handlers["chat.send"] = handleChatSend
	r.handlers["chat.cancel"] = handleChatCancel
		r.handlers["chat.confirm"] = handleChatConfirm

	// 会话管理
	r.handlers["sessions.list"] = handleSessionsList
	r.handlers["sessions.get"] = handleSessionsGet
	r.handlers["sessions.delete"] = handleSessionsDelete

	// 工作空间
	r.handlers["workspaces.list"] = handleWorkspacesList
	r.handlers["workspaces.create"] = handleWorkspacesCreate
	r.handlers["workspaces.files"] = handleWorkspaceFiles
		r.handlers["workspaces.file_content"] = handleWorkspaceFileContent
		r.handlers["workspaces.file_preview"] = handleWorkspaceFilePreview

	// 模型管理
	r.handlers["models.list"] = handleModelsList
	r.handlers["models.create"] = handleModelsCreate
	r.handlers["models.update"] = handleModelsUpdate
	r.handlers["models.set_default"] = handleModelsSetDefault

	// 技能管理
	r.handlers["skills.list"] = handleSkillsList
	r.handlers["skills.install"] = handleSkillsInstall

	// 记忆管理
	r.handlers["memory.list"] = handleMemoryList
	r.handlers["memory.toggle"] = handleMemoryToggle
	r.handlers["memory.create"] = handleMemoryCreate

	// 统计与追踪
	r.handlers["stats.tokens"] = handleStatsTokens
	r.handlers["traces.get"] = handleTracesGet

	// 多 Agent 调度
	r.handlers["experts.list"] = handleExpertsList
		r.handlers["tools.list"] = handleToolsList
	r.handlers["dispatch.run"] = handleDispatchRun

	// 心跳
	r.handlers["system.ping"] = handleSystemPing

	return r
}

// Dispatch 将请求帧分发到对应 handler。
func (r *Router) Dispatch(c *Conn, req Request) {
	log.Printf("[ws] dispatch method=%s id=%s", req.Method, req.ID)
	handler, ok := r.handlers[req.Method]
	if !ok {
		log.Printf("[ws] dispatch UNKNOWN method=%s", req.Method)
		c.sendError(req.ID, -32601, "method not found: "+req.Method)
		return
	}

	// auth.register 和 auth.login 在认证前可用；其他方法要求连接已认证
	if req.Method != "auth.register" && req.Method != "auth.login" && req.Method != "system.ping" {
		if c.UserID == "" {
			log.Printf("[ws] dispatch AUTH REJECT method=%s — UserID is empty", req.Method)
			c.sendError(req.ID, -32000, "authentication required — call auth.login first")
			return
		}
	}

	// recover panic
	defer func() {
		if rv := recover(); rv != nil {
			log.Printf("ws handler panic [method=%s]: %v", req.Method, rv)
			c.sendError(req.ID, -32603, "internal error")
		}
	}()

	handler(c, req.ID, req.Params)
}