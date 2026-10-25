package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aosphere/internal/auth"
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

func TestTenantIsolation_APIEndpoints(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)

	// 1. Seed two distinct tenants: org_alpha and org_beta
	_, err := database.Exec(`INSERT OR IGNORE INTO organizations (id, name, slug) VALUES 
		('org_alpha', 'Alpha Inc', 'alpha'),
		('org_beta', 'Beta LLC', 'beta')`)
	if err != nil {
		t.Fatalf("failed to insert test orgs: %v", err)
	}

	// 2. Insert records into org_alpha
	_, _ = database.Exec(`INSERT INTO customers (id, org_id, name, email) VALUES ('cust_a', 'org_alpha', 'Customer Alpha', 'a@alpha.com')`)
	_, _ = database.Exec(`INSERT INTO deals (id, org_id, title, customer_id, value, stage) VALUES ('deal_a', 'org_alpha', 'Deal Alpha', 'cust_a', 1000.0, 'lead')`)
	_, _ = database.Exec(`INSERT INTO invoices (id, org_id, invoice_number, customer_id, status, total_amount, subtotal, tax_amount, issue_date, due_date)
		VALUES ('inv_a', 'org_alpha', 'INV-A-1', 'cust_a', 'paid', 500.0, 450.0, 50.0, '2026-01-01', '2026-02-01')`)
	_, _ = database.Exec(`INSERT INTO expenses (id, org_id, category, amount, vendor, description, date)
		VALUES ('exp_a', 'org_alpha', 'Supplies', 120.0, 'Vendor Alpha', 'Office gear', '2026-01-01')`)
	_, _ = database.Exec(`INSERT INTO employees (id, org_id, name, role, department, hourly_rate, status)
		VALUES ('emp_a', 'org_alpha', 'Alice Alpha', 'Developer', 'Engineering', 50.0, 'active')`)
	_, _ = database.Exec(`INSERT INTO shifts (id, org_id, employee_id, shift_date, start_time, end_time, station, status)
		VALUES ('shift_a', 'org_alpha', 'emp_a', '2026-01-02', '09:00', '17:00', 'Station 1', 'scheduled')`)
	_, _ = database.Exec(`INSERT INTO tasks (id, org_id, title, status) VALUES ('task_a', 'org_alpha', 'Task Alpha', 'todo')`)
	_, _ = database.Exec(`INSERT INTO reviews (id, org_id, platform, author_name, rating, review_text)
		VALUES ('rev_a', 'org_alpha', 'Google', 'Reviewer Alpha', 5, 'Great Alpha experience')`)
	_, _ = database.Exec(`INSERT OR REPLACE INTO local_listings (id, org_id, directory, nap_business_name, nap_address, nap_phone, nap_website)
		VALUES ('list_a', 'org_alpha', 'Google Business', 'Alpha Inc', '1 Alpha St', '+1-555-ALPHA', 'https://alpha.com')`)

	// 3. Insert records into org_beta
	_, _ = database.Exec(`INSERT INTO customers (id, org_id, name, email) VALUES ('cust_b', 'org_beta', 'Customer Beta', 'b@beta.com')`)
	_, _ = database.Exec(`INSERT INTO deals (id, org_id, title, customer_id, value, stage) VALUES ('deal_b', 'org_beta', 'Deal Beta', 'cust_b', 9000.0, 'qualified')`)
	_, _ = database.Exec(`INSERT INTO invoices (id, org_id, invoice_number, customer_id, status, total_amount, subtotal, tax_amount, issue_date, due_date)
		VALUES ('inv_b', 'org_beta', 'INV-B-1', 'cust_b', 'paid', 8000.0, 7200.0, 8000.0, '2026-01-01', '2026-02-01')`)
	_, _ = database.Exec(`INSERT INTO expenses (id, org_id, category, amount, vendor, description, date)
		VALUES ('exp_b', 'org_beta', 'Legal', 3000.0, 'Vendor Beta', 'Legal advice', '2026-01-01')`)
	_, _ = database.Exec(`INSERT INTO employees (id, org_id, name, role, department, hourly_rate, status)
		VALUES ('emp_b', 'org_beta', 'Bob Beta', 'Accountant', 'Finance', 60.0, 'active')`)
	_, _ = database.Exec(`INSERT INTO shifts (id, org_id, employee_id, shift_date, start_time, end_time, station, status)
		VALUES ('shift_b', 'org_beta', 'emp_b', '2026-01-02', '10:00', '18:00', 'Station 2', 'scheduled')`)
	_, _ = database.Exec(`INSERT INTO tasks (id, org_id, title, status) VALUES ('task_b', 'org_beta', 'Task Beta', 'todo')`)
	_, _ = database.Exec(`INSERT INTO reviews (id, org_id, platform, author_name, rating, review_text)
		VALUES ('rev_b', 'org_beta', 'Yelp', 'Reviewer Beta', 1, 'Terrible Beta experience')`)
	_, _ = database.Exec(`INSERT OR REPLACE INTO local_listings (id, org_id, directory, nap_business_name, nap_address, nap_phone, nap_website)
		VALUES ('list_b', 'org_beta', 'Google Business', 'Beta LLC', '2 Beta Blvd', '+1-555-BETA', 'https://beta.com')`)

	alphaAuth := &auth.AuthContext{
		UserID: "usr_alpha_admin",
		OrgID:  "org_alpha",
		Role:   auth.RoleAdmin,
		Email:  "admin@alpha.com",
	}

	execGet := func(endpoint string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, endpoint, nil)
		req = req.WithContext(context.WithValue(req.Context(), AuthCtxKey, alphaAuth))
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr
	}

	// Verify /api/deals returns only org_alpha
	rrDeals := execGet("/api/deals")
	if rrDeals.Code != http.StatusOK {
		t.Fatalf("expected 200 for deals, got %d", rrDeals.Code)
	}
	dealsStr := rrDeals.Body.String()
	if !strings.Contains(dealsStr, "Deal Alpha") || strings.Contains(dealsStr, "Deal Beta") {
		t.Fatalf("cross-tenant data leakage in /api/deals: %s", dealsStr)
	}

	// Verify cross-tenant mutation of deal stage is rejected
	stageReqBody, _ := json.Marshal(map[string]string{"deal_id": "deal_b", "stage": "won"})
	reqStage := httptest.NewRequest(http.MethodPost, "/api/deals/stage", bytes.NewReader(stageReqBody))
	reqStage.Header.Set("Content-Type", "application/json")
	reqStage = reqStage.WithContext(context.WithValue(reqStage.Context(), AuthCtxKey, alphaAuth))
	rrStage := httptest.NewRecorder()
	mux.ServeHTTP(rrStage, reqStage)
	if rrStage.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when attempting cross-tenant deal stage mutation, got %d: %s", rrStage.Code, rrStage.Body.String())
	}

	// Verify /api/customers returns only org_alpha
	rrCust := execGet("/api/customers")
	custStr := rrCust.Body.String()
	if !strings.Contains(custStr, "Customer Alpha") || strings.Contains(custStr, "Customer Beta") {
		t.Fatalf("cross-tenant data leakage in /api/customers: %s", custStr)
	}

	// Verify /api/financial-summary returns only org_alpha
	rrFin := execGet("/api/financial-summary")
	var finSummary db.FinancialSummary
	_ = json.NewDecoder(rrFin.Body).Decode(&finSummary)
	if finSummary.TotalRevenue != 500.0 || finSummary.TotalExpense != 120.0 {
		t.Fatalf("cross-tenant financial aggregation leak! Got rev=%.2f, exp=%.2f (expected 500.0 and 120.0)", finSummary.TotalRevenue, finSummary.TotalExpense)
	}

	// Verify /api/expenses returns only org_alpha
	rrExp := execGet("/api/expenses")
	expStr := rrExp.Body.String()
	if !strings.Contains(expStr, "Office gear") || strings.Contains(expStr, "Legal advice") {
		t.Fatalf("cross-tenant data leakage in /api/expenses: %s", expStr)
	}

	// Verify /api/employees returns only org_alpha
	rrEmp := execGet("/api/employees")
	empStr := rrEmp.Body.String()
	if !strings.Contains(empStr, "Alice Alpha") || strings.Contains(empStr, "Bob Beta") {
		t.Fatalf("cross-tenant data leakage in /api/employees: %s", empStr)
	}

	// Verify /api/shifts returns only org_alpha
	rrShifts := execGet("/api/shifts")
	shiftsStr := rrShifts.Body.String()
	if !strings.Contains(shiftsStr, "shift_a") || strings.Contains(shiftsStr, "shift_b") {
		t.Fatalf("cross-tenant data leakage in /api/shifts: %s", shiftsStr)
	}

	// Verify cross-tenant shift mutation is rejected
	shiftReqBody, _ := json.Marshal(map[string]string{"shift_id": "shift_b", "status": "completed"})
	reqShift := httptest.NewRequest(http.MethodPost, "/api/shifts/status", bytes.NewReader(shiftReqBody))
	reqShift.Header.Set("Content-Type", "application/json")
	reqShift = reqShift.WithContext(context.WithValue(reqShift.Context(), AuthCtxKey, alphaAuth))
	rrShift := httptest.NewRecorder()
	mux.ServeHTTP(rrShift, reqShift)
	if rrShift.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when attempting cross-tenant shift status mutation, got %d: %s", rrShift.Code, rrShift.Body.String())
	}

	// Verify /api/tasks returns only org_alpha
	rrTasks := execGet("/api/tasks")
	tasksStr := rrTasks.Body.String()
	if !strings.Contains(tasksStr, "Task Alpha") || strings.Contains(tasksStr, "Task Beta") {
		t.Fatalf("cross-tenant data leakage in /api/tasks: %s", tasksStr)
	}

	// Verify cross-tenant task mutation is rejected
	taskReqBody, _ := json.Marshal(map[string]string{"task_id": "task_b", "status": "done"})
	reqTask := httptest.NewRequest(http.MethodPost, "/api/tasks/status", bytes.NewReader(taskReqBody))
	reqTask.Header.Set("Content-Type", "application/json")
	reqTask = reqTask.WithContext(context.WithValue(reqTask.Context(), AuthCtxKey, alphaAuth))
	rrTask := httptest.NewRecorder()
	mux.ServeHTTP(rrTask, reqTask)
	if rrTask.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when attempting cross-tenant task status mutation, got %d: %s", rrTask.Code, rrTask.Body.String())
	}

	// Verify /api/reviews returns only org_alpha
	rrReviews := execGet("/api/reviews")
	revsStr := rrReviews.Body.String()
	if !strings.Contains(revsStr, "Great Alpha experience") || strings.Contains(revsStr, "Terrible Beta experience") {
		t.Fatalf("cross-tenant data leakage in /api/reviews: %s", revsStr)
	}

	// Verify cross-tenant review reply is rejected
	revReqBody, _ := json.Marshal(map[string]interface{}{"review_id": "rev_b", "reply_text": "Unauthorized reply", "publish": true})
	reqRev := httptest.NewRequest(http.MethodPost, "/api/reviews/reply", bytes.NewReader(revReqBody))
	reqRev.Header.Set("Content-Type", "application/json")
	reqRev = reqRev.WithContext(context.WithValue(reqRev.Context(), AuthCtxKey, alphaAuth))
	rrRev := httptest.NewRecorder()
	mux.ServeHTTP(rrRev, reqRev)
	if rrRev.Code != http.StatusInternalServerError && rrRev.Code != http.StatusBadRequest {
		t.Fatalf("expected error when attempting cross-tenant review reply, got %d: %s", rrRev.Code, rrRev.Body.String())
	}

	// Verify /api/listings returns only org_alpha
	rrListings := execGet("/api/listings")
	listingsStr := rrListings.Body.String()
	if !strings.Contains(listingsStr, "Alpha Inc") || strings.Contains(listingsStr, "Beta LLC") {
		t.Fatalf("cross-tenant data leakage in /api/listings: %s", listingsStr)
	}

	// Verify /api/listings/sync does NOT overwrite org_beta listings
	syncReqBody, _ := json.Marshal(map[string]string{
		"business_name": "Alpha Corp Updated",
		"address":       "999 Alpha St",
		"phone":         "+1-555-ALPHA99",
		"website":       "https://alpha-new.com",
	})
	reqSync := httptest.NewRequest(http.MethodPost, "/api/listings/sync", bytes.NewReader(syncReqBody))
	reqSync.Header.Set("Content-Type", "application/json")
	reqSync = reqSync.WithContext(context.WithValue(reqSync.Context(), AuthCtxKey, alphaAuth))
	rrSync := httptest.NewRecorder()
	mux.ServeHTTP(rrSync, reqSync)
	if rrSync.Code != http.StatusOK {
		t.Fatalf("expected 200 for listings sync, got %d", rrSync.Code)
	}

	// Confirm Beta listing was untouched
	var betaName string
	_ = database.QueryRow("SELECT nap_business_name FROM local_listings WHERE id = 'list_b'").Scan(&betaName)
	if betaName != "Beta LLC" {
		t.Fatalf("cross-tenant NAP sync pollution! Beta listing overwritten: %s", betaName)
	}
}

