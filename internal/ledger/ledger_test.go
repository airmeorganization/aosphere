package ledger

import (
	"context"
	"path/filepath"
	"testing"

	"aosphere/internal/db"
	"aosphere/internal/finance"
)

func setupTestLedger(t *testing.T) (*LedgerService, func()) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_ledger.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Ensure ledger tables exist
	_, err = database.Exec(`
	CREATE TABLE IF NOT EXISTS sequences (
		org_id TEXT NOT NULL,
		sequence_type TEXT NOT NULL,
		year INTEGER NOT NULL,
		current_value INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (org_id, sequence_type, year)
	);

	CREATE TABLE IF NOT EXISTS accounts (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		normal_balance TEXT NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(org_id, code)
	);

	CREATE TABLE IF NOT EXISTS journal_entries (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		entry_number TEXT NOT NULL,
		source_type TEXT NOT NULL,
		source_id TEXT,
		memo TEXT,
		status TEXT NOT NULL DEFAULT 'posted',
		posted_at DATETIME NOT NULL,
		created_by TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(org_id, entry_number)
	);

	CREATE TABLE IF NOT EXISTS journal_lines (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		entry_id TEXT NOT NULL REFERENCES journal_entries(id),
		account_id TEXT NOT NULL REFERENCES accounts(id),
		entry_type TEXT NOT NULL, -- debit, credit
		amount_cents INTEGER NOT NULL,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Seed default chart of accounts
	INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance) VALUES
		('acc_1000', 'org_default', '1000', 'Operating Cash / Bank', 'asset', 'debit'),
		('acc_1100', 'org_default', '1100', 'Accounts Receivable', 'asset', 'debit'),
		('acc_1200', 'org_default', '1200', 'Inventory Asset', 'asset', 'debit'),
		('acc_2000', 'org_default', '2000', 'Accounts Payable', 'liability', 'credit'),
		('acc_2100', 'org_default', '2100', 'Sales Tax Payable', 'liability', 'credit'),
		('acc_3000', 'org_default', '3000', 'Retained Earnings', 'equity', 'credit'),
		('acc_4000', 'org_default', '4000', 'Sales Revenue', 'revenue', 'credit'),
		('acc_5000', 'org_default', '5000', 'Cost of Goods Sold', 'expense', 'debit'),
		('acc_5100', 'org_default', '5100', 'Operating Expenses', 'expense', 'debit');
	`)
	if err != nil {
		t.Fatalf("failed to create ledger tables: %v", err)
	}

	svc := NewLedgerService(database)
	cleanup := func() {
		database.Close()
	}
	return svc, cleanup
}

func TestLedger_UnbalancedEntryRejection(t *testing.T) {
	svc, cleanup := setupTestLedger(t)
	defer cleanup()

	ctx := context.Background()
	tx, err := svc.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	defer tx.Rollback()

	// Debits $100, Credits $90 -> Must be rejected!
	_, err = svc.PostJournalEntry(ctx, tx, JournalEntry{
		OrgID:      "org_default",
		SourceType: "manual",
		Memo:       "Unbalanced test entry",
		Lines: []JournalLine{
			{AccountCode: "1000", EntryType: "debit", AmountCents: 10000},
			{AccountCode: "4000", EntryType: "credit", AmountCents: 9000},
		},
	})
	if err == nil {
		t.Fatalf("expected ErrUnbalancedEntry, got nil")
	}
	t.Logf("Correctly rejected unbalanced entry: %v", err)
}

func TestLedger_InvoiceAndPaymentPosting(t *testing.T) {
	svc, cleanup := setupTestLedger(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Post Invoice Issued ($1,000 subtotal, $80 tax, $1,080 total)
	tx1, _ := svc.db.BeginTx(ctx, nil)
	invEntry, err := svc.PostInvoiceIssued(ctx, tx1, "org_default", "inv_001", "INV-2026-00001", 100000, 8000, 108000, "finance_user")
	if err != nil {
		t.Fatalf("PostInvoiceIssued failed: %v", err)
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("tx1 commit failed: %v", err)
	}
	if invEntry.EntryNumber == "" || invEntry.Status != "posted" {
		t.Errorf("unexpected invoice entry: %+v", invEntry)
	}

	// 2. Post Invoice Paid ($1,080 collected into cash)
	tx2, _ := svc.db.BeginTx(ctx, nil)
	payEntry, err := svc.PostInvoicePaid(ctx, tx2, "org_default", "inv_001", "INV-2026-00001", 108000, "finance_user")
	if err != nil {
		t.Fatalf("PostInvoicePaid failed: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("tx2 commit failed: %v", err)
	}
	if payEntry.EntryNumber == "" {
		t.Errorf("unexpected payment entry: %+v", payEntry)
	}

	// 3. Post PO Goods Receipt ($500 inventory received)
	tx3, _ := svc.db.BeginTx(ctx, nil)
	poEntry, err := svc.PostPOReceipt(ctx, tx3, "org_default", "po_001", "PO-2026-00001", 50000, "warehouse_user")
	if err != nil {
		t.Fatalf("PostPOReceipt failed: %v", err)
	}
	if err := tx3.Commit(); err != nil {
		t.Fatalf("tx3 commit failed: %v", err)
	}
	if poEntry.EntryNumber == "" {
		t.Errorf("unexpected PO receipt entry: %+v", poEntry)
	}

	// 4. Compute and Verify Trial Balance
	tb, err := svc.GetTrialBalance(ctx, "org_default")
	if err != nil {
		t.Fatalf("GetTrialBalance failed: %v", err)
	}

	if !tb.IsBalanced {
		t.Errorf("trial balance is not balanced! Total debits: %d, Total credits: %d", tb.TotalDebitsCents, tb.TotalCreditsCents)
	}

	// Total Debits should equal Total Credits ($1,080 invoice AR + $1,080 cash + $500 inventory = $2,660)
	if tb.TotalDebitsCents != tb.TotalCreditsCents {
		t.Errorf("debits (%d) != credits (%d)", tb.TotalDebitsCents, tb.TotalCreditsCents)
	}
	t.Logf("Trial balance verified! Total Debits: %s, Total Credits: %s (Balanced: %t)",
		finance.FormatUSD(tb.TotalDebitsCents), finance.FormatUSD(tb.TotalCreditsCents), tb.IsBalanced)
}
