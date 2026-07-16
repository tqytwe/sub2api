package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestDefaultTeamRewardConfigUsesApprovedRules(t *testing.T) {
	cfg := defaultTeamRewardConfig()

	require.True(t, cfg.Enabled)
	require.Equal(t, "250", cfg.Cap.String())
	require.Equal(t, []TeamRewardTier{
		{Threshold: decimal.NewFromInt(20), Rate: decimal.RequireFromString("0.02")},
		{Threshold: decimal.NewFromInt(100), Rate: decimal.RequireFromString("0.03")},
		{Threshold: decimal.NewFromInt(500), Rate: decimal.RequireFromString("0.04")},
		{Threshold: decimal.NewFromInt(2000), Rate: decimal.RequireFromString("0.05")},
	}, cfg.Tiers)
	require.NoError(t, validateTeamRewardConfig(cfg))
}

func TestValidateTeamRewardConfigRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*TeamRewardConfig)
	}{
		{name: "non-positive cap", mutate: func(cfg *TeamRewardConfig) {
			cfg.Cap = decimal.Zero
		}},
		{name: "no tiers", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers = nil
		}},
		{name: "non-positive threshold", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[0].Threshold = decimal.Zero
		}},
		{name: "duplicate threshold", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[1].Threshold = cfg.Tiers[0].Threshold
		}},
		{name: "decreasing threshold", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[1].Threshold = cfg.Tiers[0].Threshold.Sub(decimal.NewFromInt(1))
		}},
		{name: "non-positive rate", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[0].Rate = decimal.Zero
		}},
		{name: "rate above one", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[0].Rate = decimal.RequireFromString("1.00000001")
		}},
		{name: "duplicate rate", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[1].Rate = cfg.Tiers[0].Rate
		}},
		{name: "decreasing rate", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[1].Rate = cfg.Tiers[0].Rate.Sub(decimal.RequireFromString("0.001"))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := cloneTeamRewardConfig(defaultTeamRewardConfig())
			tt.mutate(&cfg)

			require.Error(t, validateTeamRewardConfig(cfg))
		})
	}
}

func TestValidateTeamRewardDecimal20Scale8RejectsUnsafeValuesWithoutExponentAlignment(t *testing.T) {
	tests := []struct {
		name  string
		value decimal.Decimal
	}{
		{name: "too many integer digits", value: decimal.RequireFromString("1000000000000")},
		{name: "too many fractional digits", value: decimal.RequireFromString("1.000000001")},
		{name: "extreme positive exponent", value: decimal.New(1, 2147483647)},
		{name: "extreme negative exponent", value: decimal.New(1, -2147483648)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, validateTeamRewardDecimal20Scale8(tt.value))
		})
	}
}

func TestValidateTeamRewardConfigRejectsUnsafeDecimal20Scale8Fields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*TeamRewardConfig)
	}{
		{name: "threshold integer overflow", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[0].Threshold = decimal.RequireFromString("1000000000000")
		}},
		{name: "threshold scale overflow", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[0].Threshold = decimal.RequireFromString("0.000000001")
		}},
		{name: "threshold extreme exponent", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[0].Threshold = decimal.New(1, 2147483647)
		}},
		{name: "rate integer overflow", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[0].Rate = decimal.RequireFromString("1000000000000")
		}},
		{name: "rate scale overflow", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[0].Rate = decimal.RequireFromString("0.000000001")
		}},
		{name: "rate extreme exponent", mutate: func(cfg *TeamRewardConfig) {
			cfg.Tiers[0].Rate = decimal.New(1, 2147483647)
		}},
		{name: "cap integer overflow", mutate: func(cfg *TeamRewardConfig) {
			cfg.Cap = decimal.RequireFromString("1000000000000")
		}},
		{name: "cap scale overflow", mutate: func(cfg *TeamRewardConfig) {
			cfg.Cap = decimal.RequireFromString("250.000000001")
		}},
		{name: "cap extreme exponent", mutate: func(cfg *TeamRewardConfig) {
			cfg.Cap = decimal.New(1, 2147483647)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := cloneTeamRewardConfig(defaultTeamRewardConfig())
			tt.mutate(&cfg)

			require.Error(t, validateTeamRewardConfig(cfg))
		})
	}
}

