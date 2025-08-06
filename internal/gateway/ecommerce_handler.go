package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// ECommerceHandler handles MockECommerce integration
type ECommerceHandler struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewECommerceHandler creates a new e-commerce handler
func NewECommerceHandler(baseURL string, logger *zap.Logger) *ECommerceHandler {
	return &ECommerceHandler{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// RegisterECommerceRoutes registers e-commerce routes to existing router
func (h *ECommerceHandler) RegisterECommerceRoutes(router *mux.Router, authHandler *AuthHandler) {
	// E-commerce subrouter with API key authentication (simpler than JWT for external access)
	ecommerce := router.PathPrefix("/api/v1/ecommerce").Subrouter()
	ecommerce.Use(h.APIKeyMiddleware)

	// Product management
	ecommerce.HandleFunc("/products", h.GetProducts).Methods("GET")
	ecommerce.HandleFunc("/products", h.CreateProduct).Methods("POST")
	ecommerce.HandleFunc("/products/{id}", h.GetProduct).Methods("GET")
	ecommerce.HandleFunc("/products/{id}", h.UpdateProduct).Methods("PUT")
	ecommerce.HandleFunc("/products/{id}", h.DeleteProduct).Methods("DELETE")
	ecommerce.HandleFunc("/products/statistics", h.GetProductStatistics).Methods("GET")

	// Order management
	ecommerce.HandleFunc("/orders", h.GetOrders).Methods("GET")

	h.logger.Info("E-commerce routes registered successfully")
}

// APIKeyMiddleware validates API key from X-API-Key header
func (h *ECommerceHandler) APIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, `{"success": false, "error": "API key required"}`, http.StatusUnauthorized)
			return
		}

		valid, _, err := h.ValidateAPIKey(apiKey)
		if err != nil {
			h.logger.Error("API key validation error", zap.Error(err))
			http.Error(w, `{"success": false, "error": "API key validation failed"}`, http.StatusInternalServerError)
			return
		}

		if !valid {
			http.Error(w, `{"success": false, "error": "Invalid API key"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ValidateAPIKey validates the API key - since MockECommerce doesn't have validation endpoint, use fallback
func (h *ECommerceHandler) ValidateAPIKey(apiKey string) (bool, string, error) {
	// MockECommerce doesn't have a validation endpoint, so we use fallback validation
	// In production, you would validate against your own API key database
	return h.fallbackAPIKeyValidation(apiKey), "mock-seller", nil
}

// fallbackAPIKeyValidation provides simple API key validation when MockECommerce is unavailable
func (h *ECommerceHandler) fallbackAPIKeyValidation(apiKey string) bool {
	// Real API keys from MockECommerce
	validKeys := []string{
		"mec_Mcw2eIpGmLyShgFNbT9w4rHFHzQLHzXLePkm9gglMx8", // STOX Gateway Integration key
		"test-api-key",        // Development fallback
		"development-key",     // Development fallback
	}
	
	for _, validKey := range validKeys {
		if apiKey == validKey {
			return true
		}
	}
	
	// For development, also accept MockECommerce format keys that start with "mec_"
	if len(apiKey) >= 4 && apiKey[:4] == "mec_" {
		return true
	}
	
	return false
}

// ProxyToMockECommerce proxies request to MockECommerce with API key
func (h *ECommerceHandler) ProxyToMockECommerce(w http.ResponseWriter, r *http.Request, endpoint, apiKey string) {
	// Create request to MockECommerce
	var reqBody io.Reader
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			h.writeErrorResponse(w, http.StatusBadRequest, "Failed to read request body")
			return
		}
		reqBody = bytes.NewBuffer(bodyBytes)
	}

	req, err := http.NewRequest(r.Method, h.baseURL+endpoint, reqBody)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to create request")
		return
	}

	// Copy headers and add API key
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("X-API-Key", apiKey)

	// Make request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.logger.Error("MockECommerce request failed", zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadGateway, "External service unavailable")
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Copy status code and body
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// GetProducts handles GET /products
func (h *ECommerceHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	apiKey := h.getAPIKeyFromUser(r)
	if apiKey == "" {
		h.writeErrorResponse(w, http.StatusUnauthorized, "API key required")
		return
	}

	endpoint := "/api/v1/external/products"
	if r.URL.RawQuery != "" {
		endpoint += "?" + r.URL.RawQuery
	}

	h.ProxyToMockECommerce(w, r, endpoint, apiKey)
}

// CreateProduct handles POST /products
func (h *ECommerceHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	apiKey := h.getAPIKeyFromUser(r)
	if apiKey == "" {
		h.writeErrorResponse(w, http.StatusUnauthorized, "API key required")
		return
	}

	h.ProxyToMockECommerce(w, r, "/api/v1/external/products", apiKey)
}

// GetProduct handles GET /products/{id}
func (h *ECommerceHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	apiKey := h.getAPIKeyFromUser(r)
	if apiKey == "" {
		h.writeErrorResponse(w, http.StatusUnauthorized, "API key required")
		return
	}

	h.ProxyToMockECommerce(w, r, "/api/v1/external/products/"+productID, apiKey)
}

// UpdateProduct handles PUT /products/{id}
func (h *ECommerceHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	apiKey := h.getAPIKeyFromUser(r)
	if apiKey == "" {
		h.writeErrorResponse(w, http.StatusUnauthorized, "API key required")
		return
	}

	h.ProxyToMockECommerce(w, r, "/api/v1/external/products/"+productID, apiKey)
}

// DeleteProduct handles DELETE /products/{id}
func (h *ECommerceHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	apiKey := h.getAPIKeyFromUser(r)
	if apiKey == "" {
		h.writeErrorResponse(w, http.StatusUnauthorized, "API key required")
		return
	}

	h.ProxyToMockECommerce(w, r, "/api/v1/external/products/"+productID, apiKey)
}

// GetProductStatistics handles GET /products/statistics
func (h *ECommerceHandler) GetProductStatistics(w http.ResponseWriter, r *http.Request) {
	apiKey := h.getAPIKeyFromUser(r)
	if apiKey == "" {
		h.writeErrorResponse(w, http.StatusUnauthorized, "API key required")
		return
	}

	h.ProxyToMockECommerce(w, r, "/api/v1/external/products/statistics", apiKey)
}

// GetOrders handles GET /orders
func (h *ECommerceHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	apiKey := h.getAPIKeyFromUser(r)
	if apiKey == "" {
		h.writeErrorResponse(w, http.StatusUnauthorized, "API key required")
		return
	}

	// For MockECommerce, we'll use the seller orders endpoint
	// Since we don't have user authentication, we'll use a default seller endpoint
	endpoint := "/api/v1/order/my-orders"
	
	h.ProxyToMockECommerce(w, r, endpoint, apiKey)
}

// getAPIKeyFromUser gets API key from user context or header
func (h *ECommerceHandler) getAPIKeyFromUser(r *http.Request) string {
	// First try to get from X-API-Key header
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return apiKey
	}

	// For now, return a test API key - in production this would come from user profile
	return "test-api-key-for-development"
}

// Helper methods
func (h *ECommerceHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := map[string]string{
		"error": message,
	}
	json.NewEncoder(w).Encode(response)
}
