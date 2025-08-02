package grpcclients

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "stox-gateway/internal/proto/image-service"
)

// ImageClient represents a gRPC client for the image service
type ImageClient struct {
	client pb.ImageServiceClient
	conn   *grpc.ClientConn
	logger *zap.Logger
}

// NewImageClient creates a new image client
func NewImageClient(host string, port int, logger *zap.Logger) (*ImageClient, error) {
	address := fmt.Sprintf("%s:%d", host, port)

	logger.Info("Connecting to image service",
		zap.String("address", address),
	)

	// Create insecure connection (use TLS in production)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect to image service", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to image service: %v", err)
	}

	logger.Info("Connected to image service")

	client := pb.NewImageServiceClient(conn)

	return &ImageClient{
		client: client,
		conn:   conn,
		logger: logger,
	}, nil
}

// Close closes the gRPC connection
func (c *ImageClient) Close() error {
	c.logger.Info("Closing connection to image service")
	return c.conn.Close()
}

// ProcessImage processes an image using the image service
func (c *ImageClient) ProcessImage(ctx context.Context, imageData []byte, mimeType, productName string) (*pb.ProcessImageResponse, error) {
	c.logger.Debug("Processing image",
		zap.String("mimeType", mimeType),
		zap.String("productName", productName),
		zap.Int("imageSize", len(imageData)),
	)

	req := &pb.ProcessImageRequest{
		ImageData:   imageData,
		MimeType:    mimeType,
		ProductName: productName,
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second) // Longer timeout for image processing
	defer cancel()

	resp, err := c.client.ProcessImage(ctx, req)
	if err != nil {
		c.logger.Error("Process image request failed", zap.Error(err))
		st, ok := status.FromError(err)
		if ok {
			return nil, fmt.Errorf("process image failed: %s", st.Message())
		}
		return nil, fmt.Errorf("process image failed: %v", err)
	}

	c.logger.Debug("Process image request successful",
		zap.String("responseMimeType", resp.MimeType),
		zap.String("message", resp.Message),
		zap.Int("processedImageSize", len(resp.ProcessedImageData)),
	)

	return resp, nil
}
