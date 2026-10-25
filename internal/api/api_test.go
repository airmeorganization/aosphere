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
	"aosphere/internal/service"
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
	wfService := service.NewWorkflowService(appStore)
	testAPI := NewAPI(appStore, wfService, "test")
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

func TestEmployeeShiftTask_API(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)

	authCtx := &auth.AuthContext{
		UserID: "usr_admin",
		OrgID:  "org_default",
		Role:   auth.RoleOwner,
	}

	// 1. Create Employee
	empPayload := map[string]interface{}{
		"name":        "Elena Rostova",
		"role":        "Robotics Engineer",
		"department":  "Engineering",
		"email":       "elena@aosphere.local",
		"phone":       "+1-555-4921",
		"hourly_rate": 65.0,
	}
	body, _ := json.Marshal(empPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/employees", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), AuthCtxKey, authCtx))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for POST /api/employees, got %d: %s", rr.Code, rr.Body.String())
	}
	var emp db.Employee
	_ = json.Unmarshal(rr.Body.Bytes(), &emp)
	if emp.Name != "Elena Rostova" || emp.HourlyRate != 65.0 {
		t.Fatalf("unexpected employee created: %+v", emp)
	}

	// 2. Schedule Shift
	shiftPayload := map[string]interface{}{
		"employee_id": emp.ID,
		"shift_date":  "2026-11-01",
		"start_time":  "08:00",
		"end_time":    "16:30",
		"station":     "Robotics Lab Alpha",
		"status":      "scheduled",
	}
	sBody, _ := json.Marshal(shiftPayload)
	sReq := httptest.NewRequest(http.MethodPost, "/api/shifts", bytes.NewReader(sBody))
	sReq = sReq.WithContext(context.WithValue(sReq.Context(), AuthCtxKey, authCtx))
	sRR := httptest.NewRecorder()
	mux.ServeHTTP(sRR, sReq)

	if sRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 for POST /api/shifts, got %d: %s", sRR.Code, sRR.Body.String())
	}
	var shf db.Shift
	_ = json.Unmarshal(sRR.Body.Bytes(), &shf)
	if shf.Station != "Robotics Lab Alpha" || shf.EmployeeName != "Elena Rostova" {
		t.Fatalf("unexpected shift created: %+v", shf)
	}

	// 3. Create Task
	taskPayload := map[string]interface{}{
		"title":        "Calibrate Sensor Arrays",
		"description":  "Perform laser telemetry alignment on MCU v2",
		"project_name": "Quantum Drone Initiative",
		"assignee_id":  emp.ID,
		"priority":     "high",
		"due_date":     "2026-11-05",
	}
	tBody, _ := json.Marshal(taskPayload)
	tReq := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewReader(tBody))
	tReq = tReq.WithContext(context.WithValue(tReq.Context(), AuthCtxKey, authCtx))
	tRR := httptest.NewRecorder()
	mux.ServeHTTP(tRR, tReq)

	if tRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 for POST /api/tasks, got %d: %s", tRR.Code, tRR.Body.String())
	}
	var tsk db.Task
	_ = json.Unmarshal(tRR.Body.Bytes(), &tsk)
	if tsk.Title != "Calibrate Sensor Arrays" || tsk.Priority != "high" {
		t.Fatalf("unexpected task created: %+v", tsk)
	}
}

