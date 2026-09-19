package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type mobileVideoAvailabilityGroupsFake struct{ groups []Group }

func (f mobileVideoAvailabilityGroupsFake) GetNextChatSelectableGroups(context.Context, int64) ([]Group, error) {
	return append([]Group(nil), f.groups...), nil
}

type mobileVideoAvailabilityCatalogFake struct{ entries []SiteModelCatalogEntry }

func (f mobileVideoAvailabilityCatalogFake) ListCatalog(context.Context, CatalogListFilter) ([]SiteModelCatalogEntry, error) {
	return append([]SiteModelCatalogEntry(nil), f.entries...), nil
}

type mobileVideoAvailabilityVisibilityCatalogFake struct {
	entries []SiteModelCatalogEntry
	filters []CatalogListFilter
}

func (f *mobileVideoAvailabilityVisibilityCatalogFake) ListCatalog(_ context.Context, filter CatalogListFilter) ([]SiteModelCatalogEntry, error) {
	f.filters = append(f.filters, filter)
	entries := make([]SiteModelCatalogEntry, 0, len(f.entries))
	for _, entry := range f.entries {
		if filter.VisibleAuth != nil && entry.VisibleAuth != *filter.VisibleAuth {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

type mobileVideoAvailabilityModelsFake struct {
	models           map[int64][]string
	modelsByPlatform map[int64]map[string][]string
	platforms        map[int64]map[string]struct{}
}

func (f mobileVideoAvailabilityModelsFake) GetAvailableModels(_ context.Context, groupID *int64, platform string) []string {
	if groupID == nil {
		return nil
	}
	if byPlatform, ok := f.modelsByPlatform[*groupID]; ok {
		return append([]string(nil), byPlatform[platform]...)
	}
	return append([]string(nil), f.models[*groupID]...)
}

func (f mobileVideoAvailabilityModelsFake) GetSchedulablePlatforms(_ context.Context, groupID *int64) map[string]struct{} {
	if groupID == nil {
		return nil
	}
	out := make(map[string]struct{}, len(f.platforms[*groupID]))
	for platform := range f.platforms[*groupID] {
		out[platform] = struct{}{}
	}
	return out
}

func videoCapabilityDeclaration(t *testing.T, adapter string) json.RawMessage {
	t.Helper()
	return json.RawMessage(`{
		"version":"v1",
		"adapter":"` + adapter + `",
		"modalities":["video"],
		"video":{
			"operations":["generate"],
			"supported_resolutions":["720p"],
			"supported_aspect_ratios":["16:9"],
			"durations_seconds":[8]
		}
	}`)
}

func TestCatalogMobileVideoAvailabilityRequiresDeclaredMappedAndPricedCapability(t *testing.T) {
	price := 0.12
	resolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 22, Name: "video", Platform: PlatformGrok, AllowImageGeneration: true,
			VideoModelPrices: map[string]map[string]float64{"video-alpha": {"720p": price}},
		}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ID: 1, ModelName: "video-alpha", Platform: PlatformGrok, GroupIDs: []int64{22},
			MediaCapabilities: videoCapabilityDeclaration(t, MobileVideoAdapterGrok),
		}}},
		mobileVideoAvailabilityModelsFake{
			models:    map[int64][]string{22: {"video-alpha"}},
			platforms: map[int64]map[string]struct{}{22: {PlatformGrok: {}}},
		},
	)

	bootstrap, err := resolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, bootstrap.Groups, 1)
	require.True(t, bootstrap.Groups[0].VideoAvailable)
	require.Len(t, bootstrap.Groups[0].Models, 1)
	require.Equal(t, "video-alpha", bootstrap.Groups[0].Models[0].Model)
	require.Equal(t, MobileVideoAdapterGrok, bootstrap.Groups[0].Models[0].Adapter)

	resolved, err := resolver.Resolve(context.Background(), 7, 22, "video-alpha")
	require.NoError(t, err)
	require.Equal(t, MobileVideoAdapterGrok, resolved.Adapter)
	require.True(t, resolved.Capabilities.SupportsResolution("720p"))
	require.True(t, resolved.Capabilities.SupportsDuration(8))
	require.NotNil(t, resolved.PriceForResolution("720p"))
}

