package service

import (
	"context"
	"strings"
	"testing"

	"aosphere/internal/auth"
)

func TestLifecycle_POPayment(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	authCtx := &auth.AuthContext{UserID: "usr_admin", OrgID: "org_default", Role: auth.RoleOwner}

	// 1. Create a PO below $1000 threshold so it is placed directly into 'ordered'
	res, err := svc.CreatePurchaseOrder(ctx, "org_default", CreatePORequest{
		SupplierID: "sup_01",
		Items: []POItem{
			{SKUCode: "SKU-CORE-100", Quantity: 10, UnitCost: 45.00},
		},
		Notes: "Lifecycle test PO",
	}, authCtx, "")
	if err != nil {
		t.Fatalf("failed to create PO: %v", err)
	}
	if res.PurchaseOrder.Status != POStatusOrdered {
		t.Fatalf("expected PO status ordered, got %s", res.PurchaseOrder.Status)
	}

	// 2. Receive the PO
	recPO, err := svc.ReceivePurchaseOrder(ctx, "org_default", res.PurchaseOrder.ID, authCtx, "")
	if err != nil {
		t.Fatalf("failed to receive PO: %v", err)
	}
	if recPO.Status != POStatusReceived {
		t.Fatalf("expected PO status received, got %s", recPO.Status)
	}

	// 3. Pay the PO (Debits AP, Credits Cash)
	paidPO, err := svc.PayPurchaseOrder(ctx, "org_default", recPO.ID, authCtx, "")
	if err != nil {
		t.Fatalf("failed to pay PO: %v", err)
	}
	if paidPO.Status != "paid" {
		t.Fatalf("expected PO status paid, got %s", paidPO.Status)
	}

	// 4. Verify Trial Balance remains strictly balanced
	tb, err := svc.TrialBalance(ctx, "org_default")
	if err != nil {
		t.Fatalf("failed to get trial balance: %v", err)
	}
	if !tb.IsBalanced {
		t.Fatalf("expected trial balance to be balanced after PO payment, debits=%d, credits=%d",
			tb.TotalDebitsCents, tb.TotalCreditsCents)
	}
}

func TestLifecycle_QuoteAndDealConversion(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	authCtx := &auth.AuthContext{UserID: "usr_admin", OrgID: "org_default", Role: auth.RoleOwner}

	// 1. Generate Quote from deal_01
	quoteItems := []InvoiceItem{
		{Desc: "Custom drone controller batch", Qty: 2, Price: 500.00},
		{Desc: "Software telemetry module", Qty: 1, Price: 200.00},
	}
	quote, err := svc.GenerateQuote(ctx, "org_default", "deal_01", quoteItems, authCtx)
	if err != nil {
		t.Fatalf("failed to generate quote: %v", err)
	}
	if quote.Subtotal != 1200.00 {
		t.Errorf("expected subtotal 1200.00, got %.2f", quote.Subtotal)
	}
	if quote.TaxAmount != 120.00 {
		t.Errorf("expected tax 120.00 (10%%), got %.2f", quote.TaxAmount)
	}
	if quote.TotalAmount != 1320.00 {
		t.Errorf("expected total 1320.00, got %.2f", quote.TotalAmount)
	}
	if !strings.HasPrefix(quote.QuoteNumber, "QTE-") {
		t.Errorf("expected quote number to start with QTE-, got %s", quote.QuoteNumber)
	}

	// 2. Convert Deal to Invoice
	inv, err := svc.ConvertDealToInvoice(ctx, "org_default", "deal_01", authCtx, "")
	if err != nil {
		t.Fatalf("failed to convert deal to invoice: %v", err)
	}
	if !strings.HasPrefix(inv.InvoiceNumber, "INV-") {
		t.Errorf("expected invoice number to start with INV-, got %s", inv.InvoiceNumber)
	}
	if inv.TotalAmount <= 0 {
		t.Errorf("expected positive invoice amount, got %.2f", inv.TotalAmount)
	}

	// Verify deal stage updated to 'won'
	var dealStage string
	_ = svc.store.DB().QueryRow("SELECT stage FROM deals WHERE id = 'deal_01'").Scan(&dealStage)
	if dealStage != "won" {
		t.Errorf("expected deal stage 'won', got %s", dealStage)
	}
}

