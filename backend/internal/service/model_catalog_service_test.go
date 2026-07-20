package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type modelCatalogVisibilityRepoStub struct {
	ModelCatalogRepository
	entries []SiteModelCatalogEntry
	err     error
}

func (r *modelCatalogVisibilityRepoStub) ListCatalog(_ context.Context, filter CatalogListFilter) ([]SiteModelCatalogEntry, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make([]SiteModelCatalogEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		if filter.VisiblePublic != nil && entry.VisiblePublic != *filter.VisiblePublic {
			continue
		}
		if filter.VisibleAuth != nil && entry.VisibleAuth != *filter.VisibleAuth {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

type modelPricingSettingRepoStub struct {
	SettingRepository
	values map[string]string
}

func (r *modelPricingSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

type modelPricingUserRepoStub struct {
	UserRepository
	user *User
}

func (r *modelPricingUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	return r.user, nil
}

type modelPricingGroupRepoStub struct {
	GroupRepository
	groups []Group
}

func (r *modelPricingGroupRepoStub) ListActive(context.Context) ([]Group, error) {
	return r.groups, nil
}

type modelPricingSubscriptionRepoStub struct {
	UserSubscriptionRepository
}

func (r *modelPricingSubscriptionRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return []UserSubscription{}, nil
}

type modelPricingUserRateRepoStub struct {
	UserGroupRateRepository
	rates map[int64]float64
}

func (r *modelPricingUserRateRepoStub) GetByUserID(context.Context, int64) (map[int64]float64, error) {
	return r.rates, nil
}

type modelPricingChannelRepoStub struct {
	ChannelRepository
	channels []Channel
}

type modelPricingAPIKeyRepoStub struct {
	APIKeyRepository
	keys []APIKey
}

func (r *modelPricingAPIKeyRepoStub) ListAllByUserID(context.Context, int64, APIKeyListFilters) ([]APIKey, error) {
	return r.keys, nil
}

func (r *modelPricingChannelRepoStub) ListAll(context.Context) ([]Channel, error) {
	return r.channels, nil
}

func modelPricingSettingService(multiplier string) *SettingService {
	return NewSettingService(&modelPricingSettingRepoStub{values: map[string]string{
		SettingKeyAvailableChannelsEnabled:  "true",
		SettingKeyPublicModelRateMultiplier: multiplier,
	}}, &config.Config{})
}

func TestModelCatalogService_ListPublicPricingFailsClosed(t *testing.T) {
	t.Run("no guest-visible models", func(t *testing.T) {
		svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{}, nil, nil, nil, nil, nil, nil)
		require.Empty(t, svc.ListPublicPricing(context.Background()))
	})

	t.Run("catalog query error", func(t *testing.T) {
		svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{err: errors.New("database unavailable")}, nil, nil, nil, nil, nil, nil)
		require.Empty(t, svc.ListPublicPricing(context.Background()))
	})
}

func TestModelCatalogService_ListMyPricingShowsVisibleCatalogWithoutChannelMatch(t *testing.T) {
	officialAIn, officialAOut := 10e-6, 20e-6
	officialBIn, officialBOut := 5e-6, 30e-6
	explicitSiteAIn := 2e-6
	repo := &modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{
		{
			ModelName:           "hidden-model",
			Platform:            PlatformOpenAI,
			VisibleAuth:         false,
			SortOrder:           1,
			OfficialInputPrice:  &officialAIn,
			OfficialOutputPrice: &officialAOut,
		},
		{
			ModelName:           "model-b",
			Platform:            PlatformOpenAI,
			VisibleAuth:         true,
			SortOrder:           20,
			OfficialInputPrice:  &officialBIn,
			OfficialOutputPrice: &officialBOut,
		},
		{
			ModelName:           "model-a",
			Platform:            PlatformAnthropic,
			VisibleAuth:         true,
			SortOrder:           10,
			OfficialInputPrice:  &officialAIn,
			OfficialOutputPrice: &officialAOut,
			InputPrice:          &explicitSiteAIn,
		},
	}}
	svc := NewModelCatalogService(repo, nil, nil, nil, modelPricingSettingService("0.8"), nil, nil)

	resp, err := svc.ListMyPricing(context.Background(), 4)

	require.NoError(t, err)
	require.True(t, resp.Enabled)
	require.Len(t, resp.Models, 2)
	require.Equal(t, "model-a", resp.Models[0].Name)
	require.Equal(t, "model-b", resp.Models[1].Name)
	require.Empty(t, resp.Models[0].Groups)
	require.Nil(t, resp.Models[0].EffectiveInputPrice)
	require.Nil(t, resp.Models[0].EffectiveOutputPrice)
	require.Equal(t, explicitSiteAIn, *resp.Models[0].SiteInputPrice)
	require.InDelta(t, 20e-6, *resp.Models[0].SiteOutputPrice, 1e-12)
	require.InDelta(t, 5e-6, *resp.Models[1].SiteInputPrice, 1e-12)
	require.InDelta(t, 30e-6, *resp.Models[1].SiteOutputPrice, 1e-12)
}

func TestModelCatalogService_ListMyPricingKeepsChannelEffectivePricing(t *testing.T) {
	baseIn, baseOut := 2e-6, 8e-6
	officialIn, officialOut := 5e-6, 30e-6
	group := Group{
		ID:               2,
		Name:             "codex",
		Platform:         PlatformOpenAI,
		RateMultiplier:   0.18,
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeStandard,
	}
	groupRepo := &modelPricingGroupRepoStub{groups: []Group{group}}
	channelService := NewChannelService(&modelPricingChannelRepoStub{channels: []Channel{{
		ID:       1,
		Name:     "primary",
		Status:   StatusActive,
		GroupIDs: []int64{group.ID},
		ModelPricing: []ChannelModelPricing{{
			Platform:    PlatformOpenAI,
			Models:      []string{"gpt-test"},
			BillingMode: BillingModeToken,
			InputPrice:  &baseIn,
			OutputPrice: &baseOut,
		}},
	}}}, groupRepo, nil, nil)
	apiKeyService := NewAPIKeyService(
		nil,
		&modelPricingUserRepoStub{user: &User{ID: 4, Status: StatusActive}},
		groupRepo,
		&modelPricingSubscriptionRepoStub{},
		&modelPricingUserRateRepoStub{rates: map[int64]float64{group.ID: 0.25}},
		nil,
		nil,
	)
	repo := &modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{{
		ModelName:           "gpt-test",
		Platform:            PlatformOpenAI,
		VisibleAuth:         true,
		SortOrder:           1,
		OfficialInputPrice:  &officialIn,
		OfficialOutputPrice: &officialOut,
	}}}
	svc := NewModelCatalogService(repo, channelService, nil, nil, modelPricingSettingService("0.8"), apiKeyService, nil)

	resp, err := svc.ListMyPricing(context.Background(), 4)

	require.NoError(t, err)
	require.Len(t, resp.Models, 1)
	row := resp.Models[0]
	require.Equal(t, "primary", row.Channel)
	require.Len(t, row.Groups, 1)
	require.Equal(t, group.ID, row.Groups[0].ID)
	require.Equal(t, 0.25, row.Groups[0].RateMultiplier)
	require.Equal(t, baseIn, *row.BaseInputPrice)
	require.Equal(t, baseOut, *row.BaseOutputPrice)
	require.InDelta(t, 0.5e-6, *row.EffectiveInputPrice, 1e-12)
	require.InDelta(t, 2e-6, *row.EffectiveOutputPrice, 1e-12)
	require.InDelta(t, 5e-6, *row.SiteInputPrice, 1e-12)
	require.InDelta(t, 30e-6, *row.SiteOutputPrice, 1e-12)
}

