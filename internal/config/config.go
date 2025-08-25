package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the API Gateway
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Services ServicesConfig `mapstructure:"services"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	CORS     CORSConfig     `mapstructure:"cors"`
	AWS      AWSConfig      `mapstructure:"aws"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	Environment  string        `mapstructure:"environment"`
}

// ServicesConfig holds microservice endpoints
type ServicesConfig struct {
	Auth            ServiceConfig       `mapstructure:"auth"`
	Image           ServiceConfig       `mapstructure:"image"`
	LLM             ServiceConfig       `mapstructure:"llm"`
	Queue           ServiceConfig       `mapstructure:"queue"`
	Agent           ServiceConfig       `mapstructure:"agent"`
	ProductAnalyzer ServiceConfig       `mapstructure:"product_analyzer"`
	MockEcommerce   MockEcommerceConfig `mapstructure:"mock_ecommerce"`
}

// ServiceConfig holds individual service configuration
type ServiceConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// MockEcommerceConfig holds MockECommerce API configuration
type MockEcommerceConfig struct {
	Host                     string `mapstructure:"host"`
	Port                     int    `mapstructure:"port"`
	BaseURL                  string `mapstructure:"base_url"`
	APIKeyValidationEndpoint string `mapstructure:"api_key_validation_endpoint"`
}

// JWTConfig holds JWT-related configuration
type JWTConfig struct {
	SecretKey     string        `mapstructure:"secret_key"`
	AccessExpiry  time.Duration `mapstructure:"access_expiry"`
	RefreshExpiry time.Duration `mapstructure:"refresh_expiry"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// CORSConfig holds CORS-related configuration
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
	AllowedHeaders []string `mapstructure:"allowed_headers"`
}

// AWSConfig holds AWS-related configuration
type AWSConfig struct {
	Region     string           `mapstructure:"region"`
	S3         S3Config         `mapstructure:"s3"`
	CloudFront CloudFrontConfig `mapstructure:"cloudfront"`
}

// S3Config holds S3-related configuration
type S3Config struct {
	BucketName string `mapstructure:"bucket_name"`
	Region     string `mapstructure:"region"`
}

// CloudFrontConfig holds CloudFront-related configuration
type CloudFrontConfig struct {
	DistributionID string `mapstructure:"distribution_id"`
	DomainName     string `mapstructure:"domain_name"`
	Region         string `mapstructure:"region"`
}

// LoadConfig reads configuration from file or environment variables
func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(configPath)

	// Environment variables
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "60s")

	// Service defaults
	viper.SetDefault("services.auth.host", "auth-service")
	viper.SetDefault("services.auth.port", 50051)
	viper.SetDefault("services.image.host", "image-service")
	viper.SetDefault("services.image.port", 50061)
	viper.SetDefault("services.llm.host", "localhost")
	viper.SetDefault("services.llm.port", 50052)
	viper.SetDefault("services.queue.host", "localhost")
	viper.SetDefault("services.queue.port", 50053)
	viper.SetDefault("services.agent.host", "localhost")
	viper.SetDefault("services.agent.port", 50054)
	viper.SetDefault("services.product_analyzer.host", "localhost")
	viper.SetDefault("services.product_analyzer.port", 50071)

	// JWT defaults
	viper.SetDefault("jwt.secret_key", "your-secret-key-change-in-production")
	viper.SetDefault("jwt.access_expiry", "15m")
	viper.SetDefault("jwt.refresh_expiry", "168h") // 7 days

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	// CORS defaults - secure by default
	viper.SetDefault("cors.allowed_origins", []string{"*"})
	viper.SetDefault("cors.allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("cors.allowed_headers", []string{"Content-Type", "Authorization"})

	// AWS defaults
	viper.SetDefault("aws.region", "us-east-1")
	viper.SetDefault("aws.s3.bucket_name", "btk-stox-s3")
	viper.SetDefault("aws.s3.region", "us-east-1")
	viper.SetDefault("aws.cloudfront.distribution_id", "")
	viper.SetDefault("aws.cloudfront.domain_name", "")
	viper.SetDefault("aws.cloudfront.region", "us-east-1")
}

// GetAuthServiceAddress returns the full address for the auth service
func (c *Config) GetAuthServiceAddress() string {
	return fmt.Sprintf("%s:%d", c.Services.Auth.Host, c.Services.Auth.Port)
}

// GetImageServiceAddress returns the full address for the image service
func (c *Config) GetImageServiceAddress() string {
	return fmt.Sprintf("http://%s:%d", c.Services.Image.Host, c.Services.Image.Port)
}

// GetLLMServiceAddress returns the full address for the LLM service
func (c *Config) GetLLMServiceAddress() string {
	return fmt.Sprintf("%s:%d", c.Services.LLM.Host, c.Services.LLM.Port)
}

// GetQueueServiceAddress returns the full address for the queue service
func (c *Config) GetQueueServiceAddress() string {
	return fmt.Sprintf("%s:%d", c.Services.Queue.Host, c.Services.Queue.Port)
}

// GetAgentServiceAddress returns the full address for the agent service
func (c *Config) GetAgentServiceAddress() string {
	return fmt.Sprintf("%s:%d", c.Services.Agent.Host, c.Services.Agent.Port)
}

// GetProductAnalyzerServiceAddress returns the full address for the product analyzer service
func (c *Config) GetProductAnalyzerServiceAddress() string {
	return fmt.Sprintf("%s:%d", c.Services.ProductAnalyzer.Host, c.Services.ProductAnalyzer.Port)
}
