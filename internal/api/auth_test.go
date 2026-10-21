package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aosphere/internal/auth"
)

func TestAuthRegistrationAndLogin(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)
	handler := AuthenticateMiddleware(testAPI.store)(mux)

	// 1. Register a new company and owner
	regPayload := RegisterRequest{
		OrgName:  "Starlight Industries",
		OrgSlug:  "starlight",
		Name:     "Starlight Founder",
		Email:    "founder@starlight.io",
		Password: "SecurePassword999!",
	}
	body, _ := json.Marshal(regPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var regResp struct {
		Status string `json:"status"`
		Token  string `json:"token"`
	}
	json.NewDecoder(rr.Body).Decode(&regResp)
	if regResp.Token == "" {
		t.Fatal("expected session token in registration response")
	}

	// 2. Test Login with registered credentials
	loginPayload := LoginRequest{
		Email:    "founder@starlight.io",
		Password: "SecurePassword999!",
	}
	loginBody, _ := json.Marshal(loginPayload)
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	rrLogin := httptest.NewRecorder()

	handler.ServeHTTP(rrLogin, reqLogin)

	if rrLogin.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for valid login, got %d", rrLogin.Code)
	}

	// 3. Test Login with wrong password
	badLoginPayload := LoginRequest{
		Email:    "founder@starlight.io",
		Password: "wrongPassword",
	}
	badBody, _ := json.Marshal(badLoginPayload)
	reqBadLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(badBody))
	reqBadLogin.Header.Set("Content-Type", "application/json")
	rrBadLogin := httptest.NewRecorder()

	handler.ServeHTTP(rrBadLogin, reqBadLogin)

	if rrBadLogin.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for wrong password, got %d", rrBadLogin.Code)
	}

	// 4. Test /api/auth/me with Bearer token
	reqMe := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+regResp.Token)
	rrMe := httptest.NewRecorder()

	handler.ServeHTTP(rrMe, reqMe)

	if rrMe.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/auth/me, got %d", rrMe.Code)
	}

	var meResp struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	json.NewDecoder(rrMe.Body).Decode(&meResp)
	if meResp.Email != "founder@starlight.io" || meResp.Role != "owner" {
		t.Errorf("unexpected me profile: %+v", meResp)
	}
}

