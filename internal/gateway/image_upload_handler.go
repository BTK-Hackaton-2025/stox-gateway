package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"stox-gateway/internal/aws"
	"stox-gateway/internal/grpcclients"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// ImageUploadHandler handles image upload operations with S3 integration
type ImageUploadHandler struct {
	s3Service       *aws.S3Service
	cloudFront      *aws.CloudFrontService
	imageClient     *grpcclients.ImageClient
	authClient      *grpcclients.AuthClient
	logger          *zap.Logger
	maxFileSize     int64  // Maximum file size in bytes (e.g., 10MB)
	allowedFormats  []string
}

// ImageUploadRequest represents the upload request structure
type ImageUploadRequest struct {
	UserID      string `json:"userId"`
	ProductName string `json:"productName,omitempty"`
}

// ImageUploadResponse represents the upload response
type ImageUploadResponse struct {
	Success        bool                    `json:"success"`
	Message        string                  `json:"message"`
	OriginalImage  *aws.ImageUploadResult  `json:"originalImage,omitempty"`
	EnhancedImage  *aws.ImageUploadResult  `json:"enhancedImage,omitempty"`
	CloudFrontURL  string                  `json:"cloudFrontUrl,omitempty"`
	EnhancedURL    string                  `json:"enhancedUrl,omitempty"`
	ProcessingID   string                  `json:"processingId,omitempty"`
}

// ImageProcessResponse represents the image processing response
type ImageProcessResponse struct {
	Success      bool                    `json:"success"`
	Message      string                  `json:"message"`
	OriginalURL  string                  `json:"originalUrl"`
	EnhancedURL  string                  `json:"enhancedUrl"`
	ProcessingID string                  `json:"processingId"`
	EnhancedImage *aws.ImageUploadResult `json:"enhancedImage,omitempty"`
}

// NewImageUploadHandler creates a new image upload handler
func NewImageUploadHandler(
	s3Service *aws.S3Service,
	cloudFront *aws.CloudFrontService,
	imageClient *grpcclients.ImageClient,
	authClient *grpcclients.AuthClient,
	logger *zap.Logger,
) *ImageUploadHandler {
	return &ImageUploadHandler{
		s3Service:      s3Service,
		cloudFront:     cloudFront,
		imageClient:    imageClient,
		authClient:     authClient,
		logger:         logger,
		maxFileSize:    10 * 1024 * 1024, // 10MB
		allowedFormats: []string{"image/jpeg", "image/jpg", "image/png", "image/webp"},
	}
}

