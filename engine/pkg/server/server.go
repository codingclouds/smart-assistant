// Package server 提供引擎 HTTP API 服务的可复用初始化逻辑。
// 同时用于独立二进制（cmd/server）和嵌入式桌面应用（desktop/main.go）。
package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/smart-assistant/engine/internal/agent"
	"github.com/smart-assistant/engine/internal/auth"
	"github.com/smart-assistant/engine/internal/memory"
	"github.com/smart-assistant/engine/internal/model"
	"github.com/smart-assistant/engine/internal/session"
	"github.com/smart-assistant/engine/internal/skill"
	"github.com/smart-assistant/engine/internal/tool"
	"github.com/smart-assistant/engine/internal/trace"
	"github.com/smart-assistant/engine/internal/transport"
	"github.com/smart-assistant/engine/internal/workspace"
	"github.com/smart-assistant/engine/pkg/store"
)

// ServerConfig 引擎服务初始化配置。
type ServerConfig struct {
	Addr   string
	DBPath string
}

// New 初始化所有模块、路由，返回一个可启动的 http.Server。
// 调用方负责 .ListenAndServe() 及关闭生命周期管理。
func New(cfg ServerConfig) (*http.Server, error) {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "data/smart-assistant.db"
	}

	db, err := store.NewSQLite(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	authSvc := auth.NewService(db)
	sessionSvc := session.NewService(db)
	workspaceSvc := workspace.NewService(db)
	modelRegistry := model.NewRegistry(db)
	traceSvc := trace.NewService(db)
	skillSvc := skill.NewService(db)
	memorySvc := memory.NewService(db)

	toolRegistry := tool.NewRegistry()
	toolRegistry.Register(tool.NewWebSearchTool())
	toolRegistry.Register(tool.NewCurrentTimeTool())
	toolRegistry.Register(tool.NewAskUserTool())
	toolRegistry.Register(tool.NewWriteFileTool())
	toolRegistry.Register(tool.NewExecuteCommandTool())

	agentLoop := agent.NewLoop(agent.LoopConfig{
		ModelRegistry:    modelRegistry,
		SessionService:   sessionSvc,
		TraceService:     traceSvc,
		WorkspaceService: workspaceSvc,
		ToolRegistry:     toolRegistry,
	})

	wsRouter := transport.NewRouter(
		authSvc, sessionSvc, workspaceSvc, modelRegistry, traceSvc, skillSvc, memorySvc, agentLoop,
	)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// WebSocket 升级入口 — 无超时（长连接），不走 JWT 中间件（连接级认证）
	r.Get("/ws", func(w http.ResponseWriter, req *http.Request) {
		transport.HandleUpgrade(w, req, wsRouter)
	})

	// REST API 路由带超时
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Timeout(300 * time.Second))

		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authSvc.HandleRegister)
			r.Post("/login", authSvc.HandleLogin)
		})

		r.Group(func(r chi.Router) {
			r.Use(authSvc.Middleware)

			r.Post("/chat", agentLoop.HandleChat)
			r.Post("/chat/cancel", agentLoop.HandleCancelHTTP)

			r.Get("/sessions", sessionSvc.HandleList)
			r.Get("/sessions/{id}", sessionSvc.HandleGet)
			r.Delete("/sessions/{id}", sessionSvc.HandleDelete)

			r.Get("/workspaces", workspaceSvc.HandleList)
			r.Post("/workspaces", workspaceSvc.HandleCreate)

			r.Get("/models", modelRegistry.HandleList)
			r.Post("/models", modelRegistry.HandleCreate)
			r.Put("/models/{id}", modelRegistry.HandleUpdate)

			r.Get("/skills", skillSvc.HandleList)
			r.Post("/skills/install", skillSvc.HandleInstall)

			// 统计与追踪
			r.Get("/stats/tokens", sessionSvc.HandleTokenStats)

			r.Get("/traces/{sessionId}", traceSvc.HandleGet)

			})

		// 工作空间文件操作（下载、预览、列表）— 无需 JWT 鉴权
		// 因为文件下载/预览通过 <a>/<img> 等标签直接访问，无法携带 Authorization header
		// 安全边界由 127.0.0.1 仅本地监听保证
		r.Get("/files/download", handleFileDownload(workspaceSvc))
		r.Get("/files/content", handleFileContent(workspaceSvc))
		r.Get("/files/preview", handleFilePreview(workspaceSvc))
		r.Get("/files/list", handleFileList(workspaceSvc))
	})

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}

	log.Printf("🚀 Smart Assistant Engine listening on %s", cfg.Addr)
	return srv, nil
}