func TestParseTeamRewardConfigRejectsOversizedTiersJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "json", raw: "[" + strings.Repeat(" ", teamRewardTiersJSONMaxBytes) + "]"},
		{name: "whitespace", raw: strings.Repeat(" ", teamRewardTiersJSONMaxBytes+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, diagnostic := parseTeamRewardConfig("", tt.raw, "")

			require.Equal(t, defaultTeamRewardConfig(), cfg)
			require.NotNil(t, diagnostic)
			require.Equal(t, SettingKeyPlayTeamSharedRewardTiers, diagnostic.SettingKey)
			require.Equal(t, "tiers_too_large", diagnostic.Reason)
		})
	}
}

func TestParseTeamRewardConfigRejectsExtremeExponentStrings(t *testing.T) {
	tests := []struct {
		name       string
		tiersRaw   string
		capRaw     string
		settingKey string
	}{
		{
			name:       "threshold",
			tiersRaw:   `[{"threshold":"1e2147483647","rate":"0.02"}]`,
			settingKey: SettingKeyPlayTeamSharedRewardTiers,
		},
		{
			name:       "rate",
			tiersRaw:   `[{"threshold":"20","rate":"1e2147483647"}]`,
			settingKey: SettingKeyPlayTeamSharedRewardTiers,
		},
		{
			name:       "cap",
			capRaw:     "1e2147483647",
			settingKey: SettingKeyPlayTeamSharedRewardCap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, diagnostic := parseTeamRewardConfig("", tt.tiersRaw, tt.capRaw)

			require.Equal(t, defaultTeamRewardConfig(), cfg)
			require.NotNil(t, diagnostic)
			require.Equal(t, tt.settingKey, diagnostic.SettingKey)
		})
	}
}

func TestResolveTeamRewardPoolUsesHighestReachedTierAndCap(t *testing.T) {
	cfg := defaultTeamRewardConfig()
	tests := []struct {
		name  string
		spend string
		want  string
	}{
		{name: "below first tier", spend: "19.99999999", want: "0.00000000"},
		{name: "first tier boundary", spend: "20", want: "0.40000000"},
		{name: "below second tier", spend: "99.99999999", want: "2.00000000"},
		{name: "second tier boundary", spend: "100", want: "3.00000000"},
		{name: "third tier boundary", spend: "500", want: "20.00000000"},
		{name: "fourth tier boundary", spend: "2000", want: "100.00000000"},
		{name: "below cap", spend: "4999", want: "249.95000000"},
		{name: "at cap", spend: "5000", want: "250.00000000"},
		{name: "above cap", spend: "10000", want: "250.00000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTeamRewardPool(decimal.RequireFromString(tt.spend), cfg)

			require.Equal(t, tt.want, got.StringFixed(8))
		})
	}

	disabled := cloneTeamRewardConfig(cfg)
	disabled.Enabled = false
	require.True(t, resolveTeamRewardPool(decimal.NewFromInt(100), disabled).IsZero())

	noTiers := cloneTeamRewardConfig(cfg)
	noTiers.Tiers = nil
	require.True(t, resolveTeamRewardPool(decimal.NewFromInt(100), noTiers).IsZero())
}

func TestAllocateTeamRewardAssignsRemainderToLargestContributor(t *testing.T) {
	got, err := allocateTeamReward(decimal.RequireFromString("1.00000000"), []TeamContribution{
		{UserID: 9, Amount: decimal.NewFromInt(2)},
		{UserID: 3, Amount: decimal.NewFromInt(1)},
	})

	require.NoError(t, err)
	require.Equal(t, "0.66666667", got[9].StringFixed(8))
	require.Equal(t, "0.33333333", got[3].StringFixed(8))
	require.Equal(t, "1.00000000", sumTeamRewardAllocations(got).StringFixed(8))
}