func TestModelCatalogService_ListMyPricingUsesSiteBaseForActualKeyGroupWithoutChannel(t *testing.T) {
	siteIn, siteOut := 4e-6, 24e-6
	group := Group{
		ID:               2,
		Name:             "codex",
		Platform:         PlatformOpenAI,
		RateMultiplier:   0.18,
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeStandard,
	}
	apiKeyService := NewAPIKeyService(
		&modelPricingAPIKeyRepoStub{keys: []APIKey{{
			ID:      8,
			UserID:  4,
			GroupID: &group.ID,
			Group:   &group,
			Status:  StatusActive,
		}}},
		&modelPricingUserRepoStub{user: &User{ID: 4, Status: StatusActive}},
		&modelPricingGroupRepoStub{groups: []Group{group}},
		&modelPricingSubscriptionRepoStub{},
		&modelPricingUserRateRepoStub{rates: map[int64]float64{group.ID: 0.25}},
		nil,
		nil,
	)
	repo := &modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{{
		ModelName:   "gpt-test",
		Platform:    PlatformOpenAI,
		VisibleAuth: true,
		InputPrice:  &siteIn,
		OutputPrice: &siteOut,
		BillingMode: string(BillingModeToken),
	}}}
	svc := NewModelCatalogService(repo, nil, nil, nil, modelPricingSettingService("1"), apiKeyService, nil)

	resp, err := svc.ListMyPricing(context.Background(), 4)

	require.NoError(t, err)
	require.Len(t, resp.Models, 1)
	row := resp.Models[0]
	require.Empty(t, row.Channel)
	require.Len(t, row.Groups, 1)
	require.Equal(t, group.ID, row.Groups[0].ID)
	require.Equal(t, 0.25, row.Groups[0].RateMultiplier)
	require.InDelta(t, siteIn, *row.BaseInputPrice, 1e-12)
	require.InDelta(t, siteOut, *row.BaseOutputPrice, 1e-12)
	require.InDelta(t, 1e-6, *row.EffectiveInputPrice, 1e-12)
	require.InDelta(t, 6e-6, *row.EffectiveOutputPrice, 1e-12)
}