func TestCatalogMobileVideoAvailabilityOnlyPublishesPricedResolutions(t *testing.T) {
	price := 0.12
	declaration := json.RawMessage(`{
		"version":"v1","adapter":"grok_video","modalities":["video"],
		"video":{"operations":["generate"],"supported_resolutions":["480p","720p"],
		"supported_aspect_ratios":["16:9"],"durations_seconds":[8]}
	}`)
	resolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 221, Platform: PlatformGrok,
			VideoModelPrices: map[string]map[string]float64{"video-priced": {"720p": price}},
		}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ModelName: "video-priced", Platform: PlatformGrok, GroupIDs: []int64{221}, MediaCapabilities: declaration,
		}}},
		mobileVideoAvailabilityModelsFake{
			models:    map[int64][]string{221: {"video-priced"}},
			platforms: map[int64]map[string]struct{}{221: {PlatformGrok: {}}},
		},
	)

	bootstrap, err := resolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.True(t, bootstrap.Groups[0].VideoAvailable)
	require.Equal(t, []string{"720p"}, bootstrap.Groups[0].Models[0].Capabilities.SupportedResolutions)
	require.Equal(t, map[string]float64{"720p": price}, bootstrap.Groups[0].Models[0].UnitPrices)
}

func TestCatalogMobileVideoAvailabilityRejectsUnsupportedAgnesCombination(t *testing.T) {
	price := 0.12
	declaration := json.RawMessage(`{
		"version":"v1","adapter":"agnes_video","modalities":["video"],
		"video":{"operations":["generate"],"supported_resolutions":["720p"],
		"supported_aspect_ratios":["9:16"],"durations_seconds":[8]}
	}`)
	resolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 222, Platform: PlatformOpenAI,
			VideoModelPrices: map[string]map[string]float64{AgnesVideoDefaultModel: {"720p": price}},
		}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ModelName: AgnesVideoDefaultModel, Platform: PlatformOpenAI, GroupIDs: []int64{222}, MediaCapabilities: declaration,
		}}},
		mobileVideoAvailabilityModelsFake{
			models:    map[int64][]string{222: {AgnesVideoDefaultModel}},
			platforms: map[int64]map[string]struct{}{222: {PlatformOpenAI: {}}},
		},
	)

	bootstrap, err := resolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, bootstrap.Groups[0].VideoAvailable)
	require.Equal(t, []MobileVideoSuppressedModel{{Model: AgnesVideoDefaultModel, Code: MobileVideoSuppressionAdapterUnsupported}}, bootstrap.Groups[0].Suppressed)
}

func TestCatalogMobileVideoAvailabilityReportsExplicitSuppressionReasons(t *testing.T) {
	resolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{ID: 23, Name: "video", Platform: PlatformGrok, AllowImageGeneration: true}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ID: 2, ModelName: "video-beta", Platform: PlatformGrok, GroupIDs: []int64{23},
			MediaCapabilities: videoCapabilityDeclaration(t, MobileVideoAdapterGrok),
		}}},
		mobileVideoAvailabilityModelsFake{
			models:    map[int64][]string{23: nil},
			platforms: map[int64]map[string]struct{}{23: {PlatformGrok: {}}},
		},
	)

	bootstrap, err := resolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, bootstrap.Groups, 1)
	require.False(t, bootstrap.Groups[0].VideoAvailable)
	require.Equal(t, []MobileVideoSuppressedModel{{Model: "video-beta", Code: MobileVideoSuppressionNotMapped}}, bootstrap.Groups[0].Suppressed)

	_, err = resolver.Resolve(context.Background(), 7, 23, "video-beta")
	require.ErrorIs(t, err, ErrMobileVideoModelUnavailable)
}

