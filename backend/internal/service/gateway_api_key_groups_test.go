package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type groupProbeAccountRepo struct {
	AccountRepository
	platform string
	err      error
	accounts []Account
}

func (r *groupProbeAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]Account, error) {
	r.platform = platform
	return r.accounts, r.err
}

func TestAPIKeyGroupProbeResolvesEndpointAliasBeforePlatform(t *testing.T) {
	sentinel := errors.New("account list reached")
	repo := &groupProbeAccountRepo{err: sentinel}
	svc := &GatewayService{accountRepo: repo, compositeResolver: NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{{GroupID: 7, PublicModel: "private-alias", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-5", Endpoint: CompositeRouteEndpointResponses, Enabled: true}}})}
	ctx := WithAPIKeyGroupEndpoint(context.Background(), CompositeRouteEndpointResponses)
	_, err := svc.ProbeAPIKeyGroup(ctx, &Group{ID: 7, Platform: PlatformComposite}, "private-alias", "image")
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, PlatformOpenAI, repo.platform)
}

func TestAPIKeyGroupProbeUsesExistingEndpointAccountCapabilities(t *testing.T) {
	for _, tc := range []struct {
		endpoint, accountType string
		available             bool
	}{
		{"audio", AccountTypeOAuth, false}, {"audio", AccountTypeAPIKey, true},
		{CompositeRouteEndpointEmbeddings, AccountTypeOAuth, false}, {CompositeRouteEndpointEmbeddings, AccountTypeAPIKey, true},
		{"live", AccountTypeAPIKey, false}, {"live", AccountTypeOAuth, true},
	} {
		t.Run(tc.endpoint+"/"+tc.accountType, func(t *testing.T) {
			repo := &groupProbeAccountRepo{accounts: []Account{{ID: 1, Platform: PlatformOpenAI, Type: tc.accountType, Status: StatusActive, Schedulable: true}}}
			svc := &GatewayService{accountRepo: repo}
			group := &Group{ID: 7, Platform: PlatformOpenAI, AllowLive: true}
			ctx := WithAPIKeyGroupEndpoint(context.Background(), tc.endpoint)
			got, err := svc.ProbeAPIKeyGroup(ctx, group, "", "image")
			require.NoError(t, err)
			require.Equal(t, tc.available, got.Available)
		})
	}
}

func TestAPIKeyGroupProbeRejectsCompositeTargetsUnsupportedByEndpoint(t *testing.T) {
	for _, endpoint := range []string{CompositeRouteEndpointImages, CompositeRouteEndpointEmbeddings, "live", "videos", "videos_grok"} {
		t.Run(endpoint, func(t *testing.T) {
			repo := &groupProbeAccountRepo{err: errors.New("must not list incompatible accounts")}
			svc := &GatewayService{accountRepo: repo, compositeResolver: NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{{GroupID: 7, PublicModel: "alias", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformDeepseek, UpstreamModel: "deepseek-v4", Endpoint: CompositeRouteEndpointAny, Enabled: true}}})}
			got, err := svc.ProbeAPIKeyGroup(WithAPIKeyGroupEndpoint(context.Background(), endpoint), &Group{ID: 7, Platform: PlatformComposite, AllowLive: true}, "alias", "image")
			require.NoError(t, err)
			require.False(t, got.Available)
			require.Empty(t, repo.platform)
		})
	}
}

func TestAPIKeyGroupProbeKeepsUnitBillingOutsideTokenProfitGate(t *testing.T) {
	for _, endpoint := range []string{"audio", "audio_openai", "audio_realtime", "live"} {
		t.Run(endpoint, func(t *testing.T) {
			accountType := AccountTypeAPIKey
			if endpoint == "live" {
				accountType = AccountTypeOAuth
			}
			rate := 2.0
			repo := &groupProbeAccountRepo{accounts: []Account{{ID: 1, Platform: PlatformOpenAI, Type: accountType, Status: StatusActive, Schedulable: true, RateMultiplier: &rate}}}
			svc := &GatewayService{accountRepo: repo}
			group := &Group{ID: 7, Platform: PlatformOpenAI, AllowLive: true, Hydrated: true, Status: StatusActive, RateMultiplier: 0.5, ProfitControlEnabled: true}
			got, err := svc.ProbeAPIKeyGroup(WithAPIKeyGroupEndpoint(context.Background(), endpoint), group, "", "token")
			require.NoError(t, err)
			require.True(t, got.Available, "unit-billed endpoints must retain their existing profit-control suppression")
		})
	}
}
