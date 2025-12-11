package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const CorrelationIDKey = "correlation_id"

// CorrelationID middleware tạo và truyền correlation ID qua request
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Lấy correlation ID từ header hoặc tạo mới
		corrID := r.Header.Get("X-Correlation-ID")
		if corrID == "" {
			corrID = uuid.New().String()
		}

		// Thêm vào context
		ctx := context.WithValue(r.Context(), CorrelationIDKey, corrID)
		r = r.WithContext(ctx)

		// Thêm vào response header
		w.Header().Set("X-Correlation-ID", corrID)

		// Log request với correlation ID
		log.Info().
			Str(CorrelationIDKey, corrID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Msg("Incoming request")

		next.ServeHTTP(w, r)
	})
}

// GetCorrelationID lấy correlation ID từ context
func GetCorrelationID(ctx context.Context) string {
	if corrID, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return corrID
	}
	return "unknown"
}
