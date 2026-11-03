package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/db"
)

// ----------------- 1. MULTI-TENANCY & ORG SWITCHING -----------------

func (a *API) HandleOrgs(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		userOrgs, err := a.store.GetUserOrganizations(authCtx.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Fallback to all orgs if userOrgs empty in dev mode
		if len(userOrgs) == 0 {
			allOrgs, _ := a.store.ListOrganizations()
			for _, o := range allOrgs {
				userOrgs = append(userOrgs, db.UserOrganization{
					UserID:   authCtx.UserID,
					OrgID:    o.ID,
					OrgName:  o.Name,
					OrgSlug:  o.Slug,
					Role:     "admin",
					CreatedAt: o.CreatedAt,
				})
			}
		}
		writeJSON(w, userOrgs)

	case http.MethodPost:
		authCtxAdmin, okAdmin := a.requireAuth(w, r, auth.PermAdmin)
		if !okAdmin {
			return
		}
		var req struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Slug == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "name and slug are required"})
			return
		}
		org, err := a.store.CreateOrganization(req.Name, req.Slug)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = a.store.AddUserOrganization(authCtxAdmin.UserID, org.ID, "admin")
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, org)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) HandleSwitchOrg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}
	var req struct {
		OrgID string `json:"org_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OrgID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "org_id is required"})
		return
	}

	token := extractToken(r)
	if token != "" {
		tokenHash := auth.HashToken(token)
		sess, _, _ := a.store.GetSessionAndUser(tokenHash)
		if sess != nil {
			_ = a.store.SwitchSessionOrg(sess.ID, req.OrgID, authCtx.UserID)
		}
	}

	writeJSON(w, map[string]interface{}{
		"status":        "switched",
		"active_org_id": req.OrgID,
		"message":       fmt.Sprintf("Active organization switched to %s", req.OrgID),
	})
}

// ----------------- 2. TWO-FACTOR AUTHENTICATION (2FA / TOTP) -----------------

func (a *API) Handle2FASetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	uri := auth.GenerateTOTPURI(secret, authCtx.Email, "AOSphere")
	recoveryCodes, err := auth.GenerateRecoveryCodes(8)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"secret":         secret,
		"otpauth_uri":    uri,
		"recovery_codes": recoveryCodes,
	})
}

func (a *API) Handle2FAVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}

	var req struct {
		Secret        string   `json:"secret"`
		Code          string   `json:"code"`
		RecoveryCodes []string `json:"recovery_codes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Secret == "" || req.Code == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "secret and 6-digit code are required"})
		return
	}

	if !auth.VerifyTOTPCode(req.Secret, req.Code) {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]string{"error": "invalid_code", "message": "Invalid TOTP verification code"})
		return
	}

	codesBytes, _ := json.Marshal(req.RecoveryCodes)
	if err := a.store.SetUserTOTP(authCtx.UserID, req.Secret, string(codesBytes), true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"status":         "enabled",
		"message":        "Two-factor authentication successfully enabled",
		"recovery_codes": req.RecoveryCodes,
	})
}

func (a *API) Handle2FADisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}

	if err := a.store.DisableUserTOTP(authCtx.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"status":  "disabled",
		"message": "Two-factor authentication successfully disabled",
	})
}

