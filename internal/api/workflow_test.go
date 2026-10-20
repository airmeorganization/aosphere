package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aosphere/internal/auth"
	"aosphere/internal/db"
	"aosphere/internal/service"
)

func TestWorkflowAPI_PurchaseOrderAndHITL(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)

	// Admin context with PermManageOrders and PermApproveOrders
	adminCtx := &auth.AuthContext{
		UserID: "usr_admin",
		OrgID:  "org_default",
		Role:   auth.RoleAdmin,
		Email:  "admin@aosphere.local",
	}

	// 1. Create PO over $1,000 threshold ($45 * 50 = $2,250)
	poPayload := map[string]interface{}{
		"supplier_id": "sup_01",
		"items": []map[string]interface{}{
			{
				"sku_code":  "SKU-CORE-100",
				"quantity":  50,
				"unit_cost": 45.0,
			},
		},
		"notes": "Automated replenishment batch",
	}
	bodyBytes, _ := json.Marshal(poPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/purchase-orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idemp_test_po_hitl_001")
	req = req.WithContext(context.WithValue(req.Context(), AuthCtxKey, adminCtx))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d: %s", rr.Code, rr.Body.String())
	}

	var createdPO db.PurchaseOrder
	if err := json.NewDecoder(rr.Body).Decode(&createdPO); err != nil {
		t.Fatalf("failed to parse PO response: %v", err)
	}

	if createdPO.Status != service.POStatusPendingApproval {
		t.Errorf("expected PO status %q, got %q", service.POStatusPendingApproval, createdPO.Status)
	}
	if createdPO.TotalAmount != 2250.0 {
		t.Errorf("expected total amount 2250.0, got %.2f", createdPO.TotalAmount)
	}

	// 2. Verify Idempotency replay with identical key
	reqReplay := httptest.NewRequest(http.MethodPost, "/api/purchase-orders", bytes.NewReader(bodyBytes))
	reqReplay.Header.Set("Content-Type", "application/json")
	reqReplay.Header.Set("Idempotency-Key", "idemp_test_po_hitl_001")
	reqReplay = reqReplay.WithContext(context.WithValue(reqReplay.Context(), AuthCtxKey, adminCtx))
	rrReplay := httptest.NewRecorder()

	mux.ServeHTTP(rrReplay, reqReplay)
	if rrReplay.Code != http.StatusCreated {
		t.Errorf("expected 201 on idempotent replay, got %d", rrReplay.Code)
	}
	var replayPO db.PurchaseOrder
	_ = json.NewDecoder(rrReplay.Body).Decode(&replayPO)
	if replayPO.ID != createdPO.ID {
		t.Errorf("expected replayed PO ID %s, got %s", createdPO.ID, replayPO.ID)
	}

	// 3. Find the created approval inbox record
	var apprID string
	err := database.QueryRow(`SELECT id FROM approvals WHERE entity_id = ? AND org_id = 'org_default'`, createdPO.ID).Scan(&apprID)
	if err != nil {
		t.Fatalf("expected approval record for PO %s, err: %v", createdPO.ID, err)
	}

	// 4. Try approving with an AI Agent caller -> Must be rejected with 403 Forbidden!
	agentCtx := &auth.AuthContext{
		UserID:   "agent_01",
		OrgID:    "org_default",
		Role:     auth.RoleOperations,
		IsAPIKey: true,
	}
	decidePayload := map[string]interface{}{
		"approval_id": apprID,
		"decision":    "approved",
	}
	agentBytes, _ := json.Marshal(decidePayload)
	reqAgent := httptest.NewRequest(http.MethodPost, "/api/approvals/decide", bytes.NewReader(agentBytes))
	reqAgent.Header.Set("Content-Type", "application/json")
	reqAgent = reqAgent.WithContext(context.WithValue(reqAgent.Context(), AuthCtxKey, agentCtx))
	rrAgent := httptest.NewRecorder()

	mux.ServeHTTP(rrAgent, reqAgent)
	if rrAgent.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden when agent attempts HITL approval, got %d: %s", rrAgent.Code, rrAgent.Body.String())
	}

	// 5. Human Admin authorizes the approval -> Transitions PO to ordered and reserves inventory
	adminBytes, _ := json.Marshal(decidePayload)
	reqAdmin := httptest.NewRequest(http.MethodPost, "/api/approvals/decide", bytes.NewReader(adminBytes))
	reqAdmin.Header.Set("Content-Type", "application/json")
	reqAdmin = reqAdmin.WithContext(context.WithValue(reqAdmin.Context(), AuthCtxKey, adminCtx))
	rrAdmin := httptest.NewRecorder()

	mux.ServeHTTP(rrAdmin, reqAdmin)
	if rrAdmin.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for human approval, got %d: %s", rrAdmin.Code, rrAdmin.Body.String())
	}

	// 6. Verify PO is now ordered and inventory is reserved
	var poStatus string
	database.QueryRow(`SELECT status FROM purchase_orders WHERE id = ?`, createdPO.ID).Scan(&poStatus)
	if poStatus != service.POStatusOrdered {
		t.Errorf("expected PO to be 'ordered' after approval, got %q", poStatus)
	}

	var reservedQty int
	database.QueryRow(`SELECT reserved_quantity FROM skus WHERE code = 'SKU-CORE-100' AND org_id = 'org_default'`).Scan(&reservedQty)
	if reservedQty < 50 {
		t.Errorf("expected reserved_quantity >= 50, got %d", reservedQty)
	}
}