func TestLifecycle_TimesheetsAndPayrollLock(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	authCtx := &auth.AuthContext{UserID: "usr_admin", OrgID: "org_default", Role: auth.RoleOwner}

	// 1. Insert an approved timesheet for emp_01
	tsID := "ts_test_01"
	_, err := svc.store.DB().Exec(`INSERT INTO timesheets (id, org_id, employee_id, work_date, hours_worked, overtime_hours, status, hourly_rate)
		VALUES (?, 'org_default', 'emp_01', '2026-10-18', 40.0, 5.0, 'logged', 50.0)`, tsID)
	if err != nil {
		t.Fatalf("failed to seed timesheet: %v", err)
	}

	// 2. Approve the timesheet
	if err := svc.ApproveTimesheet(ctx, "org_default", tsID, authCtx); err != nil {
		t.Fatalf("failed to approve timesheet: %v", err)
	}

	// 3. Lock Payroll for the period
	payrollRes, err := svc.LockPayroll(ctx, "org_default", "2026-10", authCtx)
	if err != nil {
		t.Fatalf("failed to lock payroll: %v", err)
	}
	if payrollRes.TimesheetsPaid < 1 {
		t.Errorf("expected at least 1 timesheet paid, got %d", payrollRes.TimesheetsPaid)
	}
	// 45 hours * $50/hr = $2250.00
	if payrollRes.TotalGrossDollars != 2250.00 {
		t.Errorf("expected $2250.00 gross total, got %.2f", payrollRes.TotalGrossDollars)
	}

	// 4. Verify timesheet status changed to 'paid'
	var status string
	_ = svc.store.DB().QueryRow("SELECT status FROM timesheets WHERE id = ?", tsID).Scan(&status)
	if status != "paid" {
		t.Errorf("expected timesheet status 'paid', got %s", status)
	}

	// 5. Verify General Ledger trial balance is balanced
	tb, err := svc.TrialBalance(ctx, "org_default")
	if err != nil {
		t.Fatalf("failed to check trial balance: %v", err)
	}
	if !tb.IsBalanced {
		t.Fatalf("expected trial balance to be balanced after payroll lock, debits=%d, credits=%d",
			tb.TotalDebitsCents, tb.TotalCreditsCents)
	}
}

func TestLifecycle_DocumentsRepository(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Create document
	doc, err := svc.CreateDocument(ctx, "org_default", "Supplier Quality Manual", "Operations", "Standard procedures for QA inspect", "Lead Eng")
	if err != nil {
		t.Fatalf("failed to create document: %v", err)
	}
	if doc.Title != "Supplier Quality Manual" {
		t.Errorf("unexpected doc title: %s", doc.Title)
	}

	// 2. List documents
	docs, err := svc.ListDocuments(ctx, "org_default")
	if err != nil {
		t.Fatalf("failed to list documents: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least 1 document")
	}

	// 3. Delete document
	if err := svc.DeleteDocument(ctx, "org_default", doc.ID); err != nil {
		t.Fatalf("failed to delete document: %v", err)
	}
	docsAfter, _ := svc.ListDocuments(ctx, "org_default")
	if len(docsAfter) != len(docs)-1 {
		t.Errorf("expected doc count to decrease by 1")
	}
}

func TestLifecycle_GlobalSearchAndExport(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Global Search
	results, err := svc.GlobalSearch(ctx, "org_default", "Sensor")
	if err != nil {
		t.Fatalf("failed to run global search: %v", err)
	}
	if len(results.SKUs) == 0 {
		t.Errorf("expected to find SKUs matching 'Sensor'")
	}

	// 2. CSV Export
	skuCSV, err := svc.ExportCSV(ctx, "org_default", "skus")
	if err != nil {
		t.Fatalf("failed to export SKUs CSV: %v", err)
	}
	if !strings.Contains(string(skuCSV), "Code,Name,Category") {
		t.Errorf("expected SKUs CSV header, got: %s", string(skuCSV))
	}

	invCSV, err := svc.ExportCSV(ctx, "org_default", "invoices")
	if err != nil {
		t.Fatalf("failed to export Invoices CSV: %v", err)
	}
	if !strings.Contains(string(invCSV), "Invoice Number,Status") {
		t.Errorf("expected Invoices CSV header, got: %s", string(invCSV))
	}
}

func TestLifecycle_NotificationsAndCompanyProfile(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Notifications
	err := svc.CreateNotification(ctx, "org_default", "Security Alert", "New login from Tokyo", "alert")
	if err != nil {
		t.Fatalf("failed to create notification: %v", err)
	}
	notifs, err := svc.ListNotifications(ctx, "org_default", true)
	if err != nil {
		t.Fatalf("failed to list notifications: %v", err)
	}
	if len(notifs) == 0 {
		t.Fatal("expected at least 1 unread notification")
	}
	notifID := notifs[0].ID

	// Mark read
	if err := svc.MarkNotificationRead(ctx, "org_default", notifID); err != nil {
		t.Fatalf("failed to mark notification read: %v", err)
	}
	unreadAfter, _ := svc.ListNotifications(ctx, "org_default", true)
	for _, n := range unreadAfter {
		if n.ID == notifID {
			t.Errorf("expected notification %s to be marked read", notifID)
		}
	}

	// 2. Company Profile
	profile, err := svc.GetCompanyProfile(ctx, "org_default")
	if err != nil {
		t.Fatalf("failed to get company profile: %v", err)
	}
	if profile.CompanyName == "" {
		t.Errorf("expected non-empty company name")
	}

	// Update Profile
	profile.CompanyName = "AO Sphere Apex Holdings"
	profile.Currency = "EUR"
	profile.TaxRate = 0.15
	if err := svc.UpdateCompanyProfile(ctx, *profile); err != nil {
		t.Fatalf("failed to update company profile: %v", err)
	}

	updated, err := svc.GetCompanyProfile(ctx, "org_default")
	if err != nil {
		t.Fatalf("failed to get updated company profile: %v", err)
	}
	if updated.CompanyName != "AO Sphere Apex Holdings" || updated.Currency != "EUR" {
		t.Errorf("profile update not reflected: %+v", updated)
	}
}
