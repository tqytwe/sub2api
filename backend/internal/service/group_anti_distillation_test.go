package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type antiDistillationSettingsStub struct {
	SettingRepository
	values map[string]string
	sets   map[string]string
}

func (s *antiDistillationSettingsStub) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *antiDistillationSettingsStub) Set(_ context.Context, key, value string) error {
	if s.sets == nil {
		s.sets = map[string]string{}
	}
	s.sets[key] = value
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

type antiDistillationGroupStub struct {
	GroupRepository
	group *Group
}

func (s antiDistillationGroupStub) GetByIDLite(_ context.Context, _ int64) (*Group, error) {
	return s.group, nil
}

type antiDistillationCacheStub struct {
	GroupAntiDistillationCache
	remaining time.Duration
	observed  []bool
}

func (s *antiDistillationCacheStub) PauseRemaining(context.Context, int64) (time.Duration, error) {
	return s.remaining, nil
}

func (s *antiDistillationCacheStub) Observe(_ context.Context, _ int64, _ string, zero bool, _ time.Duration, _ time.Time) (bool, time.Time, error) {
	s.observed = append(s.observed, zero)
	return false, time.Time{}, nil
}

func TestGroupAntiDistillationDefaultsOffAndIsOpenAIOnly(t *testing.T) {
	settings := &antiDistillationSettingsStub{}
	cache := &antiDistillationCacheStub{}
	service := NewGroupAntiDistillationService(settings, antiDistillationGroupStub{group: &Group{ID: 7, Platform: PlatformOpenAI}}, cache, nil, nil)

	remaining, err := service.Check(context.Background(), 41, 7)
	require.NoError(t, err)
	require.Zero(t, remaining)

	require.NoError(t, service.SaveConfig(context.Background(), 7, GroupAntiDistillationConfig{Enabled: true, PauseMinutes: 5}))
	require.Equal(t, `{"enabled":true,"pause_minutes":5}`, settings.sets[groupAntiDistillationSettingKey(7)])
	require.NoError(t, service.SaveConfig(context.Background(), 7, GroupAntiDistillationConfig{Enabled: false, PauseMinutes: 5}))

	service = NewGroupAntiDistillationService(settings, antiDistillationGroupStub{group: &Group{ID: 7, Platform: PlatformAnthropic}}, cache, nil, nil)
	require.Error(t, service.SaveConfig(context.Background(), 7, GroupAntiDistillationConfig{Enabled: true, PauseMinutes: 5}))
}

func TestGroupAntiDistillationValidatesPauseBounds(t *testing.T) {
	for _, minutes := range []int{0, 1441} {
		cfg := GroupAntiDistillationConfig{Enabled: true, PauseMinutes: minutes}
		require.Error(t, validateGroupAntiDistillationConfig(&cfg))
	}
	for _, minutes := range []int{1, 5, 1440} {
		cfg := GroupAntiDistillationConfig{Enabled: true, PauseMinutes: minutes}
		require.NoError(t, validateGroupAntiDistillationConfig(&cfg))
	}
}

func TestGroupAntiDistillationCountsOnlyExplicitCacheObservations(t *testing.T) {
	settings := &antiDistillationSettingsStub{values: map[string]string{
		groupAntiDistillationSettingKey(7): `{"enabled":true,"pause_minutes":5}`,
	}}
	cache := &antiDistillationCacheStub{}
	service := NewGroupAntiDistillationService(settings, antiDistillationGroupStub{group: &Group{ID: 7, Platform: PlatformOpenAI}}, cache, nil, nil)
	key := &APIKey{ID: 10, UserID: 41, GroupID: int64Ptr(7), Group: &Group{ID: 7, Platform: PlatformOpenAI}}

	service.Observe(context.Background(), key, "request-explicit-zero", 0)
	service.Observe(context.Background(), key, "request-cache-hit", 25)
	require.Equal(t, []bool{true, false}, cache.observed)
}