func TestAllocateTeamRewardTruncatesExactRatioWithoutIntermediateRounding(t *testing.T) {
	got, err := allocateTeamReward(decimal.RequireFromString("1.00000000"), []TeamContribution{
		{UserID: 1, Amount: decimal.RequireFromString("123456789999.99999999")},
		{UserID: 2, Amount: decimal.RequireFromString("876543210000.00000000")},
	})

	require.NoError(t, err)
	require.Equal(t, "0.12345678", got[1].StringFixed(8))
	require.Equal(t, "0.87654322", got[2].StringFixed(8))
	require.Equal(t, "1.00000000", sumTeamRewardAllocations(got).StringFixed(8))
}

func TestAllocateTeamRewardUsesLowestUserIDAsRemainderTieBreaker(t *testing.T) {
	got, err := allocateTeamReward(decimal.RequireFromString("1.00000000"), []TeamContribution{
		{UserID: 7, Amount: decimal.NewFromInt(1)},
		{UserID: 3, Amount: decimal.NewFromInt(1)},
		{UserID: 5, Amount: decimal.NewFromInt(1)},
	})

	require.NoError(t, err)
	require.Equal(t, "0.33333333", got[7].StringFixed(8))
	require.Equal(t, "0.33333334", got[3].StringFixed(8))
	require.Equal(t, "0.33333333", got[5].StringFixed(8))
	require.Equal(t, "1.00000000", sumTeamRewardAllocations(got).StringFixed(8))
}

func TestAllocateTeamRewardIgnoresZeroContributionAndIsInputOrderIndependent(t *testing.T) {
	pool := decimal.RequireFromString("7.12345678")
	forward := []TeamContribution{
		{UserID: 11, Amount: decimal.RequireFromString("12.5")},
		{UserID: 4, Amount: decimal.Zero},
		{UserID: 8, Amount: decimal.RequireFromString("3.75")},
		{UserID: 2, Amount: decimal.RequireFromString("12.5")},
	}
	reversed := []TeamContribution{forward[3], forward[2], forward[1], forward[0]}

	gotForward, err := allocateTeamReward(pool, forward)
	require.NoError(t, err)
	gotReversed, err := allocateTeamReward(pool, reversed)
	require.NoError(t, err)

	require.Equal(t, gotForward, gotReversed)
	require.NotContains(t, gotForward, int64(4))
	require.Equal(t, pool.StringFixed(8), sumTeamRewardAllocations(gotForward).StringFixed(8))
	for _, amount := range gotForward {
		require.Equal(t, amount, amount.Round(8))
	}
}

func TestAllocateTeamRewardReturnsNoAllocationsWithoutPoolOrContribution(t *testing.T) {
	allocations, err := allocateTeamReward(decimal.Zero, []TeamContribution{
		{UserID: 1, Amount: decimal.NewFromInt(1)},
	})
	require.NoError(t, err)
	require.Empty(t, allocations)

	allocations, err = allocateTeamReward(decimal.NewFromInt(1), []TeamContribution{
		{UserID: 1, Amount: decimal.Zero},
		{UserID: 2, Amount: decimal.NewFromInt(-1)},
	})
	require.NoError(t, err)
	require.Empty(t, allocations)
}

func TestAllocateTeamRewardRejectsNonPositiveUserID(t *testing.T) {
	for _, userID := range []int64{0, -1} {
		t.Run(fmt.Sprintf("user_id_%d", userID), func(t *testing.T) {
			allocations, err := allocateTeamReward(decimal.NewFromInt(1), []TeamContribution{
				{UserID: 7, Amount: decimal.NewFromInt(1)},
				{UserID: userID, Amount: decimal.NewFromInt(1)},
			})

			require.Error(t, err)
			require.Empty(t, allocations)
		})
	}
}

