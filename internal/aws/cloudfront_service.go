package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"go.uber.org/zap"
)

// CloudFrontService handles CloudFront distribution operations
type CloudFrontService struct {
	client         *cloudfront.Client
	distributionID string
	domainName     string
	logger         *zap.Logger
}

// CloudFrontConfig holds configuration for CloudFront service
type CloudFrontConfig struct {
	DistributionID string
	DomainName     string
	Region         string
}

// SignedURLRequest contains parameters for generating signed URLs
type SignedURLRequest struct {
	Key       string
	UserID    string
	ExpiresIn time.Duration
}

// NewCloudFrontService creates a new CloudFront service instance
func NewCloudFrontService(cfg CloudFrontConfig, logger *zap.Logger) (*CloudFrontService, error) {
	// Load AWS configuration
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for CloudFront: %w", err)
	}

	client := cloudfront.NewFromConfig(awsCfg)
	
	return &CloudFrontService{
		client:         client,
		distributionID: cfg.DistributionID,
		domainName:     cfg.DomainName,
		logger:         logger,
	}, nil
}

// CreateDistribution creates a new CloudFront distribution for the S3 bucket
// This should be called during infrastructure setup
func (cf *CloudFrontService) CreateDistribution(ctx context.Context, s3BucketName, s3Region string) (*types.Distribution, error) {
	cf.logger.Info("Creating CloudFront distribution", 
		zap.String("s3Bucket", s3BucketName),
		zap.String("s3Region", s3Region),
	)

	// Origin domain for S3 bucket
	originDomain := fmt.Sprintf("%s.s3.%s.amazonaws.com", s3BucketName, s3Region)
	
	// Unique caller reference for idempotency
	callerReference := fmt.Sprintf("stox-gateway-%d", time.Now().Unix())

	// Distribution configuration with security best practices
	distributionConfig := &types.DistributionConfig{
		CallerReference: aws.String(callerReference),
		Comment:         aws.String("STOX Gateway Image Distribution - Secure image delivery"),
		Enabled:         aws.Bool(true),
		
		// Origins configuration
		Origins: &types.Origins{
			Quantity: aws.Int32(1),
			Items: []types.Origin{
				{
					Id:         aws.String("S3-" + s3BucketName),
					DomainName: aws.String(originDomain),
					
					// S3 Origin Configuration with Origin Access Control (OAC)
					S3OriginConfig: &types.S3OriginConfig{
						OriginAccessIdentity: aws.String(""), // Using OAC instead of OAI
					},
					
					// Connection settings
					ConnectionAttempts: aws.Int32(3),
					ConnectionTimeout:  aws.Int32(10),
				},
			},
		},
		
		// Default cache behavior with security settings
		DefaultCacheBehavior: &types.DefaultCacheBehavior{
			TargetOriginId: aws.String("S3-" + s3BucketName),
			
			// Viewer protocol policy - redirect HTTP to HTTPS
			ViewerProtocolPolicy: types.ViewerProtocolPolicyRedirectToHttps,
			
			// Allowed HTTP methods
			AllowedMethods: &types.AllowedMethods{
				Quantity: aws.Int32(2),
				Items:    []types.Method{types.MethodGet, types.MethodHead},
				CachedMethods: &types.CachedMethods{
					Quantity: aws.Int32(2),
					Items:    []types.Method{types.MethodGet, types.MethodHead},
				},
			},
			
			// Cache settings for optimal image delivery
			CachePolicyId: aws.String("4135ea2d-6df8-44a3-9df3-4b5a84be39ad"), // Managed-CachingOptimized
			
			// Origin request policy for images
			OriginRequestPolicyId: aws.String("88a5eaf4-2fd4-4709-b370-b4c650ea3fcf"), // Managed-CORS-S3Origin
			
			// Compress objects automatically
			Compress: aws.Bool(true),
			
			// Trusted signers for signed URLs (empty for now, will be configured later)
			TrustedSigners: &types.TrustedSigners{
				Enabled:  aws.Bool(false),
				Quantity: aws.Int32(0),
			},
		},
		
		// Price class for cost optimization
		PriceClass: types.PriceClassPriceClass100, // Use only North America and Europe
		
		// Web ACL for additional security (can be configured later)
		WebACLId: aws.String(""),
		
		// HTTP version
		HttpVersion: types.HttpVersionHttp2,
		
		// IPv6 support
		IsIPV6Enabled: aws.Bool(true),
		
		// Logging configuration
		Logging: &types.LoggingConfig{
			Enabled:        aws.Bool(true),
			IncludeCookies: aws.Bool(false),
			Bucket:         aws.String(s3BucketName + "-logs.s3.amazonaws.com"),
			Prefix:         aws.String("cloudfront-logs/"),
		},
	}

	input := &cloudfront.CreateDistributionInput{
		DistributionConfig: distributionConfig,
	}

	result, err := cf.client.CreateDistribution(ctx, input)
	if err != nil {
		cf.logger.Error("Failed to create CloudFront distribution", zap.Error(err))
		return nil, fmt.Errorf("failed to create CloudFront distribution: %w", err)
	}

	cf.logger.Info("Successfully created CloudFront distribution", 
		zap.String("distributionId", aws.ToString(result.Distribution.Id)),
		zap.String("domainName", aws.ToString(result.Distribution.DomainName)),
		zap.String("status", aws.ToString(result.Distribution.Status)),
	)

	return result.Distribution, nil
}

