package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// ProductHandler handles external product requests that are proxied to MockECommerce
type ProductHandler struct {
	mockECommerceClient *MockECommerceClient
	logger              *zap.Logger
}

// NewProductHandler creates a new product handler
func NewProductHandler(mockECommerceClient *MockECommerceClient, logger *zap.Logger) *ProductHandler {
	return &ProductHandler{
		mockECommerceClient: mockECommerceClient,
		logger:              logger,
	}
}

// GetProducts handles GET /api/v1/external/products
func (h *ProductHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	// Get validated API key from context (JWT token artık gerekli değil)
	apiKey, ok := GetAPIKeyFromContext(r.Context())
	if !ok {
		h.logger.Error("API key not found in context")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Processing products request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	// Fetch products from MockECommerce using only API key
	products, err := h.mockECommerceClient.GetProducts(apiKey)
	if err != nil {
		h.logger.Error("Failed to fetch products from MockECommerce", zap.Error(err))
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}

	// Return products as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(products); err != nil {
		h.logger.Error("Failed to encode products response", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully returned products",
		zap.Int("product_count", len(products.Data)),
	)
}

// GetProductByID handles GET /api/v1/external/products/{id}
func (h *ProductHandler) GetProductByID(w http.ResponseWriter, r *http.Request) {
	// Get validated API key from context (JWT token artık gerekli değil)
	apiKey, ok := GetAPIKeyFromContext(r.Context())
	if !ok {
		h.logger.Error("API key not found in context")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Extract product ID from URL
	vars := mux.Vars(r)
	productIDStr, exists := vars["id"]
	if !exists {
		http.Error(w, "Product ID is required", http.StatusBadRequest)
		return
	}

	h.logger.Info("Processing product by ID request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("product_id", productIDStr),
	)

	// Fetch product from MockECommerce using only API key
	product, err := h.mockECommerceClient.GetProductByID(apiKey, productIDStr)
	if err != nil {
		if err.Error() == "product not found" {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to fetch product from MockECommerce", zap.Error(err))
		http.Error(w, "Failed to fetch product", http.StatusInternalServerError)
		return
	}

	// Return product as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(product); err != nil {
		h.logger.Error("Failed to encode product response", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully returned product",
		zap.String("product_id", product.Data.ID),
		zap.String("product_name", product.Data.Name),
	)
}

// CreateProduct handles POST /api/v1/external/products
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	// Get validated API key from context (JWT token artık gerekli değil)
	apiKey, ok := GetAPIKeyFromContext(r.Context())
	if !ok {
		h.logger.Error("API key not found in context")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Processing create product request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	// Parse request body
	var productReq CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&productReq); err != nil {
		h.logger.Error("Failed to decode product request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create product via MockECommerce API using only API key
	product, err := h.mockECommerceClient.CreateProduct(apiKey, productReq)
	if err != nil {
		h.logger.Error("Failed to create product in MockECommerce", zap.Error(err))
		http.Error(w, "Failed to create product", http.StatusInternalServerError)
		return
	}

	// Return created product as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(product); err != nil {
		h.logger.Error("Failed to encode product response", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully created product",
		zap.String("product_id", product.Data.ID),
		zap.String("product_title", product.Data.Title),
	)
}

// UpdateProduct handles PUT /api/v1/external/products/{id}
func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	// Get validated API key from context (JWT token artık gerekli değil)
	apiKey, ok := GetAPIKeyFromContext(r.Context())
	if !ok {
		h.logger.Error("API key not found in context")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Processing update product request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	// Get product ID from URL path
	vars := mux.Vars(r)
	productID := vars["id"]
	
	if productID == "" {
		h.logger.Error("Product ID is required")
		http.Error(w, "Product ID is required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var productReq UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&productReq); err != nil {
		h.logger.Error("Failed to decode product update request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update product via MockECommerce API using only API key
	product, err := h.mockECommerceClient.UpdateProduct(apiKey, productID, productReq)
	if err != nil {
		h.logger.Error("Failed to update product in MockECommerce", zap.Error(err))
		http.Error(w, "Failed to update product", http.StatusInternalServerError)
		return
	}

	// Return updated product as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(product); err != nil {
		h.logger.Error("Failed to encode product response", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully updated product",
		zap.String("product_id", product.ID),
		zap.String("product_title", product.Title),
	)
}

// DeleteProduct handles DELETE /api/v1/external/products/{id}
func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	// Get validated API key from context (JWT token artık gerekli değil)
	apiKey, ok := GetAPIKeyFromContext(r.Context())
	if !ok {
		h.logger.Error("API key not found in context")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Processing delete product request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	// Get product ID from URL path
	vars := mux.Vars(r)
	productID := vars["id"]
	
	if productID == "" {
		h.logger.Error("Product ID is required")
		http.Error(w, "Product ID is required", http.StatusBadRequest)
		return
	}

	// Delete product via MockECommerce API using only API key
	result, err := h.mockECommerceClient.DeleteProduct(apiKey, productID)
	if err != nil {
		h.logger.Error("Failed to delete product from MockECommerce", zap.Error(err))
		http.Error(w, "Failed to delete product", http.StatusInternalServerError)
		return
	}

	// Return result as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("Failed to encode delete response", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully deleted product",
		zap.String("product_id", productID),
		zap.String("message", result.Message),
	)
}
