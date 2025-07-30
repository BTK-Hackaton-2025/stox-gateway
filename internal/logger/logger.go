package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// InitLogger initializes the logger based on environment
func InitLogger(level string, format string, environment string) {
	var logLevel zapcore.Level
	switch level {
	case "debug":
		logLevel = zapcore.DebugLevel
	case "info":
		logLevel = zapcore.InfoLevel
	case "warn":
		logLevel = zapcore.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	default:
		logLevel = zapcore.InfoLevel
	}

	var config zap.Config

	// Use development config for better readability in dev, production config otherwise
	if environment == "development" && format == "console" {
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(logLevel)
	} else {
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(logLevel)
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	var err error
	Logger, err = config.Build()
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}

	// Replace the global logger so zap.L() returns our configured logger
	zap.ReplaceGlobals(Logger)

	// Redirect standard library log output to zap
	zap.RedirectStdLog(Logger)
}

// Sync flushes any buffered log entries
func Sync() {
	if Logger != nil {
		// Ignore sync errors on Windows (stdout/stderr can't be synced)
		_ = Logger.Sync() //nolint:errcheck
	}
}
