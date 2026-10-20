package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"aosphere/internal/auth"
	"aosphere/internal/store"
)

type authContextKey string

const AuthCtxKey authContextKey = "auth_context"

// GetAuthContext retrieves the authenticated identity from the HTTP context
func GetAuthContext(ctx context.Context) *auth.AuthContext {
	if val, ok := ctx.Value(AuthCtxKey).(*auth.AuthContext); ok {
		return val
	}
	return nil
}

// AuthenticateMiddleware attempts to identify the caller from session cookies, Bearer tokens, or API keys
func AuthenticateMiddleware(s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := auth.ExtractToken(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			tokenHash := auth.HashToken(token)

			// 1. Try API key lookup (for AI agents or service integrations)
			if strings.HasPrefix(token, "ao_sk_") {
				apiKey, err := s.GetAPIKeyByHash(tokenHash)
				if err == nil && apiKey != nil {
					role, _ := auth.ParseRole(apiKey.Role)
					authCtx := &auth.AuthContext{
						UserID:   apiKey.UserID,
						OrgID:    apiKey.OrgID,
						Email:    "agent@" + apiKey.Name,
						Name:     apiKey.Name,
						Role:     role,
						IsAPIKey: true,
					}
					ctx := context.WithValue(r.Context(), AuthCtxKey, authCtx)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// 2. Try User Session lookup (for human browser users)
			sess, user, err := s.GetSessionAndUser(tokenHash)
			if err == nil && user != nil && sess != nil {
				role, _ := auth.ParseRole(user.Role)
				authCtx := &auth.AuthContext{
					UserID:   user.ID,
					OrgID:    user.OrgID,
					Email:    user.Email,
					Name:     user.Name,
					Role:     role,
					IsAPIKey: false,
				}
				ctx := context.WithValue(r.Context(), AuthCtxKey, authCtx)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// If token was provided but invalid/expired, continue unauthenticated (guards will block)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth blocks unauthenticated requests with HTTP 401
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCtx := GetAuthContext(r.Context())
		if authCtx == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "unauthorized",
				"message": "Authentication required. Please provide a valid session token or API key.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePermission verifies the caller has the required granular permission
func RequirePermission(perm auth.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx := GetAuthContext(r.Context())
			if authCtx == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "unauthorized",
					"message": "Authentication required.",
				})
				return
			}

			if !authCtx.Can(perm) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":               "forbidden",
					"message":             "Access denied. Insufficient permissions.",
					"required_permission": string(perm),
					"your_role":           string(authCtx.Role),
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractToken reads session/API tokens from Headers or Cookies
func extractToken(r *http.Request) string {
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

	// 3. Query param token (used for browser SSE or WebSocket connections)
	qToken := r.URL.Query().Get("token")
	if qToken != "" {
		return strings.TrimSpace(qToken)
	}

	// 4. aosphere_session Cookie
	cookie, err := r.Cookie("aosphere_session")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return ""
}
