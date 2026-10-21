package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"aosphere/internal/api"
	"aosphere/internal/config"
	"aosphere/internal/db"
	"aosphere/internal/mcp"
	"aosphere/internal/service"
	"aosphere/internal/store"
)

//go:embed web/*
var embeddedWebFS embed.FS

func main() {
	// 1. Load application configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: Configuration error: %v\n", err)
		os.Exit(1)
	}

	// 2. Setup structured logging
	var logHandler slog.Handler
	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: logLevel}

	if cfg.Env == config.EnvProduction {
		logHandler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, opts)
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	// 3. Initialize SQLite Database (WAL mode, pure Go)
	database, err := db.InitDB(cfg.DBPath, cfg.SeedData)
	if err != nil {
		logger.Error("Database initialization failed", slog.Any("error", err), slog.String("db_path", cfg.DBPath))
		os.Exit(1)
	}
	defer database.Close()

	appStore := store.NewStore(database)
	workflowSvc := service.NewWorkflowService(appStore)
	mcpServer := mcp.NewMCPServer(appStore, workflowSvc)
	mcpServer.SetEnv(string(cfg.Env))

	// 4. Handle MCP Stdio Mode (for Claude Desktop, Antigravity, Cursor)
	if cfg.MCPStdio {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := mcpServer.HandleStdio(ctx, ""); err != nil {
			logger.Error("MCP stdio error", slog.Any("error", err))
			os.Exit(1)
		}
		return
	}

	// 5. Setup HTTP Router & API Handlers
	mux := http.NewServeMux()
	appAPI := api.NewAPI(appStore, workflowSvc, string(cfg.Env))
	appAPI.RegisterRoutes(mux)

	// MCP SSE & Message Endpoints for AI Agents
	mux.HandleFunc("/mcp/sse", mcpServer.HandleSSE)
	mux.HandleFunc("/mcp/messages", mcpServer.HandlePostMessage)

	// Embedded Web UI
	webSubFS, err := fs.Sub(embeddedWebFS, "web")
	if err != nil {
		logger.Error("Failed to load embedded web assets", slog.Any("error", err))
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(webSubFS))
	mux.Handle("/", fileServer)

	// 6. Start Durable Background Outbox Worker
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	outboxDone := workflowSvc.StartOrgOutboxWorker(workerCtx, service.DefaultOutboxWorkerConfig(), func(orgID, eventType, payload string) error {
		appAPI.BroadcastForOrg(orgID, eventType, payload)
		return nil
	})

	// Apply Middlewares: RequestID -> CORS -> Authenticate -> SecurityHeaders -> Metrics -> Logging -> Panic Recovery
	var handler http.Handler = mux
	handler = api.RecoveryMiddleware(logger)(handler)
	handler = api.LoggingMiddleware(logger)(handler)
	handler = api.MetricsMiddleware(handler)
	handler = api.SecurityHeadersMiddleware(cfg.Env == config.EnvProduction)(handler)
	handler = api.AuthenticateMiddleware(appStore, string(cfg.Env))(handler)
	handler = api.CORSMiddleware(handler)
	handler = api.RequestIDMiddleware(handler)

	serverAddr := fmt.Sprintf(":%d", cfg.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Println("==================================================================")
		fmt.Println("  AOSphere — Agent-Native Business Operating System (Go Monolith) ")
		fmt.Println("==================================================================")
		fmt.Printf("  • Environment:      %s\n", cfg.Env)
		fmt.Printf("  • Web Interface:    http://localhost:%d\n", cfg.Port)
		fmt.Printf("  • REST API:         http://localhost:%d/api\n", cfg.Port)
		fmt.Printf("  • Health Probes:    http://localhost:%d/healthz , /readyz\n", cfg.Port)
		fmt.Printf("  • Prometheus:       http://localhost:%d/metrics\n", cfg.Port)
		fmt.Printf("  • MCP SSE Endpoint: http://localhost:%d/mcp/sse\n", cfg.Port)
		fmt.Printf("  • Database Engine:  %s (SQLite WAL)\n", cfg.DBPath)
		fmt.Printf("  • Seed Demo Data:   %t\n", cfg.SeedData)
		fmt.Printf("  • Stdio Agent Mode: ./%s -mcp-stdio\n", filepath.Base(os.Args[0]))
		fmt.Println("==================================================================")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server listen failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-stop
	logger.Info("Graceful shutdown initiated...")

	// 1. Drain HTTP incoming requests
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server HTTP drain error", slog.Any("error", err))
	}

	// 2. Stop and drain Outbox background worker
	cancelWorker()
	select {
	case <-outboxDone:
		logger.Info("Outbox worker stopped cleanly")
	case <-time.After(3 * time.Second):
		logger.Warn("Outbox worker shutdown timed out")
	}

	// 3. SQLite WAL Checkpoint (TRUNCATE mode flushes WAL into main DB file)
	if _, err := database.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		logger.Warn("SQLite WAL checkpoint on shutdown warning", slog.Any("error", err))
	} else {
		logger.Info("SQLite WAL checkpoint completed")
	}

	logger.Info("AOSphere exited cleanly.")
}