func TestRBAC_PermissionEnforcement(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)
	handler := AuthenticateMiddleware(testAPI.store)(mux)

	// Create an employee user who only has 'read' permission (no 'manage_inventory')
	empUser, err := testAPI.store.CreateUser("org_default", "employee@aosphere.local", "hash1234", "Worker Bee", "employee")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	empToken, _ := auth.GenerateRandomToken("sess")
	tokenHash := auth.HashToken(empToken)
	_, err = testAPI.store.CreateSession(empUser.ID, empUser.OrgID, tokenHash, 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Employee attempts to adjust stock -> MUST BE REJECTED WITH 403 FORBIDDEN
	adjustBody := []byte(`{"sku_id":"sku_01","delta":10,"reason":"Employee adjustment"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/skus/adjust", bytes.NewReader(adjustBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+empToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for employee modifying inventory, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestTenantIsolation_APIRoutes(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)
	handler := AuthenticateMiddleware(testAPI.store)(mux)

	// Register Tenant 2
	regPayload := RegisterRequest{
		OrgName:  "Tenant Two Corp",
		OrgSlug:  "tenant-two",
		Name:     "Tenant Two CEO",
		Email:    "ceo@tenanttwo.com",
		Password: "Password1234!",
	}
	body, _ := json.Marshal(regPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var regResp struct {
		Token string `json:"token"`
	}
	json.NewDecoder(rr.Body).Decode(&regResp)

	// Tenant 2 requests SKUs -> MUST RETURN 0 (Seeded SKUs belong to org_default)
	reqSKUs := httptest.NewRequest(http.MethodGet, "/api/skus", nil)
	reqSKUs.Header.Set("Authorization", "Bearer "+regResp.Token)
	rrSKUs := httptest.NewRecorder()
	handler.ServeHTTP(rrSKUs, reqSKUs)

	if rrSKUs.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rrSKUs.Code)
	}

	var skus []interface{}
	json.NewDecoder(rrSKUs.Body).Decode(&skus)
	if len(skus) != 0 {
		t.Errorf("SECURITY LEAK: Tenant Two received %d SKUs belonging to another organization!", len(skus))
	}
}

func TestUnauthenticatedAccessBlockedInProduction(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()
	testAPI.env = "production"

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)
	handler := AuthenticateMiddleware(testAPI.store, "production")(mux)

	protectedEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/skus"},
		{http.MethodPost, "/api/skus/adjust"},
		{http.MethodGet, "/api/suppliers"},
		{http.MethodGet, "/api/purchase-orders"},
		{http.MethodPost, "/api/purchase-orders"},
		{http.MethodGet, "/api/deals"},
		{http.MethodPost, "/api/deals"},
		{http.MethodGet, "/api/invoices"},
		{http.MethodGet, "/api/financial-summary"},
		{http.MethodGet, "/api/ledger/trial-balance"},
		{http.MethodGet, "/api/employees"},
		{http.MethodGet, "/api/timesheets"},
		{http.MethodGet, "/api/approvals"},
		{http.MethodGet, "/api/audit-logs"},
		{http.MethodGet, "/api/reconciliation"},
		{http.MethodPost, "/api/backup/run"},
		{http.MethodGet, "/api/admin/kill-switch"},
		{http.MethodGet, "/api/admin/agent-executions"},
		{http.MethodGet, "/api/documents"},
		{http.MethodGet, "/api/search"},
		{http.MethodGet, "/api/export"},
		{http.MethodGet, "/api/notifications"},
		{http.MethodGet, "/api/organization"},
	}

	for _, ep := range protectedEndpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("CRITICAL SECURITY HOLE: Anonymous request to %s %s was not blocked with 401 Unauthorized! Got %d",
					ep.method, ep.path, rr.Code)
			}
		})
	}
}

func TestCORSPolicyHardening(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"1.0"}`))
	})
	handler := CORSMiddleware(mux)

	// 1. Preflight OPTIONS with specific Origin
	reqPreflight := httptest.NewRequest(http.MethodOptions, "/api/version", nil)
	reqPreflight.Header.Set("Origin", "https://app.aosphere.io")
	rrPreflight := httptest.NewRecorder()
	handler.ServeHTTP(rrPreflight, reqPreflight)

	if rrPreflight.Code != http.StatusOK {
		t.Errorf("expected 200 OK for OPTIONS preflight, got %d", rrPreflight.Code)
	}
	if origin := rrPreflight.Header().Get("Access-Control-Allow-Origin"); origin != "https://app.aosphere.io" {
		t.Errorf("expected reflected origin https://app.aosphere.io, got %s", origin)
	}
	if creds := rrPreflight.Header().Get("Access-Control-Allow-Credentials"); creds != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials: true, got %s", creds)
	}

	// 2. Regular GET with Origin
	reqGet := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	reqGet.Header.Set("Origin", "https://app.aosphere.io")
	rrGet := httptest.NewRecorder()
	handler.ServeHTTP(rrGet, reqGet)

	if origin := rrGet.Header().Get("Access-Control-Allow-Origin"); origin != "https://app.aosphere.io" {
		t.Errorf("expected reflected origin on GET, got %s", origin)
	}
	if creds := rrGet.Header().Get("Access-Control-Allow-Credentials"); creds != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials: true on GET, got %s", creds)
	}
}

func TestTenantIsolatedSSE(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	ch1 := make(chan string, 4)
	ch2 := make(chan string, 4)

	testAPI.clientsMu.Lock()
	testAPI.clients[ch1] = "org_alpha"
	testAPI.clients[ch2] = "org_beta"
	testAPI.clientsMu.Unlock()

	defer func() {
		testAPI.clientsMu.Lock()
		delete(testAPI.clients, ch1)
		delete(testAPI.clients, ch2)
		testAPI.clientsMu.Unlock()
	}()

	// Broadcast an event strictly for org_alpha
	testAPI.BroadcastForOrg("org_alpha", "invoice_created", `{"invoice_id":"inv_alpha_01"}`)

	select {
	case msg := <-ch1:
		if !strings.Contains(msg, "invoice_created") || !strings.Contains(msg, "inv_alpha_01") {
			t.Errorf("org_alpha received unexpected message: %s", msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected org_alpha subscriber to receive message")
	}

	// Verify org_beta received NOTHING
	select {
	case msg := <-ch2:
		t.Fatalf("SECURITY VIOLATION: org_beta received tenant event belonging to org_alpha: %s", msg)
	case <-time.After(100 * time.Millisecond):
		// Success: nothing leaked to org_beta
	}
}
