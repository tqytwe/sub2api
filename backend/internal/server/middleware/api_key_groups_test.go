package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type multiGroupAuthKeyRepo struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r multiGroupAuthKeyRepo) GetByKeyForAuth(context.Context, string) (*service.APIKey, error) {
	copy := *r.key
	return &copy, nil
}
func (r multiGroupAuthKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error { return nil }

type multiGroupAuthGroupRepo struct {
	service.GroupRepository
	groups map[int64]*service.Group
}

func (r multiGroupAuthGroupRepo) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	return r.groups[id], nil
}

type multiGroupAuthRouter struct{ probes *int }

func (r multiGroupAuthRouter) ProbeAPIKeyGroup(context.Context, *service.Group, string, string) (service.APIKeyGroupAvailability, error) {
	*r.probes++
	return service.APIKeyGroupAvailability{Available: true}, nil
}

func TestMultiGroupWebSocketDefersUntilModelAndExcludesUnsupportedPlatforms(t *testing.T) {
	for _, mode := range []string{config.RunModeSimple, config.RunModeStandard} {
		t.Run(mode, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			primary := int64(1)
			user := &service.User{ID: 9, Status: service.StatusActive, Concurrency: 1, Balance: 10}
			if mode == config.RunModeSimple {
				user.Balance = 0
			}
			groups := map[int64]*service.Group{
				1: {ID: 1, Platform: service.PlatformAnthropic, Status: service.StatusActive, Hydrated: true, RateMultiplier: 0.1},
				2: {ID: 2, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, RateMultiplier: 1},
			}
			key := &service.APIKey{ID: 7, UserID: 9, Key: "test-multi-key", Status: service.StatusActive, GroupID: &primary, GroupIDs: []int64{1, 2}, Group: groups[1], User: user}
			cfg := &config.Config{RunMode: mode}
			svc := service.NewAPIKeyService(multiGroupAuthKeyRepo{key: key}, nil, multiGroupAuthGroupRepo{groups: groups}, nil, nil, nil, cfg)
			probes := 0
			svc.SetGroupRouter(multiGroupAuthRouter{probes: &probes})
			router := gin.New()
			router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
			router.GET("/v1/responses", func(c *gin.Context) {
				require.Zero(t, probes, "handshake has no model and must not probe a group")
				selected, err := ResolveDeferredAPIKeyRequestGroup(c, []string{"gpt-5.4"})
				require.NoError(t, err)
				require.EqualValues(t, 2, *selected.GroupID, "cheap unsupported platform must not win")
				current, _ := GetAPIKeyFromContext(c)
				require.Same(t, selected, current)
				group, _ := c.Request.Context().Value(ctxkey.Group).(*service.Group)
				require.EqualValues(t, 2, group.ID)
				selectedAgain, err := ResolveDeferredAPIKeyRequestGroup(c, []string{"changed-model"})
				require.NoError(t, err)
				require.Same(t, selected, selectedAgain, "resolved session must not reselect billing group")
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
			req.Header.Set("Authorization", "Bearer "+key.Key)
			req.Header.Set("Upgrade", "websocket")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
		})
	}
}

func TestMultiGroupImageRequestsExcludeUnsupportedPlatforms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	primary := int64(1)
	groups := map[int64]*service.Group{
		1: {ID: 1, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 0.1, AllowImageGeneration: true},
		2: {ID: 2, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 1, AllowImageGeneration: true},
	}
	key := &service.APIKey{ID: 7, UserID: 9, GroupID: &primary, GroupIDs: []int64{1, 2}, User: &service.User{ID: 9, Balance: 10}}
	probes := 0
	cfg := &config.Config{RunMode: config.RunModeStandard}
	svc := service.NewAPIKeyService(nil, nil, multiGroupAuthGroupRepo{groups: groups}, nil, nil, nil, cfg)
	svc.SetGroupRouter(multiGroupAuthRouter{probes: &probes})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"image-model"}`))
	selected, err := selectAPIKeyRequestGroup(c, svc, nil, cfg, key, false)
	require.NoError(t, err)
	require.EqualValues(t, 2, *selected.GroupID)
}

type multiGroupAuthSubscriptionRepo struct {
	service.UserSubscriptionRepository
	sub *service.UserSubscription
}

func (r multiGroupAuthSubscriptionRepo) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*service.UserSubscription, error) {
	if r.sub.UserID != userID || r.sub.GroupID != groupID {
		return nil, service.ErrSubscriptionNotFound
	}
	copy := *r.sub
	return &copy, nil
}

func TestMultiGroupDeferredSelectionLoadsSelectedSubscriptionNotPrimary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	primary := int64(1)
	groups := map[int64]*service.Group{
		1: {ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, RateMultiplier: 0.1},
		2: {ID: 2, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, RateMultiplier: 1, SubscriptionType: service.SubscriptionTypeSubscription},
	}
	key := &service.APIKey{ID: 7, UserID: 9, GroupID: &primary, GroupIDs: []int64{1, 2}, Group: groups[1], User: &service.User{ID: 9}}
	subRepo := multiGroupAuthSubscriptionRepo{sub: &service.UserSubscription{ID: 22, UserID: 9, GroupID: 2, Status: service.SubscriptionStatusActive, StartsAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(time.Hour)}}
	now := time.Now()
	subRepo.sub.DailyWindowStart, subRepo.sub.WeeklyWindowStart, subRepo.sub.MonthlyWindowStart = &now, &now, &now
	cfg := &config.Config{RunMode: config.RunModeStandard}
	groupRepo := multiGroupAuthGroupRepo{groups: groups}
	keys := service.NewAPIKeyService(nil, nil, groupRepo, subRepo, nil, nil, cfg)
	probes := 0
	keys.SetGroupRouter(multiGroupAuthRouter{probes: &probes})
	subscriptions := service.NewSubscriptionService(groupRepo, subRepo, nil, nil, cfg)
	t.Cleanup(subscriptions.Stop)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	c.Request.Header.Set("Upgrade", "websocket")
	_, err := selectAPIKeyRequestGroup(c, keys, subscriptions, cfg, key, false)
	require.NoError(t, err)
	require.Zero(t, probes)
	selected, err := ResolveDeferredAPIKeyRequestGroup(c, []string{"gpt-5.4"})
	require.NoError(t, err)
	require.EqualValues(t, 2, *selected.GroupID, "zero balance primary cannot block a usable subscription")
	subscription, ok := GetSubscriptionFromContext(c)
	require.True(t, ok)
	require.EqualValues(t, 22, subscription.ID)
	require.Equal(t, *selected.GroupID, subscription.GroupID)
}

func TestMultiGroupSelectionSkipsGroupsWithoutEndpointPermission(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"messages", "/v1/messages", `{"model":"gpt-5.4"}`},
		{"count tokens", "/v1/messages/count_tokens", `{"model":"gpt-5.4"}`},
		{"responses image tool", "/v1/responses", `{"model":"gpt-5.4","tools":[{"type":"image_generation"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			primary := int64(1)
			groups := map[int64]*service.Group{
				1: {ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 0.1},
				2: {ID: 2, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 1, AllowMessagesDispatch: true, AllowImageGeneration: true},
			}
			key := &service.APIKey{ID: 7, UserID: 9, GroupID: &primary, GroupIDs: []int64{1, 2}, User: &service.User{ID: 9, Balance: 10}}
			probes := 0
			cfg := &config.Config{RunMode: config.RunModeStandard}
			svc := service.NewAPIKeyService(nil, nil, multiGroupAuthGroupRepo{groups: groups}, nil, nil, nil, cfg)
			svc.SetGroupRouter(multiGroupAuthRouter{probes: &probes})
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			selected, err := selectAPIKeyRequestGroup(c, svc, nil, cfg, key, false)
			require.NoError(t, err)
			require.EqualValues(t, 2, *selected.GroupID)
		})
	}
}

