//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestPaymentStorefrontConfigFallbackBuildsShelvesAndTags(t *testing.T) {
	svc := NewPaymentConfigService(nil, &paymentConfigSettingRepoStub{values: map[string]string{}}, nil)
	plans := []*dbent.SubscriptionPlan{
		{ID: 1, Name: "Daily Trial", Price: 3, ValidityDays: 1, StorefrontCategory: "daily", StorefrontFeatured: true, StorefrontBadge: "日卡", SortOrder: 1},
		{ID: 2, Name: "Monthly 29.9", Price: 29.9, ValidityDays: 30, StorefrontCategory: "pro", StorefrontBadge: "高性价比", SortOrder: 2},
		{ID: 3, Name: "Monthly 100", Price: 100, ValidityDays: 30, StorefrontCategory: "pro", SortOrder: 3},
	}

	cfg, err := svc.GetPublicPaymentStorefrontConfig(context.Background(), plans)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(cfg.Shelves), 3)
	require.Equal(t, "推荐套餐", cfg.Shelves[0].Label)
	require.Equal(t, []int64{1}, cfg.Shelves[0].PlanIDs)
	require.Equal(t, "全部套餐", cfg.Shelves[1].Label)
	require.NotNil(t, cfg.Shelves[1].DefaultPlanID)
	require.EqualValues(t, 2, *cfg.Shelves[1].DefaultPlanID)

	var tagLabels []string
	for _, tag := range cfg.Tags {
		tagLabels = append(tagLabels, tag.Label)
	}
	require.Contains(t, tagLabels, "推荐")
	require.Contains(t, tagLabels, "日卡")
	require.Contains(t, tagLabels, "高性价比")
}

func TestPaymentStorefrontConfigPublicViewFiltersDisabledAndInvalidPlans(t *testing.T) {
	raw, err := json.Marshal(PaymentStorefrontConfig{
		Shelves: []PaymentStorefrontShelf{
			{ID: "hidden", Label: "Hidden", Enabled: false, SortOrder: 1, PlanIDs: []int64{1}},
			{ID: "monthly", Label: "Monthly", Enabled: true, SortOrder: 2, PlanIDs: []int64{2, 999}, DefaultPlanID: storefrontInt64Ptr(999)},
		},
		Tags: []PaymentStorefrontTag{
			{ID: "hidden-tag", Label: "Hidden Tag", Tone: "danger", Enabled: false, SortOrder: 1, PlanIDs: []int64{2}},
			{ID: "hot", Label: "Hot", Tone: "success", Enabled: true, SortOrder: 2, PlanIDs: []int64{2, 999}},
		},
	})
	require.NoError(t, err)

	svc := NewPaymentConfigService(nil, &paymentConfigSettingRepoStub{values: map[string]string{
		SettingPaymentStorefrontConfig: string(raw),
	}}, nil)
	cfg, err := svc.GetPublicPaymentStorefrontConfig(context.Background(), []*dbent.SubscriptionPlan{
		{ID: 2, Name: "Monthly 29.9", Price: 29.9, ValidityDays: 30},
	})
	require.NoError(t, err)

	require.Len(t, cfg.Shelves, 1)
	require.Equal(t, "monthly", cfg.Shelves[0].ID)
	require.Equal(t, []int64{2}, cfg.Shelves[0].PlanIDs)
	require.Nil(t, cfg.Shelves[0].DefaultPlanID)
	require.Len(t, cfg.Tags, 1)
	require.Equal(t, "hot", cfg.Tags[0].ID)
	require.Equal(t, []int64{2}, cfg.Tags[0].PlanIDs)
}

func TestPaymentStorefrontConfigPublicViewFiltersAllPlanIDsWhenNoPlansExist(t *testing.T) {
	raw, err := json.Marshal(PaymentStorefrontConfig{
		Shelves: []PaymentStorefrontShelf{
			{ID: "stale", Label: "Stale", Enabled: true, SortOrder: 1, PlanIDs: []int64{404}},
		},
		Tags: []PaymentStorefrontTag{
			{ID: "stale-tag", Label: "Stale Tag", Tone: "warning", Enabled: true, SortOrder: 1, PlanIDs: []int64{404}},
		},
	})
	require.NoError(t, err)

	svc := NewPaymentConfigService(nil, &paymentConfigSettingRepoStub{values: map[string]string{
		SettingPaymentStorefrontConfig: string(raw),
	}}, nil)
	cfg, err := svc.GetPublicPaymentStorefrontConfig(context.Background(), nil)
	require.NoError(t, err)

	require.Empty(t, cfg.Shelves)
	require.Empty(t, cfg.Tags)
}

func TestUpdatePaymentStorefrontConfigPersistsNormalizedConfig(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigPlansTestClient(t)
	repo := &paymentStorefrontSettingRepo{values: map[string]string{}}
	svc := NewPaymentConfigService(client, repo, nil)

	plan, err := svc.CreatePlan(ctx, CreatePlanRequest{
		GroupID:      1,
		Name:         "Monthly 29.9",
		Description:  "Monthly plan",
		Price:        29.9,
		ValidityDays: 30,
		ValidityUnit: "days",
		ForSale:      true,
	})
	require.NoError(t, err)

	cfg, err := svc.UpdatePaymentStorefrontConfig(ctx, PaymentStorefrontConfig{
		Shelves: []PaymentStorefrontShelf{
			{ID: " Monthly ", Label: " 月卡 ", Enabled: true, PlanIDs: []int64{plan.ID, 999, plan.ID}, DefaultPlanID: &plan.ID},
		},
		Tags: []PaymentStorefrontTag{
			{ID: "Best Value", Label: "高性价比", Tone: "success", Enabled: true, PlanIDs: []int64{plan.ID}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "monthly", cfg.Shelves[0].ID)
	require.Equal(t, "月卡", cfg.Shelves[0].Label)
	require.Equal(t, []int64{plan.ID}, cfg.Shelves[0].PlanIDs)
	require.Equal(t, "best-value", cfg.Tags[0].ID)

	var saved PaymentStorefrontConfig
	require.NoError(t, json.Unmarshal([]byte(repo.values[SettingPaymentStorefrontConfig]), &saved))
	require.Equal(t, cfg.Shelves, saved.Shelves)
	require.Equal(t, cfg.Tags, saved.Tags)
}

func TestUpdatePaymentStorefrontConfigRejectsDuplicateLabels(t *testing.T) {
	client := newPaymentConfigPlansTestClient(t)
	svc := NewPaymentConfigService(client, &paymentStorefrontSettingRepo{values: map[string]string{}}, nil)

	_, err := svc.UpdatePaymentStorefrontConfig(context.Background(), PaymentStorefrontConfig{
		Shelves: []PaymentStorefrontShelf{
			{ID: "a", Label: "月卡", Enabled: true},
			{ID: "b", Label: "月卡", Enabled: true},
		},
	})
	require.Error(t, err)
	require.Equal(t, "PAYMENT_STOREFRONT_SHELF_LABEL_DUPLICATE", infraErrorReason(err))
}

type paymentStorefrontSettingRepo struct {
	values map[string]string
}

func (s *paymentStorefrontSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *paymentStorefrontSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *paymentStorefrontSettingRepo) Set(_ context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *paymentStorefrontSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = s.values[key]
	}
	return out, nil
}

func (s *paymentStorefrontSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *paymentStorefrontSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s *paymentStorefrontSettingRepo) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func storefrontInt64Ptr(value int64) *int64 {
	return &value
}

func infraErrorReason(err error) string {
	return infraerrors.Reason(err)
}
