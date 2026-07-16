package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestDefaultBlindboxPoolIsApprovedPool(t *testing.T) {
	pool := defaultBlindboxPool()

	require.NoError(t, ValidateBlindboxPool(pool))
	require.Equal(t, "season-1-v1", pool.Version)
	require.Equal(t, 0.5, pool.Cost)
	require.Equal(t, 0.9, pool.RTPCap)
	require.Equal(t, []PlayBlindboxTier{
		{Amount: 0.05, Weight: 4000},
		{Amount: 0.20, Weight: 3000},
		{Amount: 0.50, Weight: 1800},
		{Amount: 1, Weight: 800},
		{Amount: 3, Weight: 300},
		{Amount: 10, Weight: 90},
		{Amount: 20, Weight: 10},
	}, pool.Tiers)
}

func TestBlindboxPoolExpectedReward(t *testing.T) {
	pool := defaultBlindboxPool()

	require.InDelta(t, 0.45, pool.ExpectedReward(), 1e-12)
}

func TestValidateBlindboxPoolRejectsInvalidPools(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PlayBlindboxPool)
	}{
		{name: "empty version", mutate: func(pool *PlayBlindboxPool) { pool.Version = "" }},
		{name: "zero cost", mutate: func(pool *PlayBlindboxPool) { pool.Cost = 0 }},
		{name: "infinite cost", mutate: func(pool *PlayBlindboxPool) { pool.Cost = math.Inf(1) }},
		{name: "zero rtp cap", mutate: func(pool *PlayBlindboxPool) { pool.RTPCap = 0 }},
		{name: "rtp cap above one", mutate: func(pool *PlayBlindboxPool) { pool.RTPCap = 1.01 }},
		{name: "nan rtp cap", mutate: func(pool *PlayBlindboxPool) { pool.RTPCap = math.NaN() }},
		{name: "no tiers", mutate: func(pool *PlayBlindboxPool) { pool.Tiers = nil }},
		{name: "too many tiers", mutate: func(pool *PlayBlindboxPool) {
			pool.Tiers = make([]PlayBlindboxTier, 33)
			for i := range pool.Tiers {
				pool.Tiers[i] = PlayBlindboxTier{Weight: 1}
			}
		}},
		{name: "negative amount", mutate: func(pool *PlayBlindboxPool) { pool.Tiers[0].Amount = -0.01 }},
		{name: "infinite amount", mutate: func(pool *PlayBlindboxPool) { pool.Tiers[0].Amount = math.Inf(1) }},
		{name: "zero weight", mutate: func(pool *PlayBlindboxPool) { pool.Tiers[0].Weight = 0 }},
		{name: "weights do not total denominator", mutate: func(pool *PlayBlindboxPool) { pool.Tiers[0].Weight-- }},
		{name: "expected reward exceeds rtp cap", mutate: func(pool *PlayBlindboxPool) { pool.Tiers[6].Amount = 1000 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := cloneBlindboxPool(defaultBlindboxPool())
			tt.mutate(&pool)

			require.Error(t, ValidateBlindboxPool(pool))
		})
	}
}

func TestValidateBlindboxPoolAcceptsExactRTPCapBoundary(t *testing.T) {
	pool := PlayBlindboxPool{
		Version: "exact-cap",
		Cost:    0.1,
		RTPCap:  0.7,
		Tiers: []PlayBlindboxTier{
			{Amount: 0.07, Weight: blindboxWeightTotal},
		},
	}

	require.NoError(t, ValidateBlindboxPool(pool))
}

func TestPickBlindboxRewardAtCoversBoundaries(t *testing.T) {
	pool := defaultBlindboxPool()

	require.Equal(t, 0.05, pickBlindboxRewardAt(pool, 0))
	require.Equal(t, 0.20, pickBlindboxRewardAt(pool, 4000))
	require.Equal(t, 20.0, pickBlindboxRewardAt(pool, 9999))
}

func TestGetPlayRuntimeUsesValidCustomBlindboxPool(t *testing.T) {
	custom := PlayBlindboxPool{
		Version: "custom-v2",
		Cost:    1,
		RTPCap:  0.8,
		Tiers: []PlayBlindboxTier{
			{Amount: 0.25, Weight: 8000},
			{Amount: 3, Weight: 2000},
		},
	}
	raw, err := json.Marshal(custom)
	require.NoError(t, err)
	repo := &blindboxPoolSettingRepoStub{values: map[string]string{
		SettingKeyPlayBlindboxCost:     "0.25",
		SettingKeyPlayBlindboxPoolJSON: string(raw),
	}}

	runtime := (&SettingService{settingRepo: repo}).GetPlayRuntime(context.Background())

	require.Equal(t, custom, runtime.BlindboxPool)
	require.Equal(t, 0.25, runtime.BlindboxCost)
}

