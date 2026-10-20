package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aosphere/internal/db"
	"aosphere/internal/store"
)

func setupTestAPI(t *testing.T) (*API, *sql.DB) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_api.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	appStore := store.NewStore(database)
	testAPI := NewAPI(appStore, nil, "test")
	return testAPI, database
}

func TestHealthz(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %s", resp.Status)
	}
}

func TestReadyz(t *testing.T) {
	testAPI, database := setupTestAPI(t)

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)

	// Test ready state when DB is alive
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp ReadyResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}
	if resp.Status != "ready" || resp.Database != "connected" {
		t.Errorf("expected ready & connected, got status=%s, db=%s", resp.Status, resp.Database)
	}

	// Test unready state when DB is closed
	database.Close()

	req2 := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 when DB closed, got %d", rr2.Code)
	}
}

func TestInfoEndpoint(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp InfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Version == "" || resp.Environment != "test" {
		t.Errorf("unexpected info response: %+v", resp)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := GetRequestID(r.Context())
		if reqID == "" || reqID == "unknown" {
			t.Errorf("expected valid request ID in context, got %q", reqID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	resReqID := rr.Header().Get("X-Request-ID")
	if resReqID == "" {
		t.Error("expected X-Request-ID header in response")
	}
}
