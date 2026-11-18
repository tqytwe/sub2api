package service

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type accountPerformanceSettingRepoStub struct {
	SettingRepository
	value string
	set   string
	err   error
}

func (r *accountPerformanceSettingRepoStub) GetValue(context.Context, string) (string, error) {
	return r.value, r.err
}

func (r *accountPerformanceSettingRepoStub) Set(_ context.Context, _ string, value string) error {
	r.set = value
	return nil
}

func TestAccountPerformanceWindowSettingBoundsAndDefault(t *testing.T) {
	repo := &accountPerformanceSettingRepoStub{}
	svc := &SettingService{settingRepo: repo}

	settings, err := svc.GetAccountPerformanceSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultAccountPerformanceWindowMinutes, settings.WindowMinutes)

	for _, minutes := range []int{1, DefaultAccountPerformanceWindowMinutes, MaxAccountPerformanceWindowMinutes} {
		require.NoError(t, svc.SetAccountPerformanceSettings(context.Background(), AccountPerformanceSettings{WindowMinutes: minutes}))
		require.Equal(t, strconv.Itoa(minutes), repo.set)
	}
	for _, minutes := range []int{0, MaxAccountPerformanceWindowMinutes + 1} {
		require.Error(t, svc.SetAccountPerformanceSettings(context.Background(), AccountPerformanceSettings{WindowMinutes: minutes}))
	}
	repo.value = "not-a-number"
	_, err = svc.GetAccountPerformanceSettings(context.Background())
	require.Error(t, err)
}
