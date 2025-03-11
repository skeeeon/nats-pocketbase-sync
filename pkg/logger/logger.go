package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

// LogConfig contains configuration for the logger
type LogConfig struct {
	Level    string
	FilePath string
}

// Init initializes the logger with the given configuration
func Init(config LogConfig) {
	// Parse log level
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(config.Level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create JSON encoder for structured logging
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Set up the core for logging (console, file, or both)
	var core zapcore.Core
	
	// Always add console writer
	consoleWriter := zapcore.Lock(os.Stdout)
	
	if config.FilePath != "" {
		// Create log directory if it doesn't exist
		logDir := filepath.Dir(config.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			// If directory creation fails, fall back to console-only logging
			core = zapcore.NewCore(encoder, consoleWriter, zapLevel)
		} else {
			// Try to open the log file
			fileWriter, err := os.OpenFile(
				config.FilePath,
				os.O_APPEND|os.O_CREATE|os.O_WRONLY,
				0644,
			)
			if err != nil {
				// If file creation fails, fall back to console-only logging
				core = zapcore.NewCore(encoder, consoleWriter, zapLevel)
			} else {
				// Log to both console and file
				fileSync := zapcore.AddSync(fileWriter)
				core = zapcore.NewTee(
					zapcore.NewCore(encoder, consoleWriter, zapLevel),
					zapcore.NewCore(encoder, fileSync, zapLevel),
				)
			}
		}
	} else {
		// Console-only logging
		core = zapcore.NewCore(encoder, consoleWriter, zapLevel)
	}

	// Create the logger
	log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
}

// GetLogger returns the configured logger instance
func GetLogger() *zap.Logger {
	if log == nil {
		// Initialize with default settings if not already initialized
		Init(LogConfig{
			Level:    "info",
			FilePath: "",
		})
	}
	return log
}

// Sync flushes any buffered log entries
func Sync() {
	if log != nil {
		_ = log.Sync()
	}
}

// Fatal logs a fatal error message and exits the program
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
	os.Exit(1)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// Info logs an informational message
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}