// UploadImage handles the image upload process
// Flow: Frontend -> Gateway -> S3 (original) -> Response -> Gateway -> Image Service -> S3 (enhanced)
func (h *ImageUploadHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	h.logger.Info("Starting image upload process")
	
	// Extract and validate user from JWT token
	userID, err := h.extractUserIDFromToken(r)
	if err != nil {
		h.logger.Error("Failed to extract user ID from token", zap.Error(err))
		h.writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized: Invalid token")
		return
	}
	
	// Parse multipart form
	err = r.ParseMultipartForm(h.maxFileSize)
	if err != nil {
		h.logger.Error("Failed to parse multipart form", zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "Failed to parse upload form")
		return
	}
	
	// Get the file from the form
	file, fileHeader, err := r.FormFile("image")
	if err != nil {
		h.logger.Error("Failed to get image file from form", zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "No image file provided")
		return
	}
	defer file.Close() // Close file immediately after obtaining it
	
	// Validate file
	if err := h.validateFile(file, fileHeader); err != nil {
		h.logger.Error("File validation failed", zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	
	// Get product name from form (optional)
	productName := r.FormValue("productName")
	
	// Reset file pointer to beginning
	if _, err := file.Seek(0, 0); err != nil {
		h.logger.Error("Failed to reset file pointer", zap.Error(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to process uploaded file")
		return
	}
	
	// Upload original image to S3
	originalResult, err := h.s3Service.UploadOriginalImage(
		ctx,
		userID,
		fileHeader.Filename,
		file,
		fileHeader.Header.Get("Content-Type"),
	)
	if err != nil {
		h.logger.Error("Failed to upload original image to S3", zap.Error(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to upload image")
		return
	}
	
	h.logger.Info("Successfully uploaded original image to S3", 
		zap.String("userID", userID),
		zap.String("key", originalResult.Key),
	)
	
	// Generate CloudFront URL for the uploaded image
	cloudFrontURL := h.cloudFront.GetImageURL(originalResult.Key)

	// Process image enhancement synchronously
	h.logger.Info("Starting synchronous image enhancement", zap.String("userID", userID))
	enhancedResult, err := h.ProcessImageEnhancement(ctx, userID, originalResult, productName)
	
	var enhancedCloudFrontURL string
	if err != nil {
		h.logger.Error("Failed to process image enhancement", zap.Error(err))
		// Continue without enhanced image - user still has original
	} else {
		enhancedCloudFrontURL = enhancedResult.EnhancedURL
		h.logger.Info("Successfully created enhanced image", 
			zap.String("userID", userID),
			zap.String("enhancedURL", enhancedCloudFrontURL),
		)
	}

	// Prepare response
	response := ImageUploadResponse{
		Success:       true,
		OriginalImage: originalResult,
		CloudFrontURL: cloudFrontURL,
	}

	if enhancedResult != nil {
		response.Message = "Image uploaded and enhanced successfully"
		response.EnhancedImage = enhancedResult.EnhancedImage
		response.EnhancedURL = enhancedCloudFrontURL
	} else {
		response.Message = "Image uploaded successfully. Enhancement failed - please try again"
	}
	
	h.writeJSONResponse(w, http.StatusOK, response)
}

// ProcessImageEnhancement handles the image enhancement process
func (h *ImageUploadHandler) ProcessImageEnhancement(ctx context.Context, userID string, originalResult *aws.ImageUploadResult, productName string) (*ImageProcessResponse, error) {
	h.logger.Info("Starting image enhancement process", 
		zap.String("userID", userID),
		zap.String("originalKey", originalResult.Key),
	)
	
	// Download the original image from S3 for processing
	imageData, err := h.s3Service.DownloadImage(ctx, originalResult.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to download original image: %w", err)
	}
	
	// Call image service for enhancement
	processResponse, err := h.imageClient.ProcessImage(ctx, imageData, originalResult.ContentType, productName)
	if err != nil {
		return nil, fmt.Errorf("failed to process image: %w", err)
	}
	
	// Upload enhanced image to S3
	enhancedFileName := fmt.Sprintf("enhanced_%s", originalResult.FileName)
	enhancedResult, err := h.s3Service.UploadEnhancedImage(
		ctx,
		userID,
		enhancedFileName,
		bytes.NewReader(processResponse.ProcessedImageData),
		processResponse.MimeType,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upload enhanced image: %w", err)
	}
	
	h.logger.Info("Successfully processed and uploaded enhanced image", 
		zap.String("userID", userID),
		zap.String("enhancedKey", enhancedResult.Key),
	)
	
	// Generate CloudFront URLs
	originalURL := h.cloudFront.GetImageURL(originalResult.Key)
	enhancedURL := h.cloudFront.GetImageURL(enhancedResult.Key)
	
	return &ImageProcessResponse{
		Success:       true,
		Message:       "Image successfully enhanced",
		OriginalURL:   originalURL,
		EnhancedURL:   enhancedURL,
		ProcessingID:  fmt.Sprintf("proc_%d", time.Now().Unix()),
		EnhancedImage: enhancedResult,
	}, nil
}

// validateFile validates the uploaded file
func (h *ImageUploadHandler) validateFile(file multipart.File, header *multipart.FileHeader) error {
	// Check file size
	if header.Size > h.maxFileSize {
		return fmt.Errorf("file size exceeds maximum allowed size of %d bytes", h.maxFileSize)
	}
	
	// Check file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	validExts := []string{".jpg", ".jpeg", ".png", ".webp"}
	isValidExt := false
	for _, validExt := range validExts {
		if ext == validExt {
			isValidExt = true
			break
		}
	}
	if !isValidExt {
		return fmt.Errorf("invalid file extension. Allowed: %v", validExts)
	}
	
	// Check MIME type
	contentType := header.Header.Get("Content-Type")
	isValidMIME := false
	for _, allowedFormat := range h.allowedFormats {
		if contentType == allowedFormat {
			isValidMIME = true
			break
		}
	}
	if !isValidMIME {
		return fmt.Errorf("invalid MIME type. Allowed: %v", h.allowedFormats)
	}
	
	// Additional file content validation could go here
	// (e.g., reading file header to verify it's actually an image)
	
	return nil
}

// extractUserIDFromToken extracts user ID from JWT token
func (h *ImageUploadHandler) extractUserIDFromToken(r *http.Request) (string, error) {
	// Try to get user ID from context first (set by middleware)
	if userID, ok := r.Context().Value(UserIDKey()).(string); ok && userID != "" {
		return userID, nil
	}
	
	// Fallback to manual token validation for backward compatibility
	// Get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("authorization header missing")
	}
	
	// Extract Bearer token
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		return "", fmt.Errorf("invalid authorization header format")
	}
	
	token := tokenParts[1]
	
	// Validate token with auth service
	validateResponse, err := h.authClient.ValidateToken(r.Context(), token)
	if err != nil {
		return "", fmt.Errorf("token validation failed: %w", err)
	}
	
	if !validateResponse.Valid {
		return "", fmt.Errorf("invalid token")
	}
	
	return validateResponse.UserId, nil
}

// GetUserImages returns all images for a specific user
func (h *ImageUploadHandler) GetUserImages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract user ID from token
	userID, err := h.extractUserIDFromToken(r)
	if err != nil {
		h.logger.Error("Failed to extract user ID from token", zap.Error(err))
		h.writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized: Invalid token")
		return
	}
	
	// List user images from S3
	imageKeys, err := h.s3Service.ListUserImages(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to list user images", zap.Error(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve images")
		return
	}
	
	// Generate CloudFront URLs
	var imageURLs []map[string]string
	for _, key := range imageKeys {
		imageURLs = append(imageURLs, map[string]string{
			"key": key,
			"url": h.cloudFront.GetImageURL(key),
		})
	}
	
	response := map[string]interface{}{
		"success": true,
		"images":  imageURLs,
		"count":   len(imageURLs),
	}
	
	h.writeJSONResponse(w, http.StatusOK, response)
}

// DeleteUserImage deletes a specific image for a user
func (h *ImageUploadHandler) DeleteUserImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract user ID from token
	userID, err := h.extractUserIDFromToken(r)
	if err != nil {
		h.logger.Error("Failed to extract user ID from token", zap.Error(err))
		h.writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized: Invalid token")
		return
	}
	
	// Get image ID from URL path parameter
	vars := mux.Vars(r)
	imageId := vars["imageId"]
	if imageId == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Image ID is required")
		return
	}
	
	// Validate user access to the image (assuming imageId is the key)
	if !h.s3Service.ValidateUserAccess(userID, imageId) {
		h.logger.Warn("User attempted to access unauthorized image", 
			zap.String("userID", userID),
			zap.String("imageId", imageId),
		)
		h.writeErrorResponse(w, http.StatusForbidden, "Access denied")
		return
	}
	
	// Delete image from S3
	err = h.s3Service.DeleteImage(ctx, imageId)
	if err != nil {
		h.logger.Error("Failed to delete image from S3", zap.Error(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete image")
		return
	}
	
	// Invalidate CloudFront cache
	go func() {
		_, err := h.cloudFront.InvalidateCache(context.Background(), []string{imageId})
		if err != nil {
			h.logger.Error("Failed to invalidate CloudFront cache", zap.Error(err))
		}
	}()
	
	response := map[string]interface{}{
		"success": true,
		"message": "Image deleted successfully",
	}
	
	h.writeJSONResponse(w, http.StatusOK, response)
}

// Helper methods

func (h *ImageUploadHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
	}
}

func (h *ImageUploadHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := map[string]interface{}{
		"success": false,
		"error":   message,
	}
	h.writeJSONResponse(w, statusCode, response)
}