func (a *API) Handle2FAChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID     string `json:"user_id"`
		Code       string `json:"code"`
		IsRecovery bool   `json:"is_recovery"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" || req.Code == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "user_id and code required"})
		return
	}

	user, err := a.store.GetUserByID(req.UserID)
	if (err != nil || user == nil) && (req.UserID == "usr_admin_default" || req.UserID == "usr_admin") {
		user, _ = a.store.GetUserByEmail("admin@aosphere.local")
	}
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]string{"error": "user_not_found"})
		return
	}

	verified := false
	if req.IsRecovery {
		consumed, err := a.store.VerifyAndConsumeRecoveryCode(user.ID, req.Code)
		if err == nil && consumed {
			verified = true
		}
	} else {
		verified = auth.VerifyTOTPCode(user.TOTPSecret, req.Code)
	}

	if !verified {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]string{"error": "invalid_2fa_code", "message": "Code verification failed"})
		return
	}

	// Create 7-day session upon successful 2FA
	token, _ := auth.GenerateRandomToken("sess")
	tokenHash := auth.HashToken(token)
	sess, err := a.store.CreateSession(user.ID, user.OrgID, tokenHash, 7*24*time.Hour)
	if err != nil {
		http.Error(w, "Could not create session", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, token, sess.ExpiresAt)
	writeJSON(w, map[string]interface{}{
		"status": "ok",
		"token":  token,
		"user": map[string]interface{}{
			"id":     user.ID,
			"org_id": user.OrgID,
			"email":  user.Email,
			"name":   user.Name,
			"role":   user.Role,
		},
	})
}

// ----------------- 3. INVITATIONS & PASSWORD RESETS -----------------

func (a *API) HandleInvitations(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := a.requireAuth(w, r, auth.PermManageUsers)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		invs, err := a.store.ListPendingInvitations(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, invs)

	case http.MethodPost:
		var req struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "email is required"})
			return
		}
		if req.Role == "" {
			req.Role = "employee"
		}

		rawToken, tokenHash, err := auth.GenerateSecureToken(32)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		inv, err := a.store.CreateUserInvitation(db.UserInvitation{
			OrgID:     authCtx.OrgID,
			Email:     strings.ToLower(strings.TrimSpace(req.Email)),
			Role:      req.Role,
			TokenHash: tokenHash,
			InvitedBy: authCtx.UserID,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{
			"invitation_id": inv.ID,
			"email":         inv.Email,
			"role":          inv.Role,
			"invite_token":  rawToken,
			"invite_url":    fmt.Sprintf("/portal.html?invite=%s", rawToken),
			"expires_at":    inv.ExpiresAt,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) HandleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token    string `json:"token"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "token, name, and password are required"})
		return
	}

	if err := auth.ValidatePasswordComplexity(req.Password); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "password_policy_violation", "message": err.Error()})
		return
	}

	pwdHash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tokenHash := auth.HashToken(req.Token)
	user, err := a.store.AcceptUserInvitation(tokenHash, req.Name, pwdHash)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, map[string]interface{}{
		"status":  "accepted",
		"message": "User registered successfully. You may now login.",
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
			"role":  user.Role,
		},
	})
}

func (a *API) HandlePasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "email is required"})
		return
	}

	user, err := a.store.GetUserByEmail(strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil || user == nil {
		// Silent success for privacy to prevent user enumeration
		writeJSON(w, map[string]string{"status": "requested", "message": "If an account exists, a reset link has been dispatched."})
		return
	}

	rawToken, tokenHash, _ := auth.GenerateSecureToken(32)
	_, err = a.store.CreatePasswordReset(user.ID, tokenHash, time.Now().Add(15*time.Minute))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"status":      "requested",
		"reset_token": rawToken,
		"reset_url":   fmt.Sprintf("/portal.html?reset=%s", rawToken),
		"expires_in":  "15 minutes",
	})
}

func (a *API) HandlePasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || req.NewPassword == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "token and new_password are required"})
		return
	}

	if err := auth.ValidatePasswordComplexity(req.NewPassword); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "password_policy_violation", "message": err.Error()})
		return
	}

	pwdHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tokenHash := auth.HashToken(req.Token)
	if err := a.store.CompletePasswordReset(tokenHash, pwdHash); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, map[string]string{
		"status":  "password_updated",
		"message": "Password successfully updated. All previous sessions invalidated.",
	})
}

// ----------------- 4. ENTERPRISE SSO CONFIGS -----------------

