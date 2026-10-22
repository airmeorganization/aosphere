package auth

import (
	"testing"
)

func TestPasswordHashing(t *testing.T) {
	password := "superSecret123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !VerifyPassword(hash, password) {
		t.Error("VerifyPassword returned false for correct password")
	}

	if VerifyPassword(hash, "wrongPassword") {
		t.Error("VerifyPassword returned true for incorrect password")
	}

	// Test short password rejection
	_, err = HashPassword("short")
	if err == nil {
		t.Error("expected error for password < 8 characters, got nil")
	}
}

func TestTokenGeneration(t *testing.T) {
	token1, err := GenerateRandomToken("sess")
	if err != nil {
		t.Fatalf("GenerateRandomToken failed: %v", err)
	}

	token2, err := GenerateRandomToken("sess")
	if err != nil {
		t.Fatalf("GenerateRandomToken failed: %v", err)
	}

	if token1 == token2 {
		t.Error("expected distinct random tokens, got identical tokens")
	}

	hash1 := HashToken(token1)
	hash2 := HashToken(token2)

	if hash1 == hash2 {
		t.Error("expected distinct token hashes")
	}
	if len(hash1) != 64 { // SHA-256 hex length
		t.Errorf("expected 64 character hex hash, got %d", len(hash1))
	}
}

func TestRBACMatrix(t *testing.T) {
	// Owner can do everything
	ownerCtx := &AuthContext{Role: RoleOwner}
	if !ownerCtx.Can(PermRead) || !ownerCtx.Can(PermManageFinance) || !ownerCtx.Can(PermAdmin) {
		t.Error("Owner should have all permissions")
	}

	// Operations can manage inventory and orders, but not finance
	opsCtx := &AuthContext{Role: RoleOperations}
	if !opsCtx.Can(PermManageInventory) {
		t.Error("Operations must be able to manage inventory")
	}
	if opsCtx.Can(PermManageFinance) {
		t.Error("Operations should not be able to manage finance")
	}
	if opsCtx.Can(PermAdmin) {
		t.Error("Operations should not have admin permission")
	}

	// ReadOnly can only read
	roCtx := &AuthContext{Role: RoleReadOnly}
	if !roCtx.Can(PermRead) {
		t.Error("ReadOnly must be able to read")
	}
	if roCtx.Can(PermManageInventory) || roCtx.Can(PermManageOrders) {
		t.Error("ReadOnly must not have write permissions")
	}

	// Nil context check
	var nilCtx *AuthContext
	if nilCtx.Can(PermRead) {
		t.Error("nil AuthContext must return false for all permissions")
	}
}

func TestParseRole(t *testing.T) {
	role, err := ParseRole("admin")
	if err != nil || role != RoleAdmin {
		t.Errorf("expected RoleAdmin, got %s (err: %v)", role, err)
	}

	role, err = ParseRole("FINANCE")
	if err != nil || role != RoleFinance {
		t.Errorf("expected RoleFinance, got %s (err: %v)", role, err)
	}

	_, err = ParseRole("superuser")
	if err == nil {
		t.Error("expected error for invalid role 'superuser', got nil")
	}
}

func TestCORSAllowlist(t *testing.T) {
	defer ResetAllowedOrigins()

	// Default origins include app.aosphere.io and localhost
	if !IsOriginAllowed("https://app.aosphere.io") {
		t.Error("expected https://app.aosphere.io to be allowed by default")
	}
	if !IsOriginAllowed("http://localhost:8080") {
		t.Error("expected http://localhost:8080 to be allowed by default")
	}
	if IsOriginAllowed("https://attacker.evil.com") {
		t.Error("expected untrusted origin to be denied")
	}
	if IsOriginAllowed("") {
		t.Error("expected empty origin to return false")
	}

	// Custom override
	SetAllowedOrigins([]string{"https://custom.portal.com"})
	if !IsOriginAllowed("https://custom.portal.com") {
		t.Error("expected custom origin to be allowed")
	}
	if IsOriginAllowed("https://app.aosphere.io") {
		t.Error("expected previous default origin to be disallowed after custom override")
	}
}
