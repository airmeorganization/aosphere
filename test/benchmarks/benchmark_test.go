package benchmarks

import (
	"context"
	"path/filepath"
	"testing"

	"aosphere/internal/db"
	"aosphere/internal/ledger"
	"aosphere/internal/service"
	"aosphere/internal/store"
)

func setupBenchmarkService(b *testing.B) (*service.WorkflowService, func()) {
	b.Helper()
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		b.Fatalf("Failed to init db: %v", err)
	}

	appStore := store.NewStore(database)
	svc := service.NewWorkflowService(appStore)

	cleanup := func() {
		database.Close()
	}
	return svc, cleanup
}

func BenchmarkStore_GetSKUs(b *testing.B) {
	svc, cleanup := setupBenchmarkService(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Store().GetSKUsForOrg("org_default", false)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkService_GlobalSearch(b *testing.B) {
	svc, cleanup := setupBenchmarkService(b)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.GlobalSearch(ctx, "org_default", "Sensor")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLedger_PostJournalEntry(b *testing.B) {
	svc, cleanup := setupBenchmarkService(b)
	defer cleanup()

	ctx := context.Background()
	lines := []ledger.JournalLine{
		{AccountCode: ledger.AccountCash, EntryType: "debit", AmountCents: 5000, Description: "Debit test"},
		{AccountCode: ledger.AccountSalesRevenue, EntryType: "credit", AmountCents: 5000, Description: "Credit test"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, err := svc.Store().DB().Begin()
		if err != nil {
			b.Fatal(err)
		}
		_, err = svc.Ledger().PostJournalEntry(ctx, tx, ledger.JournalEntry{
			OrgID:      "org_default",
			SourceType: "bench",
			SourceID:   "bench_01",
			Memo:       "Benchmark entry",
			CreatedBy:  "benchmarker",
			Lines:      lines,
		})
		if err != nil {
			tx.Rollback()
			b.Fatal(err)
		}
		_ = tx.Commit()
	}
}
