package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorResponse là cấu trúc response lỗi chuẩn
type ErrorResponse struct {
	Error         string `json:"error"`
	Message       string `json:"message,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Code          string `json:"code,omitempty"`
}

// ErrorHandler middleware xử lý lỗi và trả về response chuẩn
func ErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap response writer để bắt lỗi
		ww := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Recover từ panic
		defer func() {
			if err := recover(); err != nil {
				corrID := GetCorrelationID(r.Context())
				log.Error().
					Str("correlation_id", corrID).
					Interface("error", err).
					Msg("Panic recovered")

				WriteErrorResponse(w, r.Context(), http.StatusInternalServerError, "Internal Server Error", "INTERNAL_ERROR", "")
			}
		}()

		next.ServeHTTP(ww, r)
	})
}

// WriteErrorResponse viết error response chuẩn
func WriteErrorResponse(w http.ResponseWriter, ctx context.Context, statusCode int, message, code, details string) {
	corrID := GetCorrelationID(ctx)

	response := ErrorResponse{
		Error:         message,
		Message:       details,
		CorrelationID: corrID,
		Code:          code,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// HandleGRPCError xử lý lỗi từ gRPC và chuyển đổi sang HTTP status code
func HandleGRPCError(err error) (int, string, string) {
	if err == nil {
		return http.StatusOK, "", ""
	}

	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError, "Internal Server Error", "INTERNAL_ERROR"
	}

	switch st.Code() {
	case codes.OK:
		return http.StatusOK, "", ""
	case codes.InvalidArgument:
		return http.StatusBadRequest, "Invalid Request", "INVALID_ARGUMENT"
	case codes.NotFound:
		return http.StatusNotFound, "Not Found", "NOT_FOUND"
	case codes.AlreadyExists:
		return http.StatusConflict, "Resource Already Exists", "ALREADY_EXISTS"
	case codes.PermissionDenied:
		return http.StatusForbidden, "Permission Denied", "PERMISSION_DENIED"
	case codes.Unauthenticated:
		return http.StatusUnauthorized, "Unauthorized", "UNAUTHENTICATED"
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests, "Resource Exhausted", "RESOURCE_EXHAUSTED"
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed, "Precondition Failed", "FAILED_PRECONDITION"
	case codes.Aborted:
		return http.StatusConflict, "Request Aborted", "ABORTED"
	case codes.OutOfRange:
		return http.StatusBadRequest, "Out of Range", "OUT_OF_RANGE"
	case codes.Unimplemented:
		return http.StatusNotImplemented, "Not Implemented", "UNIMPLEMENTED"
	case codes.Internal:
		return http.StatusInternalServerError, "Internal Server Error", "INTERNAL_ERROR"
	case codes.Unavailable:
		return http.StatusServiceUnavailable, "Service Unavailable", "UNAVAILABLE"
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout, "Request Timeout", "DEADLINE_EXCEEDED"
	default:
		return http.StatusInternalServerError, "Internal Server Error", "INTERNAL_ERROR"
	}
}

// responseWriter wrap http.ResponseWriter để bắt status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) StatusCode() int {
	return rw.statusCode
}
