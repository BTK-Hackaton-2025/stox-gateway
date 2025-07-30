package grpcclients

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "stox-gateway/internal/proto/auth"
)

// AuthClient represents a gRPC client for the auth service
type AuthClient struct {
	client pb.AuthServiceClient
	conn   *grpc.ClientConn
}

// NewAuthClient creates a new auth client
func NewAuthClient(host string, port int) (*AuthClient, error) {
	address := fmt.Sprintf("%s:%d", host, port)

	// Create insecure connection (use TLS in production)
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to auth service: %v", err)
	}

	client := pb.NewAuthServiceClient(conn)

	return &AuthClient{
		client: client,
		conn:   conn,
	}, nil
}

// Close closes the gRPC connection
func (c *AuthClient) Close() error {
	return c.conn.Close()
}

// Register registers a new user
func (c *AuthClient) Register(ctx context.Context, email, password, firstName, lastName, role string) (*pb.AuthResponse, error) {
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
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("register failed: %s", st.Message())
		}
		return nil, fmt.Errorf("register failed: %v", err)
	}

	return resp, nil
}

// Login authenticates a user
func (c *AuthClient) Login(ctx context.Context, email, password string) (*pb.AuthResponse, error) {
	req := &pb.LoginRequest{
		Email:    email,
		Password: password,
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.Login(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("login failed: %s", st.Message())
		}
		return nil, fmt.Errorf("login failed: %v", err)
	}

	return resp, nil
}

// ValidateToken validates a JWT token
func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*pb.ValidateTokenResponse, error) {
	req := &pb.ValidateTokenRequest{
		Token: token,
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.ValidateToken(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("token validation failed: %s", st.Message())
		}
		return nil, fmt.Errorf("token validation failed: %v", err)
	}

	return resp, nil
}

// RefreshToken refreshes an access token using a refresh token
func (c *AuthClient) RefreshToken(ctx context.Context, refreshToken string) (*pb.AuthResponse, error) {
	req := &pb.RefreshTokenRequest{
		RefreshToken: refreshToken,
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.RefreshToken(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("token refresh failed: %s", st.Message())
		}
		return nil, fmt.Errorf("token refresh failed: %v", err)
	}

	return resp, nil
}

// GetProfile retrieves a user's profile
func (c *AuthClient) GetProfile(ctx context.Context, userID string) (*pb.UserProfileResponse, error) {
	req := &pb.GetProfileRequest{
		UserId: userID,
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.GetProfile(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("get profile failed: %s", st.Message())
		}
		return nil, fmt.Errorf("get profile failed: %v", err)
	}

	return resp, nil
}
