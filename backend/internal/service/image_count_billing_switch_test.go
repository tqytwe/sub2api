//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveBillableImageCount(t *testing.T) {
	for _, tc := range []struct {
		name          string
		reported      int
		requested     int
		fallbackToOne bool
		want          int
	}{
		{name: "actual output wins", reported: 2, requested: 4, fallbackToOne: true, want: 2},
		{name: "opted-in unknown output gets one-image floor", fallbackToOne: true, want: 1},
		{name: "legacy non-stream request keeps n", requested: 3, want: 3},
		{name: "unknown streaming output stays zero by default", want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, resolveBillableImageCount(tc.reported, tc.requested, tc.fallbackToOne))
		})
	}
}

func TestCalculateRecordUsageCost_ImageCountBillingKeepsExplicitTokenPricing(t *testing.T) {
	groupID := int64(901)
	model := "gpt-image-2"
	resolver := newOpenAITokenImageChannelPricingResolverForTest(t, groupID, model)
	group := &Group{
		ID:                   groupID,
		Platform:             PlatformComposite,
		Status:               StatusActive,
		Hydrated:             true,
		AllowImageGeneration: true,
		ImageCountBilling:    true,
		ModelPricing: []ChannelModelPricing{{
			Models: []string{model}, BillingMode: BillingModeToken,
			InputPrice: f64p(3e-6), OutputPrice: f64p(15e-6),
		}},
	}
	apiKey := &APIKey{ID: 1, GroupID: &groupID, Group: group}
	svc := &GatewayService{billingService: resolver.billingService, resolver: resolver}

	cost := svc.calculateRecordUsageCost(context.Background(), &ForwardResult{
		Model: model, ImageCount: 2, ImageSize: "1K",
		Usage: ClaudeUsage{InputTokens: 1000, OutputTokens: 1000},
	}, apiKey, model, 1, 1, time.Time{})

	require.NotNil(t, cost)
	require.InDelta(t, 0.018, cost.TotalCost, 1e-12)
	require.Equal(t, string(BillingModeToken), cost.BillingMode)
}

func TestGroupServiceImageCountBillingValidation(t *testing.T) {
	ctx := context.Background()
	t.Run("creation rejects unsupported Anthropic group", func(t *testing.T) {
		svc := NewGroupService(nil, nil)
		_, err := svc.Create(ctx, CreateGroupRequest{Name: "g", ImageCountBilling: true})
		require.ErrorContains(t, err, "image_count_billing")
	})
	t.Run("update rejects groups without image support", func(t *testing.T) {
		repo := &groupRepoStubForAdmin{getByID: &Group{ID: 1, Platform: PlatformComposite}}
		svc := NewGroupService(repo, nil)
		_, err := svc.Update(ctx, 1, UpdateGroupRequest{ImageCountBilling: boolPtrForImageBillingTest(true)})
		require.ErrorContains(t, err, "image_count_billing")
	})
	t.Run("disable image generation clears the switch", func(t *testing.T) {
		repo := &groupRepoStubForAdmin{getByID: &Group{
			ID: 2, Platform: PlatformOpenAI, AllowImageGeneration: true, ImageCountBilling: true,
		}}
		svc := NewGroupService(repo, nil)
		updated, err := svc.Update(ctx, 2, UpdateGroupRequest{AllowImageGeneration: boolPtrForImageBillingTest(false)})
		require.NoError(t, err)
		require.False(t, updated.ImageCountBilling)
	})
}

func boolPtrForImageBillingTest(value bool) *bool { return &value }
