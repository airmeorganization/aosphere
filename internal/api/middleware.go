package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/metrics"
	"github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

// statusResponseWriter wraps http.ResponseWriter to capture HTTP status and response size
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

// RequestIDMiddleware extracts or generates a unique Request ID (X-Request-ID)
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = "req_" + uuid.New().String()[:12]
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
		w.Header().Set("X-Request-ID", reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from the context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return "unknown"
}

// LoggingMiddleware logs every HTTP request with execution duration, status code, and request ID
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(sw, r)

			duration := time.Since(start)
			reqID := GetRequestID(r.Context())

			// Skip detailed logs for static assets or SSE loops if status is 200 to keep logs clean
			if r.URL.Path == "/healthz" && sw.statusCode == http.StatusOK {
				return
			}

			logger.InfoContext(r.Context(), "HTTP request",
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.statusCode),
				slog.String("remote_addr", r.RemoteAddr),
				slog.Duration("duration", duration),
				slog.Int("bytes", sw.size),
			)
		})
	}
}

// RecoveryMiddleware gracefully catches panics, logs stack traces, and returns HTTP 500
func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					reqID := GetRequestID(r.Context())
					logger.ErrorContext(r.Context(), "Unhandled panic recovered",
						slog.String("request_id", reqID),
						slog.Any("error", rec),
						slog.String("stack", string(debug.Stack())),
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, `{"error":"internal_server_error","request_id":"%s"}`+"\n", reqID)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// MetricsMiddleware records HTTP metrics to the default registry
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw, ok := w.(*statusResponseWriter)
		if !ok {
			sw = &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			w = sw
		}

		next.ServeHTTP(sw, r)

		duration := time.Since(start)
		metrics.Default().RecordHTTPRequest(r.Method, r.URL.Path, sw.statusCode, duration)
	})
}

// SecurityHeadersMiddleware sets strict production security headers
func SecurityHeadersMiddleware(isProd bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'")

			if isProd {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware restricts CORS headers to explicit allowlisted origins
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := auth.ApplyCORSHeaders(w, r)

		if origin != "" && !allowed {
			if r.Method == http.MethodOptions {
				http.Error(w, "CORS origin forbidden", http.StatusForbidden)
				return
			}
			// For non-OPTIONS requests with unauthorized origins, no CORS headers are granted
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Idempotency-Key, X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
