package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestIdempotencyRepositoryTryReclaimUsesStatusAndExpiredLockGuard(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &idempotencyRepository{sql: db}
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	lockedUntil := now.Add(100 * time.Millisecond)
	expiresAt := now.Add(24 * time.Hour)

	// The predicate must keep the status transition and lock-expiry comparison
	// in the same UPDATE. A separate read would permit two retries to reclaim.
	mock.ExpectExec(`(?s)UPDATE idempotency_records.*WHERE id = \$1.*AND status = \$5.*AND \(locked_until IS NULL OR locked_until <= \$6\)`).
		WithArgs(
			int64(42),
			service.IdempotencyStatusProcessing,
			lockedUntil,
			expiresAt,
			service.IdempotencyStatusFailedRetryable,
			now,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	reclaimed, err := repo.TryReclaim(
		context.Background(),
		42,
		service.IdempotencyStatusFailedRetryable,
		now,
		lockedUntil,
		expiresAt,
	)
	require.NoError(t, err)
	require.True(t, reclaimed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIdempotencyRepositoryTryReclaimDoesNotTakeActiveLock(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &idempotencyRepository{sql: db}
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec(`(?s)UPDATE idempotency_records.*locked_until <= \$6`).
		WithArgs(
			int64(42),
			service.IdempotencyStatusProcessing,
			now.Add(time.Second),
			now.Add(24*time.Hour),
			service.IdempotencyStatusFailedRetryable,
			now,
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	reclaimed, err := repo.TryReclaim(
		context.Background(),
		42,
		service.IdempotencyStatusFailedRetryable,
		now,
		now.Add(time.Second),
		now.Add(24*time.Hour),
	)
	require.NoError(t, err)
	require.False(t, reclaimed)
	require.NoError(t, mock.ExpectationsWereMet())
}