func TestWorkflowAPI_OptimisticLockingAndReconciliation(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)

	adminCtx := &auth.AuthContext{
		UserID: "usr_admin",
		OrgID:  "org_default",
		Role:   auth.RoleAdmin,
		Email:  "admin@aosphere.local",
	}

	// 1. Fetch current SKU version
	var currentVer int
	var skuID string
	err := database.QueryRow(`SELECT id, version FROM skus WHERE code = 'SKU-CORE-100' AND org_id = 'org_default'`).Scan(&skuID, &currentVer)
	if err != nil {
		t.Fatalf("failed to query SKU: %v", err)
	}

	// 2. Adjust stock with correct expected_version -> Succeeds
	adjPayload := map[string]interface{}{
		"sku_id":           skuID,
		"delta":            5,
		"reason":           "Manual recount",
		"expected_version": currentVer,
	}
	b, _ := json.Marshal(adjPayload)
	req1 := httptest.NewRequest(http.MethodPost, "/api/skus/adjust", bytes.NewReader(b))
	req1.Header.Set("Content-Type", "application/json")
	req1 = req1.WithContext(context.WithValue(req1.Context(), AuthCtxKey, adminCtx))
	rr1 := httptest.NewRecorder()
	mux.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for stock adjust, got %d: %s", rr1.Code, rr1.Body.String())
	}

	// 3. Adjust stock with stale expected_version -> 409 Conflict!
	bStale, _ := json.Marshal(adjPayload) // still has old version
	reqStale := httptest.NewRequest(http.MethodPost, "/api/skus/adjust", bytes.NewReader(bStale))
	reqStale.Header.Set("Content-Type", "application/json")
	reqStale = reqStale.WithContext(context.WithValue(reqStale.Context(), AuthCtxKey, adminCtx))
	rrStale := httptest.NewRecorder()
	mux.ServeHTTP(rrStale, reqStale)

	if rrStale.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict on stale version, got %d", rrStale.Code)
	}

	// 4. Test Reconciliation endpoint
	recReq := httptest.NewRequest(http.MethodGet, "/api/reconciliation", nil)
	recReq = recReq.WithContext(context.WithValue(recReq.Context(), AuthCtxKey, adminCtx))
	rrRec := httptest.NewRecorder()
	mux.ServeHTTP(rrRec, recReq)

	if rrRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for reconciliation endpoint, got %d", rrRec.Code)
	}

	var report service.ReconciliationReport
	if err := json.NewDecoder(rrRec.Body).Decode(&report); err != nil {
		t.Fatalf("failed to parse reconciliation report: %v", err)
	}
	t.Logf("Reconciliation complete. Total checks: %d, Discrepancies: %d", report.TotalChecked, len(report.Discrepancies))
}

