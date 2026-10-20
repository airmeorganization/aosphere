package store

import (
	"path/filepath"
	"testing"

	"aosphere/internal/db"
)

func setupTestStore(t *testing.T) (*Store, func()) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_store.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	s := NewStore(database)
	cleanup := func() {
		database.Close()
	}
	return s, cleanup
}

func TestStore_GetSKUs(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	skus, err := s.GetSKUs(false)
	if err != nil {
		t.Fatalf("GetSKUs failed: %v", err)
	}

	if len(skus) == 0 {
		t.Errorf("expected seeded skus, got 0")
	}

	belowThreshold, err := s.GetSKUs(true)
	if err != nil {
		t.Fatalf("GetSKUs(below_threshold) failed: %v", err)
	}

	for _, item := range belowThreshold {
		if item.StockQuantity >= item.MinThreshold {
			t.Errorf("sku %s stock %d should be below threshold %d", item.Code, item.StockQuantity, item.MinThreshold)
		}
	}
}

func TestStore_UpdateStock(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	skus, err := s.GetSKUs(false)
	if err != nil || len(skus) == 0 {
		t.Fatalf("failed to retrieve skus: %v", err)
	}

	target := skus[0]
	initialStock := target.StockQuantity

	updated, err := s.UpdateStock(target.ID, 5, "Received inventory shipment", "human", "admin")
	if err != nil {
		t.Fatalf("UpdateStock failed: %v", err)
	}

	if updated.StockQuantity != initialStock+5 {
		t.Errorf("expected stock %d, got %d", initialStock+5, updated.StockQuantity)
	}

	// Verify audit log entry was created
	logs, err := s.GetAuditLogs(10)
	if err != nil {
		t.Fatalf("GetAuditLogs failed: %v", err)
	}

	found := false
	for _, l := range logs {
		if l.EntityType == "skus" && l.EntityID == target.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected audit log entry for sku %s update", target.ID)
	}
}

func TestStore_AuditLog(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_audit.db")

	database, err := db.InitDB(dbPath, false) // unseeded database
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	s := NewStore(database)

	err = s.LogAudit("agent", "agent_1", "test_action", "order", "ord_123", "Automated test reasoning", `{"meta":"data"}`)
	if err != nil {
		t.Fatalf("LogAudit failed: %v", err)
	}

	logs, err := s.GetAuditLogs(5)
	if err != nil {
		t.Fatalf("GetAuditLogs failed: %v", err)
	}

	if len(logs) == 0 {
		t.Fatalf("expected audit logs, got 0")
	}

	latest := logs[0]
	if latest.ActorType != "agent" || latest.Action != "test_action" {
		t.Errorf("unexpected audit log: %+v", latest)
	}
}
