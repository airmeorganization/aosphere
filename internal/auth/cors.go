package auth

import (
	"net/http"
	"os"
	"strings"
	"sync"
)

var (
	defaultAllowedOrigins = []string{
		"https://app.aosphere.io",
		"https://aosphere.io",
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
	}
	allowedOriginsMu sync.RWMutex
	customOrigins    []string
)

// SetAllowedOrigins sets custom allowed CORS origins (useful for configuration or tests).
func SetAllowedOrigins(origins []string) {
	allowedOriginsMu.Lock()
	defer allowedOriginsMu.Unlock()
	customOrigins = origins
}

// ResetAllowedOrigins resets origins to default / environment configuration.
func ResetAllowedOrigins() {
	allowedOriginsMu.Lock()
	defer allowedOriginsMu.Unlock()
	customOrigins = nil
}

// GetAllowedOrigins returns the currently configured allowed origins.
func GetAllowedOrigins() []string {
	allowedOriginsMu.RLock()
	defer allowedOriginsMu.RUnlock()
	if customOrigins != nil {
		return customOrigins
	}
	if env := os.Getenv("AOSPHERE_ALLOWED_ORIGINS"); env != "" {
		parts := strings.Split(env, ",")
		var cleaned []string
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				cleaned = append(cleaned, trimmed)
			}
		}
		if len(cleaned) > 0 {
			return cleaned
		}
	}
	return defaultAllowedOrigins
}

// IsOriginAllowed checks whether the requested Origin matches an explicit allowlist entry.
func IsOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	allowed := GetAllowedOrigins()
	normOrigin := strings.ToLower(strings.TrimRight(origin, "/"))
	for _, a := range allowed {
		if strings.ToLower(strings.TrimRight(a, "/")) == normOrigin {
			return true
		}
	}
	return false
}

// ApplyCORSHeaders evaluates request origin against the allowlist.
// If valid, sets Access-Control-Allow-Origin, Access-Control-Allow-Credentials, and Vary: Origin.
// Returns true if origin was valid or empty, false if an untrusted origin was provided.
func ApplyCORSHeaders(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Direct HTTP or same-origin request
		return true
	}

	if IsOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Vary", "Origin")
		return true
	}

	// Origin header present but NOT in allowlist
	w.Header().Set("Vary", "Origin")
	return false
}
