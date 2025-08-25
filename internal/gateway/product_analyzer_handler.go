package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"stox-gateway/internal/grpcclients"

	"go.uber.org/zap"
)

// ProductAnalyzerHandler handles product analysis requests
type ProductAnalyzerHandler struct {
	productAnalyzerClient *grpcclients.ProductAnalyzerClient
	authClient            *grpcclients.AuthClient
	logger                *zap.Logger
}

// NewProductAnalyzerHandler creates a new product analyzer handler
func NewProductAnalyzerHandler(
	productAnalyzerClient *grpcclients.ProductAnalyzerClient,
	authClient *grpcclients.AuthClient,
	logger *zap.Logger,
) *ProductAnalyzerHandler {
	return &ProductAnalyzerHandler{
		productAnalyzerClient: productAnalyzerClient,
		authClient:            authClient,
		logger:                logger,
	}
}

// ProductAnalysisRequest represents the analysis request for URL-based analysis
type ProductAnalysisRequest struct {
	ImageURL string `json:"image_url"`
}

// ProductAnalysisResponse represents the analysis response
type ProductAnalysisResponse struct {
	Success     bool   `json:"success"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	SearchInfo  string `json:"search_info,omitempty"`
	Error       string `json:"error,omitempty"`
}

// AnalyzeProductImage handles product image analysis with file upload
func (h *ProductAnalyzerHandler) AnalyzeProductImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify authentication
	token := r.Header.Get("Authorization")
	if token == "" {
		h.logger.Warn("Product analysis request without authorization token")
		http.Error(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	// Remove "Bearer " prefix if present
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// Verify token with auth service
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	validateResp, err := h.authClient.ValidateToken(ctx, token)
	if err != nil || !validateResp.Valid {
		h.logger.Warn("Invalid token for product analysis request", zap.Error(err))
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Parse multipart form
	err = r.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		h.logger.Error("Failed to parse multipart form", zap.Error(err))
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get the uploaded file
	file, fileHeader, err := r.FormFile("image")
	if err != nil {
		h.logger.Error("Failed to get uploaded file", zap.Error(err))
		http.Error(w, "No image file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file type
	contentType := fileHeader.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		h.logger.Warn("Invalid file type uploaded", zap.String("content_type", contentType))
		http.Error(w, "Only image files are allowed", http.StatusBadRequest)
		return
	}

	// Read file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		h.logger.Error("Failed to read file content", zap.Error(err))
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Validate file size (10MB max)
	if len(fileContent) > 10*1024*1024 {
		h.logger.Warn("File too large", zap.Int("size", len(fileContent)))
		http.Error(w, "File too large. Maximum size is 10MB", http.StatusBadRequest)
		return
	}

	h.logger.Info("Processing product analysis request",
		zap.String("user_id", validateResp.UserId),
		zap.String("filename", fileHeader.Filename),
		zap.String("content_type", contentType),
		zap.Int("file_size", len(fileContent)),
	)

	// Send to product analyzer service
	ctx, cancel = context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	analyzerResp, err := h.productAnalyzerClient.GenerateFromImage(
		ctx,
		fileContent,
		fileHeader.Filename,
		contentType,
	)
	if err != nil {
		h.logger.Error("Product analyzer service failed", zap.Error(err))
		response := ProductAnalysisResponse{
			Success: false,
			Error:   "Product analyzer service error: " + err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Prepare response
	response := ProductAnalysisResponse{
		Success:     true,
		Title:       analyzerResp.Title,
		Description: analyzerResp.Description,
		SearchInfo:  analyzerResp.SearchInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AnalyzeProductImageFromURL handles product image analysis from URL
func (h *ProductAnalyzerHandler) AnalyzeProductImageFromURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify authentication
	token := r.Header.Get("Authorization")
	if token == "" {
		h.logger.Warn("Product analysis request without authorization token")
		http.Error(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	validateResp, err := h.authClient.ValidateToken(ctx, token)
	if err != nil || !validateResp.Valid {
		h.logger.Warn("Invalid token for product analysis request", zap.Error(err))
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var analysisReq ProductAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&analysisReq); err != nil {
		h.logger.Error("Failed to decode analysis request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if analysisReq.ImageURL == "" {
		http.Error(w, "Image URL is required", http.StatusBadRequest)
		return
	}

	h.logger.Info("Processing product analysis request from URL",
		zap.String("user_id", validateResp.UserId),
		zap.String("image_url", analysisReq.ImageURL),
	)

	// Send to product analyzer service
	ctx, cancel = context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	analyzerResp, err := h.productAnalyzerClient.GenerateFromImageUrl(ctx, analysisReq.ImageURL)
	if err != nil {
		h.logger.Error("Product analyzer service failed", zap.Error(err))
		response := ProductAnalysisResponse{
			Success: false,
			Error:   "Product analyzer service error: " + err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Prepare response
	response := ProductAnalysisResponse{
		Success:     true,
		Title:       analyzerResp.Title,
		Description: analyzerResp.Description,
		SearchInfo:  analyzerResp.SearchInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
