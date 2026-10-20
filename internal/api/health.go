package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse represents the payload returned by /healthz
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// ReadyResponse represents the payload returned by /readyz
type ReadyResponse struct {
	Status      string `json:"status"`
	Environment string `json:"environment"`
	Database    string `json:"database"`
	Timestamp   string `json:"timestamp"`
	Error       string `json:"error,omitempty"`
}

// InfoResponse represents the payload returned by /api/version
type InfoResponse struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	APIVersions []string `json:"api_versions"`
	Environment string   `json:"environment"`
	Runtime     string   `json:"runtime"`
}

// HandleHealthz handles liveness checks
func (a *API) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// HandleReadyz handles readiness checks (validates database availability)
func (a *API) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := a.store.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ReadyResponse{
			Status:      "not_ready",
			Environment: a.env,
			Database:    "disconnected",
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
			Error:       err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ReadyResponse{
		Status:      "ready",
		Environment: a.env,
		Database:    "connected",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	})
}

// HandleInfo handles service version and contract discovery
func (a *API) HandleInfo(w http.ResponseWriter, r *http.Request) {
	resp := InfoResponse{
		Name:        "AOSphere Business Operating System",
		Version:     "0.1.0",
		APIVersions: []string{"v1"},
		Environment: a.env,
		Runtime:     "Go Monolith (WAL Mode SQLite)",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