func TestCreateSKU_API(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)

	authCtx := &auth.AuthContext{
		UserID: "usr_ops",
		OrgID:  "org_default",
		Role:   auth.RoleOperations,
	}

	skuPayload := map[string]interface{}{
		"code":             "SKU-FIBER-99",
		"name":             "Fiber Optic Transceiver",
		"category":         "Networking",
		"stock_quantity":   25,
		"min_threshold":    5,
		"reorder_quantity": 40,
		"unit_cost":        85.50,
		"selling_price":    199.00,
		"supplier_id":      "sup_01",
	}
	body, _ := json.Marshal(skuPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/skus", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), AuthCtxKey, authCtx))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/skus, got %d: %s", rr.Code, rr.Body.String())
	}

	var createdSKU db.SKU
	if err := json.Unmarshal(rr.Body.Bytes(), &createdSKU); err != nil {
		t.Fatalf("failed to parse created SKU response: %v", err)
	}
	if createdSKU.Code != "SKU-FIBER-99" || createdSKU.StockQuantity != 25 {
		t.Fatalf("unexpected SKU created data: %+v", createdSKU)
	}

	// Verify it shows up in GET /api/skus
	reqGet := httptest.NewRequest(http.MethodGet, "/api/skus", nil)
	reqGet = reqGet.WithContext(context.WithValue(reqGet.Context(), AuthCtxKey, authCtx))
	rrGet := httptest.NewRecorder()
	mux.ServeHTTP(rrGet, reqGet)

	if !strings.Contains(rrGet.Body.String(), "SKU-FIBER-99") {
		t.Fatalf("created SKU not found in GET /api/skus response: %s", rrGet.Body.String())
	}
}
