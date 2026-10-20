package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aosphere/internal/auth"
)

type RegisterRequest struct {
	OrgName  string `json:"org_name"`
	OrgSlug  string `json:"org_slug"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateAPIKeyRequest struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	AllowedTools string `json:"allowed_tools"`
}

// HandleRegister creates a new organization and root owner account
func (a *API) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "invalid_request", "message": "Invalid JSON payload"})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" || req.Name == "" || req.OrgName == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "validation_error", "message": "Missing required fields"})
		return
	}

	// Password validation
	passHash, err := auth.HashPassword(req.Password)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "weak_password", "message": err.Error()})
		return
	}

	if req.OrgSlug == "" {
		req.OrgSlug = strings.ToLower(strings.ReplaceAll(req.OrgName, " ", "-"))
	}

	// 1. Create Organization
	org, err := a.store.CreateOrganization(req.OrgName, req.OrgSlug)
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]string{"error": "org_exists", "message": "Organization slug or name already exists"})
		return
	}

	// 2. Create Owner User
	user, err := a.store.CreateUser(org.ID, req.Email, passHash, req.Name, string(auth.RoleOwner))
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]string{"error": "user_exists", "message": "Email is already registered"})
		return
	}

	// 3. Create Session (7 days validity)
	token, _ := auth.GenerateRandomToken("sess")
	tokenHash := auth.HashToken(token)
	sess, err := a.store.CreateSession(user.ID, org.ID, tokenHash, 7*24*time.Hour)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]string{"error": "session_error", "message": "Failed to create user session"})
		return
	}

	setSessionCookie(w, token, sess.ExpiresAt)

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]interface{}{
		"status":       "ok",
		"token":        token,
		"user":         user,
		"organization": org,
	})
}

// HandleLogin authenticates users and sets session cookies/tokens
func (a *API) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "invalid_request", "message": "Invalid JSON body"})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	user, err := a.store.GetUserByEmail(req.Email)
	if err != nil || user == nil || !auth.VerifyPassword(user.PasswordHash, req.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]string{"error": "invalid_credentials", "message": "Invalid email or password"})
		return
	}

	if !user.IsActive {
		w.WriteHeader(http.StatusForbidden)
		writeJSON(w, map[string]string{"error": "account_suspended", "message": "This account is currently deactivated"})
		return
	}

	// Create 7-day session
	token, _ := auth.GenerateRandomToken("sess")
	tokenHash := auth.HashToken(token)
	sess, err := a.store.CreateSession(user.ID, user.OrgID, tokenHash, 7*24*time.Hour)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]string{"error": "session_error", "message": "Could not create session"})
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

// HandleLogout terminates the active session
func (a *API) HandleLogout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token != "" {
		tokenHash := auth.HashToken(token)
		_ = a.store.DeleteSession(tokenHash)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "aosphere_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	writeJSON(w, map[string]string{"status": "ok", "message": "Successfully logged out"})
}

// HandleMe returns the verified identity and permissions of the caller
func (a *API) HandleMe(w http.ResponseWriter, r *http.Request) {
	authCtx := GetAuthContext(r.Context())
	if authCtx == nil {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	org, _ := a.store.GetOrganization(authCtx.OrgID)

	writeJSON(w, map[string]interface{}{
		"user_id":      authCtx.UserID,
		"org_id":       authCtx.OrgID,
		"organization": org,
		"email":        authCtx.Email,
		"name":         authCtx.Name,
		"role":         authCtx.Role,
		"is_api_key":   authCtx.IsAPIKey,
		"permissions": map[string]bool{
			"manage_inventory": authCtx.Can(auth.PermManageInventory),
			"manage_orders":    authCtx.Can(auth.PermManageOrders),
			"approve_orders":   authCtx.Can(auth.PermApproveOrders),
			"manage_finance":   authCtx.Can(auth.PermManageFinance),
			"manage_payroll":   authCtx.Can(auth.PermManagePayroll),
			"manage_users":     authCtx.Can(auth.PermManageUsers),
			"admin":            authCtx.Can(auth.PermAdmin),
		},
	})
}

// HandleAPIKeys provisions and lists scoped API keys for agents
func (a *API) HandleAPIKeys(w http.ResponseWriter, r *http.Request) {
	authCtx := GetAuthContext(r.Context())
	if authCtx == nil {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	// Only Admins or Owners can provision API keys
	if !authCtx.Can(auth.PermAdmin) {
		w.WriteHeader(http.StatusForbidden)
		writeJSON(w, map[string]string{"error": "forbidden", "message": "Only Administrators can manage API keys"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		keys, err := a.store.ListAPIKeys(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, keys)

	case http.MethodPost:
		var req CreateAPIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "invalid_payload"})
			return
		}

		if req.Name == "" {
			req.Name = "AI Agent Client"
		}
		if req.Role == "" {
			req.Role = string(auth.RoleOperations)
		}

		rawKey, _ := auth.GenerateRandomToken("ao_sk")
		keyHash := auth.HashToken(rawKey)

		apiKey, err := a.store.CreateAPIKey(authCtx.OrgID, authCtx.UserID, req.Name, keyHash, req.Role, req.AllowedTools)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]string{"error": "creation_failed", "message": err.Error()})
			return
		}

		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{
			"status":    "created",
			"key_id":    apiKey.ID,
			"name":      apiKey.Name,
			"role":      apiKey.Role,
			"api_token": rawKey, // Displayed only once!
			"warning":   "Store this token securely; it cannot be recovered.",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "aosphere_session",
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
