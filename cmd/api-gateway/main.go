package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stox-gateway/internal/config"
	"stox-gateway/internal/gateway"
	"stox-gateway/internal/grpcclients"
	"stox-gateway/internal/logger"

	"go.uber.org/zap"
)

func main() {
	// Load configuration first to get logging settings
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		// Use basic logging before we have our configured logger
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with config-driven settings
	if err := logger.InitLogger(cfg.Logging.Level, cfg.Logging.Format, cfg.Server.Environment); err != nil {
		// Use basic logging before we have our configured logger
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	log := logger.Logger

	// Create auth client
	authClient, err := grpcclients.NewAuthClient(cfg.Services.Auth.Host, cfg.Services.Auth.Port, log)
	if err != nil {
		log.Fatal("Failed to create auth client", zap.Error(err))
	}
	defer authClient.Close()

	log.Info("Auth client created successfully",
		zap.String("host", cfg.Services.Auth.Host),
		zap.Int("port", cfg.Services.Auth.Port),
	)

	// Create handlers
	authHandler := gateway.NewAuthHandler(authClient)

	// Create router
	router := gateway.NewRouter(authHandler)

	// Apply middleware
	handler := gateway.CORSMiddleware(&cfg.CORS)(gateway.LoggingMiddleware(router))

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Starting API Gateway",
			zap.Int("port", cfg.Server.Port),
			zap.String("environment", cfg.Server.Environment),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited gracefully")
}
