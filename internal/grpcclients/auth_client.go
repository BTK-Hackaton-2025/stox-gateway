package grpcclients

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

    pb "stox-gateway/internal/proto/auth"
)

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// safeLogAuthResponse creates a safe version of AuthResponse for logging
// that excludes sensitive token data
func safeLogAuthResponse(resp *pb.AuthResponse) map[string]interface{} {
	if resp == nil {
		return map[string]interface{}{"response": "nil"}
	}

	safeResp := map[string]interface{}{
		"success": resp.Success,
		"message": resp.Message,
		"errors":  resp.Errors,
	}

	// Safely log user data if present
	if resp.UserData != nil {
		safeResp["userData"] = map[string]interface{}{
			"id":        resp.UserData.Id,
			"email":     resp.UserData.Email,
			"firstName": resp.UserData.FirstName,
			"lastName":  resp.UserData.LastName,
			"role":      resp.UserData.Role,
			"createdAt": resp.UserData.CreatedAt,
			"updatedAt": resp.UserData.UpdatedAt,
			"isActive":  resp.UserData.IsActive,
		}
	}

	// Safely log token data if present (excluding actual tokens)
	if resp.TokenData != nil {
		safeResp["tokenData"] = map[string]interface{}{
			"accessToken":  "[REDACTED]",
			"refreshToken": "[REDACTED]",
			"expiresIn":    resp.TokenData.ExpiresIn,
			"tokenType":    resp.TokenData.TokenType,
		}
	}

	return safeResp
}

// safeLogValidateTokenResponse creates a safe version of ValidateTokenResponse for logging
func safeLogValidateTokenResponse(resp *pb.ValidateTokenResponse) map[string]interface{} {
	if resp == nil {
		return map[string]interface{}{"response": "nil"}
	}

	return map[string]interface{}{
		"valid":   resp.Valid,
		"userId":  resp.UserId,
		"email":   resp.Email,
		"role":    resp.Role,
		"exp":     resp.Exp,
		"message": resp.Message,
	}
}

// safeLogUserProfileResponse creates a safe version of UserProfileResponse for logging
func safeLogUserProfileResponse(resp *pb.UserProfileResponse) map[string]interface{} {
	if resp == nil {
		return map[string]interface{}{"response": "nil"}
	}

	safeResp := map[string]interface{}{
		"success": resp.Success,
		"message": resp.Message,
		"errors":  resp.Errors,
	}

	// Safely log user data if present
	if resp.UserData != nil {
		safeResp["userData"] = map[string]interface{}{
			"id":        resp.UserData.Id,
			"email":     resp.UserData.Email,
			"firstName": resp.UserData.FirstName,
			"lastName":  resp.UserData.LastName,
			"role":      resp.UserData.Role,
			"createdAt": resp.UserData.CreatedAt,
			"updatedAt": resp.UserData.UpdatedAt,
			"isActive":  resp.UserData.IsActive,
		}
	}

	return safeResp
}

// AuthClient represents a gRPC client for the auth service
type AuthClient struct {
	client pb.AuthServiceClient
	conn   *grpc.ClientConn
	logger *zap.Logger
}

// NewAuthClient creates a new auth client
func NewAuthClient(host string, port int, logger *zap.Logger) (*AuthClient, error) {
	address := fmt.Sprintf("%s:%d", host, port)

	logger.Info("Connecting to auth service",
		zap.String("address", address),
	)

	// Create insecure connection (use TLS in production)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect to auth service", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to auth service: %v", err)
	}

	logger.Info("Connected to auth service")

	client := pb.NewAuthServiceClient(conn)

	return &AuthClient{
		client: client,
		conn:   conn,
		logger: logger,
	}, nil
}

// Close closes the gRPC connection
func (c *AuthClient) Close() error {
	c.logger.Info("Closing connection to auth service")
	return c.conn.Close()
}

// Register registers a new user
func (c *AuthClient) Register(ctx context.Context, email, password, firstName, lastName, role string) (*pb.AuthResponse, error) {
	c.logger.Debug("Registering new user",
		zap.String("email", email),
		zap.String("firstName", firstName),
		zap.String("lastName", lastName),
		zap.String("role", role),
	)

	req := &pb.RegisterRequest{
		Email:     email,
		Password:  password,
		FirstName: firstName,
		LastName:  lastName,
		Role:      role,
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.Register(ctx, req)
	if err != nil {
		c.logger.Error("Register request failed", zap.Error(err))
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("register failed: %s", st.Message())
		}
		return nil, fmt.Errorf("register failed: %v", err)
	}

	c.logger.Debug("Register request successful", zap.Any("response", safeLogAuthResponse(resp)))

	return resp, nil
}

// Login authenticates a user
func (c *AuthClient) Login(ctx context.Context, email, password string) (*pb.AuthResponse, error) {
	req := &pb.LoginRequest{
		Email:    email,
		Password: password,
	}

	c.logger.Debug("Sending login request to auth service",
		zap.String("email", email),
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.Login(ctx, req)
	if err != nil {
		c.logger.Error("Login request failed", zap.Error(err))
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("login failed: %s", st.Message())
		}
		return nil, fmt.Errorf("login failed: %v", err)
	}

	c.logger.Debug("Login request successful", zap.Any("response", safeLogAuthResponse(resp)))

	return resp, nil
}

// ValidateToken validates a JWT token
func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*pb.ValidateTokenResponse, error) {
	req := &pb.ValidateTokenRequest{
		Token: token,
	}

	c.logger.Debug("Sending token validation request to auth service",
		zap.String("token", token[:min(len(token), 20)]+"..."), // truncate token for security
	)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.ValidateToken(ctx, req)
	if err != nil {
		c.logger.Error("Token validation request failed", zap.Error(err))
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("token validation failed: %s", st.Message())
		}
		return nil, fmt.Errorf("token validation failed: %v", err)
	}

	c.logger.Debug("Token validation request successful", zap.Any("response", safeLogValidateTokenResponse(resp)))

	return resp, nil
}

// RefreshToken refreshes an access token using a refresh token
func (c *AuthClient) RefreshToken(ctx context.Context, refreshToken string) (*pb.AuthResponse, error) {
	req := &pb.RefreshTokenRequest{
		RefreshToken: refreshToken,
	}

	c.logger.Debug("Sending token refresh request to auth service",
		zap.String("refreshToken", refreshToken[:min(len(refreshToken), 20)]+"..."), // truncate token for security
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.RefreshToken(ctx, req)
	if err != nil {
		c.logger.Error("Token refresh request failed", zap.Error(err))
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("token refresh failed: %s", st.Message())
		}
		return nil, fmt.Errorf("token refresh failed: %v", err)
	}

	c.logger.Debug("Token refresh request successful", zap.Any("response", safeLogAuthResponse(resp)))

	return resp, nil
}

// GetProfile retrieves a user's profile
func (c *AuthClient) GetProfile(ctx context.Context, userID string) (*pb.UserProfileResponse, error) {
	req := &pb.GetProfileRequest{
		UserId: userID,
	}

	c.logger.Debug("Sending get profile request to auth service",
		zap.String("userID", userID),
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.GetProfile(ctx, req)
	if err != nil {
		c.logger.Error("Get profile request failed", zap.Error(err))
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("get profile failed: %s", st.Message())
		}
		return nil, fmt.Errorf("get profile failed: %v", err)
	}

	c.logger.Debug("Get profile request successful", zap.Any("response", safeLogUserProfileResponse(resp)))

	return resp, nil
}
