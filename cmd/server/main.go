package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vps-wordpress-control/internal/api"
	"vps-wordpress-control/internal/config"
	"vps-wordpress-control/internal/database"
	"vps-wordpress-control/internal/jobs"
	"vps-wordpress-control/internal/web"
)

type HealthResponse struct {
	Status    string `json:"status"`
	AppName   string `json:"app_name"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

func main() {
	cfg := config.LoadConfig()

	log.Printf("[Khởi động] %s (Môi trường: %s)", cfg.AppName, cfg.AppEnv)

	// Khởi tạo SQLite Database
	db, err := database.InitDB(cfg.GetDBPath())
	if err != nil {
		log.Fatalf("[Database Lỗi] %v", err)
	}
	defer db.Close()

	// Khởi tạo HTTP Router sử dụng Go 1.22+ enhanced routing
	mux := http.NewServeMux()

	// 1. Phục vụ Web Dashboard (Embedded HTML UI với No-Cache header để trình duyệt luôn nhận bản mới)
	fileServer := http.FileServer(http.FS(web.Dist))
	noCacheHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileServer.ServeHTTP(w, r)
	})
	mux.Handle("GET /{$}", noCacheHandler)
	mux.Handle("GET /index.html", noCacheHandler)

	// 2. Đăng ký các module API
	authHandler := api.NewAuthHandler(cfg)
	authHandler.RegisterRoutes(mux)

	siteHandler := api.NewSiteHandler(cfg, db.DB)
	siteHandler.RegisterRoutes(mux)

	malwareHandler := api.NewMalwareHandler(db.DB)
	malwareHandler.RegisterRoutes(mux)

	wpHandler := api.NewWPHandler(cfg, db.DB)
	wpHandler.RegisterRoutes(mux)

	backupHandler := api.NewBackupHandler(cfg, db.DB)
	backupHandler.RegisterRoutes(mux)

	// 3. Khởi tạo Job Runner (SSE + Async Tasks)
	jobRunner := jobs.NewRunner(db.DB)
	jobHandler := api.NewJobHandler(db.DB, jobRunner)
	jobHandler.RegisterRoutes(mux)

	// 3. API Health check
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthResponse{
			Status:    "ok",
			AppName:   cfg.AppName,
			Version:   "1.0.0",
			Timestamp: time.Now().Format(time.RFC3339),
		})
	})

	serverAddr := fmt.Sprintf("%s:%s", cfg.AppHost, cfg.AppPort)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      authHandler.AuthMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Xử lý Graceful Shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[Server] Bảng điều khiển đang hoạt động tại: http://%s", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Server Lỗi] %v", err)
		}
	}()

	<-stopChan
	log.Println("[Server] Nhận tín hiệu dừng, đang tắt các tác vụ...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("[Server Lỗi khi tắt] %v", err)
	}

	log.Println("[Server] Đã dừng an toàn.")
}
