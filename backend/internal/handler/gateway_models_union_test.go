package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type modelUnionKeyRepo struct {
	service.APIKeyRepository
	key     *service.APIKey
	touches int
}

func (r *modelUnionKeyRepo) GetByKeyForAuth(context.Context, string) (*service.APIKey, error) {
	copy := *r.key
	return &copy, nil
}
func (r *modelUnionKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error {
	r.touches++
	return nil
}

type modelUnionGroupRepo struct {
	service.GroupRepository
	groups map[int64]*service.Group
	failID int64
}

func (r *modelUnionGroupRepo) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	if id == r.failID {
		return nil, errors.New("group database unavailable")
	}
	if group := r.groups[id]; group != nil {
		return group, nil
	}
	return nil, service.ErrGroupNotFound
}

type modelUnionAccountRepo struct {
	gatewayModelsAccountRepoStub
	failID int64
}

func (r *modelUnionAccountRepo) ListSchedulableByGroupID(ctx context.Context, id int64) ([]service.Account, error) {
	if id == r.failID {
		return nil, errors.New("account database unavailable")
	}
	return r.gatewayModelsAccountRepoStub.ListSchedulableByGroupID(ctx, id)
}

type modelUnionFixture struct {
	key      *service.APIKey
	keys     *modelUnionKeyRepo
	groups   *modelUnionGroupRepo
	accounts *modelUnionAccountRepo
}

func newModelUnionFixture() *modelUnionFixture {
	groups := &modelUnionGroupRepo{groups: map[int64]*service.Group{
		1: {ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, RateMultiplier: 9},
		2: {ID: 2, Platform: service.PlatformGrok, Status: service.StatusActive, Hydrated: true, RateMultiplier: 0.1},
		3: {ID: 3, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, IsExclusive: true},
		4: {ID: 4, Platform: service.PlatformOpenAI, Status: service.StatusDisabled, Hydrated: true},
	}}
	primary := int64(2)
	key := &service.APIKey{ID: 17, Key: "model-union-test-key", UserID: 19, Status: service.StatusActive,
		GroupID: &primary, GroupIDs: []int64{2, 1, 3, 4, 5}, Group: groups.groups[2],
		User: &service.User{ID: 19, Status: service.StatusActive, Balance: 0, RestrictPublicGroups: true, AllowedGroups: []int64{1, 2, 4}},
	}
	account := func(id int64, platform string, models ...string) service.Account {
		mapping := map[string]any{}
		for _, model := range models {
			mapping[model] = model
		}
		return service.Account{ID: id, Platform: platform, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": mapping}}
	}
	return &modelUnionFixture{key: key, keys: &modelUnionKeyRepo{key: key}, groups: groups,
		accounts: &modelUnionAccountRepo{gatewayModelsAccountRepoStub: gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
			1: {account(11, service.PlatformOpenAI, "catalog-alpha", "shared")},
			2: {account(12, service.PlatformGrok, "catalog-beta", "shared")},
			3: {account(13, service.PlatformOpenAI, "private-secret")},
			4: {account(14, service.PlatformOpenAI, "disabled-secret")},
		}}},
	}
}

func (f *modelUnionFixture) serve(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RunMode: config.RunModeStandard}
	keys := service.NewAPIKeyService(f.keys, nil, f.groups, nil, nil, nil, cfg)
	// A directory request must not need a scheduler, a price lookup or wallet funds.
	h := newGatewayModelsHandlerForTest(f.accounts)
	router := gin.New()
	router.Use(gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(keys, nil, cfg)))
	router.GET("/v1/models", h.Models)
	router.GET("/models", h.Models)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+f.key.Key)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestGatewayModelCatalogUnionUsesAllAuthorizedGroupsWithoutRouting(t *testing.T) {
	for _, path := range []string{"/v1/models", "/models"} {
		t.Run(path, func(t *testing.T) {
			f := newModelUnionFixture()
			response := f.serve(t, path)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var got gatewayModelsResponseForTest
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &got))
			require.Equal(t, []string{"catalog-alpha", "catalog-beta", "shared"}, modelIDsForTest(got.Data))
			require.Equal(t, "openai", got.Data[2].OwnedBy, "lowest group ID wins duplicate metadata, not cheapest or primary group")
			require.EqualValues(t, 2, *f.key.GroupID, "cached auth key stays unchanged")
			require.Same(t, f.groups.groups[2], f.key.Group)
			require.Zero(t, f.keys.touches, "metadata listing must not make a key appear used")
		})
	}
}

func TestGatewayModelCatalogUnionFiltersEachGroupBeforeUnion(t *testing.T) {
	f := newModelUnionFixture()
	f.groups.groups[1].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"catalog-alpha"}}
	f.accounts.byGroup[1][0].AccountGroups = []service.AccountGroup{{GroupID: 1, AllowedModels: []string{"shared"}}}
	response := f.serve(t, "/v1/models")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &got))
	require.Equal(t, []string{"catalog-beta", "shared"}, modelIDsForTest(got.Data), "first group's allowlist/membership must neither leak alpha nor hide another group's shared model")
}

func TestGatewayModelCatalogUnionNeverExpandsAnEmptyGroupToDefaults(t *testing.T) {
	f := newModelUnionFixture()
	delete(f.accounts.byGroup, 1)
	delete(f.accounts.byGroup, 2)
	response := f.serve(t, "/v1/models")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var got struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &got))
	require.NotNil(t, got.Data)
	require.Empty(t, got.Data)
}

func TestGatewayModelCatalogUnionSurfacesReadFailures(t *testing.T) {
	for _, scope := range []string{"groups", "accounts"} {
		t.Run(scope, func(t *testing.T) {
			f := newModelUnionFixture()
			if scope == "groups" {
				f.groups.failID = 1
			} else {
				f.accounts.failID = 1
			}
			response := f.serve(t, "/v1/models")
			require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
			require.NotContains(t, response.Body.String(), "database unavailable")
			require.NotContains(t, response.Body.String(), "catalog-beta", "failed union cannot return partial results")
		})
	}
}

func TestGatewayModelCatalogUnionKeepsKeyValidityChecks(t *testing.T) {
	for _, status := range []string{service.StatusDisabled, service.StatusAPIKeyExpired, service.StatusAPIKeyQuotaExhausted} {
		t.Run(status, func(t *testing.T) {
			f := newModelUnionFixture()
			f.key.Status = status
			response := f.serve(t, "/v1/models")
			require.GreaterOrEqual(t, response.Code, 400)
			require.NotContains(t, response.Body.String(), "catalog-alpha")
		})
	}
}
