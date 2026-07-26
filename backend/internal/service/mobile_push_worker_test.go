package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mobilePushWorkerTestDispatcher struct {
	results []bool
	errAt   int
	calls   int
}

func (d *mobilePushWorkerTestDispatcher) DispatchOne(context.Context) (bool, error) {
	d.calls++
	if d.errAt > 0 && d.calls == d.errAt {
		return false, errors.New("dispatch failed")
	}
	if len(d.results) == 0 {
		return false, nil
	}
	result := d.results[0]
	d.results = d.results[1:]
	return result, nil
}

func TestMobilePushWorkerRunOnceStopsAtEmptyQueue(t *testing.T) {
	dispatcher := &mobilePushWorkerTestDispatcher{results: []bool{true, true, false, true}}
	worker := NewMobilePushWorker(dispatcher, time.Second, 10)

	processed, err := worker.RunOnce(context.Background())

	require.NoError(t, err)
	require.Equal(t, 2, processed)
	require.Equal(t, 3, dispatcher.calls)
}

func TestMobilePushWorkerRunOnceHonorsBatchLimit(t *testing.T) {
	dispatcher := &mobilePushWorkerTestDispatcher{results: []bool{true, true, true}}
	worker := NewMobilePushWorker(dispatcher, time.Second, 2)

	processed, err := worker.RunOnce(context.Background())

	require.NoError(t, err)
	require.Equal(t, 2, processed)
	require.Equal(t, 2, dispatcher.calls)
}

func TestMobilePushWorkerRunStopsOnContextCancellation(t *testing.T) {
	dispatcher := &mobilePushWorkerTestDispatcher{}
	worker := NewMobilePushWorker(dispatcher, time.Millisecond, 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, worker.Run(ctx))
}
