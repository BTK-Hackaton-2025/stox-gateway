package aws

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// S3Service handles all S3 operations for the image management system
type S3Service struct {
	client            *s3.Client
	uploader          *manager.Uploader
	downloader        *manager.Downloader
	bucketName        string
	region            string
	cloudFrontDomain  string
	logger            *zap.Logger
}

// S3Config holds configuration for S3 service
type S3Config struct {
	BucketName       string
	Region           string
	CloudFrontDomain string
}

// ImageUploadResult contains the result of an image upload operation
type ImageUploadResult struct {
	Key          string    `json:"key"`
	URL          string    `json:"url"`
	UserID       string    `json:"userId"`
	ImageType    string    `json:"imageType"` // "original" or "enhanced"
	FileName     string    `json:"fileName"`
	ContentType  string    `json:"contentType"`
	Size         int64     `json:"size"`
	UploadedAt   time.Time `json:"uploadedAt"`
	ETag         string    `json:"etag"`
}

// NewS3Service creates a new S3 service instance with AWS best practices
func NewS3Service(cfg S3Config, logger *zap.Logger) (*S3Service, error) {
	// Load AWS configuration with automatic credential chain
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)
	
	return &S3Service{
		client:           client,
		uploader:         manager.NewUploader(client),
		downloader:       manager.NewDownloader(client),
		bucketName:       cfg.BucketName,
		region:           cfg.Region,
		cloudFrontDomain: cfg.CloudFrontDomain,
		logger:           logger,
	}, nil
}

// UploadOriginalImage uploads an original image to S3 with proper folder structure
// Path: btk-stox-s3/users/{userID}/original/{filename}
func (s *S3Service) UploadOriginalImage(ctx context.Context, userID string, fileName string, imageData io.Reader, contentType string) (*ImageUploadResult, error) {
	// Generate unique filename to prevent collisions
	fileExt := filepath.Ext(fileName)
	uniqueID := uuid.New().String()
	uniqueFileName := fmt.Sprintf("%s_%s%s", strings.TrimSuffix(fileName, fileExt), uniqueID, fileExt)
	
	// Construct the S3 key following the required structure
	key := fmt.Sprintf("users/%s/original/%s", userID, uniqueFileName)
	
	s.logger.Info("Uploading original image to S3", 
		zap.String("userID", userID),
		zap.String("key", key),
		zap.String("contentType", contentType),
	)
	
	return s.uploadImage(ctx, key, imageData, contentType, userID, "original", uniqueFileName)
}

// UploadEnhancedImage uploads an enhanced image to S3 with proper folder structure
// Path: btk-stox-s3/users/{userID}/enhanced/{filename}
func (s *S3Service) UploadEnhancedImage(ctx context.Context, userID string, fileName string, imageData io.Reader, contentType string) (*ImageUploadResult, error) {
	// Generate unique filename to prevent collisions
	fileExt := filepath.Ext(fileName)
	uniqueID := uuid.New().String()
	uniqueFileName := fmt.Sprintf("%s_enhanced_%s%s", strings.TrimSuffix(fileName, fileExt), uniqueID, fileExt)
	
	// Construct the S3 key following the required structure
	key := fmt.Sprintf("users/%s/enhanced/%s", userID, uniqueFileName)
	
	s.logger.Info("Uploading enhanced image to S3", 
		zap.String("userID", userID),
		zap.String("key", key),
		zap.String("contentType", contentType),
	)
	
	return s.uploadImage(ctx, key, imageData, contentType, userID, "enhanced", uniqueFileName)
}

