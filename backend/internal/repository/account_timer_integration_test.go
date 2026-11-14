//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accounttimer"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func accountTimerRepositoryFixture(t *testing.T) (context.Context, *accountRepository, int64, *accounttimer.Store, *time.Time) {
	t.Helper()
	ctx, repo, id, _ := accountGroupPolicyConcurrencyFixture(t)
	t.Cleanup(func() {
		_, err := integrationDB.Exec("DELETE FROM account_schedulable_timer_audit WHERE account_id=$1", id)
		require.NoError(t, err)
		_, err = integrationDB.Exec("DELETE FROM account_schedulable_timers WHERE account_id=$1", id)
		require.NoError(t, err)
	})
	now := time.Now().UTC().Truncate(time.Minute)
	store := &accounttimer.Store{DB: integrationDB, Now: func() time.Time { return now }, Policy: accounttimer.DefaultPolicy(), Enqueue: func(ctx context.Context, tx *sql.Tx, id int64) error {
		return enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &id, nil, nil)
	}}
	return ctx, repo, id, store, &now
}

func saveRepositoryTimer(t *testing.T, store *accounttimer.Store, id int64, action string, minutes int64) accounttimer.Task {
	t.Helper()
	task, err := store.Save(context.Background(), id, 9, accounttimer.SaveRequest{Input: accounttimer.Input{Mode: "relative", Minutes: minutes, Timezone: "UTC"}, Action: action})
	require.NoError(t, err)
	return task
}

func TestAccountTimerProductionRepositoryProvidesPersistentStore(t *testing.T) {
	ctx, repo, id, _, _ := accountTimerRepositoryFixture(t)
	provider, ok := any(repo).(interface {
		SchedulableTimers() accounttimer.Repository
	})
	require.True(t, ok, "production account repository must expose the persistent timer capability")
	timers := provider.SchedulableTimers()
	require.NotNil(t, timers)
	_, err := timers.Save(ctx, id, 19, accounttimer.SaveRequest{Input: accounttimer.Input{Mode: "relative", Minutes: 1, Timezone: "UTC"}, Action: "close"})
	require.NoError(t, err)
	// A separate capability instance must read the same committed plan.
	snapshot, err := provider.SchedulableTimers().List(ctx, id)
	require.NoError(t, err)
	require.Len(t, snapshot.Tasks, 1)
	require.Equal(t, "close", snapshot.Tasks[0].Action)
}

