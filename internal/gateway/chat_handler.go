package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"stox-gateway/internal/aws"
	"stox-gateway/internal/grpcclients"

	"go.uber.org/zap"
)

// ChatHandler handles chat requests to the agent
type ChatHandler struct {
	agentClient       *grpcclients.AgentClient
	authClient        *grpcclients.AuthClient
	s3Service         *aws.S3Service
	cloudFrontService *aws.CloudFrontService
	logger            *zap.Logger
}

// NewChatHandler creates a new chat handler
func NewChatHandler(agentClient *grpcclients.AgentClient, authClient *grpcclients.AuthClient, s3Service *aws.S3Service, cloudFrontService *aws.CloudFrontService, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{
		agentClient:       agentClient,
		authClient:        authClient,
		s3Service:         s3Service,
		cloudFrontService: cloudFrontService,
		logger:            logger,
	}
}

// ChatRequest represents the incoming chat request
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse represents the chat response
type ChatResponse struct {
	Success           bool     `json:"success"`
	Response          string   `json:"response"`
	IntermediateSteps []string `json:"intermediate_steps,omitempty"`
	Error             string   `json:"error,omitempty"`
}

// Chat handles chat messages
func (h *ChatHandler) Chat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify authentication
	token := r.Header.Get("Authorization")
	if token == "" {
		h.logger.Warn("Chat request without authorization token")
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
		h.logger.Warn("Invalid token for chat request", zap.Error(err))
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var chatReq ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
		h.logger.Error("Failed to decode chat request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if chatReq.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	h.logger.Info("Processing chat request",
		zap.String("user_id", validateResp.UserId),
		zap.String("message", chatReq.Message),
	)

	// Send to agent
	ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	agentResp, err := h.agentClient.Chat(ctx, chatReq.Message)
	if err != nil {
		h.logger.Error("Agent chat failed", zap.Error(err))
		response := ChatResponse{
			Success: false,
			Error:   "Agent service error: " + err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Prepare response
	response := ChatResponse{
		Success:           agentResp.Success,
		Response:          agentResp.Response,
		IntermediateSteps: agentResp.IntermediateSteps,
	}

	if !agentResp.Success {
		response.Error = agentResp.Error
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SEOAnalyzeRequest represents SEO analysis request
type SEOAnalyzeRequest struct {
	ImageURL string `json:"image_url"`
}

// SEOAnalyzeResponse represents SEO analysis response
type SEOAnalyzeResponse struct {
	Success           bool     `json:"success"`
	ImageURL          string   `json:"image_url"`
	Analysis          string   `json:"analysis"`
	IntermediateSteps []string `json:"intermediate_steps,omitempty"`
	Error             string   `json:"error,omitempty"`
}

// AnalyzeSEO handles SEO analysis requests with file upload
func (h *ChatHandler) AnalyzeSEO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify authentication
	token := r.Header.Get("Authorization")
	if token == "" {
		h.logger.Warn("SEO analysis request without authorization token")
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
		h.logger.Warn("Invalid token for SEO analysis request", zap.Error(err))
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

	// Get optional message
	message := r.FormValue("message")
	if message == "" {
		message = "Please analyze this image for SEO optimization"
	}

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

	// Generate unique filename
	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("seo_%s_%d%s", 
		validateResp.UserId, 
		time.Now().Unix(), 
		ext)

	h.logger.Info("Processing SEO analysis request",
		zap.String("user_id", validateResp.UserId),
		zap.String("filename", filename),
		zap.String("content_type", contentType),
		zap.Int("file_size", len(fileContent)),
	)

	// Upload original image to S3
	ctx, uploadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer uploadCancel()
	
	uploadResult, err := h.s3Service.UploadOriginalImage(ctx, validateResp.UserId, filename, strings.NewReader(string(fileContent)), contentType)
	if err != nil {
		h.logger.Error("Failed to upload image to S3", zap.Error(err))
		http.Error(w, "Failed to upload image", http.StatusInternalServerError)
		return
	}

	// Use the uploaded image URL
	imageURL := uploadResult.URL

	h.logger.Info("Successfully uploaded image to S3",
		zap.String("user_id", validateResp.UserId),
		zap.String("s3_key", uploadResult.Key),
		zap.String("image_url", imageURL),
	)

	// Send to agent for analysis with the S3 URL
	ctx, cancel = context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	agentResp, err := h.agentClient.AnalyzeSEO(ctx, imageURL)
	if err != nil {
		h.logger.Error("Agent SEO analysis failed", zap.Error(err))
		response := SEOAnalyzeResponse{
			Success: false,
			Error:   "Agent service error: " + err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Prepare response
	response := SEOAnalyzeResponse{
		Success:           agentResp.Success,
		ImageURL:          imageURL,
		Analysis:          agentResp.Analysis,
		IntermediateSteps: agentResp.IntermediateSteps,
	}

	if !agentResp.Success {
		response.Error = agentResp.Error
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
