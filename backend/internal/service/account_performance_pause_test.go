package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type performancePauseReaderStub struct {
	AccountRepository
	paused map[int64]bool
	err    error
}

func (r performancePauseReaderStub) PerformancePaused(_ context.Context, ids []int64) (map[int64]bool, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.paused, nil
}

func TestSchedulerSnapshotFiltersLivePerformancePauses(t *testing.T) {
	svc := &SchedulerSnapshotService{accountRepo: performancePauseReaderStub{paused: map[int64]bool{12: true}}}
	got, err := svc.filterPerformanceAccounts(context.Background(), []Account{{ID: 11}, {ID: 12}})
	require.NoError(t, err)
	require.Equal(t, []Account{{ID: 11}}, got)
}

func TestSchedulerSnapshotFailsClosedWhenPerformancePauseAuthorityFails(t *testing.T) {
	svc := &SchedulerSnapshotService{accountRepo: performancePauseReaderStub{err: errors.New("database unavailable")}}
	got, err := svc.filterPerformanceAccounts(context.Background(), []Account{{ID: 11}})
	require.Error(t, err)
	require.Nil(t, got)
}

func TestPerformancePauseDoesNotMutateBaseSchedulableState(t *testing.T) {
	account := &Account{Status: StatusActive, Schedulable: true, PerformancePaused: true}
	require.False(t, account.IsSchedulable())
	require.True(t, account.Schedulable)

	account.PerformancePaused = false
	require.True(t, account.IsSchedulable())
}
