package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type duplicateReservationImageTaskQueue struct {
	requeues int
}

func (q *duplicateReservationImageTaskQueue) Submit(context.Context, *ImageTaskRecord, time.Duration, string) (string, bool, error) {
	return "", false, nil
}

func (q *duplicateReservationImageTaskQueue) Reserve(context.Context, time.Duration) (string, error) {
	return "", ErrImageTaskQueueEmpty
}

func (q *duplicateReservationImageTaskQueue) Ack(context.Context, string) error {
	return nil
}

func (q *duplicateReservationImageTaskQueue) Requeue(context.Context, string) error {
	q.requeues++
	return nil
}

func (q *duplicateReservationImageTaskQueue) Heartbeat(context.Context, string) error {
	return nil
}

func (q *duplicateReservationImageTaskQueue) RecoverStaleActive(context.Context, time.Duration, int) (int, error) {
	return 0, nil
}

func (q *duplicateReservationImageTaskQueue) TryAcquireJobLock(context.Context, string, time.Duration) (ImageTaskJobLock, bool, error) {
	return nil, false, nil
}

func (q *duplicateReservationImageTaskQueue) Ping(context.Context) error {
	return nil
}

func (q *duplicateReservationImageTaskQueue) Stats(context.Context) (ImageTaskQueueStats, error) {
	return ImageTaskQueueStats{}, nil
}

func TestImageTaskWorkerRuntime_DuplicateReservationDoesNotRemoveOwnersActiveLease(t *testing.T) {
	queue := &duplicateReservationImageTaskQueue{}
	runtime := &ImageTaskWorkerRuntime{
		queue: queue,
		opts: ImageTaskWorkerOptions{
			JobLockTTL: time.Minute,
		},
	}

	runtime.processReserved(context.Background(), "imgtask_duplicate")

	require.Zero(t, queue.requeues)
}
