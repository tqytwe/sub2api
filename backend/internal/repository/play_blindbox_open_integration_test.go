//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type blindboxIntegrationSettingRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r *blindboxIntegrationSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func newBlindboxIntegrationService(t *testing.T, pool service.PlayBlindboxPool, dailyLimit int) *service.PlayService {
	t.Helper()
	rawPool, err := json.Marshal(pool)
	require.NoError(t, err)
	settings := service.NewSettingService(&blindboxIntegrationSettingRepo{values: map[string]string{
		service.SettingKeyPlayBlindboxEnabled:    "true",
		service.SettingKeyPlayBlindboxPoolJSON:   string(rawPool),
		service.SettingKeyPlayBlindboxDailyLimit: fmt.Sprintf("%d", dailyLimit),
	}}, nil)
	repo := NewPlayRepository(testEntClient(t), integrationDB)
	return service.NewPlayService(repo, nil, nil, settings, nil, testEntClient(t))
}

func TestPlayBlindboxOpenSerializesBalanceAndDailyLimit(t *testing.T) {
	ctx := context.Background()
	user := mustCreateUser(t, testEntClient(t), &service.User{
		Email:        fmt.Sprintf("blindbox-concurrent-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      1,
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})

	svc := newBlindboxIntegrationService(t, service.PlayBlindboxPool{
		Version: "integration-zero-reward",
		Cost:    0.5,
		RTPCap:  1,
		Tiers: []service.PlayBlindboxTier{
			{Amount: 0, Weight: 10_000},
		},
	}, 1)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := svc.OpenBlindbox(ctx, user.ID, fmt.Sprintf("concurrent-%d", i))
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	var success, limited int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, service.ErrPlayBlindboxDailyLimit):
			limited++
		default:
			t.Fatalf("unexpected concurrent open error: %v", err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, limited)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT balance FROM users WHERE id = $1", user.ID,
	).Scan(&balance))
	require.InDelta(t, 0.5, balance, 0.00000001)

	var opens, ledgers int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM play_blindbox_opens WHERE user_id = $1", user.ID,
	).Scan(&opens))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM play_reward_ledger WHERE user_id = $1 AND source = $2",
		user.ID, service.PlayRewardSourceBlindbox,
	).Scan(&ledgers))
	require.Equal(t, 1, opens)
	require.Equal(t, 1, ledgers)
}

func TestPlayBlindboxOpenRollsBackAuditAndBalanceWhenLedgerFails(t *testing.T) {
	ctx := context.Background()
	user := mustCreateUser(t, testEntClient(t), &service.User{
		Email:        fmt.Sprintf("blindbox-rollback-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      1,
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})

	functionName := fmt.Sprintf("fail_blindbox_ledger_%d", user.ID)
	triggerName := fmt.Sprintf("fail_blindbox_ledger_%d", user.ID)
	_, err := integrationDB.ExecContext(ctx, fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced blindbox ledger failure';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER %s
		BEFORE INSERT ON play_reward_ledger
		FOR EACH ROW
		WHEN (NEW.user_id = %d)
		EXECUTE FUNCTION %s();
	`, functionName, triggerName, user.ID, functionName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), fmt.Sprintf(
			"DROP TRIGGER IF EXISTS %s ON play_reward_ledger; DROP FUNCTION IF EXISTS %s();",
			triggerName, functionName,
		))
	})

	svc := newBlindboxIntegrationService(t, service.PlayBlindboxPool{
		Version: "integration-rollback",
		Cost:    0.5,
		RTPCap:  1,
		Tiers: []service.PlayBlindboxTier{
			{Amount: 0, Weight: 10_000},
		},
	}, 10)

	_, err = svc.OpenBlindbox(ctx, user.ID, "rollback-ledger")
	require.ErrorContains(t, err, "forced blindbox ledger failure")

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT balance FROM users WHERE id = $1", user.ID,
	).Scan(&balance))
	require.InDelta(t, 1, balance, 0.00000001)

	var opens, ledgers int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM play_blindbox_opens WHERE user_id = $1", user.ID,
	).Scan(&opens))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM play_reward_ledger WHERE user_id = $1", user.ID,
	).Scan(&ledgers))
	require.Zero(t, opens)
	require.Zero(t, ledgers)
}

func TestPlayRepositoryUpdatePlayBalanceDoesNotIncreaseTotalRecharged(t *testing.T) {
	ctx := context.Background()
	user := mustCreateUser(t, testEntClient(t), &service.User{
		Email:        fmt.Sprintf("play-balance-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      1,
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})
	_, err := integrationDB.ExecContext(ctx,
		"UPDATE users SET total_recharged = 12.5 WHERE id = $1", user.ID)
	require.NoError(t, err)

	tx, err := testEntClient(t).Tx(ctx)
	require.NoError(t, err)
	txCtx := dbent.NewTxContext(ctx, tx)
	repo := NewPlayRepository(testEntClient(t), integrationDB)
	require.NoError(t, repo.UpdatePlayBalance(txCtx, user.ID, 3.5))
	require.NoError(t, tx.Commit())

	var balance, totalRecharged float64
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT balance, total_recharged FROM users WHERE id = $1", user.ID,
	).Scan(&balance, &totalRecharged))
	require.InDelta(t, 4.5, balance, 0.00000001)
	require.InDelta(t, 12.5, totalRecharged, 0.00000001)
}
