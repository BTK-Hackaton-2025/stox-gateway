package grpcclients

import (
	"context"
	"fmt"
	"time"

	pb "stox-gateway/internal/proto/product-analyzer"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ProductAnalyzerClient wraps the gRPC connection to the product analyzer service
type ProductAnalyzerClient struct {
	conn   *grpc.ClientConn
	client pb.ProductAnalyzerClient
	logger *zap.Logger
}

// NewProductAnalyzerClient creates a new product analyzer service client
func NewProductAnalyzerClient(host string, port int, logger *zap.Logger) (*ProductAnalyzerClient, error) {
	address := fmt.Sprintf("%s:%d", host, port)

	logger.Info("Connecting to product analyzer service", zap.String("address", address))

	// Create gRPC connection
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to product analyzer service: %w", err)
	}

	client := pb.NewProductAnalyzerClient(conn)

	// Test connection with health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("product analyzer service health check failed: %w", err)
	}

	logger.Info("Connected to product analyzer service")

	return &ProductAnalyzerClient{
		conn:   conn,
		client: client,
		logger: logger,
	}, nil
}

// Close closes the gRPC connection
func (c *ProductAnalyzerClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GenerateFromImage generates product content from image data
func (c *ProductAnalyzerClient) GenerateFromImage(ctx context.Context, imageData []byte, filename, contentType string) (*pb.ImageResponse, error) {
	request := &pb.ImageRequest{
		Image:       imageData,
		Filename:    filename,
		ContentType: contentType,
	}

	c.logger.Debug("Sending image analysis request to product analyzer",
		zap.String("filename", filename),
		zap.String("contentType", contentType),
		zap.Int("imageSize", len(imageData)),
	)

	// Longer timeout for AI processing
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	response, err := c.client.GenerateFromImage(ctx, request)
	if err != nil {
		c.logger.Error("Product analysis request failed", zap.Error(err))
		return nil, fmt.Errorf("product analysis request failed: %w", err)
	}

	c.logger.Debug("Product analysis request successful",
		zap.String("title", response.Title),
		zap.Int("descriptionLength", len(response.Description)),
	)

	return response, nil
}

// GenerateFromImageUrl generates product content from image URL
func (c *ProductAnalyzerClient) GenerateFromImageUrl(ctx context.Context, imageURL string) (*pb.ImageResponse, error) {
	request := &pb.ImageUrlRequest{
		ImageUrl: imageURL,
	}

	c.logger.Debug("Sending image URL analysis request to product analyzer", zap.String("imageUrl", imageURL))

	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	response, err := c.client.GenerateFromImageUrl(ctx, request)
	if err != nil {
		c.logger.Error("Product analysis URL request failed", zap.Error(err))
		return nil, fmt.Errorf("product analysis URL request failed: %w", err)
	}

	c.logger.Debug("Product analysis URL request successful",
		zap.String("title", response.Title),
		zap.Int("descriptionLength", len(response.Description)),
	)

	return response, nil
}
