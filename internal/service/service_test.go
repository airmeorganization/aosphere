package service

import (
	"context"
	"path/filepath"
	"testing"

	"aosphere/internal/auth"
	"aosphere/internal/db"
	"aosphere/internal/store"
)

func setupTestService(t *testing.T) (*WorkflowService, func()) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_service.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	appStore := store.NewStore(database)
	svc := NewWorkflowService(appStore)

	cleanup := func() {
		database.Close()
	}
	return svc, cleanup
}

func TestStateMachines(t *testing.T) {
	// 1. PO State Machine tests
	if err := ValidatePOTransition(POStatusDraft, POStatusOrdered); err != nil {
		t.Errorf("expected draft -> ordered to be valid, got: %v", err)
	}
	if err := ValidatePOTransition(POStatusOrdered, POStatusReceived); err != nil {
		t.Errorf("expected ordered -> received to be valid, got: %v", err)
	}
	if err := ValidatePOTransition(POStatusReceived, POStatusDraft); err == nil {
		t.Errorf("expected received -> draft to be rejected, got nil")
	}
	if err := ValidatePOTransition(POStatusDraft, POStatusReceived); err == nil {
		t.Errorf("expected draft -> received directly to be rejected, got nil")
	}

	// 2. Invoice State Machine tests
	if err := ValidateInvoiceTransition(InvoiceStatusDraft, InvoiceStatusSent); err != nil {
		t.Errorf("expected draft -> sent to be valid, got: %v", err)
	}
	if err := ValidateInvoiceTransition(InvoiceStatusSent, InvoiceStatusPaid); err != nil {
		t.Errorf("expected sent -> paid to be valid, got: %v", err)
	}
	if err := ValidateInvoiceTransition(InvoiceStatusPaid, InvoiceStatusDraft); err == nil {
		t.Errorf("expected paid -> draft to be rejected, got nil")
	}
}

func TestCreatePurchaseOrder_ServerSidePricingAndHITLRouting(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	agentCtx := &auth.AuthContext{
		UserID:   "usr_agent_01",
		OrgID:    "org_default",
		Email:    "agent@ops.bot",
		Name:     "ops_bot",
		Role:     auth.RoleOperations,
		IsAPIKey: true,
	}

	// 1. Invalid supplier rejection
	_, err := svc.CreatePurchaseOrder(ctx, "org_default", CreatePORequest{
		SupplierID: "non_existent_supplier",
		Items:      []POItem{{SKUCode: "SKU-CORE-100", Quantity: 5, UnitCost: 10.0}},
	}, agentCtx, "")
	if err == nil {
		t.Fatalf("expected error for non-existent supplier, got nil")
	}

	// 2. Small order under $1000 threshold -> Ordered directly
	resSmall, err := svc.CreatePurchaseOrder(ctx, "org_default", CreatePORequest{
		SupplierID: "sup_01",
		Items:      []POItem{{SKUCode: "SKU-CORE-100", Quantity: 5, UnitCost: 20.0}}, // $100 total
	}, agentCtx, "")
	if err != nil {
		t.Fatalf("small PO creation failed: %v", err)
	}
	if resSmall.RequiresApproval {
		t.Errorf("order under $1000 should not require approval")
	}
	if resSmall.PurchaseOrder.Status != POStatusOrdered {
		t.Errorf("expected status 'ordered', got %s", resSmall.PurchaseOrder.Status)
	}
	if resSmall.PurchaseOrder.TotalAmount != 100.0 {
		t.Errorf("expected total 100.0, got %.2f", resSmall.PurchaseOrder.TotalAmount)
	}

	// 3. Large order exceeding $1000 threshold -> Routed to HITL Approval Inbox
	resLarge, err := svc.CreatePurchaseOrder(ctx, "org_default", CreatePORequest{
		SupplierID: "sup_01",
		Items:      []POItem{{SKUCode: "SKU-CORE-100", Quantity: 100, UnitCost: 25.0}}, // $2500 total
		Notes:      "Urgent bulk restock",
	}, agentCtx, "")
	if err != nil {
		t.Fatalf("large PO creation failed: %v", err)
	}
	if !resLarge.RequiresApproval {
		t.Errorf("order exceeding $1000 by agent MUST require approval")
	}
	if resLarge.PurchaseOrder.Status != POStatusPendingApproval {
		t.Errorf("expected status 'pending_approval', got %s", resLarge.PurchaseOrder.Status)
	}
	if resLarge.Approval == nil || resLarge.Approval.Status != "pending" {
		t.Errorf("expected pending approval record to be linked, got: %+v", resLarge.Approval)
	}
	if resLarge.Approval.PayloadHash == "" {
		t.Errorf("expected approval to have non-empty payload hash")
	}
}

