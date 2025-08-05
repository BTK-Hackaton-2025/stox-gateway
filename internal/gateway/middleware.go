package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"stox-gateway/internal/config"
	pb "stox-gateway/internal/proto/auth"

	"go.uber.org/zap"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
)

// UserIDKey returns the context key for user ID (exported for use in handlers)
func UserIDKey() contextKey {
	return userIDKey
}

// generateRequestID creates a random request ID
func generateRequestID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Return a fallback string if random number generation fails
		return fmt.Sprintf("fallback-request-id-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// LoggingMiddleware logs HTTP requests with correlation IDs
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Generate request ID for correlation
		requestID := generateRequestID()

		// Add request ID to context
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		r = r.WithContext(ctx)

		// Add request ID to response headers for client correlation
		w.Header().Set("X-Request-ID", requestID)

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Create request-scoped logger with correlation fields
		logger := zap.L().With(
			zap.String("request_id", requestID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)

		// Log request start
		logger.Debug("HTTP Request Started",
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
		)

		next.ServeHTTP(wrapped, r)

		// Log request completion
		logger.Info("HTTP Request Completed",
			zap.Int("status_code", wrapped.statusCode),
			zap.Duration("duration", time.Since(start)),
		)
	})
}

// CORSMiddleware handles CORS headers with configurable allowed origins
func CORSMiddleware(corsConfig *config.CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is in the allowed list
			allowedOrigin := ""
			for _, allowedOrig := range corsConfig.AllowedOrigins {
				if origin == allowedOrig {
					allowedOrigin = origin
					break
				}
			}

			// Set CORS headers only if origin is allowed
			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Set other CORS headers
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(corsConfig.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(corsConfig.AllowedHeaders, ", "))

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// AuthMiddleware validates JWT tokens and adds user context
func AuthMiddleware(authClient interface {
	ValidateToken(ctx context.Context, token string) (*pb.ValidateTokenResponse, error)
}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"success": false, "error": "Authorization header missing"}`, http.StatusUnauthorized)
				return
			}
			
			// Extract Bearer token
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
				http.Error(w, `{"success": false, "error": "Invalid authorization header format"}`, http.StatusUnauthorized)
				return
			}
			
			token := tokenParts[1]
			
			// Validate token
			validateResponse, err := authClient.ValidateToken(r.Context(), token)
			if err != nil {
				http.Error(w, `{"success": false, "error": "Token validation failed"}`, http.StatusUnauthorized)
				return
			}
			
			if !validateResponse.Valid {
				http.Error(w, `{"success": false, "error": "Invalid token"}`, http.StatusUnauthorized)
				return
			}
			
			// Add user ID to request context
			ctx := context.WithValue(r.Context(), userIDKey, validateResponse.UserId)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ValidateResponse represents token validation response  
type ValidateResponse struct {
	Valid  bool   `json:"valid"`
	UserId string `json:"userId"`
}
