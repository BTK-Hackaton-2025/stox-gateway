package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"stox-gateway/internal/grpcclients"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	authClient *grpcclients.AuthClient
}

// ImageHandler handles image processing-related HTTP requests
type ImageHandler struct {
	imageClient *grpcclients.ImageClient
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authClient *grpcclients.AuthClient) *AuthHandler {
	return &AuthHandler{
		authClient: authClient,
	}
}

// GetAuthClient returns the auth client for middleware use
func (h *AuthHandler) GetAuthClient() *grpcclients.AuthClient {
	return h.authClient
}

// NewImageHandler creates a new image handler
func NewImageHandler(imageClient *grpcclients.ImageClient) *ImageHandler {
	return &ImageHandler{
		imageClient: imageClient,
	}
}

// RegisterRequest represents the JSON request for user registration
type RegisterRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Role      string `json:"role,omitempty"`
}

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// validateRequired checks if required fields are present and not empty
func validateRequired(req *RegisterRequest) []ValidationError {
	var errors []ValidationError

	if strings.TrimSpace(req.Email) == "" {
		errors = append(errors, ValidationError{Field: "email", Message: "Email is required"})
	}
	if strings.TrimSpace(req.Password) == "" {
		errors = append(errors, ValidationError{Field: "password", Message: "Password is required"})
	}
	if strings.TrimSpace(req.FirstName) == "" {
		errors = append(errors, ValidationError{Field: "firstName", Message: "First name is required"})
	}
	if strings.TrimSpace(req.LastName) == "" {
		errors = append(errors, ValidationError{Field: "lastName", Message: "Last name is required"})
	}

	return errors
}

// validateEmail checks if the email format is valid
func validateEmail(email string) *ValidationError {
	if !emailRegex.MatchString(strings.TrimSpace(email)) {
		return &ValidationError{Field: "email", Message: "Invalid email format"}
	}
	return nil
}

// validatePasswordStrength checks if password meets strength requirements
func validatePasswordStrength(password string) *ValidationError {
	if len(password) < 8 {
		return &ValidationError{Field: "password", Message: "Password must be at least 8 characters long"}
	}

	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`\d`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return &ValidationError{
			Field:   "password",
			Message: "Password must contain at least one uppercase letter, one lowercase letter, one digit, and one special character",
		}
	}

	return nil
}

// validateRegisterRequest validates the entire register request
func validateRegisterRequest(req *RegisterRequest) []ValidationError {
	var allErrors []ValidationError

	// Check required fields
	requiredErrors := validateRequired(req)
	allErrors = append(allErrors, requiredErrors...)

	// If email is present, validate format
	if strings.TrimSpace(req.Email) != "" {
		if emailError := validateEmail(req.Email); emailError != nil {
			allErrors = append(allErrors, *emailError)
		}
	}

	// If password is present, validate strength
	if strings.TrimSpace(req.Password) != "" {
		if passwordError := validatePasswordStrength(req.Password); passwordError != nil {
			allErrors = append(allErrors, *passwordError)
		}
	}

	return allErrors
}

// mapGRPCError maps gRPC error codes to appropriate HTTP status codes
func mapGRPCError(err error) (int, string) {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.InvalidArgument:
			return http.StatusBadRequest, st.Message()
		case codes.AlreadyExists:
			return http.StatusConflict, st.Message()
		case codes.NotFound:
			return http.StatusNotFound, st.Message()
		case codes.Unauthenticated:
			return http.StatusUnauthorized, st.Message()
		case codes.PermissionDenied:
			return http.StatusForbidden, st.Message()
		case codes.ResourceExhausted:
			return http.StatusTooManyRequests, st.Message()
		case codes.FailedPrecondition:
			return http.StatusBadRequest, st.Message()
		case codes.Unimplemented:
			return http.StatusNotImplemented, st.Message()
		case codes.Unavailable:
			return http.StatusServiceUnavailable, st.Message()
		case codes.DeadlineExceeded:
			return http.StatusRequestTimeout, st.Message()
		default:
			// For codes like Internal, Unknown, etc., return 500
			return http.StatusInternalServerError, st.Message()
		}
	}
	// Fallback for non-gRPC errors
	return http.StatusInternalServerError, err.Error()
}

// validateLoginRequired checks if required fields are present and not empty for login
func validateLoginRequired(req *LoginRequest) []ValidationError {
	var errors []ValidationError

	if strings.TrimSpace(req.Email) == "" {
		errors = append(errors, ValidationError{Field: "email", Message: "Email is required"})
	}
	if strings.TrimSpace(req.Password) == "" {
		errors = append(errors, ValidationError{Field: "password", Message: "Password is required"})
	}

	return errors
}

// validateLoginRequest validates the entire login request
func validateLoginRequest(req *LoginRequest) []ValidationError {
	var allErrors []ValidationError

	// Check required fields
	requiredErrors := validateLoginRequired(req)
	allErrors = append(allErrors, requiredErrors...)

	// If email is present, validate format
	if strings.TrimSpace(req.Email) != "" {
		if emailError := validateEmail(req.Email); emailError != nil {
			allErrors = append(allErrors, *emailError)
		}
	}

	return allErrors
}

// validateTokenRequired checks if token field is present and not empty
func validateTokenRequired(req *ValidateTokenRequest) []ValidationError {
	var errors []ValidationError

	if strings.TrimSpace(req.Token) == "" {
		errors = append(errors, ValidationError{Field: "token", Message: "Token is required"})
	}

	return errors
}