func TestSupplyChain_WarehousesAndGRN_API(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)

	authCtx := &auth.AuthContext{
		UserID: "usr_admin",
		OrgID:  "org_default",
		Role:   auth.RoleOwner,
	}

	// 1. Create Warehouse
	whPayload := map[string]interface{}{
		"name":    "West Coast Distribution Hub",
		"code":    "WH-WEST",
		"address": "770 Pacific Way, Gate 4",
	}
	wBody, _ := json.Marshal(whPayload)
	wReq := httptest.NewRequest(http.MethodPost, "/api/warehouses", bytes.NewReader(wBody))
	wReq = wReq.WithContext(context.WithValue(wReq.Context(), AuthCtxKey, authCtx))
	wRR := httptest.NewRecorder()
	mux.ServeHTTP(wRR, wReq)

	if wRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 for POST /api/warehouses, got %d: %s", wRR.Code, wRR.Body.String())
	}
	var wh db.Warehouse
	_ = json.Unmarshal(wRR.Body.Bytes(), &wh)
	if wh.Code != "WH-WEST" {
		t.Fatalf("unexpected warehouse created: %+v", wh)
	}

	// 2. Create Purchase Order to receive against
	poPayload := map[string]interface{}{
		"supplier_id": "sup_01",
		"items": []map[string]interface{}{
			{"sku_code": "SKU-CORE-100", "quantity": 10, "unit_cost": 45.0},
		},
		"notes": "Fast-track shipment",
	}
	poBody, _ := json.Marshal(poPayload)
	poReq := httptest.NewRequest(http.MethodPost, "/api/purchase-orders", bytes.NewReader(poBody))
	poReq = poReq.WithContext(context.WithValue(poReq.Context(), AuthCtxKey, authCtx))
	poRR := httptest.NewRecorder()
	mux.ServeHTTP(poRR, poReq)

	if poRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 for PO creation, got %d: %s", poRR.Code, poRR.Body.String())
	}
	var po db.PurchaseOrder
	_ = json.Unmarshal(poRR.Body.Bytes(), &po)

	// 3. Receive GRN (Partial receipt: 4 of 10)
	grnPayload := map[string]interface{}{
		"po_id":        po.ID,
		"warehouse_id": wh.ID,
		"received_by":  "Marcus Vance",
		"notes":        "Initial pallet delivery",
		"items": []map[string]interface{}{
			{
				"sku_code":     "SKU-CORE-100",
				"quantity":     4,
				"bin_location": "B-02-14",
				"batch_number": "BATCH-2026-001",
			},
		},
	}
	grnBody, _ := json.Marshal(grnPayload)
	grnReq := httptest.NewRequest(http.MethodPost, "/api/grns", bytes.NewReader(grnBody))
	grnReq = grnReq.WithContext(context.WithValue(grnReq.Context(), AuthCtxKey, authCtx))
	grnRR := httptest.NewRecorder()
	mux.ServeHTTP(grnRR, grnReq)

	if grnRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 for GRN creation, got %d: %s", grnRR.Code, grnRR.Body.String())
	}
	var grnRes service.ReceiveGRNResult
	_ = json.Unmarshal(grnRR.Body.Bytes(), &grnRes)
	if !grnRes.IsPartial || grnRes.PurchaseOrder.Status != service.POStatusPartiallyReceived {
		t.Fatalf("expected partial receipt status, got: %+v", grnRes)
	}

	// 4. Create Supplier Bill and run 3-Way Match
	billPayload := map[string]interface{}{
		"supplier_id": "sup_01",
		"po_id":       po.ID,
		"amount":      450.0,
		"notes":       "Invoice for 10x SKU-CORE-100",
	}
	bBody, _ := json.Marshal(billPayload)
	bReq := httptest.NewRequest(http.MethodPost, "/api/supplier-bills", bytes.NewReader(bBody))
	bReq = bReq.WithContext(context.WithValue(bReq.Context(), AuthCtxKey, authCtx))
	bRR := httptest.NewRecorder()
	mux.ServeHTTP(bRR, bReq)

	if bRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 for Supplier Bill, got %d: %s", bRR.Code, bRR.Body.String())
	}
	var bill db.SupplierBill
	_ = json.Unmarshal(bRR.Body.Bytes(), &bill)

	// Run 3-Way Match (shortfall expected: received 4 vs ordered 10)
	matchPayload := map[string]interface{}{"bill_id": bill.ID}
	mBody, _ := json.Marshal(matchPayload)
	mReq := httptest.NewRequest(http.MethodPost, "/api/supplier-bills/match", bytes.NewReader(mBody))
	mReq = mReq.WithContext(context.WithValue(mReq.Context(), AuthCtxKey, authCtx))
	mRR := httptest.NewRecorder()
	mux.ServeHTTP(mRR, mReq)

	if mRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for 3-way match, got %d: %s", mRR.Code, mRR.Body.String())
	}
	var matchRes service.ThreeWayMatchResult
	_ = json.Unmarshal(mRR.Body.Bytes(), &matchRes)
	if matchRes.Status != "discrepancy_qty" || !matchRes.RequiresApproval {
		t.Fatalf("expected discrepancy_qty requiring approval, got: %+v", matchRes)
	}
}