// GetDistributionConfig retrieves the current distribution configuration
func (cf *CloudFrontService) GetDistributionConfig(ctx context.Context) (*types.Distribution, error) {
	input := &cloudfront.GetDistributionInput{
		Id: aws.String(cf.distributionID),
	}

	result, err := cf.client.GetDistribution(ctx, input)
	if err != nil {
		cf.logger.Error("Failed to get CloudFront distribution", 
			zap.Error(err),
			zap.String("distributionId", cf.distributionID),
		)
		return nil, fmt.Errorf("failed to get CloudFront distribution: %w", err)
	}

	return result.Distribution, nil
}

// InvalidateCache invalidates CloudFront cache for specific paths
// This is useful when images are updated or deleted
func (cf *CloudFrontService) InvalidateCache(ctx context.Context, paths []string) (*types.Invalidation, error) {
	cf.logger.Info("Creating CloudFront cache invalidation", 
		zap.Strings("paths", paths),
		zap.String("distributionId", cf.distributionID),
	)

	// Convert paths to CloudFront invalidation paths
	var items []string
	for _, path := range paths {
		// Ensure path starts with /
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		items = append(items, path)
	}

	callerReference := fmt.Sprintf("stox-invalidation-%d", time.Now().Unix())

	input := &cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(cf.distributionID),
		InvalidationBatch: &types.InvalidationBatch{
			CallerReference: aws.String(callerReference),
			Paths: &types.Paths{
				Quantity: aws.Int32(int32(len(items))),
				Items:    items,
			},
		},
	}

	result, err := cf.client.CreateInvalidation(ctx, input)
	if err != nil {
		cf.logger.Error("Failed to create CloudFront invalidation", 
			zap.Error(err),
			zap.Strings("paths", paths),
		)
		return nil, fmt.Errorf("failed to create CloudFront invalidation: %w", err)
	}

	cf.logger.Info("Successfully created CloudFront invalidation", 
		zap.String("invalidationId", aws.ToString(result.Invalidation.Id)),
		zap.String("status", aws.ToString(result.Invalidation.Status)),
	)

	return result.Invalidation, nil
}

// GetImageURL generates a CloudFront URL for an image
// This provides optimized global delivery through CloudFront edge locations
func (cf *CloudFrontService) GetImageURL(key string) string {
	// Remove leading slash if present
	key = strings.TrimPrefix(key, "/")
	
	url := fmt.Sprintf("https://%s/%s", cf.domainName, key)
	
	cf.logger.Debug("Generated CloudFront URL", 
		zap.String("key", key),
		zap.String("url", url),
	)
	
	return url
}

// UpdateSecurityHeaders updates the distribution to include security headers
func (cf *CloudFrontService) UpdateSecurityHeaders(ctx context.Context) error {
	cf.logger.Info("Updating CloudFront distribution with security headers")

	// First, get the current distribution configuration
	getResult, err := cf.client.GetDistribution(ctx, &cloudfront.GetDistributionInput{
		Id: aws.String(cf.distributionID),
	})
	if err != nil {
		return fmt.Errorf("failed to get distribution config: %w", err)
	}

	config := getResult.Distribution.DistributionConfig
	etag := getResult.ETag

	// Add response headers policy for security
	// This would typically reference a managed policy or custom policy ID
	// For now, we'll add it to the cache behavior
	if config.DefaultCacheBehavior.ResponseHeadersPolicyId == nil {
		// Use AWS managed security headers policy
		config.DefaultCacheBehavior.ResponseHeadersPolicyId = aws.String("67f7725c-6f97-4210-82d7-5512b31e9d03") // Managed-SecurityHeadersPolicy
	}

	// Update the distribution
	updateInput := &cloudfront.UpdateDistributionInput{
		Id:                 aws.String(cf.distributionID),
		DistributionConfig: config,
		IfMatch:           etag,
	}

	_, err = cf.client.UpdateDistribution(ctx, updateInput)
	if err != nil {
		cf.logger.Error("Failed to update CloudFront distribution", zap.Error(err))
		return fmt.Errorf("failed to update CloudFront distribution: %w", err)
	}

	cf.logger.Info("Successfully updated CloudFront distribution with security headers")
	return nil
}

// WaitForDistributionDeployed waits for the distribution to be fully deployed
func (cf *CloudFrontService) WaitForDistributionDeployed(ctx context.Context) error {
	cf.logger.Info("Waiting for CloudFront distribution to be deployed", 
		zap.String("distributionId", cf.distributionID),
	)

	waiter := cloudfront.NewDistributionDeployedWaiter(cf.client)
	
	err := waiter.Wait(ctx, &cloudfront.GetDistributionInput{
		Id: aws.String(cf.distributionID),
	}, 25*time.Minute) // CloudFront deployments can take up to 25 minutes

	if err != nil {
		cf.logger.Error("Failed waiting for CloudFront distribution deployment", zap.Error(err))
		return fmt.Errorf("failed waiting for CloudFront distribution deployment: %w", err)
	}

	cf.logger.Info("CloudFront distribution successfully deployed")
	return nil
}
