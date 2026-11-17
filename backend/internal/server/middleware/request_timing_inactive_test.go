//go:build unit

package middleware

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestRequestTimingInactiveCredentialIsObservationOnly(t *testing.T) {
	cfg := &config.Config{RunMode: config.RunModeSimple}
	group := int64(8)
	repo := &stubApiKeyRepo{getByKey: func(_ context.Context, key string) (*service.APIKey, error) {
		if key != "known" {
			return nil, service.ErrAPIKeyNotFound
		}
		return &service.APIKey{ID: 7, UserID: 9, GroupID: &group, Group: &service.Group{ID: group, Status: service.StatusActive, Hydrated: true}, Status: service.StatusActive, User: &service.User{ID: 9, Status: "disabled", Username: "disabled-example"}}, nil
	}}
	svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
	for _, key := range []string{"known", "unknown"} {
		recorder := &connectionRecorderTest{}
		r := gin.New()
		require.NoError(t, r.SetTrustedProxies(nil))
		upstream := 0
		r.Use(RequestTiming(recorder), func(c *gin.Context) {
			c.Next()
			_, authenticated := GetAPIKeyFromContext(c)
			require.False(t, authenticated)
			_, user := GetAuthSubjectFromContext(c)
			require.False(t, user)
			require.Nil(t, c.Request.Context().Value(ctxkey.UserID))
		})
		r.POST("/v1/responses", gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)), func(c *gin.Context) { upstream++; c.Status(200) })
		req := httptest.NewRequest("POST", "/v1/responses", nil)
		req.RemoteAddr = "8.8.8.8:1234"
		req.Header.Set("x-api-key", key)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, 401, w.Code, w.Body.String())
		require.Zero(t, upstream)
		require.Len(t, recorder.records, 1)
		got := recorder.records[0]
		if key == "known" {
			require.EqualValues(t, 9, got.UserID)
			require.EqualValues(t, 7, got.APIKeyID)
			require.EqualValues(t, 8, got.GroupID)
			require.Equal(t, "USER_INACTIVE", got.Detail.AuthRejection)
			require.Equal(t, "recognized_credential", got.Detail.IdentitySource)
			require.Empty(t, got.Detail.Attempts)
		} else {
			require.Zero(t, got.UserID)
			require.Zero(t, got.APIKeyID)
		}
	}
}
