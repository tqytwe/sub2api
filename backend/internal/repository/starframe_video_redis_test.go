package repository

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type starframeRedisUpstream struct {
	service.HTTPUpstream
	posts int
}

func (u *starframeRedisUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	u.posts++
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":"original","status":"queued"}`))}, nil
}
func TestStarframeActualRedisLoss(t *testing.T) {
	addr := os.Getenv("STARFRAME_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("isolated Redis not configured")
	}
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer func() { _ = client.Close() }()
	require.NoError(t, client.Ping(ctx).Err())
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "ledger.db"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	ddl, err := os.ReadFile("../../migrations/272_starframe_video_submissions.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(ddl))
	require.NoError(t, err)
	upstream := &starframeRedisUpstream{}
	build := func() *service.OpenAIGatewayService {
		return service.NewOpenAIGatewayServiceWithLiveBilling(nil, nil, nil, nil, nil, nil, NewGatewayCache(client), &config.Config{}, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, NewStarframeVideoRepository(db), nil)
	}
	svc := build()
	owner := service.StarframeVideoOwner{UserID: 10, APIKeyID: 20, GroupID: 30}
	account := &service.Account{ID: 9, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-only", "base_url": "https://relay.example/v1", "video_protocol": "starframe", "openai_capabilities": []string{"starframe"}}}
	body := []byte(`{"model":"ch-custom","prompt":"waves","mode":"references","client_task_id":"redis-loss","duration":5,"resolution":"720p"}`)
	billing := &service.StarframeVideoBilling{Model: "ch-custom", Duration: 5, Resolution: "720p", UnitPrice: 0.037}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/videos", nil)
	result, err := svc.ForwardStarframeVideo(ctx, c, account, service.AgnesVideoEndpointCreate, nil, body, owner, billing)
	require.NoError(t, err)
	keys, err := client.Keys(ctx, "*"+result.ResponseID).Result()
	require.NoError(t, err)
	require.NotEmpty(t, keys)
	for _, key := range keys {
		require.NoError(t, client.Expire(ctx, key, -time.Second).Err())
	}
	restarted := build()
	_, err = restarted.LoadStarframeVideoTask(ctx, result.ResponseID, owner)
	require.NoError(t, err)
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/videos", nil)
	_, err = restarted.ForwardStarframeVideo(ctx, c, account, service.AgnesVideoEndpointCreate, nil, body, owner, billing)
	require.Error(t, err)
	require.Equal(t, 1, upstream.posts)
}
