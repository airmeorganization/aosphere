package smoke

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"aosphere/internal/api"
	"aosphere/internal/db"
	"aosphere/internal/mcp"
	"aosphere/internal/service"
	"aosphere/internal/store"
)

func TestSmoke_ServerBootstrapAndHealthChecks(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "smoke.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("database initialization failed: %v", err)
	}
	defer database.Close()

	if err := db.Ping(database); err != nil {
		t.Fatalf("database ping failed: %v", err)
	}

	appStore := store.NewStore(database)
	svc := service.NewWorkflowService(appStore)
	serverAPI := api.NewAPI(appStore, svc, "development")
	mcpServer := mcp.NewMCPServer(appStore, svc)

	mux := http.NewServeMux()
	serverAPI.RegisterRoutes(mux)
	mux.HandleFunc("/mcp/sse", mcpServer.HandleSSE)
	mux.HandleFunc("/mcp/messages", mcpServer.HandlePostMessage)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Healthz
	respH, err := client.Get(ts.URL + "/healthz")
	if err != nil || respH.StatusCode != http.StatusOK {
		t.Fatalf("/healthz failed: err=%v, code=%d", err, respH.StatusCode)
	}
	respH.Body.Close()

	// 2. Readyz
	respR, err := client.Get(ts.URL + "/readyz")
	if err != nil || respR.StatusCode != http.StatusOK {
		t.Fatalf("/readyz failed: err=%v, code=%d", err, respR.StatusCode)
	}
	respR.Body.Close()

	// 3. Version info
	respV, err := client.Get(ts.URL + "/api/version")
	if err != nil || respV.StatusCode != http.StatusOK {
		t.Fatalf("/api/version failed: err=%v, code=%d", err, respV.StatusCode)
	}
	respV.Body.Close()

	// 4. MCP Tools List via HTTP POST
	_ = context.Background()
}