func TestCatalogMobileVideoAvailabilityFailsClosedForSubscriptionGroups(t *testing.T) {
	price := 0.12
	resolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 2_608_270, Name: "subscription video", Platform: PlatformGrok,
			SubscriptionType: SubscriptionTypeSubscription,
			VideoModelPrices: map[string]map[string]float64{"video-alpha": {"720p": price}},
		}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ModelName: "video-alpha", Platform: PlatformGrok, GroupIDs: []int64{2_608_270},
			MediaCapabilities: videoCapabilityDeclaration(t, MobileVideoAdapterGrok),
		}}},
		mobileVideoAvailabilityModelsFake{
			modelsByPlatform: map[int64]map[string][]string{2_608_270: {PlatformGrok: {"video-alpha"}}},
			platforms:        map[int64]map[string]struct{}{2_608_270: {PlatformGrok: {}}},
		},
	)

	bootstrap, err := resolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, bootstrap.Groups, 1)
	require.False(t, bootstrap.Groups[0].VideoAvailable)
	require.Empty(t, bootstrap.Groups[0].Models)
	require.Empty(t, bootstrap.Groups[0].Suppressed)
	require.Equal(t, MobileVideoSuppressionSubscriptionReservationUnsupported, bootstrap.Groups[0].VideoUnavailableCode)

	_, err = resolver.Resolve(context.Background(), 7, 2_608_270, "video-alpha")
	require.ErrorIs(t, err, ErrMobileVideoCapabilityUnavailable)
	require.Equal(t, MobileVideoSuppressionSubscriptionReservationUnsupported, MobileVideoAvailabilityCode(err))
}

func TestCatalogMobileVideoAvailabilityDistinguishesNoAccountFromMissingCapability(t *testing.T) {
	noAccount := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{ID: 24, Platform: PlatformGrok, AllowImageGeneration: true}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ModelName: "video-gamma", Platform: PlatformGrok, GroupIDs: []int64{24},
			MediaCapabilities: videoCapabilityDeclaration(t, MobileVideoAdapterGrok),
		}}},
		mobileVideoAvailabilityModelsFake{},
	)
	bootstrap, err := noAccount.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, []MobileVideoSuppressedModel{{Model: "video-gamma", Code: MobileVideoSuppressionNoSchedulableAccount}}, bootstrap.Groups[0].Suppressed)

	missingCapability := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{ID: 25, Platform: PlatformGrok, AllowImageGeneration: true, VideoModelPrices: map[string]map[string]float64{"video-delta": {"720p": 1}}}}},
		mobileVideoAvailabilityCatalogFake{},
		mobileVideoAvailabilityModelsFake{
			models:    map[int64][]string{25: {"video-delta"}},
			platforms: map[int64]map[string]struct{}{25: {PlatformGrok: {}}},
		},
	)
	bootstrap, err = missingCapability.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, []MobileVideoSuppressedModel{{Model: "video-delta", Code: MobileVideoSuppressionCapabilityNotDeclared}}, bootstrap.Groups[0].Suppressed)
}

func TestCatalogMobileVideoAvailabilityRequiresAdapterPlatformAccount(t *testing.T) {
	price := 0.12
	resolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 26, Name: "video", Platform: PlatformGrok, AllowImageGeneration: true,
			VideoModelPrices: map[string]map[string]float64{"video-grok": {"720p": price}},
		}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ModelName: "video-grok", Platform: PlatformGrok, GroupIDs: []int64{26},
			MediaCapabilities: videoCapabilityDeclaration(t, MobileVideoAdapterGrok),
		}}},
		mobileVideoAvailabilityModelsFake{
			modelsByPlatform: map[int64]map[string][]string{26: {PlatformGrok: {"video-grok"}}},
			// An OpenAI account in a Grok group cannot execute a Grok video
			// request, even when a stale mapping still contains the model name.
			platforms: map[int64]map[string]struct{}{26: {PlatformOpenAI: {}}},
		},
	)

	bootstrap, err := resolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, bootstrap.Groups[0].VideoAvailable)
	require.Equal(t, []MobileVideoSuppressedModel{{Model: "video-grok", Code: MobileVideoSuppressionNoSchedulableAccount}}, bootstrap.Groups[0].Suppressed)
}

func TestCatalogMobileVideoAvailabilityCompositeUsesDeclaredAdapterPlatform(t *testing.T) {
	price := 0.12
	resolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 27, Name: "mixed video", Platform: PlatformComposite, AllowImageGeneration: true,
			VideoModelPrices: map[string]map[string]float64{"video-grok": {"720p": price}},
		}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ModelName: "video-grok", Platform: PlatformGrok, GroupIDs: []int64{27},
			MediaCapabilities: videoCapabilityDeclaration(t, MobileVideoAdapterGrok),
		}}},
		mobileVideoAvailabilityModelsFake{
			modelsByPlatform: map[int64]map[string][]string{27: {PlatformOpenAI: {"video-grok"}}},
			// The model mapping on an OpenAI account must not make a Grok
			// adapter executable in a composite group.
			platforms: map[int64]map[string]struct{}{27: {PlatformOpenAI: {}}},
		},
	)

	bootstrap, err := resolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, bootstrap.Groups[0].VideoAvailable)
	require.Equal(t, []MobileVideoSuppressedModel{{Model: "video-grok", Code: MobileVideoSuppressionNoSchedulableAccount}}, bootstrap.Groups[0].Suppressed)
}

