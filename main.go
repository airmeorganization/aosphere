package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"aosphere/internal/api"
	"aosphere/internal/db"
	"aosphere/internal/mcp"
	"aosphere/internal/store"
)

//go:embed web/*
var embeddedWebFS embed.FS

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	dbPath := flag.String("db", filepath.Join("data", "aosphere.db"), "Path to SQLite database file")
	mcpStdio := flag.Bool("mcp-stdio", false, "Run as an MCP server over stdio for AI agent desktop hosts")
	flag.Parse()

	// 1. Initialize SQLite Database (WAL mode, pure Go)
	database, err := db.InitDB(*dbPath)
	if err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}
	defer database.Close()

	appStore := store.NewStore(database)
	mcpServer := mcp.NewMCPServer(appStore)

	// 2. Handle MCP Stdio Mode (for Claude Desktop, Antigravity, Cursor)
	if *mcpStdio {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := mcpServer.HandleStdio(ctx); err != nil {
			log.Fatalf("MCP stdio error: %v", err)
		}
		return
	}

	// 3. Setup HTTP & API Router
	mux := http.NewServeMux()
	appAPI := api.NewAPI(appStore)
	appAPI.RegisterRoutes(mux)

	// MCP SSE & Message Endpoints for AI Agents
	mux.HandleFunc("/mcp/sse", mcpServer.HandleSSE)
	mux.HandleFunc("/mcp/messages", mcpServer.HandlePostMessage)

	// Embedded Web UI
	webSubFS, err := fs.Sub(embeddedWebFS, "web")
	if err != nil {
		log.Fatalf("Fatal: Failed to load embedded web assets: %v", err)
	}
	fileServer := http.FileServer(http.FS(webSubFS))
	mux.Handle("/", fileServer)

	serverAddr := fmt.Sprintf(":%d", *port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Println("==================================================================")
		fmt.Println("  AOSphere — Agent-Native Business Operating System (Go Monolith) ")
		fmt.Println("==================================================================")
		fmt.Printf("  • Web Interface:    http://localhost:%d\n", *port)
		fmt.Printf("  • REST API:         http://localhost:%d/api\n", *port)
		fmt.Printf("  • MCP SSE Endpoint: http://localhost:%d/mcp/sse\n", *port)
		fmt.Printf("  • Database Engine:  %s (SQLite WAL)\n", *dbPath)
		fmt.Printf("  • Stdio Agent Mode: ./%s -mcp-stdio\n", filepath.Base(os.Args[0]))
		fmt.Println("==================================================================")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server listen failed: %v", err)
		}
	}()

	<-stop
	fmt.Println("\nShutting down AOSphere...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	fmt.Println("AOSphere exited cleanly.")
}
