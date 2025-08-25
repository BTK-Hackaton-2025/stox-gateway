package gateway

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// APIKeyMiddleware creates a middleware for validating API keys with MockECommerce
type APIKeyMiddleware struct {
	mockECommerceClient *MockECommerceClient
	logger              *zap.Logger
}

// NewAPIKeyMiddleware creates a new API key middleware
func NewAPIKeyMiddleware(mockECommerceClient *MockECommerceClient, logger *zap.Logger) *APIKeyMiddleware {
	return &APIKeyMiddleware{
		mockECommerceClient: mockECommerceClient,
		logger:              logger,
	}
}

// ValidateJWTAndAPIKey is a middleware that validates both JWT and API keys for external product endpoints
func (m *APIKeyMiddleware) ValidateJWTAndAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract and validate JWT token first
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			m.logger.Warn("JWT token missing in request")
			http.Error(w, "JWT Bearer token required", http.StatusUnauthorized)
			return
		}

		jwtToken := strings.TrimPrefix(authHeader, "Bearer ")

		// 2. Extract API key from X-API-Key header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			m.logger.Warn("API key missing in request")
			http.Error(w, "X-API-Key header required", http.StatusUnauthorized)
			return
		}

		// 3. Store both in request context for later use
		ctx := context.WithValue(r.Context(), "jwt_token", jwtToken)
		ctx = context.WithValue(ctx, "api_key", apiKey)
		r = r.WithContext(ctx)

		m.logger.Info("JWT and API key extracted successfully",
			zap.String("jwt_prefix", jwtToken[:min(20, len(jwtToken))]),
			zap.String("api_key_prefix", apiKey[:min(8, len(apiKey))]),
		)

		// Call the next handler
		next.ServeHTTP(w, r)
	}
}

// GetAPIKeyFromContext retrieves the validated API key from request context
func GetAPIKeyFromContext(ctx context.Context) (string, bool) {
	apiKey, ok := ctx.Value("api_key").(string)
	return apiKey, ok
}

// GetJWTTokenFromContext retrieves the JWT token from request context
func GetJWTTokenFromContext(ctx context.Context) (string, bool) {
	jwtToken, ok := ctx.Value("jwt_token").(string)
	return jwtToken, ok
}
