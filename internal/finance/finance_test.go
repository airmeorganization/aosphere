package finance

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"aosphere/internal/db"
)

func TestMoney_ConversionsAndFormatting(t *testing.T) {
	cases := []struct {
		dollars  float64
		expected int64
		format   string
	}{
		{0.00, 0, "$0.00"},
		{10.50, 1050, "$10.50"},
		{0.10, 10, "$0.10"},
		{0.20, 20, "$0.20"},
		{1234.56, 123456, "$1234.56"},
		{99.999, 10000, "$100.00"}, // rounds up
	}

	for _, tc := range cases {
		cents := DollarsToCents(tc.dollars)
		if cents != tc.expected {
			t.Errorf("DollarsToCents(%.3f) = %d, expected %d", tc.dollars, cents, tc.expected)
		}
		dollarsBack := CentsToDollars(cents)
		expectedBack := float64(tc.expected) / 100.0
		if dollarsBack != expectedBack {
			t.Errorf("CentsToDollars(%d) = %.2f, expected %.2f", cents, dollarsBack, expectedBack)
		}
		fmtStr := FormatUSD(cents)
		if fmtStr != tc.format {
			t.Errorf("FormatUSD(%d) = %q, expected %q", cents, fmtStr, tc.format)
		}
	}
}

func TestMoney_CalculateTax(t *testing.T) {
	// $100.00 subtotal (10000 cents) at 8% (800 basis points) -> $8.00 (800 cents)
	tax := CalculateTax(10000, 800)
	if tax != 800 {
		t.Errorf("expected 800 cents tax on $100, got %d", tax)
	}

	// $15.55 subtotal (1555 cents) at 8% (800 basis points)
	// 1555 * 800 = 1,244,000 -> (1,244,000 + 5000) / 10000 = 124 cents ($1.24)
	tax2 := CalculateTax(1555, 800)
	if tax2 != 124 {
		t.Errorf("expected 124 cents tax on $15.55, got %d", tax2)
	}

	// 0 subtotal
	if CalculateTax(0, 800) != 0 {
		t.Errorf("expected 0 tax on 0 subtotal")
	}
}

func TestSequence_CollisionFreeUnderConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_seq.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer database.Close()

	// Ensure sequences table exists
	_, err = database.Exec(`CREATE TABLE IF NOT EXISTS sequences (
		org_id TEXT NOT NULL,
		sequence_type TEXT NOT NULL,
		year INTEGER NOT NULL,
		current_value INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (org_id, sequence_type, year)
	)`)
	if err != nil {
		t.Fatalf("failed to create sequences table: %v", err)
	}

	const concurrency = 20
	var wg sync.WaitGroup
	generated := make(chan string, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, err := database.BeginTx(context.Background(), nil)
			if err != nil {
				t.Errorf("failed to begin tx: %v", err)
				return
			}
			defer tx.Rollback()

			seq, err := GenerateSequenceNumber(context.Background(), tx, "org_default", "INV")
			if err != nil {
				t.Errorf("GenerateSequenceNumber failed: %v", err)
				return
			}
			if err := tx.Commit(); err != nil {
				t.Errorf("commit failed: %v", err)
				return
			}
			generated <- seq
		}()
	}

	wg.Wait()
	close(generated)

	seen := make(map[string]bool)
	count := 0
	for seq := range generated {
		if seen[seq] {
			t.Errorf("collision detected for sequence %s", seq)
		}
		seen[seq] = true
		count++
	}

	if count != concurrency {
		t.Errorf("expected %d sequences generated, got %d", concurrency, count)
	}
}
