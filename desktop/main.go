package main

import (
	"context"
	"embed"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/smart-assistant/engine/pkg/server"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 使用用户主目录下的固定路径，避免工作目录不同导致数据丢失
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	dataDir := filepath.Join(homeDir, ".smart-assistant")
	dbPath := filepath.Join(dataDir, "smart-assistant.db")

	// 确保数据目录存在
	os.MkdirAll(dataDir, 0755)

	// 自动迁移旧数据到新位置
	migrateOldData(homeDir, dbPath)

	// 启动嵌入式引擎 HTTP 服务
	engineAddr := os.Getenv("ENGINE_ADDR")
	if engineAddr == "" {
		engineAddr = "127.0.0.1:8080"
	}

	engineSrv, err := server.New(server.ServerConfig{
		Addr:   engineAddr,
		DBPath: dbPath,
	})
	if err != nil {
		log.Fatalf("failed to create engine: %v", err)
	}

	// 后台启动引擎
	go func() {
		if err := engineSrv.ListenAndServe(); err != nil {
			log.Printf("engine server stopped: %v", err)
		}
	}()

	// 等待引擎就绪
	time.Sleep(200 * time.Millisecond)

	// 捕获系统信号，优雅关闭
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// 创建 Wails 应用
	app := application.New(application.Options{
		Name:        "Smart Assistant",
		Description: "智能助手客户端套件",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Smart Assistant",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 500,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(15, 23, 42),
		URL:              "/",
	})

	// 后台监听系统信号
	go func() {
		<-ctx.Done()
		log.Println("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		engineSrv.Shutdown(shutdownCtx)
	}()

	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("application exited")
}

// migrateOldData 将旧的相对路径 DB 迁移到新的固定位置。
func migrateOldData(homeDir, targetPath string) {
	if _, err := os.Stat(targetPath); err == nil {
		return // 目标已存在，不覆盖
	}
	// 常见的旧位置（相对路径 data/smart-assistant.db，因工作目录不同可能落在不同地点）
	candidates := []string{
		filepath.Join(homeDir, "data", "smart-assistant.db"),
	}
	for _, src := range candidates {
		if _, err := os.Stat(src); err == nil {
			log.Printf("[migrate] copying DB from %s to %s", src, targetPath)
			data, err := os.ReadFile(src)
			if err != nil {
				log.Printf("[migrate] read error: %v", err)
				continue
			}
			if err := os.WriteFile(targetPath, data, 0644); err != nil {
				log.Printf("[migrate] write error: %v", err)
				continue
			}
			log.Printf("[migrate] done")
			return
		}
	}
}