func TestAllocateTeamRewardMergesDuplicatePositiveUserIDs(t *testing.T) {
	allocations, err := allocateTeamReward(decimal.NewFromInt(1), []TeamContribution{
		{UserID: 5, Amount: decimal.NewFromInt(1)},
		{UserID: 9, Amount: decimal.NewFromInt(1)},
		{UserID: 5, Amount: decimal.NewFromInt(1)},
	})

	require.NoError(t, err)
	require.Len(t, allocations, 2)
	require.Equal(t, "0.66666667", allocations[5].StringFixed(8))
	require.Equal(t, "0.33333333", allocations[9].StringFixed(8))
}

func TestGetPlayRuntimeUsesSharedTeamRewardSettingsIndependentlyOfAffiliate(t *testing.T) {
	repo := &teamRewardSettingRepoStub{values: map[string]string{
		SettingKeyPlayTeamSharedRewardEnabled:    "false",
		SettingKeyPlayTeamSharedRewardTiers:      `[{"threshold":"10","rate":"0.1"},{"threshold":"50","rate":"0.2"}]`,
		SettingKeyPlayTeamSharedRewardCap:        "75.12345678",
		SettingKeyPlayTeamSharedRewardStartMonth: " 2026-07 ",
		SettingKeyPlayTeamAffiliateEnabled:       "true",
	}}

	runtime := (&SettingService{settingRepo: repo}).GetPlayRuntime(context.Background())

	require.False(t, runtime.TeamSharedRewardEnabled)
	require.Equal(t, []TeamRewardTier{
		{Threshold: decimal.NewFromInt(10), Rate: decimal.RequireFromString("0.1")},
		{Threshold: decimal.NewFromInt(50), Rate: decimal.RequireFromString("0.2")},
	}, runtime.TeamSharedRewardTiers)
	require.Equal(t, "75.12345678", runtime.TeamSharedRewardCap.String())
	require.Equal(t, "2026-07", runtime.TeamSharedRewardStartMonth)
	require.True(t, runtime.TeamAffiliateEnabled)
}

func TestGetPlayRuntimeDefaultsSharedTeamRewardsToEnabled(t *testing.T) {
	repo := &teamRewardSettingRepoStub{values: map[string]string{
		SettingKeyPlayTeamAffiliateEnabled: "false",
	}}

	runtime := (&SettingService{settingRepo: repo}).GetPlayRuntime(context.Background())
	defaults := defaultTeamRewardConfig()

	require.True(t, runtime.TeamSharedRewardEnabled)
	require.Equal(t, defaults.Tiers, runtime.TeamSharedRewardTiers)
	require.True(t, defaults.Cap.Equal(runtime.TeamSharedRewardCap))
	require.False(t, runtime.TeamAffiliateEnabled)
}

func TestGetPlayRuntimeRejectsInvalidSharedTeamRewardStartMonthWithDiagnostic(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		reason string
	}{
		{name: "invalid format", raw: "private-start-month", reason: "invalid_format"},
		{name: "single digit month", raw: "2026-7", reason: "invalid_format"},
		{name: "invalid month", raw: "2026-13", reason: "invalid_month"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, observed := observer.New(zap.WarnLevel)
			ctx := logger.IntoContext(context.Background(), zap.New(core))
			repo := &teamRewardSettingRepoStub{values: map[string]string{
				SettingKeyPlayTeamSharedRewardStartMonth: tt.raw,
			}}

			runtime := (&SettingService{settingRepo: repo}).GetPlayRuntime(ctx)

			require.Empty(t, runtime.TeamSharedRewardStartMonth)
			warnings := observed.FilterMessage("invalid play team shared reward start month; using empty value").All()
			require.Len(t, warnings, 1)
			fields := warnings[0].ContextMap()
			require.Equal(t, SettingKeyPlayTeamSharedRewardStartMonth, fields["setting_key"])
			require.Equal(t, tt.reason, fields["reason"])
			require.NotContains(t, fields, "raw")
			require.NotContains(t, fields, "value")
			require.NotContains(t, warnings[0].Message+" "+fmt.Sprint(fields), tt.raw)
		})
	}
}

