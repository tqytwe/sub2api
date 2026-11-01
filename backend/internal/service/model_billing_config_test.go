package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelBillingConfigMultiplierForModel(t *testing.T) {
	cfg := ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{
		{Model: "gpt-*", Multiplier: 2},
		{Model: "gpt-6-luna*", Multiplier: 10, GroupIDs: []int64{5, 6}},
		{Model: "gpt-6-luna-low", Multiplier: 3, GroupIDs: []int64{7}},
	}}

	for _, tc := range []struct {
		name    string
		model   string
		groupID *int64
		want    float64
	}{
		{name: "scoped longest prefix", model: " GPT-6-LUNA-LOW ", groupID: modelBillingPtrInt64(5), want: 10},
		{name: "scoped exact rule", model: "gpt-6-luna-low", groupID: modelBillingPtrInt64(7), want: 3},
		{name: "global fallback", model: "gpt-6-luna-low", groupID: modelBillingPtrInt64(8), want: 2},
		{name: "unknown group does not match scoped rule", model: "gpt-6-luna-low", want: 2},
		{name: "nil group does not match scoped rule", model: "gpt-6-luna", want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, cfg.MultiplierForModel(tc.model, tc.groupID))
		})
	}

	cfg.Enabled = false
	require.Equal(t, 1.0, cfg.MultiplierForModel("gpt-6-luna", modelBillingPtrInt64(5)))
}

func TestModelBillingConfigValidation(t *testing.T) {
	valid := ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{{Model: "gpt-6-luna*", Multiplier: 10, GroupIDs: []int64{5, 5}}}}
	require.NoError(t, ValidateModelBillingConfig(valid))

	for _, tc := range []struct {
		name string
		cfg  ModelBillingConfig
	}{
		{name: "enabled without rules", cfg: ModelBillingConfig{Enabled: true}},
		{name: "duplicate model", cfg: ModelBillingConfig{Rules: []ModelBillingRule{{Model: "gpt", Multiplier: 2}, {Model: " GPT ", Multiplier: 3}}}},
		{name: "wildcard in the middle", cfg: ModelBillingConfig{Rules: []ModelBillingRule{{Model: "g*pt", Multiplier: 2}}}},
		{name: "zero multiplier", cfg: ModelBillingConfig{Rules: []ModelBillingRule{{Model: "gpt", Multiplier: 0}}}},
		{name: "negative group", cfg: ModelBillingConfig{Rules: []ModelBillingRule{{Model: "gpt", Multiplier: 2, GroupIDs: []int64{-1}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, ValidateModelBillingConfig(tc.cfg))
		})
	}
}

func TestModelBillingConfigJSONPreservesScopeByModel(t *testing.T) {
	cfg := ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{
		{Model: "gpt-*", Multiplier: 2, GroupIDs: []int64{5}},
		{Model: "luna", Multiplier: 10, GroupIDs: []int64{6}},
	}}

	require.NoError(t, json.Unmarshal([]byte(`{"rules":[{"model":" LUNA ","multiplier":3},{"model":"new","multiplier":2,"group_ids":[]},{"model":"gpt-*","multiplier":4}]}`), &cfg))
	require.Equal(t, []int64{6}, cfg.Rules[0].GroupIDs)
	require.Empty(t, cfg.Rules[1].GroupIDs)
	require.Equal(t, []int64{5}, cfg.Rules[2].GroupIDs)

	var scoped ModelBillingConfig
	require.NoError(t, json.Unmarshal([]byte(`{"enabled":true,"rules":[{"model":"gpt","multiplier":2,"group_ids":[5,5]}]}`), &scoped))
	require.Equal(t, []int64{5}, scoped.Rules[0].GroupIDs)
	input := ModelBillingConfig{Rules: []ModelBillingRule{{Model: "gpt", Multiplier: 2, GroupIDs: []int64{5, 5}}}}
	normalized := NormalizeModelBillingConfig(input)
	require.Equal(t, []int64{5}, normalized.Rules[0].GroupIDs)
	require.Equal(t, []int64{5, 5}, input.Rules[0].GroupIDs)
}

func TestModelBillingConfigJSONRejectsImplicitNewScopedRule(t *testing.T) {
	cfg := ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{{Model: "gpt", Multiplier: 2, GroupIDs: []int64{5}}}}
	require.Error(t, json.Unmarshal([]byte(`{"rules":[{"model":"new","multiplier":3}]}`), &cfg))
}

func modelBillingPtrInt64(value int64) *int64 { return &value }
