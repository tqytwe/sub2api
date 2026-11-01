package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModelBillingRuntimeAppliesOnlyToTokenCustomerCost(t *testing.T) {
	groupID := int64(7)
	cfg := ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{{Model: "gpt-*", Multiplier: 3, GroupIDs: []int64{7}}}}
	token := &CostBreakdown{TotalCost: 2, ActualCost: 2, BillingMode: string(BillingModeToken)}
	applyModelBillingMultiplier(token, cfg, "gpt-6-luna", &groupID)
	require.Equal(t, 2.0, token.TotalCost)
	require.Equal(t, 6.0, token.ActualCost)
	require.Equal(t, 3.0, costModelBillingMultiplier(token))

	for _, mode := range []BillingMode{BillingModeImage, BillingModePerRequest, BillingModeVideo} {
		cost := &CostBreakdown{TotalCost: 2, ActualCost: 2, BillingMode: string(mode)}
		applyModelBillingMultiplier(cost, cfg, "gpt-6-luna", &groupID)
		require.Equal(t, 2.0, cost.ActualCost, mode)
		require.Equal(t, 1.0, costModelBillingMultiplier(cost), mode)
	}
	otherGroup := int64(8)
	untouched := &CostBreakdown{ActualCost: 2, BillingMode: string(BillingModeToken)}
	applyModelBillingMultiplier(untouched, cfg, "gpt-6-luna", &otherGroup)
	require.Equal(t, 2.0, untouched.ActualCost)
	require.False(t, responseModelBillingAdoptable(&CostBreakdown{TotalCost: 1, ActualCost: .2}, &CostBreakdown{TotalCost: .5, ActualCost: 1}, false, false))
}

func TestModelBillingRuntimeFreezesPolicyPerRequest(t *testing.T) {
	repo := &modelBillingCASRepo{value: `{"enabled":true,"rules":[{"model":"gpt-*","multiplier":3,"group_ids":[]}]}`, revision: "r1"}
	settings := NewSettingService(repo, nil)
	ctx := withModelBillingConfig(context.Background(), settings, nil)
	repo.value = `{"enabled":true,"rules":[{"model":"gpt-*","multiplier":9,"group_ids":[]}]}`
	groupID := int64(1)
	cost := &CostBreakdown{ActualCost: 1, BillingMode: string(BillingModeToken)}
	applyModelBillingMultiplier(cost, settings.modelBillingConfigForUsage(ctx), "gpt-5", &groupID)
	require.Equal(t, 3.0, cost.ActualCost)
	// The cache serves subsequent requests until its TTL, while a forced expiry
	// picks up the new revision for the next request.
	settings.modelBillingRuntimeCache.expires = time.Time{}
	next := &CostBreakdown{ActualCost: 1, BillingMode: string(BillingModeToken)}
	applyModelBillingMultiplier(next, settings.modelBillingConfigForUsage(context.Background()), "gpt-5", &groupID)
	require.Equal(t, 9.0, next.ActualCost)
}

type modelBillingRuntimeRepo struct {
	modelBillingCASRepo
	err      error
	deadline time.Time
}

func (r *modelBillingRuntimeRepo) GetValueWithRevision(ctx context.Context, key string) (string, string, error) {
	r.deadline, _ = ctx.Deadline()
	if r.err != nil {
		return "", "", r.err
	}
	return r.modelBillingCASRepo.GetValueWithRevision(ctx, key)
}

func TestModelBillingRuntimeRefreshFailureAndGroupSnapshot(t *testing.T) {
	repo := &modelBillingRuntimeRepo{modelBillingCASRepo: modelBillingCASRepo{value: `{"enabled":true,"rules":[{"model":"gpt-*","multiplier":3,"group_ids":[7]}]}`, revision: "r1"}}
	settings := NewSettingService(repo, nil)
	groupID := int64(7)
	ctx := withModelBillingConfig(context.Background(), settings, &groupID)
	require.WithinDuration(t, time.Now().Add(3*time.Second), repo.deadline, time.Second)
	groupID = 8
	require.Equal(t, int64(7), *modelBillingGroupIDForUsage(ctx, &groupID))
	repo.err = errors.New("storage unavailable")
	settings.modelBillingRuntimeCache.expires = time.Time{}
	require.Equal(t, 3.0, settings.modelBillingConfigForUsage(context.Background()).MultiplierForModel("gpt-5", modelBillingPtrInt64(7)))
	cold := NewSettingService(repo, nil)
	require.False(t, cold.modelBillingConfigForUsage(context.Background()).Enabled)
	var absent *SettingService
	require.False(t, absent.modelBillingConfigForUsage(context.Background()).Enabled)
}
