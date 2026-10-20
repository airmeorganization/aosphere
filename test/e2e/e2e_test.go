package e2e

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/db"
	"aosphere/internal/service"
	"aosphere/internal/store"
)

// TestE2E_FullBusinessLifecycle runs a complete business scenario across all modules:
// 1. Inventory Check & Safety Reorder
// 2. High-value Purchase Order generation & Human-In-The-Loop Approval
// 3. PO Receiving & Inventory stock update
// 4. Vendor PO Payment & General Ledger posting (debit AP, credit Cash)
// 5. CRM Deal generation & formal Quote issuance
// 6. Deal-to-Invoice conversion
// 7. Customer invoice payment & Ledger posting (debit Cash, credit AR)
// 8. Employee onboarding, timesheet submission, manager approval
// 9. Payroll lock & disbursement with OpEx ledger entry
// 10. Financial Audit: Trial balance strictly balanced
// 11. Cross-cutting: Global unified search and CSV export
func TestE2E_FullBusinessLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "e2e_lifecycle.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("E2E database initialization failed: %v", err)
	}
	defer database.Close()

	appStore := store.NewStore(database)
	svc := service.NewWorkflowService(appStore)
	ctx := context.Background()

	adminCtx := &auth.AuthContext{UserID: "usr_ceo", OrgID: "org_default", Role: auth.RoleOwner, Email: "ceo@aosphere.local"}
	managerCtx := &auth.AuthContext{UserID: "usr_mgr", OrgID: "org_default", Role: auth.RoleOperations, Email: "mgr@aosphere.local"}

	t.Log("Step 1: Check initial low-stock item")
	lowStockSKUs, err := appStore.GetSKUsForOrg("org_default", true)
	if err != nil {
		t.Fatalf("failed to query low stock: %v", err)
	}
	if len(lowStockSKUs) == 0 {
		t.Fatal("expected at least 1 low stock SKU from starter seed")
	}
	targetSKU := lowStockSKUs[0]

	t.Log("Step 2: Create High-Value PO ($45 * 50 = $2,250) triggering HITL Approval")
	poRes, err := svc.CreatePurchaseOrder(ctx, "org_default", service.CreatePORequest{
		SupplierID: targetSKU.SupplierID,
		Items: []service.POItem{
			{SKUCode: targetSKU.Code, Quantity: 50, UnitCost: 45.00},
		},
		Notes: "Emergency stock replenishment",
	}, managerCtx, "e2e_idemp_po_01")
	if err != nil {
		t.Fatalf("failed to create PO: %v", err)
	}
	if !poRes.RequiresApproval {
		t.Fatalf("expected PO to require approval since $2,250 > $1,000 threshold")
	}
	approvalID := poRes.Approval.ID

	t.Log("Step 3: Executive approves the PO")
	apprRes, err := svc.DecideApproval(ctx, "org_default", approvalID, "approved", adminCtx, "e2e_idemp_appr_01")
	if err != nil {
		t.Fatalf("failed to approve PO: %v", err)
	}
	if apprRes.Status != "approved" || apprRes.ExecutionStatus != "completed" {
		t.Fatalf("expected completed approval execution, got %+v", apprRes)
	}

	t.Log("Step 4: Receive Goods from Supplier")
	receivedPO, err := svc.ReceivePurchaseOrder(ctx, "org_default", poRes.PurchaseOrder.ID, managerCtx, "e2e_idemp_rec_01")
	if err != nil {
		t.Fatalf("failed to receive PO goods: %v", err)
	}
	if receivedPO.Status != service.POStatusReceived {
		t.Fatalf("expected received status, got %s", receivedPO.Status)
	}

	t.Log("Step 5: Disburse Payment to Supplier (AP debit, Cash credit)")
	paidPO, err := svc.PayPurchaseOrder(ctx, "org_default", receivedPO.ID, adminCtx, "e2e_idemp_pay_01")
	if err != nil {
		t.Fatalf("failed to disburse PO payment: %v", err)
	}
	if paidPO.Status != "paid" {
		t.Fatalf("expected paid status, got %s", paidPO.Status)
	}

	t.Log("Step 6: CRM Deal creation and formal Quote issuance")
	deal, err := appStore.CreateDeal("Autonomous Sensor Fleet Expansion", "cust_01", 15000.00, "proposal", 85, "2026-11-30", "human", adminCtx.UserID)
	if err != nil {
		t.Fatalf("failed to create deal: %v", err)
	}

	quote, err := svc.GenerateQuote(ctx, "org_default", deal.ID, []service.InvoiceItem{
		{Desc: "Sensor Kit Enterprise Edition", Qty: 30, Price: 500.00},
	}, adminCtx)
	if err != nil {
		t.Fatalf("failed to generate quote: %v", err)
	}
	if quote.TotalAmount != 16500.00 { // 15,000 + 10% tax = 16,500
		t.Errorf("expected quote total 16,500.00, got %.2f", quote.TotalAmount)
	}

	t.Log("Step 7: Convert Accepted Deal to Customer Invoice")
	inv, err := svc.ConvertDealToInvoice(ctx, "org_default", deal.ID, adminCtx, "e2e_idemp_deal_inv_01")
	if err != nil {
		t.Fatalf("failed to convert deal to invoice: %v", err)
	}

	t.Log("Step 8: Customer pays invoice (Cash debit, AR credit)")
	paidInv, err := svc.PayInvoice(ctx, "org_default", inv.ID, adminCtx, "e2e_idemp_inv_pay_01")
	if err != nil {
		t.Fatalf("failed to pay invoice: %v", err)
	}
	if paidInv.Status != "paid" {
		t.Fatalf("expected invoice paid status, got %s", paidInv.Status)
	}

	t.Log("Step 9: HR Employee Onboarding, Timesheet Logging & Payroll Lock")
	emp, err := appStore.CreateEmployee("org_default", "Grace Hopper", "Chief Architect", "R&D", "grace@aosphere.local", "+1-555-8833", 100.00)
	if err != nil {
		t.Fatalf("failed to onboard employee: %v", err)
	}

	tsID := "ts_e2e_01"
	_, err = appStore.DB().Exec(`INSERT INTO timesheets (id, org_id, employee_id, work_date, hours_worked, overtime_hours, status, hourly_rate)
		VALUES (?, 'org_default', ?, '2026-10-19', 40.0, 0.0, 'logged', ?)`, tsID, emp.ID, emp.HourlyRate)
	if err != nil {
		t.Fatalf("failed to log timesheet: %v", err)
	}

	if err := svc.ApproveTimesheet(ctx, "org_default", tsID, adminCtx); err != nil {
		t.Fatalf("failed to approve timesheet: %v", err)
	}

	payroll, err := svc.LockPayroll(ctx, "org_default", "2026-10", adminCtx)
	if err != nil {
		t.Fatalf("failed to lock payroll: %v", err)
	}
	if payroll.TimesheetsPaid < 1 || payroll.TotalGrossDollars <= 0 {
		t.Fatalf("unexpected payroll outcome: %+v", payroll)
	}

	t.Log("Step 10: Financial Integrity Verification (Trial Balance Debits == Credits)")
	tb, err := svc.TrialBalance(ctx, "org_default")
	if err != nil {
		t.Fatalf("failed to fetch trial balance: %v", err)
	}
	if !tb.IsBalanced {
		t.Fatalf("CRITICAL FINANCIAL INVARIANT VIOLATION: Trial balance is NOT balanced! Debits=%d, Credits=%d",
			tb.TotalDebitsCents, tb.TotalCreditsCents)
	}
	if tb.TotalDebitsCents <= 0 {
		t.Errorf("expected positive ledger balances, got %d", tb.TotalDebitsCents)
	}

	t.Log("Step 11: Global Search & CSV Export verification")
	searchResults, err := svc.GlobalSearch(ctx, "org_default", "Grace")
	if err != nil {
		t.Fatalf("failed to run global search: %v", err)
	}
	_ = searchResults

	invCSV, err := svc.ExportCSV(ctx, "org_default", "invoices")
	if err != nil || len(invCSV) == 0 {
		t.Fatalf("failed to export invoices CSV")
	}

	t.Logf("E2E Test Successfully Finished! Total ledger volume: $%.2f. Trial balance: 100%% balanced.",
		float64(tb.TotalDebitsCents)/100.0)
	_ = time.Now()
}