func TestCatalogMobileVideoAvailabilitySelectsPlatformAndMappedAdapterAcrossDuplicateModelRows(t *testing.T) {
	price := 0.12
	agnesDeclaration := json.RawMessage(`{
		"version":"v1","adapter":"agnes_video","modalities":["video"],
		"video":{"operations":["generate"],"supported_resolutions":["480p"],
		"supported_aspect_ratios":["16:9"],"durations_seconds":[3]}
	}`)
	grokDeclaration := videoCapabilityDeclaration(t, MobileVideoAdapterGrok)

	for _, entries := range [][]SiteModelCatalogEntry{
		{
			{ModelName: "shared-video", Platform: PlatformOpenAI, GroupIDs: []int64{33}, MediaCapabilities: agnesDeclaration},
			{ModelName: "shared-video", Platform: PlatformGrok, GroupIDs: []int64{33}, MediaCapabilities: grokDeclaration},
		},
		{
			{ModelName: "shared-video", Platform: PlatformGrok, GroupIDs: []int64{33}, MediaCapabilities: grokDeclaration},
			{ModelName: "shared-video", Platform: PlatformOpenAI, GroupIDs: []int64{33}, MediaCapabilities: agnesDeclaration},
		},
	} {
		resolver := NewCatalogMobileVideoAvailabilityResolver(
			mobileVideoAvailabilityGroupsFake{groups: []Group{{
				ID: 33, Name: "mixed video", Platform: PlatformComposite,
				VideoModelPrices: map[string]map[string]float64{"shared-video": {"720p": price}},
			}}},
			mobileVideoAvailabilityCatalogFake{entries: entries},
			mobileVideoAvailabilityModelsFake{
				modelsByPlatform: map[int64]map[string][]string{33: {PlatformGrok: {"shared-video"}}},
				platforms:        map[int64]map[string]struct{}{33: {PlatformGrok: {}}},
			},
		)

		bootstrap, err := resolver.Bootstrap(context.Background(), 7)
		require.NoError(t, err)
		require.True(t, bootstrap.Groups[0].VideoAvailable)
		require.Equal(t, []MobileVideoResolvedModel{{
			ID: "shared-video", Name: "shared-video", Model: "shared-video",
			Modalities: []string{"video"}, Platform: PlatformGrok, CapabilityVersion: MobileVideoCapabilitiesVersion,
			GroupID: 33, GroupName: "mixed video", GroupPlatform: PlatformComposite,
			Adapter: MobileVideoAdapterGrok,
			Capabilities: MobileVideoCapabilities{
				Operations: []string{"generate"}, SupportedResolutions: []string{"720p"},
				SupportedRatios: []string{"16:9"}, SupportedDurations: []int{8},
			},
			UnitPrices: map[string]float64{"720p": price},
		}}, bootstrap.Groups[0].Models)
		require.Empty(t, bootstrap.Groups[0].Suppressed)
	}
}

