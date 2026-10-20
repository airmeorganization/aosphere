package concurrency

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"aosphere/internal/auth"
	"aosphere/internal/db"
	"aosphere/internal/ledger"
	"aosphere/internal/service"
	"aosphere/internal/store"
)

func setupTestService(t *testing.T) (*service.WorkflowService, func()) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_race.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	appStore := store.NewStore(database)
	svc := service.NewWorkflowService(appStore)

	cleanup := func() {
		database.Close()
	}
	return svc, cleanup
}

// TestConcurrency_StockAdjustments verifies optimistic locking and serialized consistency
func TestConcurrency_StockAdjustments(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	authCtx := &auth.AuthContext{UserID: "usr_worker", OrgID: "org_default", Role: auth.RoleOperations}

	// Initial stock for sku_02 is 85
	var initialStock int
	_ = svc.Store().DB().QueryRow("SELECT stock_quantity FROM skus WHERE id = 'sku_02'").Scan(&initialStock)

	const workers = 15
	var successCount int32
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			idemp := fmt.Sprintf("idemp_race_stock_%d", workerID)
			_, err := svc.AdjustStock(ctx, "org_default", "sku_02", 1, "Concurrency test add", authCtx, 0, idemp)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}
	wg.Wait()

	// Verify all succeeded without race conditions
	if successCount != int32(workers) {
		t.Errorf("expected %d successful adjustments, got %d", workers, successCount)
	}

	var finalStock int
	_ = svc.Store().DB().QueryRow("SELECT stock_quantity FROM skus WHERE id = 'sku_02'").Scan(&finalStock)
	expectedFinal := initialStock + workers
	if finalStock != expectedFinal {
		t.Errorf("expected final stock %d, got %d", expectedFinal, finalStock)
	}
}

// TestConcurrency_SingleApprovalExecution verifies that concurrent approvals on the same request commit exactly once
func TestConcurrency_SingleApprovalExecution(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	adminCtx := &auth.AuthContext{UserID: "usr_admin", OrgID: "org_default", Role: auth.RoleAdmin}

	const workers = 8
	var approvedCount int32
	var conflictCount int32
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			idemp := fmt.Sprintf("idemp_race_appr_%d", workerID)
			_, err := svc.DecideApproval(ctx, "org_default", "appr_01", "approved", adminCtx, idemp)
			if err == nil {
				atomic.AddInt32(&approvedCount, 1)
			} else {
				atomic.AddInt32(&conflictCount, 1)
			}
		}(i)
	}
	wg.Wait()

	if approvedCount != 1 {
		t.Errorf("expected exactly 1 approval to succeed, got %d", approvedCount)
	}
	if conflictCount != int32(workers-1) {
		t.Errorf("expected %d conflict rejections, got %d", workers-1, conflictCount)
	}
}

// TestConcurrency_LedgerBalanceInvariant verifies debits == credits invariant under concurrent postings
func TestConcurrency_LedgerBalanceInvariant(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	const concurrentPostings = 10
	var wg sync.WaitGroup

	for i := 0; i < concurrentPostings; i++ {
		wg.Add(1)
		go func(entryNum int) {
			defer wg.Done()
			amountCents := int64((entryNum + 1) * 1000)
			tx, err := svc.Store().DB().Begin()
			if err != nil {
				return
			}
			defer tx.Rollback()

			lines := []ledger.JournalLine{
				{AccountCode: ledger.AccountCash, EntryType: "debit", AmountCents: amountCents, Description: "Cash Inflow"},
				{AccountCode: ledger.AccountSalesRevenue, EntryType: "credit", AmountCents: amountCents, Description: "Sales Revenue"},
			}
			_, err = svc.Ledger().PostJournalEntry(ctx, tx, ledger.JournalEntry{
				OrgID:      "org_default",
				SourceType: "concurrent_test",
				SourceID:   fmt.Sprintf("test_%d", entryNum),
				Memo:       "Concurrent ledger test entry",
				CreatedBy:  "tester",
				Lines:      lines,
			})
			if err == nil {
				_ = tx.Commit()
			}
		}(i)
	}
	wg.Wait()

	// Verify trial balance is completely balanced
	tb, err := svc.TrialBalance(ctx, "org_default")
	if err != nil {
		t.Fatalf("failed to retrieve trial balance: %v", err)
	}
	if !tb.IsBalanced {
		t.Fatalf("trial balance unbalanced after concurrent operations: debits=%d, credits=%d",
			tb.TotalDebitsCents, tb.TotalCreditsCents)
	}
}