func TestGetPlayRuntimeFallsBackToDefaultBlindboxPool(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantWarning bool
	}{
		{name: "missing", raw: "", wantWarning: false},
		{name: "whitespace", raw: " \t\n", wantWarning: false},
		{name: "malformed json", raw: `{"version":"private-pool-secret",`, wantWarning: true},
		{name: "overflowing sensitive number", raw: `{"version":"pool","cost":123456789012345678901234567890e9999,"rtp_cap":1,"tiers":[{"amount":0.1,"weight":10000}]}`, wantWarning: true},
		{name: "invalid weights", raw: `{"version":"private-pool-secret","cost":1,"rtp_cap":1,"tiers":[{"amount":0.1,"weight":9999}]}`, wantWarning: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, observed := observer.New(zap.WarnLevel)
			ctx := logger.IntoContext(context.Background(), zap.New(core))
			repo := &blindboxPoolSettingRepoStub{values: map[string]string{
				SettingKeyPlayBlindboxPoolJSON: tt.raw,
			}}

			runtime := (&SettingService{settingRepo: repo}).GetPlayRuntime(ctx)

			require.Equal(t, defaultBlindboxPool(), runtime.BlindboxPool)
			warnings := observed.FilterMessage("invalid play blindbox pool configuration; using approved default").All()
			if tt.wantWarning {
				require.Len(t, warnings, 1)
				fields := warnings[0].ContextMap()
				require.NotContains(t, fields, "raw")
				require.NotContains(t, fields, "value")
				require.NotContains(t, fields, "config")
				reason, ok := fields["reason"].(string)
				require.True(t, ok)
				logged := warnings[0].Message + " " + reason + " " + fmt.Sprint(fields)
				require.NotContains(t, logged, "private-pool-secret")
				require.NotContains(t, logged, "123456789012345678901234567890e9999")
			} else {
				require.Empty(t, warnings)
			}
		})
	}
}

func TestGetPlayRuntimeBlindboxPoolReadFailureIsFailClosed(t *testing.T) {
	repo := &blindboxPoolSettingRepoStub{err: errors.New("settings unavailable")}

	runtime := (&SettingService{settingRepo: repo}).GetPlayRuntime(context.Background())

	require.Equal(t, PlayRuntime{}, runtime)
}

func TestBlindboxPoolDrawSourceRejectsErrorsAndOutOfRangeDraws(t *testing.T) {
	pool := defaultBlindboxPool()
	service := NewPlayService(nil, nil, nil, nil, nil, nil)
	require.NotNil(t, service.blindboxDrawSource)

	wantErr := errors.New("random source failed")
	service.blindboxDrawSource = func(max int64) (int64, error) {
		require.Equal(t, blindboxWeightTotal, max)
		return 0, wantErr
	}
	_, err := service.pickBlindboxReward(pool)
	require.ErrorIs(t, err, wantErr)

	for _, draw := range []int64{-1, blindboxWeightTotal} {
		service.blindboxDrawSource = func(max int64) (int64, error) {
			return draw, nil
		}
		_, err = service.pickBlindboxReward(pool)
		require.Error(t, err)
	}
}

func TestPlayServicePickBlindboxRewardCoversEveryTierBoundary(t *testing.T) {
	pool := defaultBlindboxPool()
	tests := []struct {
		name   string
		draw   int64
		reward float64
	}{
		{name: "tier 1 start", draw: 0, reward: 0.05},
		{name: "tier 1 end", draw: 3999, reward: 0.05},
		{name: "tier 2 start", draw: 4000, reward: 0.20},
		{name: "tier 2 end", draw: 6999, reward: 0.20},
		{name: "tier 3 start", draw: 7000, reward: 0.50},
		{name: "tier 3 end", draw: 8799, reward: 0.50},
		{name: "tier 4 start", draw: 8800, reward: 1},
		{name: "tier 4 end", draw: 9599, reward: 1},
		{name: "tier 5 start", draw: 9600, reward: 3},
		{name: "tier 5 end", draw: 9899, reward: 3},
		{name: "tier 6 start", draw: 9900, reward: 10},
		{name: "tier 6 end", draw: 9989, reward: 10},
		{name: "tier 7 start", draw: 9990, reward: 20},
		{name: "tier 7 end", draw: 9999, reward: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewPlayService(nil, nil, nil, nil, nil, nil)
			svc.blindboxDrawSource = func(max int64) (int64, error) {
				require.Equal(t, blindboxWeightTotal, max)
				return tt.draw, nil
			}

			reward, err := svc.pickBlindboxReward(pool)

			require.NoError(t, err)
			require.Equal(t, tt.reward, reward)
		})
	}
}

func cloneBlindboxPool(pool PlayBlindboxPool) PlayBlindboxPool {
	pool.Tiers = append([]PlayBlindboxTier(nil), pool.Tiers...)
	return pool
}

type blindboxPoolSettingRepoStub struct {
	values map[string]string
	err    error
}

func (s *blindboxPoolSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *blindboxPoolSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *blindboxPoolSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *blindboxPoolSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

func (s *blindboxPoolSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *blindboxPoolSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *blindboxPoolSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}
