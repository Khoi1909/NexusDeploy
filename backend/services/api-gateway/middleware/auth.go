package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/nexusdeploy/backend/pkg/middleware"
	pb "github.com/nexusdeploy/backend/services/auth-service/proto"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

// ContextKey defines type-safe keys for request context values.
type ContextKey string

const (
	UserIDContextKey   ContextKey = "user_id"
	UsernameContextKey ContextKey = "username"
	PlanContextKey     ContextKey = "plan"
)

// AuthClient là interface cho Auth Service client
type AuthClient interface {
	ValidateToken(ctx context.Context, in *pb.ValidateTokenRequest, opts ...grpc.CallOption) (*pb.ValidateTokenResponse, error)
}

// AuthMiddleware xác thực JWT token bằng cách gọi Auth Service
func AuthMiddleware(authClient AuthClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			corrID := middleware.GetCorrelationID(r.Context())

			// Lấy token từ Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Warn().
					Str("correlation_id", corrID).
					Msg("Missing Authorization header")

				middleware.WriteErrorResponse(w, r.Context(), http.StatusUnauthorized, "Unauthorized", "UNAUTHENTICATED", "Missing Authorization header")
				return
			}

			// Parse Bearer token
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				log.Warn().
					Str("correlation_id", corrID).
					Msg("Invalid Authorization header format")

				middleware.WriteErrorResponse(w, r.Context(), http.StatusUnauthorized, "Unauthorized", "UNAUTHENTICATED", "Invalid Authorization header format")
				return
			}

			token := parts[1]

			// Gọi Auth Service để validate token
			ctx := r.Context()
			resp, err := authClient.ValidateToken(ctx, &pb.ValidateTokenRequest{
				Token: token,
			})

			if err != nil {
				// Xử lý lỗi gRPC
				httpStatus, msg, code := middleware.HandleGRPCError(err)
				log.Warn().
					Str("correlation_id", corrID).
					Err(err).
					Msg("Token validation failed")

				middleware.WriteErrorResponse(w, r.Context(), httpStatus, msg, code, err.Error())
				return
			}

			// Check response
			if !resp.Valid {
				log.Warn().
					Str("correlation_id", corrID).
					Str("error", resp.Error).
					Msg("Token is invalid")

				middleware.WriteErrorResponse(w, r.Context(), http.StatusUnauthorized, "Unauthorized", "UNAUTHENTICATED", resp.Error)
				return
			}

			// Thêm thông tin user vào context
			ctx = context.WithValue(ctx, UserIDContextKey, resp.UserId)
			ctx = context.WithValue(ctx, UsernameContextKey, resp.Username)
			ctx = context.WithValue(ctx, PlanContextKey, resp.Plan)
			r = r.WithContext(ctx)

			log.Info().
				Str("correlation_id", corrID).
				Str("user_id", resp.UserId).
				Str("username", resp.Username).
				Str("plan", resp.Plan).
				Msg("User authenticated successfully")

			next.ServeHTTP(w, r)
		})
	}
}

// GetUserID lấy user ID từ context
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDContextKey).(string); ok {
		return userID
	}
	return ""
}

// GetUsername lấy username từ context
func GetUsername(ctx context.Context) string {
	if username, ok := ctx.Value(UsernameContextKey).(string); ok {
		return username
	}
	return ""
}

// GetPlan lấy plan từ context
func GetPlan(ctx context.Context) string {
	if plan, ok := ctx.Value(PlanContextKey).(string); ok {
		return plan
	}
	return ""
}
