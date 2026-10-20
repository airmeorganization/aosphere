package db

import (
	"path/filepath"
	"testing"
)

func TestInitDB_NoSeed(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "unseeded.db")

	database, err := InitDB(dbPath, false)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	if err := Ping(database); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM skus").Scan(&count); err != nil {
		t.Fatalf("Failed to query skus: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 skus in unseeded database, got %d", count)
	}
}

func TestInitDB_WithSeed(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "seeded.db")

	database, err := InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM skus").Scan(&count); err != nil {
		t.Fatalf("Failed to query skus: %v", err)
	}
	if count <= 0 {
		t.Errorf("Expected > 0 skus in seeded database, got %d", count)
	}
}