func TestMultiGroupSelectionRespectsLiveAndVideoHandlers(t *testing.T) {
	for _, tc := range []struct {
		path, firstPlatform, secondPlatform string
		allowLive                           bool
	}{
		{"/v1/live", service.PlatformGrok, service.PlatformOpenAI, true},
		{"/v1/live", service.PlatformOpenAI, service.PlatformOpenAI, true},
		{"/v1/videos", service.PlatformAnthropic, service.PlatformOpenAI, false},
		{"/v1/videos/generations", service.PlatformOpenAI, service.PlatformGrok, false},
		{"/v1/videos/edits", service.PlatformOpenAI, service.PlatformGrok, false},
		{"/v1/videos/extensions", service.PlatformOpenAI, service.PlatformGrok, false},
	} {
		t.Run(tc.path+"/"+tc.firstPlatform, func(t *testing.T) {
			primary := int64(1)
			groups := map[int64]*service.Group{
				1: {ID: 1, Platform: tc.firstPlatform, Status: service.StatusActive, RateMultiplier: 0.1},
				2: {ID: 2, Platform: tc.secondPlatform, Status: service.StatusActive, RateMultiplier: 1, AllowLive: tc.allowLive},
			}
			key := &service.APIKey{ID: 7, UserID: 9, GroupID: &primary, GroupIDs: []int64{1, 2}, User: &service.User{ID: 9, Balance: 10}}
			probes := 0
			cfg := &config.Config{RunMode: config.RunModeStandard}
			svc := service.NewAPIKeyService(nil, nil, multiGroupAuthGroupRepo{groups: groups}, nil, nil, nil, cfg)
			svc.SetGroupRouter(multiGroupAuthRouter{probes: &probes})
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"model":"model","session":{"model":"model"}}`))
			c.Request.Header.Set("Content-Type", "application/json")
			selected, err := selectAPIKeyRequestGroup(c, svc, nil, cfg, key, false)
			require.NoError(t, err)
			require.EqualValues(t, 2, *selected.GroupID)
		})
	}
}

func TestMultiGroupSelectionRespectsAudioPlatformDispatch(t *testing.T) {
	for _, tc := range []struct{ path, firstPlatform, secondPlatform string }{
		{"/v1/audio/speech", service.PlatformAnthropic, service.PlatformOpenAI},
		{"/v1/audio/transcriptions", service.PlatformComposite, service.PlatformGrok},
		{"/v1/audio/translations", service.PlatformGrok, service.PlatformOpenAI},
		{"/v1/realtime", service.PlatformComposite, service.PlatformOpenAI},
		{"/v1/tts", service.PlatformOpenAI, service.PlatformGrok},
		{"/v1/stt", service.PlatformOpenAI, service.PlatformGrok},
		{"/v1/custom-voices", service.PlatformOpenAI, service.PlatformGrok},
	} {
		t.Run(tc.path, func(t *testing.T) {
			primary := int64(1)
			groups := map[int64]*service.Group{
				1: {ID: 1, Platform: tc.firstPlatform, Status: service.StatusActive, RateMultiplier: 0.1},
				2: {ID: 2, Platform: tc.secondPlatform, Status: service.StatusActive, RateMultiplier: 1},
			}
			key := &service.APIKey{ID: 7, UserID: 9, GroupID: &primary, GroupIDs: []int64{1, 2}, User: &service.User{ID: 9, Balance: 10}}
			probes := 0
			cfg := &config.Config{RunMode: config.RunModeStandard}
			svc := service.NewAPIKeyService(nil, nil, multiGroupAuthGroupRepo{groups: groups}, nil, nil, nil, cfg)
			svc.SetGroupRouter(multiGroupAuthRouter{probes: &probes})
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"model":"audio-model"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			selected, err := selectAPIKeyRequestGroup(c, svc, nil, cfg, key, false)
			require.NoError(t, err)
			require.EqualValues(t, 2, *selected.GroupID)
		})
	}
}
