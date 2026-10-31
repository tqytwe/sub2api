package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type liveMultiGroupStore struct {
	service.GatewayCache
	service.LiveCallStore
	record *service.LiveCallRecord
}

func (s liveMultiGroupStore) GetLiveCall(context.Context, string) (*service.LiveCallRecord, error) {
	copy := *s.record
	return &copy, nil
}

type liveMultiGroupKeyRepo struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r liveMultiGroupKeyRepo) GetByKeyForAuth(context.Context, string) (*service.APIKey, error) {
	copy := *r.key
	return &copy, nil
}
func (liveMultiGroupKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error { return nil }

type liveMultiGroupRepo struct {
	service.GroupRepository
	groups map[int64]*service.Group
}

func (r liveMultiGroupRepo) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	if g, ok := r.groups[id]; ok {
		return g, nil
	}
	return nil, service.ErrGroupNotFound
}

type liveMultiGroupRouter struct{ probes *int }

func (r liveMultiGroupRouter) ProbeAPIKeyGroup(context.Context, *service.Group, string, string) (service.APIKeyGroupAvailability, error) {
	*r.probes++
	return service.APIKeyGroupAvailability{Available: true}, nil
}

func TestLiveSidebandMultiGroupRestoresCallGroupAndRejectsUnauthorizedCallers(t *testing.T) {
	for _, route := range []string{"/v1/live/:call_id", "/backend-api/codex/:call_id"} {
		for _, tc := range []struct {
			name   string
			status int
			alter  func(*service.APIKey, *service.LiveCallRecord)
		}{
			{"original group despite cheaper primary", http.StatusBadRequest, func(*service.APIKey, *service.LiveCallRecord) {}},
			{"another key", http.StatusForbidden, func(_ *service.APIKey, r *service.LiveCallRecord) { r.APIKeyID++ }},
			{"another user", http.StatusForbidden, func(_ *service.APIKey, r *service.LiveCallRecord) { r.UserID++ }},
			{"binding removed", http.StatusForbidden, func(k *service.APIKey, _ *service.LiveCallRecord) { k.GroupIDs = []int64{1, 3} }},
			{"permission revoked", http.StatusForbidden, func(k *service.APIKey, _ *service.LiveCallRecord) { k.User.AllowedGroups = []int64{1} }},
			{"closed call", http.StatusNotFound, func(_ *service.APIKey, r *service.LiveCallRecord) { r.Controller = service.LiveControllerClosed }},
		} {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				gin.SetMode(gin.TestMode)
				primary := int64(1)
				groups := map[int64]*service.Group{
					1: {ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, AllowLive: true, RateMultiplier: 0.1},
					2: {ID: 2, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, AllowLive: true, RateMultiplier: 10},
				}
				key := &service.APIKey{ID: 7, UserID: 8, Key: "test-live-key", Status: service.StatusActive, GroupID: &primary, GroupIDs: []int64{1, 2}, Group: groups[1], User: &service.User{ID: 8, Status: service.StatusActive, RestrictPublicGroups: true, AllowedGroups: []int64{1, 2}}}
				record := &service.LiveCallRecord{CallID: "call_fixed_group", APIKeyID: 7, UserID: 8, GroupID: 2, Controller: service.LiveControllerPending}
				tc.alter(key, record)
				cfg := &config.Config{RunMode: config.RunModeSimple, Security: config.SecurityConfig{AccountSessionEgressEnabled: true}}
				keys := service.NewAPIKeyService(liveMultiGroupKeyRepo{key: key}, nil, liveMultiGroupRepo{groups: groups}, nil, nil, nil, cfg)
				probes := 0
				keys.SetGroupRouter(liveMultiGroupRouter{probes: &probes})
				gateway := service.NewOpenAIGatewayService(nil, nil, nil, nil, nil, nil, liveMultiGroupStore{record: record}, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
				h := &OpenAIGatewayHandler{gatewayService: gateway, apiKeyService: keys, cfg: cfg}
				router := gin.New()
				router.Use(gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(keys, nil, cfg)))
				router.GET(route, func(c *gin.Context) {
					h.LiveSideband(c)
					if tc.status == http.StatusBadRequest {
						selected, ok := middleware.GetAPIKeyFromContext(c)
						require.True(t, ok)
						require.EqualValues(t, 2, *selected.GroupID)
					}
				})
				req := httptest.NewRequest(http.MethodGet, strings.ReplaceAll(route, ":call_id", "call_fixed_group"), nil)
				req.Header.Set("Authorization", "Bearer "+key.Key)
				// An incomplete handshake stops before dialing upstream, after auth
				// and the stored call's group have both been verified.
				req.Header.Set("Connection", "Upgrade")
				req.Header.Set("Upgrade", "websocket")
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
				require.Zero(t, probes, "a sideband must not reselect an existing call by price or load")
			})
		}
	}
}
