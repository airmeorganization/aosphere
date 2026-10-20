package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
