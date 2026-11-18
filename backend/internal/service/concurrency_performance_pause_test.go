package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAcquireAccountSlotHonorsLivePerformancePauseBeforeUnlimitedConcurrency(t *testing.T) {
	svc := NewConcurrencyService(nil)
	svc.performancePauses = performancePauseReaderStub{paused: map[int64]bool{84: true}}

	result, err := svc.AcquireAccountSlot(context.Background(), 84, 0)
	require.NoError(t, err)
	require.False(t, result.Acquired)
}

func TestAcquireAccountSlotFailsClosedWhenPerformancePauseCannotBeRead(t *testing.T) {
	svc := NewConcurrencyService(nil)
	svc.performancePauses = performancePauseReaderStub{err: errors.New("database unavailable")}

	result, err := svc.AcquireAccountSlot(context.Background(), 85, 0)
	require.Error(t, err)
	require.Nil(t, result)
}