func TestCatalogMobileVideoAvailabilityIgnoresCrossPlatformDuplicateInConcreteGroup(t *testing.T) {
	price := 0.12
	agnesDeclaration := json.RawMessage(`{
		"version":"v1","adapter":"agnes_video","modalities":["video"],
		"video":{"operations":["generate"],"supported_resolutions":["480p"],
		"supported_aspect_ratios":["16:9"],"durations_seconds":[3]}
	}`)
	grokDeclaration := videoCapabilityDeclaration(t, MobileVideoAdapterGrok)

	for _, entries := range [][]SiteModelCatalogEntry{
		{
			{ModelName: "shared-video", Platform: PlatformOpenAI, GroupIDs: []int64{34}, MediaCapabilities: agnesDeclaration},
			{ModelName: "shared-video", Platform: PlatformGrok, GroupIDs: []int64{34}, MediaCapabilities: grokDeclaration},
		},
		{
			{ModelName: "shared-video", Platform: PlatformGrok, GroupIDs: []int64{34}, MediaCapabilities: grokDeclaration},
			{ModelName: "shared-video", Platform: PlatformOpenAI, GroupIDs: []int64{34}, MediaCapabilities: agnesDeclaration},
		},
	} {
		resolver := NewCatalogMobileVideoAvailabilityResolver(
			mobileVideoAvailabilityGroupsFake{groups: []Group{{
				ID: 34, Name: "Grok Heavy", Platform: PlatformGrok,
				VideoModelPrices: map[string]map[string]float64{"shared-video": {"720p": price}},
			}}},
			mobileVideoAvailabilityCatalogFake{entries: entries},
			mobileVideoAvailabilityModelsFake{
				modelsByPlatform: map[int64]map[string][]string{34: {PlatformGrok: {"shared-video"}}},
				platforms:        map[int64]map[string]struct{}{34: {PlatformGrok: {}}},
			},
		)

		bootstrap, err := resolver.Bootstrap(context.Background(), 7)
		require.NoError(t, err)
		require.True(t, bootstrap.Groups[0].VideoAvailable)
		require.Len(t, bootstrap.Groups[0].Models, 1)
		require.Equal(t, MobileVideoAdapterGrok, bootstrap.Groups[0].Models[0].Adapter)
		require.Empty(t, bootstrap.Groups[0].Suppressed)
	}
}

func TestCatalogMobileVideoAvailabilityRequiresAuthenticatedCatalogVisibility(t *testing.T) {
	price := 0.12
	catalog := &mobileVideoAvailabilityVisibilityCatalogFake{entries: []SiteModelCatalogEntry{{
		ModelName: "hidden-video", Platform: PlatformGrok, VisibleAuth: false, GroupIDs: []int64{28},
		MediaCapabilities: videoCapabilityDeclaration(t, MobileVideoAdapterGrok),
	}}}
	resolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 28, Name: "video", Platform: PlatformGrok, AllowImageGeneration: true,
			VideoModelPrices: map[string]map[string]float64{"hidden-video": {"720p": price}},
		}}},
		catalog,
		mobileVideoAvailabilityModelsFake{
			models:    map[int64][]string{28: {"hidden-video"}},
			platforms: map[int64]map[string]struct{}{28: {PlatformGrok: {}}},
		},
	)

	bootstrap, err := resolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, catalog.filters, 2)
	require.NotNil(t, catalog.filters[0].VisibleAuth)
	require.True(t, *catalog.filters[0].VisibleAuth)
	require.NotNil(t, catalog.filters[1].VisibleAuth)
	require.False(t, *catalog.filters[1].VisibleAuth)
	require.False(t, bootstrap.Groups[0].VideoAvailable)
	require.Empty(t, bootstrap.Groups[0].Models, "hidden catalog rows must never become executable video models")
	require.Empty(t, bootstrap.Groups[0].Suppressed, "hidden catalog rows must not leak through unavailable-model diagnostics")
	_, err = resolver.Resolve(context.Background(), 7, 28, "hidden-video")
	require.ErrorIs(t, err, ErrMobileVideoModelUnavailable)
}

