package auth

import (
	"testing"
	"time"
)

func TestTOTPGenerationAndVerification(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret failed: %v", err)
	}
	if len(secret) < 16 {
		t.Fatalf("Secret too short: %s", secret)
	}

	uri := GenerateTOTPURI(secret, "user@example.com", "AOSphere")
	if len(uri) == 0 || uri[:15] != "otpauth://totp/" {
		t.Fatalf("Invalid TOTP URI: %s", uri)
	}

	now := time.Now()
	code, err := ComputeTOTPCode(secret, now)
	if err != nil {
		t.Fatalf("ComputeTOTPCode failed: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("Expected 6-digit code, got: %s", code)
	}

	if !VerifyTOTPCode(secret, code) {
		t.Fatalf("VerifyTOTPCode failed for valid code: %s", code)
	}

	if VerifyTOTPCode(secret, "000000") && code != "000000" {
		t.Fatalf("VerifyTOTPCode succeeded for invalid code")
	}

	recoveryCodes, err := GenerateRecoveryCodes(8)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes failed: %v", err)
	}
	if len(recoveryCodes) != 8 {
		t.Fatalf("Expected 8 recovery codes, got %d", len(recoveryCodes))
	}
	for _, rc := range recoveryCodes {
		if len(rc) != 9 || rc[4] != '-' {
			t.Fatalf("Invalid recovery code format: %s", rc)
		}
	}
}

func TestPasswordComplexity(t *testing.T) {
	tests := []struct {
		pwd   string
		valid bool
	}{
		{"short", false},
		{"alllowercaseno1!", false},
		{"ALLUPPERCASENO1!", false},
		{"NoNumberOrSpecial", false},
		{"NoSpecialChar123", false},
		{"ValidPass123!", true},
		{"SuperSecret#2026", true},
	}

	for _, tt := range tests {
		err := ValidatePasswordComplexity(tt.pwd)
		if tt.valid && err != nil {
			t.Errorf("expected %q to be valid, got: %v", tt.pwd, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("expected %q to be invalid, but passed", tt.pwd)
		}
	}
}

func TestAuditChainVerification(t *testing.T) {
	var blocks []ChainBlockVerificationItem
	now := time.Now().UTC()

	prevHash := GenesisBlockHash
	for i := int64(1); i <= 5; i++ {
		ts := now.Add(time.Duration(i) * time.Second)
		tsStr := ts.Format(time.RFC3339)
		pHash := ComputePayloadHash(`{"data":"sample"}`)
		bHash := ComputeAuditBlockHash(i, prevHash, "UPDATE_SKU", "usr_admin", pHash, tsStr)

		blocks = append(blocks, ChainBlockVerificationItem{
			SequenceNumber: i,
			PrevBlockHash:  prevHash,
			BlockHash:      bHash,
			Action:         "UPDATE_SKU",
			ActorID:        "usr_admin",
			PayloadHash:    pHash,
			Timestamp:      ts,
		})
		prevHash = bHash
	}

	valid, count, err := VerifyAuditChain(blocks)
	if !valid || err != nil || count != 5 {
		t.Fatalf("VerifyAuditChain failed on valid chain: %v", err)
	}

	// Tamper with block 3
	blocks[2].Action = "TAMPERED_ACTION"
	valid, _, err = VerifyAuditChain(blocks)
	if valid || err == nil {
		t.Fatalf("Expected VerifyAuditChain to detect tamper, but it passed")
	}
}

