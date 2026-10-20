package db

import (
	"path/filepath"
	"testing"
)

func TestMigrations_RunAndIdempotency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "migration_test.db")

	database, err := InitDB(dbPath, false)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	// 1. Verify schema_migrations table has all migrations recorded
	var migrationCount int
	err = database.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount)
	if err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}
	if migrationCount != len(Migrations) {
		t.Errorf("expected %d migrations recorded, got %d", len(Migrations), migrationCount)
	}

	// 2. Verify all expected migration versions exist
	for _, m := range Migrations {
		var exists bool
		err := database.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", m.Version).Scan(&exists)
		if err != nil || !exists {
			t.Errorf("expected migration version %d (%s) to be marked as applied", m.Version, m.Name)
		}
	}

	// 3. Re-run RunMigrations — MUST BE IDEMPOTENT (no-op, no errors)
	if err := RunMigrations(database); err != nil {
		t.Fatalf("expected idempotent re-run of RunMigrations to succeed, got: %v", err)
	}

	// 4. Verify count hasn't changed
	var countAfter int
	_ = database.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&countAfter)
	if countAfter != migrationCount {
		t.Errorf("expected migration count to remain %d, got %d", migrationCount, countAfter)
	}
}
