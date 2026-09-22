package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type modelCatalogVisibilityRepoStub struct {
	ModelCatalogRepository
	entries []SiteModelCatalogEntry
	err     error
}

type modelCatalogMutationRepoStub struct {
	ModelCatalogRepository
	existing *SiteModelCatalogEntry
	updated  *SiteModelCatalogEntry
	upserted *SiteModelCatalogEntry
}

func (r *modelCatalogMutationRepoStub) GetCatalogEntry(context.Context, int64) (*SiteModelCatalogEntry, error) {
	return r.existing, nil
}

func (r *modelCatalogMutationRepoStub) UpdateCatalogEntry(_ context.Context, entry *SiteModelCatalogEntry) error {
	copy := *entry
	copy.MediaCapabilities = append(json.RawMessage(nil), entry.MediaCapabilities...)
	r.updated = &copy
	return nil
}

func (r *modelCatalogMutationRepoStub) UpsertCatalogEntry(_ context.Context, entry *SiteModelCatalogEntry) error {
	copy := *entry
	copy.MediaCapabilities = append(json.RawMessage(nil), entry.MediaCapabilities...)
	r.upserted = &copy
	return nil
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

func TestModelCatalogServiceSaveValidatesAndPreservesMediaCapabilities(t *testing.T) {
	valid := json.RawMessage(`{"version":"v1","adapter":"sensenova","modalities":["image"],"image":{"operations":["create"]}}`)
	repo := &modelCatalogMutationRepoStub{existing: &SiteModelCatalogEntry{
		ID:                41,
		ModelName:         "sensenova-u1-fast",
		Platform:          PlatformOpenAI,
		VisibleAuth:       true,
		MediaCapabilities: valid,
	}}
	svc := NewModelCatalogService(repo, nil, nil, nil, nil, nil, nil)

	invalid := &SiteModelCatalogEntry{
		ModelName:         "sensenova-u1-fast",
		Platform:          PlatformOpenAI,
		VisibleAuth:       true,
		MediaCapabilities: json.RawMessage(`{"version":"v1","adapter":"sensenova","modalities":["image"]}`),
	}
	require.Error(t, svc.SaveCatalogEntry(context.Background(), invalid))
	require.Nil(t, repo.upserted)
	require.Nil(t, repo.updated)

	updateWithoutField := &SiteModelCatalogEntry{
		ID:          41,
		ModelName:   "sensenova-u1-fast",
		Platform:    PlatformOpenAI,
		VisibleAuth: true,
	}
	require.NoError(t, svc.SaveCatalogEntry(context.Background(), updateWithoutField))
	require.NotNil(t, repo.updated)
	require.JSONEq(t, string(valid), string(repo.updated.MediaCapabilities))
}

func TestModelCatalogServiceSaveRejectsInexecutableMediaAdapter(t *testing.T) {
	repo := &modelCatalogMutationRepoStub{}
	svc := NewModelCatalogService(repo, nil, nil, nil, nil, nil, nil)

	err := svc.SaveCatalogEntry(context.Background(), &SiteModelCatalogEntry{
		ModelName:   "sensenova-u1-fast",
		Platform:    PlatformOpenAI,
		VisibleAuth: true,
		MediaCapabilities: json.RawMessage(`{
			"version":"v1",
			"adapter":"agnes",
			"modalities":["image"],
			"image":{"operations":["create"]}
		}`),
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter")
	require.Nil(t, repo.upserted)
}

func TestModelCatalogServiceSaveRejectsCapabilitiesOutsideExactAdapterProfile(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		declaration string
	}{
		{
			name:  "U1 Fast cannot declare edit",
			model: "sensenova-u1-fast",
			declaration: `{
				"version":"v1","adapter":"sensenova","modalities":["image"],
				"image":{"operations":["create","edit"]}
			}`,
		},
		{
			name:  "U1 Fast cannot accept reference images",
			model: "sensenova-u1-fast",
			declaration: `{
				"version":"v1","adapter":"sensenova","modalities":["image"],
				"image":{"operations":["create"],"max_reference_images":1}
			}`,
		},
		{
			name:  "U1 Fast cannot advertise an unregistered fixed size",
			model: "sensenova-u1-fast",
			declaration: `{
				"version":"v1","adapter":"sensenova","modalities":["image"],
				"image":{"operations":["create"],"sizing_kind":"fixed","supported_sizes":["1024x1024"]}
			}`,
		},
		{
			name:  "U1.5 cannot advertise unsupported output format",
			model: "sensenova-u1.5-lite",
			declaration: `{
				"version":"v1","adapter":"sensenova","modalities":["image"],
				"image":{"operations":["create"],"supported_output_formats":["gif"]}
			}`,
		},
		{
			name:  "U1.5 cannot exceed the global reference limit",
			model: "sensenova-u1.5-lite",
			declaration: `{
				"version":"v1","adapter":"sensenova","modalities":["image"],
				"image":{"operations":["create","edit"],"max_reference_images":5}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &modelCatalogMutationRepoStub{}
			svc := NewModelCatalogService(repo, nil, nil, nil, nil, nil, nil)

			err := svc.SaveCatalogEntry(context.Background(), &SiteModelCatalogEntry{
				ModelName:         tt.model,
				Platform:          PlatformOpenAI,
				VisibleAuth:       true,
				MediaCapabilities: json.RawMessage(tt.declaration),
			})

			require.Error(t, err)
			require.Nil(t, repo.upserted)
			require.Nil(t, repo.updated)
		})
	}
}

func TestModelCatalogServiceSaveAllowsNarrowingExactAdapterCapabilities(t *testing.T) {
	repo := &modelCatalogMutationRepoStub{}
	svc := NewModelCatalogService(repo, nil, nil, nil, nil, nil, nil)

	err := svc.SaveCatalogEntry(context.Background(), &SiteModelCatalogEntry{
		ModelName:   "sensenova-u1.5-lite",
		Platform:    PlatformOpenAI,
		VisibleAuth: true,
		MediaCapabilities: json.RawMessage(`{
			"version":"v1","adapter":"sensenova","modalities":["image"],
			"image":{
				"operations":["create"],
				"sizing_kind":"custom_dimensions",
				"min_dimension":1024,
				"max_dimension":2048,
				"dimension_step":64,
				"max_aspect_ratio":2,
				"supported_output_formats":["png"],
				"max_reference_images":2
			}
		}`),
	})

	require.NoError(t, err)
	require.NotNil(t, repo.upserted)
}

func TestModelCatalogService_NextChatDisplayShowsVisibleCatalogWithoutChannelMatch(t *testing.T) {
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

	resp, err := svc.ListNextChatDisplayMetadata(context.Background(), 4)

	require.NoError(t, err)
	require.True(t, resp.Enabled)
	require.Len(t, resp.Models, 2)
	require.Equal(t, "model-a", resp.Models[0].Name)
	require.Equal(t, "model-b", resp.Models[1].Name)
	require.Empty(t, resp.Models[0].Groups)
	require.Nil(t, resp.Models[0].EffectiveInputPrice)
	require.Nil(t, resp.Models[0].EffectiveOutputPrice)
	require.InDelta(t, 10e-6, *resp.Models[0].SiteInputPrice, 1e-12)
	require.InDelta(t, 20e-6, *resp.Models[0].SiteOutputPrice, 1e-12)
	require.InDelta(t, 5e-6, *resp.Models[1].SiteInputPrice, 1e-12)
	require.InDelta(t, 30e-6, *resp.Models[1].SiteOutputPrice, 1e-12)
}

func TestModelCatalogService_NextChatDisplayKeepsChannelEffectivePricing(t *testing.T) {
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
	}}}, groupRepo, nil, nil, nil)
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

	resp, err := svc.ListNextChatDisplayMetadata(context.Background(), 4)

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

func TestModelCatalogService_NextChatDisplayUsesOfficialFallbackWithoutChannel(t *testing.T) {
	officialIn, officialOut := 4e-6, 24e-6
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
		ModelName:           "gpt-test",
		Platform:            PlatformOpenAI,
		VisibleAuth:         true,
		OfficialInputPrice:  &officialIn,
		OfficialOutputPrice: &officialOut,
		BillingMode:         string(BillingModeToken),
	}}}
	svc := NewModelCatalogService(repo, nil, nil, nil, modelPricingSettingService("1"), apiKeyService, nil)

	resp, err := svc.ListNextChatDisplayMetadata(context.Background(), 4)

	require.NoError(t, err)
	require.Len(t, resp.Models, 1)
	row := resp.Models[0]
	require.Empty(t, row.Channel)
	require.Len(t, row.Groups, 1)
	require.Equal(t, group.ID, row.Groups[0].ID)
	require.Equal(t, 0.25, row.Groups[0].RateMultiplier)
	require.InDelta(t, officialIn, *row.BaseInputPrice, 1e-12)
	require.InDelta(t, officialOut, *row.BaseOutputPrice, 1e-12)
	require.InDelta(t, officialIn*0.25, *row.EffectiveInputPrice, 1e-12)
	require.InDelta(t, officialOut*0.25, *row.EffectiveOutputPrice, 1e-12)
}

func TestModelCatalogService_ExplicitCatalogGroupsOverridePlatformMatching(t *testing.T) {
	officialIn, officialOut := 4e-6, 24e-6
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
		ModelName:           "qwen3.5-plus",
		Platform:            PlatformOpenAI,
		VisibleAuth:         true,
		GroupIDs:            []int64{domestic.ID},
		OfficialInputPrice:  &officialIn,
		OfficialOutputPrice: &officialOut,
	}}}
	svc := NewModelCatalogService(repo, nil, nil, nil, modelPricingSettingService("1"), apiKeyService, nil)

	resp, err := svc.ListNextChatDisplayMetadata(context.Background(), 4)

	require.NoError(t, err)
	require.Len(t, resp.Models, 1)
	require.Len(t, resp.Models[0].Groups, 1)
	require.Equal(t, domestic.ID, resp.Models[0].Groups[0].ID)
	require.Equal(t, domestic.Name, resp.Models[0].Groups[0].Name)
	require.InDelta(t, officialIn*domestic.RateMultiplier, *resp.Models[0].EffectiveInputPrice, 1e-12)
}

func TestModelCatalogService_ExplicitCatalogGroupUsesAvailableGroupWithoutKey(t *testing.T) {
	officialIn, officialOut := 4e-6, 24e-6
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
		ModelName:           "deepseek-v4-flash",
		Platform:            PlatformOpenAI,
		VisibleAuth:         true,
		GroupIDs:            []int64{domestic.ID},
		OfficialInputPrice:  &officialIn,
		OfficialOutputPrice: &officialOut,
	}}}
	svc := NewModelCatalogService(repo, nil, nil, nil, modelPricingSettingService("1"), apiKeyService, nil)

	resp, err := svc.ListNextChatDisplayMetadata(context.Background(), 2)

	require.NoError(t, err)
	require.Len(t, resp.Models, 1)
	require.Len(t, resp.Models[0].Groups, 1)
	require.Equal(t, domestic.ID, resp.Models[0].Groups[0].ID)
	require.Equal(t, domestic.Name, resp.Models[0].Groups[0].Name)
	require.InDelta(t, officialIn*domestic.RateMultiplier, *resp.Models[0].EffectiveInputPrice, 1e-12)
	require.InDelta(t, officialOut*domestic.RateMultiplier, *resp.Models[0].EffectiveOutputPrice, 1e-12)
}

func TestResolveCatalogManualFlag_FieldScopedOwnership(t *testing.T) {
	oldValue := 4.5e-6
	newValue := 6e-6
	cleared := (*float64)(nil)

	require.True(t, resolveCatalogManualFlag(nil, &newValue, false, false), "a new non-empty official value is manual")
	require.False(t, resolveCatalogManualFlag(&oldValue, &oldValue, false, false), "unchanged sync-owned value remains sync-owned")
	require.True(t, resolveCatalogManualFlag(&oldValue, &oldValue, true, false), "existing manual ownership is retained")
	require.True(t, resolveCatalogManualFlag(&oldValue, &newValue, true, false), "a changed value is owned by the new edit")
	require.False(t, resolveCatalogManualFlag(&oldValue, cleared, true, false), "clearing a field releases ownership")
}
