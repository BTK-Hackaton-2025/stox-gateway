package ecommerce

import (
	"encoding/json"
	"net/http"
	"strconv"

	"stox-gateway/internal/config"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// ECommerceHandler handles e-commerce related routes
type ECommerceHandler struct {
	client *MockECommerceClient
	logger *zap.Logger
}

// NewECommerceHandler creates a new e-commerce handler
func NewECommerceHandler(cfg *config.Config, logger *zap.Logger) *ECommerceHandler {
	client := NewMockECommerceClient(cfg.ECommerce.BaseURL, cfg.ECommerce.APIKey)
	
	return &ECommerceHandler{
		client: client,
		logger: logger,
	}
}

// RegisterRoutes registers e-commerce routes
func (h *ECommerceHandler) RegisterRoutes(router *mux.Router) {
	// Product management routes
	router.HandleFunc("/ecommerce/products", h.GetProducts).Methods("GET")
	router.HandleFunc("/ecommerce/products/{id}", h.GetProduct).Methods("GET")
	router.HandleFunc("/ecommerce/products", h.CreateProduct).Methods("POST")
	router.HandleFunc("/ecommerce/products/{id}", h.UpdateProduct).Methods("PUT")
	router.HandleFunc("/ecommerce/products/{id}", h.DeleteProduct).Methods("DELETE")
	router.HandleFunc("/ecommerce/products/statistics", h.GetProductStatistics).Methods("GET")

	// Order management routes
	router.HandleFunc("/ecommerce/orders", h.GetOrders).Methods("GET")
}

// GetProducts retrieves seller's products with pagination
func (h *ECommerceHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	page := 1
	pageSize := 20

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("pageSize"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	h.logger.Info("Getting seller products", 
		zap.Int("page", page),
		zap.Int("pageSize", pageSize),
	)

	products, totalCount, err := h.client.GetSellerProducts(page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get seller products", zap.Error(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve products", err.Error())
		return
	}

	totalPages := (totalCount + pageSize - 1) / pageSize

	response := map[string]interface{}{
		"success": true,
		"data":    products,
		"pagination": map[string]interface{}{
			"page":       page,
			"pageSize":   pageSize,
			"totalCount": totalCount,
			"totalPages": totalPages,
		},
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// GetProduct retrieves a specific product by ID
func (h *ECommerceHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]
	if productID == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Product ID is required", "")
		return
	}

	h.logger.Info("Getting product by ID", zap.String("productId", productID))

	product, err := h.client.GetProductByID(productID)
	if err != nil {
		h.logger.Error("Failed to get product", zap.Error(err), zap.String("productId", productID))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve product", err.Error())
		return
	}

	if product == nil {
		h.writeErrorResponse(w, http.StatusNotFound, "Product not found", "")
		return
	}

	response := map[string]interface{}{
		"success": true,
		"data":    product,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// CreateProduct creates a new product
func (h *ECommerceHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request format", err.Error())
		return
	}

	// Validate required fields
	if req.Title == "" || req.Price <= 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "Title and valid price are required", "")
		return
	}

	h.logger.Info("Creating new product", 
		zap.String("title", req.Title),
		zap.Float64("price", req.Price),
	)

	product, err := h.client.CreateProduct(req)
	if err != nil {
		h.logger.Error("Failed to create product", zap.Error(err), zap.String("title", req.Title))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to create product", err.Error())
		return
	}

	response := map[string]interface{}{
		"success": true,
		"data":    product,
		"message": "Product created successfully",
	}

	h.writeJSONResponse(w, http.StatusCreated, response)
}

// UpdateProduct updates an existing product
func (h *ECommerceHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]
	if productID == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Product ID is required", "")
		return
	}

	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request format", err.Error())
		return
	}

	// Validate required fields
	if req.Title == "" || req.Price <= 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "Title and valid price are required", "")
		return
	}

	h.logger.Info("Updating product",
		zap.String("productId", productID),
		zap.String("title", req.Title),
	)

	product, err := h.client.UpdateProduct(productID, req)
	if err != nil {
		h.logger.Error("Failed to update product", zap.Error(err), zap.String("productId", productID))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to update product", err.Error())
		return
	}

	response := map[string]interface{}{
		"success": true,
		"data":    product,
		"message": "Product updated successfully",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// DeleteProduct deletes a product
func (h *ECommerceHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]
	if productID == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Product ID is required", "")
		return
	}

	h.logger.Info("Deleting product", zap.String("productId", productID))

	err := h.client.DeleteProduct(productID)
	if err != nil {
		h.logger.Error("Failed to delete product", zap.Error(err), zap.String("productId", productID))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete product", err.Error())
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Product deleted successfully",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// GetProductStatistics retrieves seller's product statistics
func (h *ECommerceHandler) GetProductStatistics(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Getting product statistics")

	stats, err := h.client.GetProductStatistics()
	if err != nil {
		h.logger.Error("Failed to get product statistics", zap.Error(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve product statistics", err.Error())
		return
	}

	response := map[string]interface{}{
		"success": true,
		"data":    stats,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// GetOrders retrieves seller's orders
func (h *ECommerceHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	// Get seller ID from context (set by auth middleware)
	userID := r.Context().Value("userID")
	if userID == nil {
		h.writeErrorResponse(w, http.StatusUnauthorized, "User not authenticated", "")
		return
	}

	sellerID, ok := userID.(string)
	if !ok {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Invalid user ID format", "")
		return
	}

	h.logger.Info("Getting seller orders", zap.String("sellerId", sellerID))

	orders, err := h.client.GetSellerOrders(sellerID)
	if err != nil {
		h.logger.Error("Failed to get seller orders", zap.Error(err), zap.String("sellerId", sellerID))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve orders", err.Error())
		return
	}

	response := map[string]interface{}{
		"success": true,
		"data":    orders,
		"count":   len(orders),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// Helper methods for consistent response formatting
func (h *ECommerceHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
	}
}

func (h *ECommerceHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message, details string) {
	response := map[string]interface{}{
		"error": message,
	}
	
	if details != "" {
		response["details"] = details
	}

	h.writeJSONResponse(w, statusCode, response)
}