// uploadImage performs the actual S3 upload with security best practices
func (s *S3Service) uploadImage(ctx context.Context, key string, imageData io.Reader, contentType, userID, imageType, fileName string) (*ImageUploadResult, error) {
	// Read the image data to get size
	data, err := io.ReadAll(imageData)
	if err != nil {
		s.logger.Error("Failed to read image data", zap.Error(err))
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}
	
	// Upload parameters with security best practices
	uploadInput := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		
		// Security: Server-side encryption
		ServerSideEncryption: types.ServerSideEncryptionAes256,
		
		// Metadata for tracking and organization
		Metadata: map[string]string{
			"user-id":     userID,
			"image-type":  imageType,
			"upload-time": time.Now().UTC().Format(time.RFC3339),
			"content-type": contentType,
		},
		
		// Cache control for CloudFront optimization
		CacheControl: aws.String("max-age=31536000"), // 1 year for images
	}
	
	// Perform the upload
	result, err := s.uploader.Upload(ctx, uploadInput)
	if err != nil {
		s.logger.Error("Failed to upload image to S3", 
			zap.Error(err),
			zap.String("key", key),
			zap.String("userID", userID),
		)
		return nil, fmt.Errorf("failed to upload image to S3: %w", err)
	}
	
	s.logger.Info("Successfully uploaded image to S3", 
		zap.String("location", result.Location),
		zap.String("etag", aws.ToString(result.ETag)),
		zap.String("userID", userID),
	)
	
	// Generate CloudFront URL
	cloudFrontURL := fmt.Sprintf("https://%s/%s", s.cloudFrontDomain, key)
	
	s.logger.Info("Generating CloudFront URL", 
		zap.String("domain", s.cloudFrontDomain),
		zap.String("key", key),
	)
	
	return &ImageUploadResult{
		Key:         key,
		URL:         cloudFrontURL,
		UserID:      userID,
		ImageType:   imageType,
		FileName:    fileName,
		ContentType: contentType,
		Size:        int64(len(data)),
		UploadedAt:  time.Now().UTC(),
		ETag:        aws.ToString(result.ETag),
	}, nil
}

// GetImageURL generates a presigned URL for accessing an image through CloudFront
// This ensures users can only access their own images
func (s *S3Service) GetImageURL(ctx context.Context, userID, imageType, fileName string) (string, error) {
	key := fmt.Sprintf("users/%s/%s/%s", userID, imageType, fileName)
	
	// Create a presigned request for GetObject
	presignClient := s3.NewPresignClient(s.client)
	
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(15 * time.Minute) // URL valid for 15 minutes
	})
	
	if err != nil {
		s.logger.Error("Failed to generate presigned URL", 
			zap.Error(err),
			zap.String("key", key),
			zap.String("userID", userID),
		)
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	
	s.logger.Info("Generated presigned URL", 
		zap.String("key", key),
		zap.String("userID", userID),
	)
	
	return request.URL, nil
}

// DownloadImage downloads an image from S3 for processing
// This is used by the image service to get images for enhancement
func (s *S3Service) DownloadImage(ctx context.Context, key string) ([]byte, error) {
	s.logger.Info("Downloading image from S3", zap.String("key", key))
	
	buf := manager.NewWriteAtBuffer([]byte{})
	
	_, err := s.downloader.Download(ctx, buf, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	
	if err != nil {
		s.logger.Error("Failed to download image from S3", 
			zap.Error(err),
			zap.String("key", key),
		)
		return nil, fmt.Errorf("failed to download image from S3: %w", err)
	}
	
	s.logger.Info("Successfully downloaded image from S3", 
		zap.String("key", key),
		zap.Int("size", len(buf.Bytes())),
	)
	
	return buf.Bytes(), nil
}

// ListUserImages lists all images for a specific user
func (s *S3Service) ListUserImages(ctx context.Context, userID string) ([]string, error) {
	prefix := fmt.Sprintf("users/%s/", userID)
	
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(prefix),
	}
	
	var images []string
	paginator := s3.NewListObjectsV2Paginator(s.client, input)
	
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			s.logger.Error("Failed to list user images", 
				zap.Error(err),
				zap.String("userID", userID),
			)
			return nil, fmt.Errorf("failed to list user images: %w", err)
		}
		
		for _, obj := range page.Contents {
			images = append(images, aws.ToString(obj.Key))
		}
	}
	
	s.logger.Info("Listed user images", 
		zap.String("userID", userID),
		zap.Int("count", len(images)),
	)
	
	return images, nil
}

// DeleteImage deletes an image from S3
func (s *S3Service) DeleteImage(ctx context.Context, key string) error {
	s.logger.Info("Deleting image from S3", zap.String("key", key))
	
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	
	if err != nil {
		s.logger.Error("Failed to delete image from S3", 
			zap.Error(err),
			zap.String("key", key),
		)
		return fmt.Errorf("failed to delete image from S3: %w", err)
	}
	
	s.logger.Info("Successfully deleted image from S3", zap.String("key", key))
	return nil
}

// ValidateUserAccess ensures a user can only access their own images
func (s *S3Service) ValidateUserAccess(userID, key string) bool {
	expectedPrefix := fmt.Sprintf("users/%s/", userID)
	return strings.HasPrefix(key, expectedPrefix)
}
