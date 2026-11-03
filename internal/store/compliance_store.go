package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/db"
	"github.com/google/uuid"
)

// ----------------- MULTI-TENANCY & ORG SWITCHING -----------------

func (s *Store) ListOrganizations() ([]db.Organization, error) {
	rows, err := s.db.Query(`SELECT id, name, slug, created_at FROM organizations ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Organization
	for rows.Next() {
		var o db.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, nil
}

func (s *Store) GetUserOrganizations(userID string) ([]db.UserOrganization, error) {
	rows, err := s.db.Query(`
		SELECT uo.user_id, uo.org_id, o.name, o.slug, uo.role, uo.created_at
		FROM user_organizations uo
		INNER JOIN organizations o ON uo.org_id = o.id
		WHERE uo.user_id = ?
		ORDER BY o.name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.UserOrganization
	for rows.Next() {
		var uo db.UserOrganization
		if err := rows.Scan(&uo.UserID, &uo.OrgID, &uo.OrgName, &uo.OrgSlug, &uo.Role, &uo.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, uo)
	}
	return list, nil
}

func (s *Store) AddUserOrganization(userID, orgID, role string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_organizations (user_id, org_id, role)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, org_id) DO UPDATE SET role = excluded.role`, userID, orgID, role)
	return err
}

func (s *Store) SwitchSessionOrg(sessionID, targetOrgID, userID string) error {
	// Verify user belongs to target org
	var exists bool
	err := s.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM user_organizations WHERE user_id = ? AND org_id = ?)`,
		userID, targetOrgID).Scan(&exists)
	if err != nil || !exists {
		return fmt.Errorf("user does not belong to target organization %s", targetOrgID)
	}

	_, err = s.db.Exec(`UPDATE sessions SET org_id = ?, last_activity_at = CURRENT_TIMESTAMP WHERE id = ?`, targetOrgID, sessionID)
	return err
}

// ----------------- 2FA / TOTP -----------------

func (s *Store) SetUserTOTP(userID, secret, recoveryCodesJSON string, enabled bool) error {
	_, err := s.db.Exec(`
		UPDATE users
		SET totp_secret = ?, recovery_codes_json = ?, totp_enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, secret, recoveryCodesJSON, enabled, userID)
	return err
}

func (s *Store) DisableUserTOTP(userID string) error {
	_, err := s.db.Exec(`
		UPDATE users
		SET totp_secret = '', recovery_codes_json = '[]', totp_enabled = 0, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, userID)
	return err
}

