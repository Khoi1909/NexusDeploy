package logger

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	CorrelationIDKey = "correlation_id"
	ServiceNameKey   = "service"
	UserIDKey        = "user_id"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const correlationIDContextKey contextKey = CorrelationIDKey

// GetCorrelationID extracts correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if v := ctx.Value(correlationIDContextKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	// Also try with string key for backward compatibility
	if v := ctx.Value(CorrelationIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// WithCorrelationIDContext adds correlation ID to context
func WithCorrelationIDContext(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDContextKey, correlationID)
}

// InitLogger khởi tạo logger với cấu hình từ biến môi trường
func InitLogger(serviceName, logLevel, logFormat string) {
	// Set log level
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set time format
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// Set format
	if logFormat == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	} else {
		// JSON format (default)
		log.Logger = log.Output(os.Stdout)
	}

	// Add service name to all logs
	log.Logger = log.Logger.With().Str(ServiceNameKey, serviceName).Logger()
}

// GetLogger trả về logger instance với correlation ID nếu có
func GetLogger(ctx interface{}) zerolog.Logger {
	logger := log.Logger

	// Nếu ctx có correlation ID, thêm vào logger
	if ctxWithCorrID, ok := ctx.(interface {
		Value(key interface{}) interface{}
	}); ok {
		if corrID := ctxWithCorrID.Value(CorrelationIDKey); corrID != nil {
			if id, ok := corrID.(string); ok {
				logger = logger.With().Str(CorrelationIDKey, id).Logger()
			}
		}
	}

	return logger
}

// WithCorrelationID tạo logger mới với correlation ID
func WithCorrelationID(correlationID string) zerolog.Logger {
	return log.Logger.With().Str(CorrelationIDKey, correlationID).Logger()
}

// WithUserID tạo logger mới với user ID
func WithUserID(userID string) zerolog.Logger {
	return log.Logger.With().Str(UserIDKey, userID).Logger()
}

// WithCorrelationIDAndUserID tạo logger mới với cả correlation ID và user ID
func WithCorrelationIDAndUserID(correlationID, userID string) zerolog.Logger {
	return log.Logger.With().
		Str(CorrelationIDKey, correlationID).
		Str(UserIDKey, userID).
		Logger()
}
