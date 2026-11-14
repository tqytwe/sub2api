package repository

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *accountRepository) WithLatestSchedulerAccounts(ctx context.Context, ids []int64, publish func([]service.Account) error) (retErr error) {
	if len(ids) == 0 {
		return publish(nil)
	}
	started := time.Now()
	phase := "begin"
	var lockSQLDuration, readDuration, publishDuration time.Duration
	var lockCompletedAt time.Time
	var lockSQLAttempted, readAttempted, publishAttempted, ownsTransaction bool
	lockCount := 0
	defer func() {
		elapsed := time.Since(started)
		var postLockDuration time.Duration
		if !lockCompletedAt.IsZero() {
			postLockDuration = time.Since(lockCompletedAt)
		}
		level := slog.LevelDebug
		if retErr != nil || elapsed >= time.Second {
			level = slog.LevelWarn
		}
		failureKind := "none"
		if retErr != nil {
			failureKind = "error"
			if errors.Is(retErr, context.DeadlineExceeded) {
				failureKind = "deadline"
			} else if errors.Is(retErr, context.Canceled) {
				failureKind = "canceled"
			}
		}
		// Durations include driver/network work; lock SQL time is not a direct
		// measurement of PostgreSQL lock wait. Never log IDs, rows or raw errors.
		slog.Log(ctx, level, "[Scheduler] account publication timing",
			"phase", phase, "failure_kind", failureKind, "requested_count", len(ids), "lock_count", lockCount,
			"owns_transaction", ownsTransaction, "elapsed_ms", elapsed.Milliseconds(),
			"lock_sql_attempted", lockSQLAttempted, "lock_sql_ms", lockSQLDuration.Milliseconds(),
			"lock_acquired", !lockCompletedAt.IsZero(), "post_lock_ms", postLockDuration.Milliseconds(),
			"read_attempted", readAttempted, "read_ms", readDuration.Milliseconds(),
			"publish_attempted", publishAttempted, "publish_ms", publishDuration.Milliseconds())
	}()
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	client := r.client
	if tx != nil {
		ownsTransaction = true
		defer func() { _ = tx.Rollback() }()
		client = tx.Client()
	}
	keys := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			keys = append(keys, int64(uint64(id)^uint64(0x5780000000000000)))
		}
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	lockCount = len(keys)
	phase = "lock"
	lockStarted := time.Now()
	if len(keys) == 1 {
		lockSQLAttempted = true
		if _, err := client.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", keys[0]); err != nil {
			lockSQLDuration = time.Since(lockStarted)
			return err
		}
	} else if len(keys) > 1 {
		lockSQLAttempted = true
		// Keep the account-level publication fence shared with single-account
		// feedback, but acquire it in one database round trip. The ordered
		// subquery feeds the volatile lock function in the same global order as
		// before, including when two buckets overlap on only some accounts.
		if _, err := client.ExecContext(ctx, `SELECT pg_advisory_xact_lock(lock_key)
FROM (SELECT unnest($1::bigint[]) AS lock_key ORDER BY lock_key) AS ordered_locks`, pq.Array(keys)); err != nil {
			lockSQLDuration = time.Since(lockStarted)
			return err
		}
	}
	if lockSQLAttempted {
		lockSQLDuration = time.Since(lockStarted)
		lockCompletedAt = time.Now()
	}
	scoped := *r
	scoped.client = client
	scoped.sql = client
	scoped.schedulerCache = nil
	phase = "read"
	readAttempted = true
	readStarted := time.Now()
	accounts, err := scoped.GetByIDs(ctx, ids)
	readDuration = time.Since(readStarted)
	if err != nil {
		return err
	}
	values := make([]service.Account, 0, len(accounts))
	for _, a := range accounts {
		values = append(values, *a)
	}
	phase = "publish"
	publishAttempted = true
	publishStarted := time.Now()
	err = publish(values)
	publishDuration = time.Since(publishStarted)
	if err != nil {
		return err
	}
	if tx != nil {
		phase = "commit"
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	phase = "complete"
	return nil
}

func (r *accountRepository) publishSchedulerAccountSnapshot(ctx context.Context, id int64) error {
	if r.schedulerCache == nil || id <= 0 {
		return nil
	}
	return r.WithLatestSchedulerAccounts(ctx, []int64{id}, func(accounts []service.Account) error {
		if len(accounts) == 0 {
			return r.schedulerCache.DeleteAccount(ctx, id)
		}
		return r.schedulerCache.SetAccount(ctx, &accounts[0])
	})
}