// validateTokenRequest validates the entire token validation request
func validateTokenRequest(req *ValidateTokenRequest) []ValidationError {
	return validateTokenRequired(req)
}

// validateUserID checks if userID has a valid format (UUID or numeric)
func validateUserID(userID string) *ValidationError {
	if strings.TrimSpace(userID) == "" {
		return &ValidationError{Field: "userId", Message: "User ID is required"}
	}

	trimmedID := strings.TrimSpace(userID)

	// Check if it's a valid UUID (8-4-4-4-12 format)
	uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	if uuidRegex.MatchString(trimmedID) {
		return nil
	}

	// Check if it's a valid numeric ID (positive integer)
	numericRegex := regexp.MustCompile(`^[1-9]\d*$`)
	if numericRegex.MatchString(trimmedID) {
		return nil
	}

	return &ValidationError{Field: "userId", Message: "User ID must be a valid UUID or positive integer"}
}

// LoginRequest represents the JSON request for user login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ValidateTokenRequest represents the JSON request for token validation
type ValidateTokenRequest struct {
	Token string `json:"token"`
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	validationErrors := validateRegisterRequest(&req)
	if len(validationErrors) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		validationResponse := ValidationErrors{Errors: validationErrors}
		if err := json.NewEncoder(w).Encode(validationResponse); err != nil {
			// If JSON encoding fails, fall back to plain text error
			http.Error(w, "Internal server error: failed to encode validation errors", http.StatusInternalServerError)
		}
		return
	}

	// Call gRPC service
	resp, err := h.authClient.Register(r.Context(), req.Email, req.Password, req.FirstName, req.LastName, req.Role)
	if err != nil {
		// Map gRPC error to appropriate HTTP status code
		statusCode, message := mapGRPCError(err)
		http.Error(w, message, statusCode)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If JSON encoding fails, log the error and return 500
		http.Error(w, "Internal server error: failed to encode response", http.StatusInternalServerError)
		return
	}
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	validationErrors := validateLoginRequest(&req)
	if len(validationErrors) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		validationResponse := ValidationErrors{Errors: validationErrors}
		if err := json.NewEncoder(w).Encode(validationResponse); err != nil {
			// If JSON encoding fails, fall back to plain text error
			http.Error(w, "Internal server error: failed to encode validation errors", http.StatusInternalServerError)
		}
		return
	}

	// Call gRPC service
	resp, err := h.authClient.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		// Map gRPC error to appropriate HTTP status code
		statusCode, message := mapGRPCError(err)
		http.Error(w, message, statusCode)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If JSON encoding fails, log the error and return 500
		http.Error(w, "Internal server error: failed to encode response", http.StatusInternalServerError)
		return
	}
}

// ValidateToken handles token validation
func (h *AuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	validationErrors := validateTokenRequest(&req)
	if len(validationErrors) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		validationResponse := ValidationErrors{Errors: validationErrors}
		if err := json.NewEncoder(w).Encode(validationResponse); err != nil {
			// If JSON encoding fails, fall back to plain text error
			http.Error(w, "Internal server error: failed to encode validation errors", http.StatusInternalServerError)
		}
		return
	}

	// Call gRPC service
	resp, err := h.authClient.ValidateToken(r.Context(), req.Token)
	if err != nil {
		// Map gRPC error to appropriate HTTP status code
		statusCode, message := mapGRPCError(err)
		http.Error(w, message, statusCode)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If JSON encoding fails, log the error and return 500
		http.Error(w, "Internal server error: failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GetProfile handles user profile retrieval
func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("userId")

	// Validate userID format
	if validationError := validateUserID(userID); validationError != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		validationResponse := ValidationErrors{Errors: []ValidationError{*validationError}}
		if err := json.NewEncoder(w).Encode(validationResponse); err != nil {
			// If JSON encoding fails, fall back to plain text error
			http.Error(w, "Internal server error: failed to encode validation errors", http.StatusInternalServerError)
		}
		return
	}

	// Call gRPC service
	resp, err := h.authClient.GetProfile(r.Context(), userID)
	if err != nil {
		// Map gRPC error to appropriate HTTP status code
		statusCode, message := mapGRPCError(err)
		http.Error(w, message, statusCode)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If JSON encoding fails, log the error and return 500
		http.Error(w, "Internal server error: failed to encode response", http.StatusInternalServerError)
		return
	}
}

// ProcessImage handles image processing requests
func (h *ImageHandler) ProcessImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	// Get file from form
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "No image file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file data
	imageData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read image data", http.StatusInternalServerError)
		return
	}

	// Get MIME type from header
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		// Try to detect from filename extension or use default
		filename := header.Filename
		if strings.HasSuffix(strings.ToLower(filename), ".jpg") || strings.HasSuffix(strings.ToLower(filename), ".jpeg") {
			mimeType = "image/jpeg"
		} else if strings.HasSuffix(strings.ToLower(filename), ".png") {
			mimeType = "image/png"
		} else {
			mimeType = "application/octet-stream"
		}
	}

	// Get optional product name from form
	productName := r.FormValue("product_name")

	// Call gRPC service
	resp, err := h.imageClient.ProcessImage(r.Context(), imageData, mimeType, productName)
	if err != nil {
		// Map gRPC error to appropriate HTTP status code
		statusCode, message := mapGRPCError(err)
		http.Error(w, message, statusCode)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", resp.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"processed_%s\"", header.Filename))

	// Return processed image data
	if _, err := w.Write(resp.ProcessedImageData); err != nil {
		http.Error(w, "Failed to write image response", http.StatusInternalServerError)
		return
	}
}
