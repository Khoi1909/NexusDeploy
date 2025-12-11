package grpc

import (
	"context"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	CorrelationIDKey = "correlation-id"
)

// ClientConfig chứa cấu hình cho gRPC client
type ClientConfig struct {
	Address     string
	Timeout     time.Duration
	MaxRetries  int
	ServiceName string
}

// NewClient tạo gRPC client connection với retry và timeout
func NewClient(ctx context.Context, cfg ClientConfig) (*grpc.ClientConn, error) {
	// Retry options
	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithMax(uint(cfg.MaxRetries)),
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(100 * time.Millisecond)),
		grpc_retry.WithCodes(codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted),
	}

	// Dial options
	dialOpts := []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(
			grpc_retry.UnaryClientInterceptor(retryOpts...),
			correlationIDInterceptor,
		),
		grpc.WithInsecure(), // TODO: Thêm TLS trong production
	}

	// Tạo connection với timeout
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.Address, dialOpts...)
	if err != nil {
		log.Error().
			Str("service", cfg.ServiceName).
			Str("address", cfg.Address).
			Err(err).
			Msg("Failed to connect to gRPC service")
		return nil, err
	}

	log.Info().
		Str("service", cfg.ServiceName).
		Str("address", cfg.Address).
		Msg("Connected to gRPC service")

	return conn, nil
}

// correlationIDInterceptor là interceptor để truyền correlation ID qua gRPC metadata
func correlationIDInterceptor(
	ctx context.Context,
	method string,
	req interface{},
	reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	// Lấy correlation ID từ context
	corrID := getCorrelationID(ctx)
	if corrID != "" {
		// Thêm vào metadata
		md := metadata.New(map[string]string{
			CorrelationIDKey: corrID,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	return invoker(ctx, method, req, reply, cc, opts...)
}

// getCorrelationID lấy correlation ID từ context
func getCorrelationID(ctx context.Context) string {
	// Thử lấy từ context value
	if corrID, ok := ctx.Value("correlation_id").(string); ok {
		return corrID
	}

	// Thử lấy từ metadata (nếu đã có)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(CorrelationIDKey); len(values) > 0 {
			return values[0]
		}
	}

	return ""
}

// GetCorrelationIDFromContext lấy correlation ID từ context (public function)
func GetCorrelationIDFromContext(ctx context.Context) string {
	return getCorrelationID(ctx)
}

// AddCorrelationIDToContext thêm correlation ID vào context
func AddCorrelationIDToContext(ctx context.Context, corrID string) context.Context {
	return context.WithValue(ctx, "correlation_id", corrID)
}

// IsRetryableError kiểm tra xem lỗi có thể retry được không
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	st, ok := status.FromError(err)
	if !ok {
		return false
	}

	// Các code có thể retry
	retryableCodes := []codes.Code{
		codes.Unavailable,
		codes.DeadlineExceeded,
		codes.ResourceExhausted,
		codes.Internal,
	}

	for _, code := range retryableCodes {
		if st.Code() == code {
			return true
		}
	}

	return false
}