func TestModelCatalogService_ExplicitCatalogGroupsOverridePlatformMatching(t *testing.T) {
	siteIn, siteOut := 4e-6, 24e-6
	codex := Group{ID: 2, Name: "codex", Platform: PlatformOpenAI, RateMultiplier: 0.18, Status: StatusActive}
	domestic := Group{ID: 14, Name: "国产分组", Platform: PlatformOpenAI, RateMultiplier: 0.05, Status: StatusActive}
	apiKeyService := NewAPIKeyService(
		&modelPricingAPIKeyRepoStub{keys: []APIKey{
			{ID: 8, UserID: 4, GroupID: &codex.ID, Group: &codex, Status: StatusActive},
			{ID: 9, UserID: 4, GroupID: &domestic.ID, Group: &domestic, Status: StatusActive},
		}},
		&modelPricingUserRepoStub{user: &User{ID: 4, Status: StatusActive}},
		&modelPricingGroupRepoStub{groups: []Group{codex, domestic}},
		&modelPricingSubscriptionRepoStub{},
		&modelPricingUserRateRepoStub{rates: map[int64]float64{}},
		nil,
		nil,
	)
	repo := &modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{{
		ModelName:   "qwen3.5-plus",
		Platform:    PlatformOpenAI,
		VisibleAuth: true,
		GroupIDs:    []int64{domestic.ID},
		InputPrice:  &siteIn,
		OutputPrice: &siteOut,
	}}}
	svc := NewModelCatalogService(repo, nil, nil, nil, modelPricingSettingService("1"), apiKeyService, nil)

	resp, err := svc.ListMyPricing(context.Background(), 4)

	require.NoError(t, err)
	require.Len(t, resp.Models, 1)
	require.Len(t, resp.Models[0].Groups, 1)
	require.Equal(t, domestic.ID, resp.Models[0].Groups[0].ID)
	require.Equal(t, domestic.Name, resp.Models[0].Groups[0].Name)
	require.InDelta(t, siteIn*domestic.RateMultiplier, *resp.Models[0].EffectiveInputPrice, 1e-12)
}

func TestModelCatalogService_ExplicitCatalogGroupUsesAvailableGroupWithoutKey(t *testing.T) {
	siteIn, siteOut := 4e-6, 24e-6
	domestic := Group{
		ID:               14,
		Name:             "国产分组",
		Platform:         PlatformOpenAI,
		RateMultiplier:   0.05,
		Status:           StatusActive,
		SubscriptionType: SubscriptionTypeStandard,
	}
	apiKeyService := NewAPIKeyService(
		&modelPricingAPIKeyRepoStub{keys: []APIKey{}},
		&modelPricingUserRepoStub{user: &User{ID: 2, Status: StatusActive}},
		&modelPricingGroupRepoStub{groups: []Group{domestic}},
		&modelPricingSubscriptionRepoStub{},
		&modelPricingUserRateRepoStub{rates: map[int64]float64{}},
		nil,
		nil,
	)
	repo := &modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{{
		ModelName:   "deepseek-v4-flash",
		Platform:    PlatformOpenAI,
		VisibleAuth: true,
		GroupIDs:    []int64{domestic.ID},
		InputPrice:  &siteIn,
		OutputPrice: &siteOut,
	}}}
	svc := NewModelCatalogService(repo, nil, nil, nil, modelPricingSettingService("1"), apiKeyService, nil)

	resp, err := svc.ListMyPricing(context.Background(), 2)

	require.NoError(t, err)
	require.Len(t, resp.Models, 1)
	require.Len(t, resp.Models[0].Groups, 1)
	require.Equal(t, domestic.ID, resp.Models[0].Groups[0].ID)
	require.Equal(t, domestic.Name, resp.Models[0].Groups[0].Name)
	require.InDelta(t, siteIn*domestic.RateMultiplier, *resp.Models[0].EffectiveInputPrice, 1e-12)
	require.InDelta(t, siteOut*domestic.RateMultiplier, *resp.Models[0].EffectiveOutputPrice, 1e-12)
}
