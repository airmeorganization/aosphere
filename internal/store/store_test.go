package store

import (
	"path/filepath"
	"testing"
	"time"

	"aosphere/internal/auth"
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
	org, err := s.CreateOrganization("Audit Test Corp", "audit-test")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	err = s.LogAuditForOrg(org.ID, "agent", "agent_1", "test_action", "order", "ord_123", "Automated test reasoning", `{"meta":"data"}`)
	if err != nil {
		t.Fatalf("LogAudit failed: %v", err)
	}

	logs, err := s.GetAuditLogsForOrg(org.ID, 5)
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

func TestStore_TenantIsolation(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	// 1. Create a second organization (Org B)
	orgB, err := s.CreateOrganization("Tenant Beta Corp", "tenant-beta")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	// 2. Query SKUs for Org B - should be 0 because seeded SKUs belong to org_default
	skusB, err := s.GetSKUsForOrg(orgB.ID, false)
	if err != nil {
		t.Fatalf("GetSKUsForOrg failed: %v", err)
	}
	if len(skusB) != 0 {
		t.Errorf("expected 0 SKUs for Org B, got %d", len(skusB))
	}

	// 3. Attempting to update an Org A SKU from Org B must fail with not found error
	skusA, err := s.GetSKUsForOrg("org_default", false)
	if err != nil || len(skusA) == 0 {
		t.Fatalf("expected seeded SKUs in org_default")
	}

	targetSKUA := skusA[0]
	_, err = s.UpdateStockForOrg(orgB.ID, targetSKUA.ID, 10, "Cross-tenant unauthorized edit", "human", "hacker")
	if err == nil {
		t.Errorf("SECURITY VIOLATION: Org B successfully updated Org A SKU!")
	}
}

func TestStore_AuthAndSessions(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	// 1. Test Admin User retrieval (seeded in org_default)
	user, err := s.GetUserByEmail("admin@aosphere.local")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if user.Role != "owner" {
		t.Errorf("expected role 'owner', got %s", user.Role)
	}
	if !auth.VerifyPassword(user.PasswordHash, "Admin1234!") {
		t.Error("VerifyPassword failed for seeded admin password")
	}

	// 2. Test Session Creation and Validation
	rawToken, _ := auth.GenerateRandomToken("sess")
	tokenHash := auth.HashToken(rawToken)

	sess, err := s.CreateSession(user.ID, user.OrgID, tokenHash, 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if sess.UserID != user.ID {
		t.Errorf("session user mismatch: expected %s, got %s", user.ID, sess.UserID)
	}

	// 3. Test Session & User Lookup
	lookedUpSess, lookedUpUser, err := s.GetSessionAndUser(tokenHash)
	if err != nil {
		t.Fatalf("GetSessionAndUser failed: %v", err)
	}
	if lookedUpSess.ID != sess.ID || lookedUpUser.Email != user.Email {
		t.Errorf("session lookup mismatch: %+v / %+v", lookedUpSess, lookedUpUser)
	}

	// 4. Test Session Deletion (Logout)
	if err := s.DeleteSession(tokenHash); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	_, _, err = s.GetSessionAndUser(tokenHash)
	if err == nil {
		t.Errorf("expected error looking up deleted session, got nil")
	}
}

func TestStore_APIKeys(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	// Test default agent key seeded in schema
	keyHash := auth.HashToken("ao_sk_agent_default_token_12345")
	key, err := s.GetAPIKeyByHash(keyHash)
	if err != nil {
		t.Fatalf("GetAPIKeyByHash failed: %v", err)
	}
	if key.Role != "operations" {
		t.Errorf("expected role 'operations', got %s", key.Role)
	}

	// Test create custom API key
	newToken, _ := auth.GenerateRandomToken("ao_sk")
	newHash := auth.HashToken(newToken)

	createdKey, err := s.CreateAPIKey("org_default", key.UserID, "Custom Finance Agent", newHash, "finance", "adjust_stock,create_po")
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}
	if createdKey.Role != "finance" {
		t.Errorf("expected role finance, got %s", createdKey.Role)
	}

	// Verify list api keys
	keys, err := s.ListAPIKeys("org_default")
	if err != nil || len(keys) < 2 {
		t.Fatalf("ListAPIKeys failed or count < 2: %v (count: %d)", err, len(keys))
	}
}
