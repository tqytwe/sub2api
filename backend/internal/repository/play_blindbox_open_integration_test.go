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
	svc := service.NewPlayService(repo, nil, nil, settings, nil, testEntClient(t))
	svc.SetCouponRewardIssuer(&integrationCouponRewardIssuer{})
	return svc
}

// integrationCouponRewardIssuer keeps the legacy database accounting tests
// independent from coupon-table fixtures. The game transaction still sees a
// real coupon branch and must commit or roll back it together with the ledger.
// It also remembers issued coupons so same-key retry coverage exercises the
// replay reader rather than issuing a second fixture coupon.
type integrationCouponRewardIssuer struct {
	mu                  sync.Mutex
	issuedByIdempotency map[string]*service.CouponRewardIssueResult
}

func (i *integrationCouponRewardIssuer) DrawAndIssueInTx(_ context.Context, request service.CouponRewardDrawRequest) (*service.CouponRewardIssueResult, error) {
	issuedAt := request.IssuedAt
	if issuedAt.IsZero() {
		issuedAt = time.Now()
	}
	result := &service.CouponRewardIssueResult{
		PoolVersion:  "integration-coupon-v1",
		TemplateID:   1,
		UserCouponID: 1,
		Coupon: service.UserCoupon{
			ID:           1,
			TemplateID:   1,
			TemplateName: "integration coupon",
			UserID:       request.UserID,
			Status:       service.UserCouponStatusAvailable,
			ValidFrom:    issuedAt,
			ExpiresAt:    issuedAt.Add(time.Hour),
		},
		ValidFrom: issuedAt,
		ExpiresAt: issuedAt.Add(time.Hour),
	}
	i.mu.Lock()
	if i.issuedByIdempotency == nil {
		i.issuedByIdempotency = make(map[string]*service.CouponRewardIssueResult)
	}
	i.issuedByIdempotency[request.IdempotencyKey] = result
	i.mu.Unlock()
	return result, nil
}

func (i *integrationCouponRewardIssuer) GetCouponRewardIssueByIdempotency(_ context.Context, _ int64, idempotencyKey string) (*service.CouponRewardIssueResult, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	result := i.issuedByIdempotency[idempotencyKey]
	if result == nil {
		return nil, nil
	}
	copy := *result
	copy.Coupon = result.Coupon
	copy.Coupon.TermsSnapshot = result.Coupon.TermsSnapshot
	copy.Coupon.TermsSnapshot.ApplicableScopes = append([]service.CouponScope(nil), result.Coupon.TermsSnapshot.ApplicableScopes...)
	return &copy, nil
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

func TestPlayBlindboxOpenSameIdempotencyKeySettlesOnlyOnce(t *testing.T) {
	ctx := context.Background()
	user := mustCreateUser(t, testEntClient(t), &service.User{
		Email:        fmt.Sprintf("blindbox-same-key-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      1,
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})

	svc := newBlindboxIntegrationService(t, service.PlayBlindboxPool{
		Version: "integration-same-key",
		Cost:    0.5,
		RTPCap:  1,
		Tiers: []service.PlayBlindboxTier{
			{Amount: 0, Weight: 10_000},
		},
	}, 1)

	start := make(chan struct{})
	results := make(chan *service.PlayBlindboxOpenResult, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := svc.OpenBlindbox(ctx, user.ID, "same-client-request")
			results <- result
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	var completed []*service.PlayBlindboxOpenResult
	for err := range errs {
		require.NoError(t, err)
	}
	for result := range results {
		require.NotNil(t, result)
		completed = append(completed, result)
	}
	require.Len(t, completed, 2)
	require.Equal(t, completed[0].CostAmount, completed[1].CostAmount)
	require.Equal(t, completed[0].RewardAmount, completed[1].RewardAmount)
	require.Equal(t, completed[0].NetAmount, completed[1].NetAmount)
	require.Equal(t, completed[0].RewardType, completed[1].RewardType)
	require.Equal(t, completed[0].CouponPoolVersion, completed[1].CouponPoolVersion)
	require.Equal(t, completed[0].PoolVersion, completed[1].PoolVersion)

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