func TestDecideApproval_AtomicConditionAndExecution(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	agentCtx := &auth.AuthContext{
		UserID:   "usr_agent_01",
		OrgID:    "org_default",
		Email:    "agent@ops.bot",
		Name:     "ops_bot",
		Role:     auth.RoleOperations,
		IsAPIKey: true,
	}

	humanOwnerCtx := &auth.AuthContext{
		UserID:   "usr_owner_01",
		OrgID:    "org_default",
		Email:    "owner@company.com",
		Name:     "Company Owner",
		Role:     auth.RoleOwner,
		IsAPIKey: false,
	}

	// Create pending PO requiring approval
	res, err := svc.CreatePurchaseOrder(ctx, "org_default", CreatePORequest{
		SupplierID: "sup_01",
		Items:      []POItem{{SKUCode: "SKU-CORE-100", Quantity: 50, UnitCost: 30.0}}, // $1500
	}, agentCtx, "")
	if err != nil || res.Approval == nil {
		t.Fatalf("setup pending PO failed: %v", err)
	}
	apprID := res.Approval.ID
	poID := res.PurchaseOrder.ID

	// 1. Agent attempting to approve -> MUST BE REJECTED
	_, err = svc.DecideApproval(ctx, "org_default", apprID, "approved", agentCtx, "")
	if err == nil {
		t.Errorf("agent should be prohibited from approving HITL actions")
	}

	// 2. Human Owner approves -> MUST SUCCEED and execute underlying PO to 'ordered'
	apprDecided, err := svc.DecideApproval(ctx, "org_default", apprID, "approved", humanOwnerCtx, "")
	if err != nil {
		t.Fatalf("human approval decision failed: %v", err)
	}
	if apprDecided.Status != "approved" || apprDecided.ExecutionStatus != ExecutionStatusCompleted {
		t.Errorf("unexpected approval state after decision: %+v", apprDecided)
	}

	// Verify PO is now ordered
	var poStatus string
	err = svc.Store().DB().QueryRow(`SELECT status FROM purchase_orders WHERE id = ?`, poID).Scan(&poStatus)
	if err != nil || poStatus != POStatusOrdered {
		t.Errorf("expected PO status to be updated to 'ordered', got %s, err: %v", poStatus, err)
	}

	// 3. Second approval attempt -> MUST FAIL (atomic condition WHERE status = 'pending')
	_, err = svc.DecideApproval(ctx, "org_default", apprID, "approved", humanOwnerCtx, "")
	if err == nil {
		t.Errorf("second approval decision on decided item MUST fail with ErrApprovalAlreadyDecided")
	}
}

func TestIdempotencyEngine(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	humanCtx := &auth.AuthContext{
		UserID:   "usr_owner_01",
		OrgID:    "org_default",
		Email:    "owner@company.com",
		Name:     "Company Owner",
		Role:     auth.RoleOwner,
		IsAPIKey: false,
	}

	idempKey := "idemp_test_po_unique_key_001"
	req := CreatePORequest{
		SupplierID: "sup_01",
		Items:      []POItem{{SKUCode: "SKU-CORE-100", Quantity: 10, UnitCost: 15.0}},
	}

	// 1. First execution
	res1, err := svc.CreatePurchaseOrder(ctx, "org_default", req, humanCtx, idempKey)
	if err != nil {
		t.Fatalf("first execution failed: %v", err)
	}
	if res1.IsIdempotentReplay {
		t.Errorf("first execution should not be marked as replay")
	}

	// 2. Replay with identical idempotency key -> Returns cached response without creating second PO
	res2, err := svc.CreatePurchaseOrder(ctx, "org_default", req, humanCtx, idempKey)
	if err != nil {
		t.Fatalf("idempotent replay failed: %v", err)
	}
	if !res2.IsIdempotentReplay {
		t.Errorf("second execution with same key MUST be marked as idempotent replay")
	}
	if res2.PurchaseOrder.ID != res1.PurchaseOrder.ID {
		t.Errorf("replay PO ID mismatch: %s vs %s", res2.PurchaseOrder.ID, res1.PurchaseOrder.ID)
	}

	// Verify only 1 PO exists in DB with that ID
	var count int
	_ = svc.Store().DB().QueryRow(`SELECT COUNT(*) FROM purchase_orders WHERE id = ?`, res1.PurchaseOrder.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 PO in database, got %d", count)
	}
}