func TestGetPlayRuntimeFallsBackFromInvalidSharedTeamRewardSettingsWithDiagnostic(t *testing.T) {
	tests := []struct {
		name       string
		values     map[string]string
		settingKey string
		reason     string
	}{
		{
			name: "invalid enabled",
			values: map[string]string{
				SettingKeyPlayTeamSharedRewardEnabled: "yes",
				SettingKeyPlayTeamSharedRewardTiers:   `[{"threshold":"10","rate":"0.1"}]`,
				SettingKeyPlayTeamSharedRewardCap:     "100",
			},
			settingKey: SettingKeyPlayTeamSharedRewardEnabled,
			reason:     "invalid_enabled",
		},
		{
			name: "malformed tiers",
			values: map[string]string{
				SettingKeyPlayTeamSharedRewardEnabled: "false",
				SettingKeyPlayTeamSharedRewardTiers:   `[{"threshold":"private-secret",`,
				SettingKeyPlayTeamSharedRewardCap:     "100",
			},
			settingKey: SettingKeyPlayTeamSharedRewardTiers,
			reason:     "malformed_tiers",
		},
		{
			name: "invalid tier ordering",
			values: map[string]string{
				SettingKeyPlayTeamSharedRewardTiers: `[{"threshold":"20","rate":"0.03"},{"threshold":"20","rate":"0.04"}]`,
				SettingKeyPlayTeamSharedRewardCap:   "100",
			},
			settingKey: SettingKeyPlayTeamSharedRewardTiers,
			reason:     "invalid_tiers",
		},
		{
			name: "invalid cap",
			values: map[string]string{
				SettingKeyPlayTeamSharedRewardTiers: `[{"threshold":"20","rate":"0.02"}]`,
				SettingKeyPlayTeamSharedRewardCap:   "0",
			},
			settingKey: SettingKeyPlayTeamSharedRewardCap,
			reason:     "invalid_cap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, observed := observer.New(zap.WarnLevel)
			ctx := logger.IntoContext(context.Background(), zap.New(core))
			repo := &teamRewardSettingRepoStub{values: tt.values}

			runtime := (&SettingService{settingRepo: repo}).GetPlayRuntime(ctx)
			defaults := defaultTeamRewardConfig()

			require.Equal(t, tt.values[SettingKeyPlayTeamSharedRewardEnabled] != "false", runtime.TeamSharedRewardEnabled)
			require.Equal(t, defaults.Tiers, runtime.TeamSharedRewardTiers)
			require.True(t, defaults.Cap.Equal(runtime.TeamSharedRewardCap))
			warnings := observed.FilterMessage("invalid play team shared reward configuration; using approved default").All()
			require.Len(t, warnings, 1)
			fields := warnings[0].ContextMap()
			require.Equal(t, tt.settingKey, fields["setting_key"])
			require.Equal(t, tt.reason, fields["reason"])
			require.NotContains(t, fields, "raw")
			require.NotContains(t, fields, "value")
			require.NotContains(t, fields, "config")
			require.NotContains(t, warnings[0].Message+" "+fmt.Sprint(fields), "private-secret")
		})
	}
}

func cloneTeamRewardConfig(cfg TeamRewardConfig) TeamRewardConfig {
	cfg.Tiers = append([]TeamRewardTier(nil), cfg.Tiers...)
	return cfg
}

func sumTeamRewardAllocations(allocations map[int64]decimal.Decimal) decimal.Decimal {
	total := decimal.Zero
	for _, amount := range allocations {
		total = total.Add(amount)
	}
	return total
}

type teamRewardSettingRepoStub struct {
	values map[string]string
}

func (s *teamRewardSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *teamRewardSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *teamRewardSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *teamRewardSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

func (s *teamRewardSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *teamRewardSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *teamRewardSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}