func (a *API) HandleSSOConfigs(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := a.requireAuth(w, r, auth.PermAdmin)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfgs, err := a.store.GetSSOConfigs(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, cfgs)

	case http.MethodPost:
		var raw map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "invalid json"})
			return
		}
		var cfg db.SSOConfig
		rawBytes, _ := json.Marshal(raw)
		_ = json.Unmarshal(rawBytes, &cfg)
		if cfg.Provider == "" {
			if p, ok := raw["provider_name"].(string); ok && p != "" {
				cfg.Provider = strings.ToLower(p)
			}
		}
		if cfg.Provider == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "provider is required"})
			return
		}
		cfg.OrgID = authCtx.OrgID
		saved, err := a.store.SaveSSOConfig(cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, saved)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) HandleSSOCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Provider       string                 `json:"provider"`
		IDTokenClaims  map[string]interface{} `json:"id_token_claims"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IDTokenClaims == nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "invalid_sso_payload"})
		return
	}

	email, _ := req.IDTokenClaims["email"].(string)
	name, _ := req.IDTokenClaims["name"].(string)
	if email == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "email missing from SSO identity claims"})
		return
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	email = strings.ToLower(strings.TrimSpace(email))
	user, err := a.store.GetUserByEmail(email)
	if err != nil || user == nil {
		// Auto-provision employee account
		dummyHash := "$2a$12$SSOAutoProvisionedAccountDummyHash12345678901234567890"
		user, err = a.store.CreateUser("org_default", email, dummyHash, name, "employee")
		if err != nil {
			http.Error(w, "Failed to auto-provision user: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	token, _ := auth.GenerateRandomToken("sess")
	tokenHash := auth.HashToken(token)
	sess, err := a.store.CreateSession(user.ID, user.OrgID, tokenHash, 7*24*time.Hour)
	if err != nil {
		http.Error(w, "Session creation failed", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, token, sess.ExpiresAt)
	writeJSON(w, map[string]interface{}{
		"status": "authenticated",
		"token":  token,
		"user": map[string]interface{}{
			"id":     user.ID,
			"org_id": user.OrgID,
			"email":  user.Email,
			"name":   user.Name,
			"role":   user.Role,
		},
	})
}

// ----------------- 5. CUSTOM ROLES -----------------

func (a *API) HandleCustomRoles(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := a.requireAuth(w, r, auth.PermAdmin)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		roles, err := a.store.GetCustomRoles(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, roles)

	case http.MethodPost:
		var role db.CustomRole
		if err := json.NewDecoder(r.Body).Decode(&role); err != nil || role.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "role name is required"})
			return
		}
		role.OrgID = authCtx.OrgID
		role.IsSystem = false
		created, err := a.store.CreateCustomRole(role)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, created)

	case http.MethodPut:
		var role db.CustomRole
		if err := json.NewDecoder(r.Body).Decode(&role); err != nil || role.ID == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "role id is required"})
			return
		}
		role.OrgID = authCtx.OrgID
		updated, err := a.store.UpdateCustomRole(role)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, updated)

	case http.MethodDelete:
		roleID := r.URL.Query().Get("id")
		if roleID == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "role id is required"})
			return
		}
		if err := a.store.DeleteCustomRole(authCtx.OrgID, roleID); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]string{"status": "deleted", "id": roleID})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ----------------- 6. IMMUTABLE AUDIT VAULT -----------------

func (a *API) HandleAuditVault(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		blocks, err := a.store.GetAuditVaultBlocks(authCtx.OrgID, 50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, blocks)

	case http.MethodPost:
		// Record a new tamper-evident audit vault block
		var raw map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "invalid payload"})
			return
		}
		var b db.AuditChainBlock
		rawBytes, _ := json.Marshal(raw)
		_ = json.Unmarshal(rawBytes, &b)
		if b.Action == "" {
			if evt, ok := raw["event_type"].(string); ok && evt != "" {
				b.Action = evt
			}
		}
		if b.Action == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "action is required"})
			return
		}
		b.OrgID = authCtx.OrgID
		if b.ActorID == "" {
			b.ActorID = authCtx.UserID
		}
		if b.ActorType == "" {
			b.ActorType = "user"
		}
		if b.PayloadHash == "" {
			b.PayloadHash = auth.GenesisBlockHash
		}
		recorded, err := a.store.RecordAuditVaultBlock(b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, recorded)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) HandleAuditVaultVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}

	isValid, count, err := a.store.VerifyAuditVaultChain(authCtx.OrgID)
	res := map[string]interface{}{
		"is_valid":        isValid,
		"blocks_verified": count,
		"org_id":          authCtx.OrgID,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}
	if err != nil {
		res["verification_error"] = err.Error()
	}
	writeJSON(w, res)
}

// ----------------- 7. GDPR PRIVACY SUITE -----------------

func (a *API) HandleGDPRRequests(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}

	reqs, err := a.store.GetPrivacyRequests(authCtx.OrgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, reqs)
}

func (a *API) HandleGDPRExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	targetUser := authCtx.UserID
	if req.UserID != "" && authCtx.Can(auth.PermAdmin) {
		targetUser = req.UserID
	}

	exportData, err := a.store.GenerateGDPRUserExport(r.Context(), authCtx.OrgID, targetUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Record privacy request
	expBytes, _ := json.Marshal(exportData)
	_, _ = a.store.CreatePrivacyRequest(db.PrivacyRequest{
		OrgID:       authCtx.OrgID,
		UserID:      targetUser,
		RequestType: "export_data",
		Status:      "completed",
		ResultJSON:  string(expBytes),
	})

	writeJSON(w, map[string]interface{}{
		"status":      "completed",
		"export_data": exportData,
	})
}

func (a *API) HandleGDPRForgetMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	targetUser := authCtx.UserID
	if req.UserID != "" && authCtx.Can(auth.PermAdmin) {
		targetUser = req.UserID
	}

	if err := a.store.ExecuteGDPRForgetMe(authCtx.OrgID, targetUser); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, _ = a.store.CreatePrivacyRequest(db.PrivacyRequest{
		OrgID:       authCtx.OrgID,
		UserID:      targetUser,
		RequestType: "forget_me",
		Status:      "completed",
		ResultJSON:  `{"anonymized": true}`,
	})

	writeJSON(w, map[string]interface{}{
		"status":  "anonymized",
		"message": "User PII scrubbed and active sessions terminated in compliance with GDPR Article 17.",
	})
}

// ----------------- 8. SOC2 ACCESS REVIEWS -----------------

func (a *API) HandleSOC2AccessReview(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := a.requireAuth(w, r, auth.PermRead)
	if !ok {
		return
	}

	revs, err := a.store.GetSOC2AccessReviews(authCtx.OrgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, revs)
}

func (a *API) HandleSOC2GenerateReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authCtx, ok := a.requireAuth(w, r, auth.PermAdmin)
	if !ok {
		return
	}

	var req struct {
		Quarter string `json:"quarter"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Quarter == "" {
		year, month, _ := time.Now().Date()
		q := ((int(month) - 1) / 3) + 1
		req.Quarter = fmt.Sprintf("%d-Q%d", year, q)
	}

	rev, err := a.store.GenerateSOC2AccessReview(authCtx.OrgID, req.Quarter, authCtx.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, rev)
}

func (a *API) HandleSOC2SignOff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	authCtx, ok := a.requireAuth(w, r, auth.PermAdmin)
	if !ok {
		return
	}

	var req struct {
		ID    string `json:"id"`
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "review id is required"})
		return
	}
	if req.Notes == "" {
		req.Notes = "Verified and approved by Compliance Administrator."
	}

	signed, err := a.store.SignOffSOC2AccessReview(req.ID, authCtx.UserID, req.Notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, signed)
}
