package requestledger

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIngressPersistenceFailureNeverReachesAuthentication(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectBegin().WillReturnError(errors.New("fixture storage unavailable"))
	r := gin.New()
	r.Use(Middleware(New(db)))
	reached := false
	r.POST("/v1/responses", func(c *gin.Context) { reached = true; c.Status(200) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.False(t, reached)
	require.Contains(t, w.Body.String(), "request_ledger_unavailable")
	require.NotContains(t, w.Body.String(), "fixture")
	require.Equal(t, "1", w.Header().Get("Retry-After"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestManagedPathsIncludeAliasesAndEarlyRejections(t *testing.T) {
	for _, path := range []string{
		"/v1/responses", "/v1/unknown", "/v1beta/models/x:generateContent", "/responses",
		"/responses/input_tokens", "/backend-api/codex/responses", "/images/generations/async",
		"/v1/images/batches/123/cancel", "/api/v3/contents/generations/tasks/x",
		"/contents/generations/tasks", "/antigravity/models", "/antigravity/v1/messages",
		"/api/v1/image-studio/generate", "/api/v1/nextchat/image-studio/jobs/x",
		"/api/v1/mobile/video/jobs", "/api/v1/mobile/web-search", "/realtime", "/agnesapi",
	} {
		require.True(t, ManagedPath(path), path)
	}
	for _, path := range []string{"/", "/api/v1/usage", "/api/v1/admin/users", "/v10/responses", "/imagesfoo"} {
		require.False(t, ManagedPath(path), path)
	}
}

func TestOutcomeSeparatesCancellationTimeoutAndHTTPFailure(t *testing.T) {
	for _, tc := range []struct {
		status int
		err    error
		want   string
	}{
		{200, nil, "succeeded"}, {401, nil, "failed"}, {429, nil, "failed"},
		{400, nil, "failed"}, {500, nil, "failed"}, {504, nil, "timeout"},
		{200, context.Canceled, "cancelled"}, {200, context.DeadlineExceeded, "timeout"},
	} {
		require.Equal(t, tc.want, Outcome(tc.status, tc.err))
	}
}

func TestUnsafeErrorsNeverBecomeAuditMetadata(t *testing.T) {
	require.Equal(t, "upstream_error", ErrorCode(errors.New("Authorization: Bearer secret prompt=private")))
	require.Equal(t, "cancelled", ErrorCode(context.Canceled))
	require.Equal(t, "timeout", ErrorCode(context.DeadlineExceeded))
}
