package reports

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"aosphere/internal/db"
	"aosphere/internal/ledger"
)

func setupTestReportService(t *testing.T) (*ReportService, *ledger.LedgerService, func()) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_reports.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	ledgerSvc := ledger.NewLedgerService(database)
	reportSvc := NewReportService(database, ledgerSvc)

	cleanup := func() {
		database.Close()
	}
	return reportSvc, ledgerSvc, cleanup
}

func TestReportService_FinancialStatements(t *testing.T) {
	svc, ledgerSvc, cleanup := setupTestReportService(t)
	defer cleanup()
	ctx := context.Background()

	// Post an invoice payment entry
	// Debit Cash 1000 ($5,000), Credit Sales Revenue 4000 ($5,000)
	tx, err := svc.db.Begin()
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	_, err = ledgerSvc.PostJournalEntry(ctx, tx, ledger.JournalEntry{
		OrgID:      "org_default",
		SourceType: "invoice",
		SourceID:   "inv_test_001",
		Memo:       "Invoice Test Revenue",
		CreatedBy:  "test_user",
		Lines: []ledger.JournalLine{
			{AccountCode: "1000", EntryType: "debit", AmountCents: 500000, Description: "Operating cash"},
			{AccountCode: "4000", EntryType: "credit", AmountCents: 500000, Description: "Sales revenue"},
		},
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("failed to post revenue entry: %v", err)
	}

	// Post an expense entry: Debit OpEx 5100 ($1,200), Credit Cash 1000 ($1,200)
	_, err = ledgerSvc.PostJournalEntry(ctx, tx, ledger.JournalEntry{
		OrgID:      "org_default",
		SourceType: "expense",
		SourceID:   "exp_test_001",
		Memo:       "Office Software",
		CreatedBy:  "test_user",
		Lines: []ledger.JournalLine{
			{AccountCode: "5100", EntryType: "debit", AmountCents: 120000, Description: "SaaS Expense"},
			{AccountCode: "1000", EntryType: "credit", AmountCents: 120000, Description: "Cash payment"},
		},
	})
	if err != nil {
		tx.Rollback()
		t.Fatalf("failed to post expense entry: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// 1. Test P&L
	pnl, err := svc.GetProfitAndLoss(ctx, "org_default", "", "")
	if err != nil {
		t.Fatalf("GetProfitAndLoss failed: %v", err)
	}
	if pnl.RevenueCents != 500000 {
		t.Errorf("expected revenue cents 500000, got %d", pnl.RevenueCents)
	}
	if pnl.OpExCents != 120000 {
		t.Errorf("expected opex cents 120000, got %d", pnl.OpExCents)
	}
	if pnl.NetProfitCents != 380000 {
		t.Errorf("expected net profit cents 380000, got %d", pnl.NetProfitCents)
	}

	// 2. Test Balance Sheet
	bs, err := svc.GetBalanceSheet(ctx, "org_default", "")
	if err != nil {
		t.Fatalf("GetBalanceSheet failed: %v", err)
	}
	if !bs.IsBalanced {
		t.Errorf("expected balance sheet to be balanced: TotalAssets=%.2f, Liabilities=%.2f, Equity=%.2f",
			bs.TotalAssets, bs.TotalLiabilities, bs.TotalEquity)
	}
	if bs.TotalAssetsCents != 380000 { // Net Cash $3,800
		t.Errorf("expected total assets cents 380000, got %d", bs.TotalAssetsCents)
	}

	// 3. Test Cash Flow
	cf, err := svc.GetCashFlowStatement(ctx, "org_default", "", "")
	if err != nil {
		t.Fatalf("GetCashFlowStatement failed: %v", err)
	}
	if cf.NetCashChangeCents != 380000 {
		t.Errorf("expected net cash change 380000, got %d", cf.NetCashChangeCents)
	}
}

func TestReportService_AgingAndAnalytics(t *testing.T) {
	svc, _, cleanup := setupTestReportService(t)
	defer cleanup()
	ctx := context.Background()

	// Seed customer and supplier
	_, err := svc.db.ExecContext(ctx, `INSERT OR REPLACE INTO customers (id, org_id, name, email, status)
		VALUES ('cust_default', 'org_default', 'Acme Customer', 'acme@test.local', 'active')`)
	if err != nil {
		t.Fatalf("failed to insert customer: %v", err)
	}

	_, err = svc.db.ExecContext(ctx, `INSERT OR REPLACE INTO suppliers (id, org_id, name, contact_email)
		VALUES ('sup_default', 'org_default', 'Global Supplier', 'supplier@test.local')`)
	if err != nil {
		t.Fatalf("failed to insert supplier: %v", err)
	}

	// Seed open invoice (> 90 days overdue relative to 2026-11-02)
	_, err = svc.db.ExecContext(ctx, `INSERT INTO invoices (id, org_id, invoice_number, customer_id, status, issue_date, due_date, subtotal, tax_amount, total_amount)
		VALUES ('inv_overdue', 'org_default', 'INV-2026-999', 'cust_default', 'sent', '2026-06-01', '2026-06-15', 1000.0, 100.0, 1100.0)`)
	if err != nil {
		t.Fatalf("failed to insert invoice: %v", err)
	}

	// Seed SKU
	_, err = svc.db.ExecContext(ctx, `INSERT OR REPLACE INTO skus (id, org_id, code, name, category, stock_quantity, reserved_quantity, min_threshold, reorder_quantity, unit_cost, selling_price, supplier_id)
		VALUES ('sku_val_test', 'org_default', 'SKU-VAL-1', 'Valuation Widget', 'Electronics', 50, 0, 10, 20, 20.0, 50.0, 'sup_default')`)
	if err != nil {
		t.Fatalf("failed to insert sku: %v", err)
	}

	// 1. Test Aged Receivables
	agedAR, err := svc.GetAgedReceivables(ctx, "org_default", "2026-11-02")
	if err != nil {
		t.Fatalf("GetAgedReceivables failed: %v", err)
	}
	if len(agedAR.Items) == 0 {
		t.Fatalf("expected at least 1 aged invoice item")
	}
	if agedAR.Days90PlusCents != 110000 {
		t.Errorf("expected 90_plus bucket to have 110000 cents, got %d", agedAR.Days90PlusCents)
	}

	// 2. Test Inventory Valuation
	invVal, err := svc.GetInventoryValuation(ctx, "org_default")
	if err != nil {
		t.Fatalf("GetInventoryValuation failed: %v", err)
	}
	if invVal.TotalCostValuation <= 0 {
		t.Errorf("expected positive inventory cost valuation, got %.2f", invVal.TotalCostValuation)
	}

	// 3. Test Executive KPIs
	kpis, err := svc.GetExecutiveKPIs(ctx, "org_default")
	if err != nil {
		t.Fatalf("GetExecutiveKPIs failed: %v", err)
	}
	if kpis.GeneratedAt == "" {
		t.Errorf("expected generated_at timestamp")
	}

	// 4. Test Universal Export (CSV and PDF)
	csvBytes, contentType, filename, err := svc.ExportData(ctx, "org_default", "pnl", "csv", "", "")
	if err != nil {
		t.Fatalf("ExportData CSV failed: %v", err)
	}
	_ = filename
	if !strings.Contains(contentType, "csv") || !strings.Contains(string(csvBytes), "Category") {
		t.Errorf("unexpected csv output: %s", string(csvBytes))
	}

	pdfBytes, pdfType, pdfFile, err := svc.ExportData(ctx, "org_default", "inventory", "pdf", "", "")
	if err != nil {
		t.Fatalf("ExportData PDF failed: %v", err)
	}
	if pdfType != "application/pdf" || !strings.HasPrefix(string(pdfBytes), "%PDF") {
		t.Errorf("unexpected pdf output header for %s", pdfFile)
	}
}