func TestInventory_OptimisticLockAndDuplicateSuppression(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	adminCtx := &auth.AuthContext{
		UserID:   "usr_admin",
		OrgID:    "org_default",
		Email:    "admin@company.com",
		Name:     "Admin",
		Role:     auth.RoleAdmin,
		IsAPIKey: false,
	}

	// 1. Optimistic Locking Version Mismatch
	// SKU-001 initially has version 1
	_, err := svc.AdjustStock(ctx, "org_default", "sku_01", 5, "restock", adminCtx, 999, "") // Wrong expectedVersion
	if err == nil {
		t.Errorf("expected ErrStaleVersion when passing incorrect version, got nil")
	}

	// 2. Insufficient stock check
	_, err = svc.AdjustStock(ctx, "org_default", "sku_01", -9999, "excessive consumption", adminCtx, 0, "")
	if err == nil {
		t.Errorf("expected ErrInsufficientStock when consuming beyond available stock")
	}

	// 3. Stock drops below threshold -> Triggers auto-reorder PO
	// SKU-02 has min_threshold = 20, stock = 85
	// Drop stock by -70 -> new stock = 15 <= 20
	skuUpdated, err := svc.AdjustStock(ctx, "org_default", "sku_02", -70, "large sale", adminCtx, 0, "")
	if err != nil {
		t.Fatalf("stock adjustment failed: %v", err)
	}
	if skuUpdated.StockQuantity != 15 {
		t.Errorf("expected stock 15, got %d", skuUpdated.StockQuantity)
	}

	// Verify open PO was created
	var poCount int
	_ = svc.Store().DB().QueryRow(`SELECT COUNT(*) FROM purchase_orders WHERE org_id = 'org_default' AND items_json LIKE '%SKU-SENS-200%'`).Scan(&poCount)
	if poCount == 0 {
		t.Errorf("expected auto-reorder PO to be created")
	}

	// 4. Second consumption while PO is open -> MUST SUPPRESS DUPLICATE REORDER
	initialPOCount := poCount
	_, err = svc.AdjustStock(ctx, "org_default", "sku_02", -1, "additional sale", adminCtx, 0, "")
	if err != nil {
		t.Fatalf("second adjustment failed: %v", err)
	}

	var newPOCount int
	_ = svc.Store().DB().QueryRow(`SELECT COUNT(*) FROM purchase_orders WHERE org_id = 'org_default' AND items_json LIKE '%SKU-SENS-200%'`).Scan(&newPOCount)
	if newPOCount != initialPOCount {
		t.Errorf("duplicate auto-reorder was NOT suppressed: po count went from %d to %d", initialPOCount, newPOCount)
	}
}

func TestReconciliation_DiscrepancyDetection(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Insert an anomalous pending PO without an approval
	_, _ = svc.Store().DB().Exec(`INSERT INTO purchase_orders (id, org_id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes, version)
		VALUES ('po_anom_01', 'org_default', 'PO-ANOMALY', 'sup_01', 'pending_approval', 5000.0, '[]', 'agent', 'test', 'Orphaned', 1)`)

	report, err := svc.ReconcileDiscrepancies(ctx, "org_default")
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	foundOrphan := false
	for _, d := range report.Discrepancies {
		if d.Type == "ORPHAN_PENDING_PO" && d.EntityID == "po_anom_01" {
			foundOrphan = true
		}
	}
	if !foundOrphan {
		t.Errorf("expected reconciliation to catch orphan pending PO, got discrepancies: %+v", report.Discrepancies)
	}
}
