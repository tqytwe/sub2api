package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetPlayBlindboxPoolRejectsInvalidPoolWithoutChangingStoredValue(t *testing.T) {
	t.Parallel()

	original := defaultBlindboxPool()
	originalJSON, err := json.Marshal(original)
	require.NoError(t, err)

	tests := []struct {
		name   string
		mutate func(*PlayBlindboxPool)
	}{
		{
			name: "weight total 9999",
			mutate: func(pool *PlayBlindboxPool) {
				pool.Tiers[0].Weight--
			},
		},
		{
			name: "rtp above cap",
			mutate: func(pool *PlayBlindboxPool) {
				pool.Tiers[len(pool.Tiers)-1].Amount = 21
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := newBlindboxPoolAdminSettingRepo(string(originalJSON))
			svc := NewSettingService(repo, nil)
			invalid := cloneBlindboxPool(original)
			tt.mutate(&invalid)

			_, err := svc.SetPlayBlindboxPool(context.Background(), invalid)

			require.Error(t, err)
			require.Equal(t, 0, repo.setCalls)
			require.Equal(t, string(originalJSON), repo.values[SettingKeyPlayBlindboxPoolJSON])
		})
	}
}

func TestSetPlayBlindboxPoolPersistenceFailureLeavesStoredValueUntouched(t *testing.T) {
	t.Parallel()

	original := defaultBlindboxPool()
	originalJSON, err := json.Marshal(original)
	require.NoError(t, err)

	repo := newBlindboxPoolAdminSettingRepo(string(originalJSON))
	repo.setErr = errors.New("database unavailable")
	svc := NewSettingService(repo, nil)
	updated := cloneBlindboxPool(original)
	updated.Version = "season-2-v1"

	_, err = svc.SetPlayBlindboxPool(context.Background(), updated)

	require.ErrorContains(t, err, "persist blindbox pool")
	require.Equal(t, 1, repo.setCalls)
	require.Equal(t, string(originalJSON), repo.values[SettingKeyPlayBlindboxPoolJSON])
}

func TestSetPlayBlindboxPoolPersistsValidPool(t *testing.T) {
	t.Parallel()

	repo := newBlindboxPoolAdminSettingRepo("")
	svc := NewSettingService(repo, nil)
	pool := defaultBlindboxPool()
	pool.Version = " season-2-v1 "

	saved, err := svc.SetPlayBlindboxPool(context.Background(), pool)

	require.NoError(t, err)
	require.Equal(t, 1, repo.setCalls)
	require.Equal(t, "season-2-v1", saved.Version)
	require.Equal(t, pool.Cost, saved.Cost)
	require.Equal(t, pool.RTPCap, saved.RTPCap)
	require.Equal(t, pool.Tiers, saved.Tiers)

	var stored PlayBlindboxPool
	require.NoError(t, json.Unmarshal([]byte(repo.values[SettingKeyPlayBlindboxPoolJSON]), &stored))
	require.Equal(t, "season-2-v1", stored.Version)
	require.Equal(t, pool.Cost, stored.Cost)
	require.Equal(t, pool.RTPCap, stored.RTPCap)
	require.Equal(t, pool.Tiers, stored.Tiers)
}

type blindboxPoolAdminSettingRepo struct {
	values   map[string]string
	setErr   error
	setCalls int
}

func newBlindboxPoolAdminSettingRepo(raw string) *blindboxPoolAdminSettingRepo {
	return &blindboxPoolAdminSettingRepo{
		values: map[string]string{
			SettingKeyPlayBlindboxPoolJSON: raw,
		},
	}
}

func (r *blindboxPoolAdminSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *blindboxPoolAdminSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *blindboxPoolAdminSettingRepo) Set(_ context.Context, key, value string) error {
	return r.SetMultiple(context.Background(), map[string]string{key: value})
}

func (r *blindboxPoolAdminSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *blindboxPoolAdminSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	r.setCalls++
	if r.setErr != nil {
		return r.setErr
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *blindboxPoolAdminSettingRepo) GetAll(context.Context) (map[string]string, error) {
	result := make(map[string]string, len(r.values))
	for key, value := range r.values {
		result[key] = value
	}
	return result, nil
}

func (r *blindboxPoolAdminSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}
