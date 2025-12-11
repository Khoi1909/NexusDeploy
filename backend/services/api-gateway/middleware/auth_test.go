package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexusdeploy/backend/pkg/middleware"
	apimw "github.com/nexusdeploy/backend/services/api-gateway/middleware"
	authpb "github.com/nexusdeploy/backend/services/auth-service/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeAuthClient struct {
	resp *authpb.ValidateTokenResponse
	err  error
	last *authpb.ValidateTokenRequest
}

func (f *fakeAuthClient) ValidateToken(ctx context.Context, in *authpb.ValidateTokenRequest, opts ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	f.last = in
	return f.resp, f.err
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	client := &fakeAuthClient{}
	handler := apimw.AuthMiddleware(client)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	client := &fakeAuthClient{}
	handler := apimw.AuthMiddleware(client)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token value")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_GRPCError(t *testing.T) {
	client := &fakeAuthClient{
		err: status.Error(codes.Unavailable, "service down"),
	}
	handler := apimw.AuthMiddleware(client)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestAuthMiddleware_Success(t *testing.T) {
	client := &fakeAuthClient{
		resp: &authpb.ValidateTokenResponse{
			Valid:    true,
			UserId:   "user-1",
			Username: "demo",
			Plan:     "standard",
		},
	}
	nextCalled := false
	handler := apimw.AuthMiddleware(client)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		require.Equal(t, "user-1", apimw.GetUserID(r.Context()))
		require.Equal(t, "demo", apimw.GetUsername(r.Context()))
		require.Equal(t, "standard", apimw.GetPlan(r.Context()))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), middleware.CorrelationIDKey, "corr-1")
	req = req.WithContext(ctx)
	req.Header.Set("Authorization", "Bearer token123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, nextCalled)
	require.NotNil(t, client.last)
	require.Equal(t, "token123", client.last.Token)
}
