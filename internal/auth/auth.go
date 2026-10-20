package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Role defines the authorization tier of a user or API client
type Role string

const (
	RoleOwner      Role = "owner"
	RoleAdmin      Role = "admin"
	RoleFinance    Role = "finance"
	RoleOperations Role = "operations"
	RoleHR         Role = "hr"
	RoleEmployee   Role = "employee"
	RoleReadOnly   Role = "readonly"
)

// Permission defines a granular action capability
type Permission string

const (
	PermRead            Permission = "read"
	PermManageInventory Permission = "manage_inventory"
	PermManageOrders    Permission = "manage_orders"
	PermApproveOrders   Permission = "approve_orders"
	PermManageFinance   Permission = "manage_finance"
	PermManagePayroll   Permission = "manage_payroll"
	PermManageHR        Permission = "manage_hr"
	PermManageUsers     Permission = "manage_users"
	PermAdmin           Permission = "admin"
)

// rolePermissions defines the RBAC capability matrix
var rolePermissions = map[Role][]Permission{
	RoleOwner: {
		PermRead, PermManageInventory, PermManageOrders, PermApproveOrders,
		PermManageFinance, PermManagePayroll, PermManageHR, PermManageUsers, PermAdmin,
	},
	RoleAdmin: {
		PermRead, PermManageInventory, PermManageOrders, PermApproveOrders,
		PermManageFinance, PermManagePayroll, PermManageHR, PermManageUsers, PermAdmin,
	},
	RoleFinance: {
		PermRead, PermManageOrders, PermApproveOrders, PermManageFinance, PermManagePayroll,
	},
	RoleOperations: {
		PermRead, PermManageInventory, PermManageOrders,
	},
	RoleHR: {
		PermRead, PermManagePayroll, PermManageHR, PermManageUsers,
	},
	RoleEmployee: {
		PermRead,
	},
	RoleReadOnly: {
		PermRead,
	},
}

// HasPermission checks if a given role grants the requested permission
func HasPermission(role Role, perm Permission) bool {
	perms, exists := rolePermissions[role]
	if !exists {
		return false
	}
	for _, p := range perms {
		if p == perm || p == PermAdmin {
			return true
		}
	}
	return false
}

// AuthContext represents the verified identity attached to a request
type AuthContext struct {
	UserID   string `json:"user_id"`
	OrgID    string `json:"org_id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     Role   `json:"role"`
	IsAPIKey bool   `json:"is_api_key"`
}

// Can checks if the authenticated user has a specific permission
func (a *AuthContext) Can(perm Permission) bool {
	if a == nil {
		return false
	}
	return HasPermission(a.Role, perm)
}

// HashPassword hashes a plain-text password using bcrypt with cost 12
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", fmt.Errorf("password must be at least 8 characters long")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks if a plain-text password matches a bcrypt hash
func VerifyPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateRandomToken generates a cryptographically secure random token (e.g. 32 bytes hex)
func GenerateRandomToken(prefix string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	token := hex.EncodeToString(bytes)
	if prefix != "" {
		return fmt.Sprintf("%s_%s", prefix, token), nil
	}
	return token, nil
}

// HashToken creates a SHA-256 hash of a session or API token for safe database storage
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// ParseRole validates and converts a string into a known Role
func ParseRole(roleStr string) (Role, error) {
	switch Role(strings.ToLower(strings.TrimSpace(roleStr))) {
	case RoleOwner:
		return RoleOwner, nil
	case RoleAdmin:
		return RoleAdmin, nil
	case RoleFinance:
		return RoleFinance, nil
	case RoleOperations:
		return RoleOperations, nil
	case RoleHR:
		return RoleHR, nil
	case RoleEmployee:
		return RoleEmployee, nil
	case RoleReadOnly:
		return RoleReadOnly, nil
	default:
		return "", fmt.Errorf("unknown role %q", roleStr)
	}
}

// ExtractToken reads session or API tokens from Authorization header, X-API-Key header, query parameters, or cookies
func ExtractToken(r *http.Request) string {
	if r == nil {
		return ""
	}

	// 1. Authorization: Bearer <token>
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	// 2. X-API-Key header
	apiKeyHeader := r.Header.Get("X-API-Key")
	if apiKeyHeader != "" {
		return strings.TrimSpace(apiKeyHeader)
	}

	// 3. Query param 'token' (e.g. browser SSE or WebSocket handshake)
	qToken := r.URL.Query().Get("token")
	if qToken != "" {
		return strings.TrimSpace(qToken)
	}

	// 4. Query param 'api_key'
	qApiKey := r.URL.Query().Get("api_key")
	if qApiKey != "" {
		return strings.TrimSpace(qApiKey)
	}

	// 5. aosphere_session Cookie
	cookie, err := r.Cookie("aosphere_session")
	if err == nil && cookie.Value != "" {
		return strings.TrimSpace(cookie.Value)
	}

	return ""
}
