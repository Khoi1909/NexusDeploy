package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nexusdeploy/backend/services/api-gateway/handlers"
	apimw "github.com/nexusdeploy/backend/services/api-gateway/middleware"
	projectpb "github.com/nexusdeploy/backend/services/project-service/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeProjectClient struct {
	resp *projectpb.ListProjectsResponse
	err  error
	last *projectpb.ListProjectsRequest
}

func (f *fakeProjectClient) ListProjects(ctx context.Context, in *projectpb.ListProjectsRequest, opts ...grpc.CallOption) (*projectpb.ListProjectsResponse, error) {
	f.last = in
	return f.resp, f.err
}

func TestProjectHandler_ListProjects_Success(t *testing.T) {
	client := &fakeProjectClient{
		resp: &projectpb.ListProjectsResponse{
			Projects: []*projectpb.Project{
				{
					Id:           "project-1",
					Name:         "demo",
					RepoUrl:      "https://github.com/user/demo",
					Preset:       "nodejs",
					GithubRepoId: 1234,
					IsPrivate:    true,
					CreatedAt:    timestamppb.New(time.Unix(1700000000, 0)),
					UpdatedAt:    timestamppb.New(time.Unix(1700003600, 0)),
				},
			},
			Total: 1,
		},
	}

	handler := handlers.NewProjectHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/api/projects?page=2&page_size=5", nil)
	ctx := context.WithValue(req.Context(), apimw.UserIDContextKey, "user-123")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ListProjects(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, client.last)
	require.Equal(t, int32(2), client.last.Page)
	require.Equal(t, int32(5), client.last.PageSize)
	require.Equal(t, "user-123", client.last.UserId)

	var payload handlers.ListProjectsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Projects, 1)
	require.Equal(t, "project-1", payload.Projects[0].ID)
	require.Equal(t, time.Unix(1700000000, 0).UTC(), payload.Projects[0].CreatedAt)
	require.Equal(t, int32(1), payload.Total)
}

func TestProjectHandler_ListProjects_MissingUser(t *testing.T) {
	handler := handlers.NewProjectHandler(&fakeProjectClient{})

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()

	handler.ListProjects(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestProjectHandler_ListProjects_GRPCError(t *testing.T) {
	client := &fakeProjectClient{
		err: status.Error(codes.Unavailable, "service unavailable"),
	}
	handler := handlers.NewProjectHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req = req.WithContext(context.WithValue(req.Context(), apimw.UserIDContextKey, "user-1"))
	rec := httptest.NewRecorder()

	handler.ListProjects(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