func TestAccountTimerProductionManualSwitchCancelsBothPlansAtomically(t *testing.T) {
	for _, bulk := range []bool{false, true} {
		for _, outboxFailure := range []bool{false, true} {
			t.Run(fmt.Sprintf("bulk_%t/outbox_failure_%t", bulk, outboxFailure), func(t *testing.T) {
				ctx, repo, id, store, now := accountTimerRepositoryFixture(t)
				saveRepositoryTimer(t, store, id, "open", 1)
				saveRepositoryTimer(t, store, id, "close", 2)
				if outboxFailure {
					constraint := fmt.Sprintf("timer_manual_outbox_failure_%d", id)
					_, err := integrationDB.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE scheduler_outbox ADD CONSTRAINT %s CHECK(account_id IS DISTINCT FROM %d AND NOT COALESCE(payload @> '{"account_ids":[%d]}'::jsonb,false)) NOT VALID`, constraint, id, id))
					require.NoError(t, err)
					t.Cleanup(func() {
						_, _ = integrationDB.Exec("ALTER TABLE scheduler_outbox DROP CONSTRAINT IF EXISTS " + constraint)
					})
				}
				manual := accounttimer.WithManualIntent(ctx, 19)
				var err error
				if bulk {
					closed := false
					_, err = repo.BulkUpdate(manual, []int64{id}, service.AccountBulkUpdate{Schedulable: &closed})
				} else {
					err = repo.SetSchedulable(manual, id, false)
				}
				snapshot, readErr := store.List(ctx, id)
				require.NoError(t, readErr)
				account, readErr := repo.GetByID(ctx, id)
				require.NoError(t, readErr)
				if outboxFailure {
					require.Error(t, err, "manual switch cannot succeed without its transactional outbox")
					require.True(t, account.Schedulable, "outbox failure must roll back the immediate switch")
					for _, task := range snapshot.Tasks {
						require.Equal(t, "pending", task.Status, "outbox failure must roll back both cancellations")
						require.EqualValues(t, 1, task.Version)
					}
					return
				}
				require.NoError(t, err)
				require.False(t, account.Schedulable)
				for _, task := range snapshot.Tasks {
					require.Equal(t, "cancelled", task.Status)
					require.EqualValues(t, 2, task.Version)
					require.Equal(t, "manual_override", task.Reason)
				}
				var audits int
				require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM account_schedulable_timer_audit WHERE account_id=$1 AND actor_id=19 AND result='cancelled' AND reason='manual_override'", id).Scan(&audits))
				require.Equal(t, 2, audits)
				*now = now.Add(3 * time.Minute)
				require.NoError(t, store.RunDue(ctx))
				account, err = repo.GetByID(ctx, id)
				require.NoError(t, err)
				require.False(t, account.Schedulable, "cancelled opening must not undo a manual close")
			})
		}
	}
}

type accountTimerConcurrentAdminRepository struct {
	*accountRepository
	betweenReadAndUpdate func()
}

func (r *accountTimerConcurrentAdminRepository) GetByID(ctx context.Context, id int64) (*service.Account, error) {
	account, err := r.accountRepository.GetByID(ctx, id)
	if err == nil && r.betweenReadAndUpdate != nil {
		run := r.betweenReadAndUpdate
		r.betweenReadAndUpdate = nil
		run()
	}
	return account, err
}

func TestAccountTimerProductionOrdinaryEditPreservesTimerAndPendingPlans(t *testing.T) {
	for _, opens := range []bool{false, true} {
		t.Run(fmt.Sprintf("timer_opens_%t", opens), func(t *testing.T) {
			ctx, repo, id, store, now := accountTimerRepositoryFixture(t)
			_, err := integrationDB.ExecContext(ctx, "UPDATE accounts SET schedulable=$2 WHERE id=$1", id, !opens)
			require.NoError(t, err)
			action, future := "close", "open"
			if opens {
				action, future = future, action
			}
			task := saveRepositoryTimer(t, store, id, action, 1)
			saveRepositoryTimer(t, store, id, future, 3)
			writer := &accountTimerConcurrentAdminRepository{accountRepository: repo, betweenReadAndUpdate: func() {
				*now = task.ExecuteAt
				require.NoError(t, store.RunDue(ctx))
			}}
			groups := newGroupRepositoryWithSQL(integrationEntClient, integrationDB)
			svc := service.NewAdminService(nil, nil, groups, writer, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			updated, err := svc.UpdateAccount(accounttimer.WithManualIntent(ctx, 19), id, &service.UpdateAccountInput{Name: "edited-after-timer"})
			require.NoError(t, err)
			require.Equal(t, opens, updated.Schedulable, "ordinary edit must preserve the committed timer switch")
			require.Equal(t, "edited-after-timer", updated.Name)
			snapshot, err := store.List(ctx, id)
			require.NoError(t, err)
			for _, stored := range snapshot.Tasks {
				if stored.Action == future {
					require.Equal(t, "pending", stored.Status, "ordinary edit must strip inherited manual cancellation intent")
					require.EqualValues(t, 1, stored.Version)
				}
			}
		})
	}
}

func TestAccountTimerProductionExpiryWorkerExecutesPersistedCloseAtStartup(t *testing.T) {
	ctx, repo, id, store, _ := accountTimerRepositoryFixture(t)
	saveRepositoryTimer(t, store, id, "close", 1)
	_, err := integrationDB.ExecContext(ctx, "UPDATE account_schedulable_timers SET execute_at=$2,next_attempt_at=$2 WHERE account_id=$1", id, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	worker := service.NewAccountExpiryService(repo, time.Hour)
	worker.Start()
	defer worker.Stop()
	require.Eventually(t, func() bool {
		var closed bool
		err := integrationDB.QueryRowContext(ctx, "SELECT NOT schedulable FROM accounts WHERE id=$1", id).Scan(&closed)
		return err == nil && closed
	}, 2*time.Second, 10*time.Millisecond, "existing expiry worker must execute durable timers on startup")
	snapshot, err := store.List(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "executed", snapshot.Tasks[0].Status)
}
