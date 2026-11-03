package store

import (
	"context"
	"testing"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/db"
)

func TestComplianceStore_MultiTenancy(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	orgs, err := s.ListOrganizations()
	if err != nil {
		t.Fatalf("ListOrganizations failed: %v", err)
	}
	if len(orgs) < 2 {
		t.Fatalf("Expected at least 2 seeded organizations (default + emea), got %d", len(orgs))
	}

	user, err := s.GetUserByEmail("admin@aosphere.local")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}

	userOrgs, err := s.GetUserOrganizations(user.ID)
	if err != nil {
		t.Fatalf("GetUserOrganizations failed: %v", err)
	}
	if len(userOrgs) == 0 {
		t.Fatalf("Expected user organizations, got 0")
	}

	// Create session and test org switching
	tokenHash := auth.HashToken("test_session_token_123")
	sess, err := s.CreateSession(user.ID, "org_default", tokenHash, 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Switch session to org_emea
	if err := s.SwitchSessionOrg(sess.ID, "org_emea", user.ID); err != nil {
		t.Fatalf("SwitchSessionOrg to org_emea failed: %v", err)
	}

	lookedUpSess, _, err := s.GetSessionAndUser(tokenHash)
	if err != nil {
		t.Fatalf("GetSessionAndUser failed: %v", err)
	}
	if lookedUpSess.OrgID != "org_emea" {
		t.Fatalf("Expected session org_id to be org_emea, got: %s", lookedUpSess.OrgID)
	}
}

func TestComplianceStore_TOTPAndRecoveryCodes(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	user, err := s.GetUserByEmail("admin@aosphere.local")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}

	secret, _ := auth.GenerateTOTPSecret()
	codes, _ := auth.GenerateRecoveryCodes(8)
	codesJSON := `["` + codes[0] + `","` + codes[1] + `"]`

	if err := s.SetUserTOTP(user.ID, secret, codesJSON, true); err != nil {
		t.Fatalf("SetUserTOTP failed: %v", err)
	}

	u, err := s.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if !u.TOTPEnabled || u.TOTPSecret != secret {
		t.Fatalf("TOTP fields not updated correctly: %+v", u)
	}

	// Verify and consume recovery code
	consumed, err := s.VerifyAndConsumeRecoveryCode(user.ID, codes[0])
	if err != nil || !consumed {
		t.Fatalf("VerifyAndConsumeRecoveryCode failed for valid code: %v", err)
	}

	// Consuming same code twice should fail
	consumedAgain, _ := s.VerifyAndConsumeRecoveryCode(user.ID, codes[0])
	if consumedAgain {
		t.Fatalf("Consumed code should not be valid twice")
	}

	// Disable TOTP
	if err := s.DisableUserTOTP(user.ID); err != nil {
		t.Fatalf("DisableUserTOTP failed: %v", err)
	}
	uAfter, _ := s.GetUserByID(user.ID)
	if uAfter.TOTPEnabled {
		t.Fatalf("TOTP should be disabled")
	}
}

func TestComplianceStore_InvitationsAndReset(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	// 1. Invitations
	rawToken, tokenHash, _ := auth.GenerateSecureToken(32)
	inv, err := s.CreateUserInvitation(db.UserInvitation{
		OrgID:     "org_default",
		Email:     "new_hire@example.com",
		Role:      "finance",
		TokenHash: tokenHash,
		InvitedBy: "usr_admin",
	})
	if err != nil {
		t.Fatalf("CreateUserInvitation failed: %v", err)
	}

	retrievedInv, err := s.GetInvitationByHash(tokenHash)
	if err != nil || retrievedInv.ID != inv.ID {
		t.Fatalf("GetInvitationByHash failed: %v", err)
	}

	pwdHash, _ := auth.HashPassword("ComplexPass123!")
	newUser, err := s.AcceptUserInvitation(tokenHash, "New Hire", pwdHash)
	if err != nil {
		t.Fatalf("AcceptUserInvitation failed: %v", err)
	}
	if newUser.Email != "new_hire@example.com" || newUser.Role != "finance" {
		t.Fatalf("Unexpected user created from invite: %+v", newUser)
	}

	// 2. Password Reset
	_, resetHash, _ := auth.GenerateSecureToken(32)
	_, err = s.CreatePasswordReset(newUser.ID, resetHash, time.Now().Add(15*time.Minute))
	if err != nil {
		t.Fatalf("CreatePasswordReset failed: %v", err)
	}

	newPwdHash, _ := auth.HashPassword("EvenMoreComplexPass999#")
	if err := s.CompletePasswordReset(resetHash, newPwdHash); err != nil {
		t.Fatalf("CompletePasswordReset failed: %v", err)
	}
	_ = rawToken
}

func TestComplianceStore_AuditVaultChain(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	for i := 1; i <= 3; i++ {
		_, err := s.RecordAuditVaultBlock(db.AuditChainBlock{
			OrgID:        "org_default",
			ActorType:    "user",
			ActorID:      "usr_admin",
			Action:       "CREATE_INVOICE",
			ResourceType: "invoices",
			ResourceID:   "inv_001",
			PayloadHash:  auth.ComputePayloadHash(`{"amount": 5000}`),
		})
		if err != nil {
			t.Fatalf("RecordAuditVaultBlock %d failed: %v", i, err)
		}
	}

	valid, count, err := s.VerifyAuditVaultChain("org_default")
	if err != nil || !valid || count != 3 {
		t.Fatalf("VerifyAuditVaultChain failed: %v, count=%d", err, count)
	}
}

func TestComplianceStore_GDPRAndSOC2(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := s.GetUserByEmail("admin@aosphere.local")

	// GDPR Export
	export, err := s.GenerateGDPRUserExport(context.Background(), "org_default", user.ID)
	if err != nil {
		t.Fatalf("GenerateGDPRUserExport failed: %v", err)
	}
	if export["profile"] == nil {
		t.Fatalf("Missing profile in GDPR export")
	}

	// SOC2 Access Review
	review, err := s.GenerateSOC2AccessReview("org_default", "2026-Q1", "usr_admin")
	if err != nil {
		t.Fatalf("GenerateSOC2AccessReview failed: %v", err)
	}
	if review.Status != "pending" {
		t.Fatalf("Expected review status pending, got: %s", review.Status)
	}

	signed, err := s.SignOffSOC2AccessReview(review.ID, "usr_admin", "All active user accounts verified conforming to least privilege policy.")
	if err != nil || signed.Status != "approved" {
		t.Fatalf("SignOffSOC2AccessReview failed: %v", err)
	}
}