func TestWorkflowAPI_TrialBalanceAndBackup(t *testing.T) {
	testAPI, database := setupTestAPI(t)
	defer database.Close()

	mux := http.NewServeMux()
	testAPI.RegisterRoutes(mux)

	adminCtx := &auth.AuthContext{
		UserID: "usr_admin",
		OrgID:  "org_default",
		Role:   auth.RoleAdmin,
		Email:  "admin@aosphere.local",
	}

	// 1. Create and pay an invoice to generate ledger entries
	invPayload := map[string]interface{}{
		"customer_id": "cust_01",
		"items": []map[string]interface{}{
			{
				"desc":  "Technical Services",
				"qty":   2,
				"price": 500.0,
			},
		},
		"due_date": "2026-11-01",
	}
	bInv, _ := json.Marshal(invPayload)
	reqInv := httptest.NewRequest(http.MethodPost, "/api/invoices", bytes.NewReader(bInv))
	reqInv.Header.Set("Content-Type", "application/json")
	reqInv = reqInv.WithContext(context.WithValue(reqInv.Context(), AuthCtxKey, adminCtx))
	rrInv := httptest.NewRecorder()
	mux.ServeHTTP(rrInv, reqInv)

	if rrInv.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for invoice, got %d: %s", rrInv.Code, rrInv.Body.String())
	}

	var createdInv db.Invoice
	_ = json.NewDecoder(rrInv.Body).Decode(&createdInv)

	payPayload := map[string]interface{}{
		"invoice_id": createdInv.ID,
	}
	bPay, _ := json.Marshal(payPayload)
	reqPay := httptest.NewRequest(http.MethodPost, "/api/invoices/pay", bytes.NewReader(bPay))
	reqPay.Header.Set("Content-Type", "application/json")
	reqPay = reqPay.WithContext(context.WithValue(reqPay.Context(), AuthCtxKey, adminCtx))
	rrPay := httptest.NewRecorder()
	mux.ServeHTTP(rrPay, reqPay)

	if rrPay.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for invoice payment, got %d: %s", rrPay.Code, rrPay.Body.String())
	}

	// 2. Query /api/ledger/trial-balance
	reqTB := httptest.NewRequest(http.MethodGet, "/api/ledger/trial-balance", nil)
	reqTB = reqTB.WithContext(context.WithValue(reqTB.Context(), AuthCtxKey, adminCtx))
	rrTB := httptest.NewRecorder()
	mux.ServeHTTP(rrTB, reqTB)

	if rrTB.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for trial-balance, got %d: %s", rrTB.Code, rrTB.Body.String())
	}

	var tbResp struct {
		OrgID             string `json:"org_id"`
		TotalDebitsCents  int64  `json:"total_debits_cents"`
		TotalCreditsCents int64  `json:"total_credits_cents"`
		IsBalanced        bool   `json:"is_balanced"`
	}
	if err := json.NewDecoder(rrTB.Body).Decode(&tbResp); err != nil {
		t.Fatalf("failed to decode trial-balance response: %v", err)
	}

	if !tbResp.IsBalanced {
		t.Errorf("expected trial balance to be balanced, got false")
	}
	if tbResp.TotalDebitsCents != tbResp.TotalCreditsCents {
		t.Errorf("expected debits (%d) == credits (%d)", tbResp.TotalDebitsCents, tbResp.TotalCreditsCents)
	}
	if tbResp.TotalDebitsCents == 0 {
		t.Errorf("expected total debits > 0 after invoice and payment posting")
	}

	// 3. Trigger /api/backup/run
	reqBackup := httptest.NewRequest(http.MethodPost, "/api/backup/run", nil)
	reqBackup = reqBackup.WithContext(context.WithValue(reqBackup.Context(), AuthCtxKey, adminCtx))
	rrBackup := httptest.NewRecorder()
	mux.ServeHTTP(rrBackup, reqBackup)

	if rrBackup.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for backup run, got %d: %s", rrBackup.Code, rrBackup.Body.String())
	}

	var backupResp struct {
		BackupPath  string `json:"backup_path"`
		SizeBytes   int64  `json:"size_bytes"`
		IntegrityOK bool   `json:"integrity_ok"`
	}
	if err := json.NewDecoder(rrBackup.Body).Decode(&backupResp); err != nil {
		t.Fatalf("failed to decode backup response: %v", err)
	}

	if !backupResp.IntegrityOK {
		t.Errorf("expected backup integrity check to pass, got false")
	}
	if backupResp.SizeBytes <= 0 {
		t.Errorf("expected backup size > 0, got %d", backupResp.SizeBytes)
	}
}