func TestCatalogMobileVideoAvailabilityHonorsGroupModelsListWithoutImagePermission(t *testing.T) {
	price := 0.12
	displayName := "Allowed Video"
	declaration := videoCapabilityDeclaration(t, MobileVideoAdapterGrok)
	modelsListResolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 29, Name: "video", Platform: PlatformGrok, AllowImageGeneration: true,
			ModelAllowlist:   GroupModelAllowlist{Enabled: true, Models: []string{"allowed-video"}},
			VideoModelPrices: map[string]map[string]float64{"allowed-video": {"720p": price}, "hidden-video": {"720p": price}},
		}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{
			{ModelName: "allowed-video", DisplayName: &displayName, Platform: PlatformGrok, GroupIDs: []int64{29}, MediaCapabilities: declaration},
			{ModelName: "hidden-video", Platform: PlatformGrok, GroupIDs: []int64{29}, MediaCapabilities: declaration},
		}},
		mobileVideoAvailabilityModelsFake{
			models:    map[int64][]string{29: {"allowed-video", "hidden-video"}},
			platforms: map[int64]map[string]struct{}{29: {PlatformGrok: {}}},
		},
	)

	bootstrap, err := modelsListResolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, []MobileVideoResolvedModel{{
		ID: "allowed-video", Name: "Allowed Video", Model: "allowed-video",
		Modalities: []string{"video"}, Platform: PlatformGrok, CapabilityVersion: MobileVideoCapabilitiesVersion,
		GroupID: 29, GroupName: "video", GroupPlatform: PlatformGrok,
		Adapter: MobileVideoAdapterGrok,
		Capabilities: MobileVideoCapabilities{
			Operations: []string{"generate"}, SupportedResolutions: []string{"720p"},
			SupportedRatios: []string{"16:9"}, SupportedDurations: []int{8},
		},
		UnitPrices: map[string]float64{"720p": price},
	}}, bootstrap.Groups[0].Models)
	require.Empty(t, bootstrap.Groups[0].Suppressed)

	videoOnlyResolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 30, Name: "Grok Heavy", Platform: PlatformGrok, AllowImageGeneration: false,
			VideoModelPrices: map[string]map[string]float64{"grok-imagine-video": {"720p": price}},
		}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ModelName: "grok-imagine-video", Platform: PlatformGrok, GroupIDs: []int64{30}, MediaCapabilities: declaration,
		}}},
		mobileVideoAvailabilityModelsFake{
			models:    map[int64][]string{30: {"grok-imagine-video"}},
			platforms: map[int64]map[string]struct{}{30: {PlatformGrok: {}}},
		},
	)
	bootstrap, err = videoOnlyResolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.True(t, bootstrap.Groups[0].VideoAvailable)
	require.Len(t, bootstrap.Groups[0].Models, 1)
	require.Equal(t, "grok-imagine-video", bootstrap.Groups[0].Models[0].Model)
	require.Empty(t, bootstrap.Groups[0].Suppressed)
}

func TestCatalogMobileVideoAvailabilityProjectsControlsAndRejectsUnsupportedReferences(t *testing.T) {
	price := 0.12
	declaration := json.RawMessage(`{
		"version":"v1","adapter":"grok_video","modalities":["video"],
		"video":{"operations":["generate"],"supported_resolutions":["720p"],
		"supported_aspect_ratios":["16:9"],"durations_seconds":[8],
		"generate_audio":true,"watermark":true}
	}`)
	resolver := NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 31, Platform: PlatformGrok, AllowImageGeneration: true,
			VideoPrice720P: &price,
		}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ModelName: "grok-video", Platform: PlatformGrok, GroupIDs: []int64{31}, MediaCapabilities: declaration,
		}}},
		mobileVideoAvailabilityModelsFake{
			models:    map[int64][]string{31: {"grok-video"}},
			platforms: map[int64]map[string]struct{}{31: {PlatformGrok: {}}},
		},
	)

	bootstrap, err := resolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, bootstrap.Groups[0].Models, 1, "%#v", bootstrap.Groups[0])
	require.True(t, bootstrap.Groups[0].Models[0].Capabilities.GenerateAudio)
	require.True(t, bootstrap.Groups[0].Models[0].Capabilities.Watermark)

	unsupportedReferences := json.RawMessage(`{
		"version":"v1","adapter":"grok_video","modalities":["video"],
		"video":{"operations":["generate"],"supported_resolutions":["720p"],
		"supported_aspect_ratios":["16:9"],"durations_seconds":[8],"max_reference_images":1}
	}`)
	resolver = NewCatalogMobileVideoAvailabilityResolver(
		mobileVideoAvailabilityGroupsFake{groups: []Group{{
			ID: 32, Platform: PlatformGrok, AllowImageGeneration: true,
			VideoPrice720P: &price,
		}}},
		mobileVideoAvailabilityCatalogFake{entries: []SiteModelCatalogEntry{{
			ModelName: "grok-video", Platform: PlatformGrok, GroupIDs: []int64{32}, MediaCapabilities: unsupportedReferences,
		}}},
		mobileVideoAvailabilityModelsFake{
			models:    map[int64][]string{32: {"grok-video"}},
			platforms: map[int64]map[string]struct{}{32: {PlatformGrok: {}}},
		},
	)
	bootstrap, err = resolver.Bootstrap(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, bootstrap.Groups[0].VideoAvailable)
	require.Equal(t, []MobileVideoSuppressedModel{{
		Model: "grok-video", Code: MobileVideoSuppressionAdapterUnsupported,
	}}, bootstrap.Groups[0].Suppressed)
}