func (s *Store) VerifyAndConsumeRecoveryCode(userID, code string) (bool, error) {
	u, err := s.GetUserByID(userID)
	if err != nil {
		return false, err
	}

	var codes []string
	if err := json.Unmarshal([]byte(u.RecoveryCodesJSON), &codes); err != nil {
		return false, nil
	}

	code = strings.ToUpper(strings.TrimSpace(code))
	foundIndex := -1
	for i, c := range codes {
		if strings.ToUpper(strings.TrimSpace(c)) == code {
			foundIndex = i
			break
		}
	}

	if foundIndex < 0 {
		return false, nil
	}

	// Remove consumed recovery code
	codes = append(codes[:foundIndex], codes[foundIndex+1:]...)
	updatedJSON, _ := json.Marshal(codes)

	_, err = s.db.Exec(`UPDATE users SET recovery_codes_json = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, string(updatedJSON), userID)
	if err != nil {
		return false, err
	}
	return true, nil
}

// ----------------- USER INVITATIONS -----------------

func (s *Store) CreateUserInvitation(inv db.UserInvitation) (*db.UserInvitation, error) {
	if inv.ID == "" {
		inv.ID = "inv_" + uuid.New().String()[:12]
	}
	if inv.ExpiresAt.IsZero() {
		inv.ExpiresAt = time.Now().Add(72 * time.Hour)
	}
	inv.CreatedAt = time.Now().UTC()
	inv.Status = "pending"

	_, err := s.db.Exec(`
		INSERT INTO user_invitations (id, org_id, email, role, token_hash, invited_by, status, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.ID, inv.OrgID, inv.Email, inv.Role, inv.TokenHash, inv.InvitedBy, inv.Status, inv.ExpiresAt, inv.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Store) GetInvitationByHash(tokenHash string) (*db.UserInvitation, error) {
	var inv db.UserInvitation
	err := s.db.QueryRow(`
		SELECT id, org_id, email, role, token_hash, invited_by, status, expires_at, created_at
		FROM user_invitations WHERE token_hash = ? AND status = 'pending' AND expires_at > CURRENT_TIMESTAMP`, tokenHash).Scan(
		&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.TokenHash, &inv.InvitedBy, &inv.Status, &inv.ExpiresAt, &inv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Store) ListPendingInvitations(orgID string) ([]db.UserInvitation, error) {
	rows, err := s.db.Query(`
		SELECT id, org_id, email, role, token_hash, invited_by, status, expires_at, created_at
		FROM user_invitations WHERE org_id = ? AND status = 'pending' ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.UserInvitation
	for rows.Next() {
		var inv db.UserInvitation
		if err := rows.Scan(&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.TokenHash, &inv.InvitedBy, &inv.Status, &inv.ExpiresAt, &inv.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, inv)
	}
	return list, nil
}

func (s *Store) AcceptUserInvitation(tokenHash, name, passwordHash string) (*db.User, error) {
	inv, err := s.GetInvitationByHash(tokenHash)
	if err != nil {
		return nil, errors.New("invalid or expired invitation link")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	newUserID := "usr_" + uuid.New().String()[:8]
	now := time.Now().UTC()

	// 1. Create User
	_, err = tx.Exec(`
		INSERT INTO users (id, org_id, email, password_hash, name, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		newUserID, inv.OrgID, inv.Email, passwordHash, name, inv.Role, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create user from invitation: %w", err)
	}

	// 2. Associate User with Org
	_, err = tx.Exec(`
		INSERT INTO user_organizations (user_id, org_id, role, created_at)
		VALUES (?, ?, ?, ?)`, newUserID, inv.OrgID, inv.Role, now)
	if err != nil {
		return nil, err
	}

	// 3. Mark Invitation Accepted
	_, err = tx.Exec(`UPDATE user_invitations SET status = 'accepted' WHERE id = ?`, inv.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &db.User{
		ID:        newUserID,
		OrgID:     inv.OrgID,
		Email:     inv.Email,
		Name:      name,
		Role:      inv.Role,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ----------------- PASSWORD RESETS -----------------

func (s *Store) CreatePasswordReset(userID, tokenHash string, expiresAt time.Time) (*db.PasswordReset, error) {
	id := "reset_" + uuid.New().String()[:12]
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		INSERT INTO password_resets (id, user_id, token_hash, status, expires_at, created_at)
		VALUES (?, ?, ?, 'pending', ?, ?)`, id, userID, tokenHash, expiresAt, now)
	if err != nil {
		return nil, err
	}
	return &db.PasswordReset{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		Status:    "pending",
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

func (s *Store) GetPasswordResetByHash(tokenHash string) (*db.PasswordReset, error) {
	var pr db.PasswordReset
	err := s.db.QueryRow(`
		SELECT id, user_id, token_hash, status, expires_at, created_at
		FROM password_resets WHERE token_hash = ? AND status = 'pending' AND expires_at > CURRENT_TIMESTAMP`, tokenHash).Scan(
		&pr.ID, &pr.UserID, &pr.TokenHash, &pr.Status, &pr.ExpiresAt, &pr.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &pr, nil
}

func (s *Store) CompletePasswordReset(tokenHash, newPasswordHash string) error {
	pr, err := s.GetPasswordResetByHash(tokenHash)
	if err != nil {
		return errors.New("invalid or expired password reset token")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newPasswordHash, pr.UserID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE password_resets SET status = 'used' WHERE id = ?`, pr.ID)
	if err != nil {
		return err
	}

	// Invalidate active sessions for security
	_, err = tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, pr.UserID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ----------------- SSO CONFIGS -----------------

func (s *Store) GetSSOConfigs(orgID string) ([]db.SSOConfig, error) {
	rows, err := s.db.Query(`
		SELECT id, org_id, provider, client_id, client_secret, issuer_url, domain_mapping, is_enabled, created_at
		FROM sso_configs WHERE org_id = ? ORDER BY provider ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.SSOConfig
	for rows.Next() {
		var c db.SSOConfig
		if err := rows.Scan(&c.ID, &c.OrgID, &c.Provider, &c.ClientID, &c.ClientSecret, &c.IssuerURL, &c.DomainMapping, &c.IsEnabled, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (s *Store) SaveSSOConfig(cfg db.SSOConfig) (*db.SSOConfig, error) {
	if cfg.ID == "" {
		cfg.ID = "sso_" + uuid.New().String()[:12]
	}
	cfg.CreatedAt = time.Now().UTC()
	_, err := s.db.Exec(`
		INSERT INTO sso_configs (id, org_id, provider, client_id, client_secret, issuer_url, domain_mapping, is_enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(org_id, provider) DO UPDATE SET
			client_id = excluded.client_id,
			client_secret = excluded.client_secret,
			issuer_url = excluded.issuer_url,
			domain_mapping = excluded.domain_mapping,
			is_enabled = excluded.is_enabled`,
		cfg.ID, cfg.OrgID, cfg.Provider, cfg.ClientID, cfg.ClientSecret, cfg.IssuerURL, cfg.DomainMapping, cfg.IsEnabled, cfg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ----------------- CUSTOM ROLES -----------------

func (s *Store) GetCustomRoles(orgID string) ([]db.CustomRole, error) {
	rows, err := s.db.Query(`
		SELECT id, org_id, name, description, permissions_json, is_system, created_at
		FROM custom_roles WHERE org_id = ? ORDER BY is_system DESC, name ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.CustomRole
	for rows.Next() {
		var r db.CustomRole
		if err := rows.Scan(&r.ID, &r.OrgID, &r.Name, &r.Description, &r.PermissionsJSON, &r.IsSystem, &r.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

func (s *Store) CreateCustomRole(role db.CustomRole) (*db.CustomRole, error) {
	if role.ID == "" {
		role.ID = "role_" + uuid.New().String()[:12]
	}
	role.CreatedAt = time.Now().UTC()
	if role.PermissionsJSON == "" {
		role.PermissionsJSON = "[]"
	}
	_, err := s.db.Exec(`
		INSERT INTO custom_roles (id, org_id, name, description, permissions_json, is_system, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		role.ID, role.OrgID, role.Name, role.Description, role.PermissionsJSON, role.IsSystem, role.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (s *Store) UpdateCustomRole(role db.CustomRole) (*db.CustomRole, error) {
	_, err := s.db.Exec(`
		UPDATE custom_roles
		SET description = ?, permissions_json = ?
		WHERE id = ? AND org_id = ? AND is_system = 0`,
		role.Description, role.PermissionsJSON, role.ID, role.OrgID)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (s *Store) DeleteCustomRole(orgID, id string) error {
	res, err := s.db.Exec(`DELETE FROM custom_roles WHERE id = ? AND org_id = ? AND is_system = 0`, id, orgID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("cannot delete system role or role not found")
	}
	return nil
}

// ----------------- IMMUTABLE AUDIT VAULT -----------------

func (s *Store) RecordAuditVaultBlock(b db.AuditChainBlock) (*db.AuditChainBlock, error) {
	// 1. Fetch highest sequence number and last block hash
	var lastSeq sql.NullInt64
	var lastHash sql.NullString
	_ = s.db.QueryRow(`
		SELECT sequence_number, block_hash
		FROM audit_chain_vault
		WHERE org_id = ?
		ORDER BY sequence_number DESC LIMIT 1`, b.OrgID).Scan(&lastSeq, &lastHash)

	nextSeq := int64(1)
	prevHash := auth.GenesisBlockHash
	if lastSeq.Valid && lastSeq.Int64 > 0 {
		nextSeq = lastSeq.Int64 + 1
		prevHash = lastHash.String
	}

	if b.ID == "" {
		b.ID = "acv_" + uuid.New().String()[:12]
	}
	b.SequenceNumber = nextSeq
	b.PrevBlockHash = prevHash
	if b.Timestamp.IsZero() {
		b.Timestamp = time.Now().UTC()
	}

	tsStr := b.Timestamp.UTC().Format(time.RFC3339)
	b.BlockHash = auth.ComputeAuditBlockHash(b.SequenceNumber, b.PrevBlockHash, b.Action, b.ActorID, b.PayloadHash, tsStr)

	_, err := s.db.Exec(`
		INSERT INTO audit_chain_vault (id, org_id, sequence_number, actor_type, actor_id, action, resource_type, resource_id, payload_hash, prev_block_hash, block_hash, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.OrgID, b.SequenceNumber, b.ActorType, b.ActorID, b.Action, b.ResourceType, b.ResourceID, b.PayloadHash, b.PrevBlockHash, b.BlockHash, b.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed recording audit vault block: %w", err)
	}
	return &b, nil
}

func (s *Store) GetAuditVaultBlocks(orgID string, limit int) ([]db.AuditChainBlock, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, org_id, sequence_number, actor_type, actor_id, action, resource_type, resource_id, payload_hash, prev_block_hash, block_hash, timestamp
		FROM audit_chain_vault WHERE org_id = ? ORDER BY sequence_number DESC LIMIT ?`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.AuditChainBlock
	for rows.Next() {
		var b db.AuditChainBlock
		if err := rows.Scan(&b.ID, &b.OrgID, &b.SequenceNumber, &b.ActorType, &b.ActorID, &b.Action, &b.ResourceType, &b.ResourceID, &b.PayloadHash, &b.PrevBlockHash, &b.BlockHash, &b.Timestamp); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, nil
}

func (s *Store) VerifyAuditVaultChain(orgID string) (bool, int, error) {
	rows, err := s.db.Query(`
		SELECT sequence_number, prev_block_hash, block_hash, action, actor_id, payload_hash, timestamp
		FROM audit_chain_vault WHERE org_id = ? ORDER BY sequence_number ASC`, orgID)
	if err != nil {
		return false, 0, err
	}
	defer rows.Close()

	var items []auth.ChainBlockVerificationItem
	for rows.Next() {
		var item auth.ChainBlockVerificationItem
		if err := rows.Scan(&item.SequenceNumber, &item.PrevBlockHash, &item.BlockHash, &item.Action, &item.ActorID, &item.PayloadHash, &item.Timestamp); err != nil {
			return false, 0, err
		}
		items = append(items, item)
	}

	return auth.VerifyAuditChain(items)
}

// ----------------- GDPR & PRIVACY -----------------

func (s *Store) CreatePrivacyRequest(req db.PrivacyRequest) (*db.PrivacyRequest, error) {
	if req.ID == "" {
		req.ID = "priv_" + uuid.New().String()[:12]
	}
	req.RequestedAt = time.Now().UTC()
	req.Status = "pending"
	if req.ResultJSON == "" {
		req.ResultJSON = "{}"
	}

	_, err := s.db.Exec(`
		INSERT INTO privacy_requests (id, org_id, user_id, request_type, status, result_json, requested_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.OrgID, req.UserID, req.RequestType, req.Status, req.ResultJSON, req.RequestedAt)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (s *Store) GetPrivacyRequests(orgID string) ([]db.PrivacyRequest, error) {
	rows, err := s.db.Query(`
		SELECT id, org_id, user_id, request_type, status, result_json, requested_at, completed_at
		FROM privacy_requests WHERE org_id = ? ORDER BY requested_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.PrivacyRequest
	for rows.Next() {
		var pr db.PrivacyRequest
		var comp sql.NullTime
		if err := rows.Scan(&pr.ID, &pr.OrgID, &pr.UserID, &pr.RequestType, &pr.Status, &pr.ResultJSON, &pr.RequestedAt, &comp); err != nil {
			return nil, err
		}
		if comp.Valid {
			pr.CompletedAt = &comp.Time
		}
		list = append(list, pr)
	}
	return list, nil
}

func (s *Store) GenerateGDPRUserExport(ctx context.Context, orgID, userID string) (map[string]interface{}, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	export := map[string]interface{}{
		"export_timestamp": time.Now().UTC().Format(time.RFC3339),
		"organization_id":  orgID,
		"profile": map[string]interface{}{
			"id":           user.ID,
			"email":        user.Email,
			"name":         user.Name,
			"role":         user.Role,
			"is_active":    user.IsActive,
			"totp_enabled": user.TOTPEnabled,
			"created_at":   user.CreatedAt,
		},
	}

	// Fetch audit trail actions performed by this user
	auditRows, err := s.db.Query(`
		SELECT action, entity_type, entity_id, reasoning, created_at
		FROM audit_logs WHERE actor_id = ? ORDER BY created_at DESC LIMIT 50`, userID)
	if err == nil {
		defer auditRows.Close()
		var audits []map[string]interface{}
		for auditRows.Next() {
			var act, eType, eID, reason string
			var cAt time.Time
			if err := auditRows.Scan(&act, &eType, &eID, &reason, &cAt); err == nil {
				audits = append(audits, map[string]interface{}{
					"action":      act,
					"entity_type": eType,
					"entity_id":   eID,
					"reasoning":   reason,
					"created_at":  cAt,
				})
			}
		}
		export["audit_actions"] = audits
	}

	return export, nil
}

func (s *Store) ExecuteGDPRForgetMe(orgID, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Anonymize user record
	anonymizedName := fmt.Sprintf("Anonymized User %s", userID[:8])
	anonymizedEmail := fmt.Sprintf("anonymized_%s@redacted.local", userID[:8])
	dummyHash := "$2a$12$eXAMpleRedactedHashNotValid12345678901234567890123456"

	_, err = tx.Exec(`
		UPDATE users
		SET name = ?, email = ?, password_hash = ?, is_active = 0, totp_secret = '', totp_enabled = 0, recovery_codes_json = '[]', updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND org_id = ?`, anonymizedName, anonymizedEmail, dummyHash, userID, orgID)
	if err != nil {
		return err
	}

	// Terminate active sessions
	_, _ = tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)

	// Anonymize employee record if present
	_, _ = tx.Exec(`
		UPDATE employees
		SET first_name = 'Redacted', last_name = 'Employee', email = ?, phone = '', tax_id = '', bank_account_number = '', emergency_contact_name = '', emergency_contact_phone = ''
		WHERE email = (SELECT email FROM users WHERE id = ?)`, anonymizedEmail, userID)

	return tx.Commit()
}

// ----------------- SOC2 ACCESS REVIEWS -----------------

func (s *Store) GenerateSOC2AccessReview(orgID, quarter, reviewerID string) (*db.SOC2AccessReview, error) {
	// Snapshot active users and their roles
	rows, err := s.db.Query(`
		SELECT id, email, name, role, is_active, COALESCE(totp_enabled, 0), last_active_at, created_at
		FROM users WHERE org_id = ? ORDER BY name ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userList []map[string]interface{}
	for rows.Next() {
		var id, email, name, role string
		var isActive, totpEn bool
		var lastAct sql.NullTime
		var cAt time.Time
		if err := rows.Scan(&id, &email, &name, &role, &isActive, &totpEn, &lastAct, &cAt); err == nil {
			userMap := map[string]interface{}{
				"id":           id,
				"email":        email,
				"name":         name,
				"role":         role,
				"is_active":    isActive,
				"totp_enabled": totpEn,
				"created_at":   cAt,
			}
			if lastAct.Valid {
				userMap["last_active_at"] = lastAct.Time
			}
			userList = append(userList, userMap)
		}
	}

	// Snapshot active API keys
	keyRows, err := s.db.Query(`
		SELECT id, name, role, allowed_tools, is_active, created_at, last_used_at
		FROM api_keys WHERE org_id = ?`, orgID)
	var keyList []map[string]interface{}
	if err == nil {
		defer keyRows.Close()
		for keyRows.Next() {
			var kid, kname, krole, ktools string
			var kactive bool
			var kcAt time.Time
			var klast sql.NullTime
			if err := keyRows.Scan(&kid, &kname, &krole, &ktools, &kactive, &kcAt, &klast); err == nil {
				kMap := map[string]interface{}{
					"id":            kid,
					"name":          kname,
					"role":          krole,
					"allowed_tools": ktools,
					"is_active":     kactive,
					"created_at":    kcAt,
				}
				if klast.Valid {
					kMap["last_used_at"] = klast.Time
				}
				keyList = append(keyList, kMap)
			}
		}
	}

	snapshotData := map[string]interface{}{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"quarter":      quarter,
		"users_count":  len(userList),
		"keys_count":   len(keyList),
		"users":        userList,
		"api_keys":     keyList,
	}

	snapJSON, _ := json.MarshalIndent(snapshotData, "", "  ")
	id := fmt.Sprintf("soc2_rev_%s_%s", strings.ToLower(quarter), uuid.New().String()[:6])
	now := time.Now().UTC()

	review := db.SOC2AccessReview{
		ID:           id,
		OrgID:        orgID,
		Quarter:      quarter,
		Status:       "pending",
		ReviewedBy:   reviewerID,
		Notes:        fmt.Sprintf("Quarterly automated SOC2 access review snapshot for %s", quarter),
		SnapshotJSON: string(snapJSON),
		CreatedAt:    now,
	}

	_, err = s.db.Exec(`
		INSERT INTO soc2_access_reviews (id, org_id, quarter, status, reviewed_by, notes, snapshot_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		review.ID, review.OrgID, review.Quarter, review.Status, review.ReviewedBy, review.Notes, review.SnapshotJSON, review.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &review, nil
}

func (s *Store) SignOffSOC2AccessReview(id, reviewerID, notes string) (*db.SOC2AccessReview, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		UPDATE soc2_access_reviews
		SET status = 'approved', reviewed_by = ?, notes = ?, reviewed_at = ?
		WHERE id = ?`, reviewerID, notes, now, id)
	if err != nil {
		return nil, err
	}

	var rev db.SOC2AccessReview
	var revAt sql.NullTime
	err = s.db.QueryRow(`
		SELECT id, org_id, quarter, status, reviewed_by, notes, snapshot_json, reviewed_at, created_at
		FROM soc2_access_reviews WHERE id = ?`, id).Scan(
		&rev.ID, &rev.OrgID, &rev.Quarter, &rev.Status, &rev.ReviewedBy, &rev.Notes, &rev.SnapshotJSON, &revAt, &rev.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if revAt.Valid {
		rev.ReviewedAt = &revAt.Time
	}
	return &rev, nil
}

func (s *Store) GetSOC2AccessReviews(orgID string) ([]db.SOC2AccessReview, error) {
	rows, err := s.db.Query(`
		SELECT id, org_id, quarter, status, reviewed_by, notes, snapshot_json, reviewed_at, created_at
		FROM soc2_access_reviews WHERE org_id = ? ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.SOC2AccessReview
	for rows.Next() {
		var rev db.SOC2AccessReview
		var revAt sql.NullTime
		if err := rows.Scan(&rev.ID, &rev.OrgID, &rev.Quarter, &rev.Status, &rev.ReviewedBy, &rev.Notes, &rev.SnapshotJSON, &revAt, &rev.CreatedAt); err != nil {
			return nil, err
		}
		if revAt.Valid {
			rev.ReviewedAt = &revAt.Time
		}
		list = append(list, rev)
	}
	return list, nil
}
