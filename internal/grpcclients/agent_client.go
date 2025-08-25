package grpcclients

import (
	"context"
	"fmt"
	"time"

	pb "stox-gateway/internal/proto/seo_agent"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AgentClient wraps the gRPC connection to the agent service
type AgentClient struct {
	conn   *grpc.ClientConn
	client pb.SEOAgentServiceClient
	logger *zap.Logger
}

// NewAgentClient creates a new agent service client
func NewAgentClient(host string, port int, logger *zap.Logger) (*AgentClient, error) {
	address := fmt.Sprintf("%s:%d", host, port)
	
	logger.Info("Connecting to agent service", zap.String("address", address))
	
	// Create gRPC connection
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent service: %w", err)
	}
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	client := pb.NewSEOAgentServiceClient(conn)
	
	// Health check
	_, err = client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("agent service health check failed: %w", err)
	}
	
	logger.Info("Connected to agent service")
	
	return &AgentClient{
		conn:   conn,
		client: client,
		logger: logger,
	}, nil
}

// Close closes the gRPC connection
func (c *AgentClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Chat sends a chat message to the agent
func (c *AgentClient) Chat(ctx context.Context, message string) (*pb.ChatResponse, error) {
	request := &pb.ChatRequest{
		Message: message,
	}
	
	c.logger.Debug("Sending chat request to agent", zap.String("message", message))
	
	response, err := c.client.Chat(ctx, request)
	if err != nil {
		c.logger.Error("Chat request failed", zap.Error(err))
		return nil, fmt.Errorf("chat request failed: %w", err)
	}
	
	return response, nil
}

// AnalyzeSEO sends an SEO analysis request to the agent
func (c *AgentClient) AnalyzeSEO(ctx context.Context, imageURL string) (*pb.AnalyzeSEOResponse, error) {
	request := &pb.AnalyzeSEORequest{
		ImageUrl: imageURL,
	}
	
	c.logger.Debug("Sending SEO analysis request to agent", zap.String("imageUrl", imageURL))
	
	response, err := c.client.AnalyzeSEO(ctx, request)
	if err != nil {
		c.logger.Error("SEO analysis request failed", zap.Error(err))
		return nil, fmt.Errorf("SEO analysis request failed: %w", err)
	}
	
	return response, nil
}
