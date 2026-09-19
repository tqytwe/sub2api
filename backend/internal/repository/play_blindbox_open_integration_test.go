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

// qualifyBlindboxIntegrationUser builds the same persisted signals used by the
// production eligibility query. Blindbox settlement tests must reach the real
// transaction and ledger path rather than stopping at the qualification gate.
func qualifyBlindboxIntegrationUser(t *testing.T, user *service.User) {
	t.Helper()
	ctx := context.Background()
	qualifiedAt := time.Now().UTC()

	_, err := integrationDB.ExecContext(ctx,
		"UPDATE users SET created_at = $1 WHERE id = $2",
		qualifiedAt.AddDate(0, 0, -4), user.ID,
	)
	require.NoError(t, err)

	identitySuffix := fmt.Sprintf("blindbox-identity-%d", user.ID)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO auth_identities (
			user_id, provider_type, provider_key, provider_subject, verified_at
		) VALUES ($1, 'email', $2, $2, $3)`,
		user.ID, identitySuffix, qualifiedAt,
	)
	require.NoError(t, err)

	client := testEntClient(t)
	account := mustCreateAccount(t, client, &service.Account{
		Name: fmt.Sprintf("blindbox-usage-account-%d", user.ID),
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    fmt.Sprintf("sk-blindbox-usage-%d", user.ID),
		Name:   "blindbox-usage",
	})
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO usage_logs (
			user_id, api_key_id, account_id, model, input_tokens, output_tokens,
			total_cost, actual_cost, created_at
		) VALUES ($1, $2, $3, 'integration-qualification', 1, 1, 0.01, 0.01, $4)`,
		user.ID, apiKey.ID, account.ID, qualifiedAt,
	)
	require.NoError(t, err)
}

func mustCreateBlindboxIntegrationUser(t *testing.T, prefix string) *service.User {
	t.Helper()
	client := testEntClient(t)
	for attempt := 0; attempt < 100; attempt++ {
		user := mustCreateUser(t, client, &service.User{
			Email:        fmt.Sprintf("%s-%d-%d@example.com", prefix, time.Now().UnixNano(), attempt),
			PasswordHash: "hash",
			Balance:      1,
		})
		if service.GrowthRolloutAllowsUser(user.ID, 20) {
			qualifyBlindboxIntegrationUser(t, user)
			return user
		}
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		require.NoError(t, err)
	}
	t.Fatal("unable to allocate a blindbox integration user inside the 20% rollout")
	return nil
}

func approveBlindboxIntegrationGovernance(t *testing.T, actorID int64) {
	t.Helper()
	now := time.Now().UTC()
	abnormalRatio := 0.02
	appealRatio := 0.01
	input := service.PlayGrowthGovernanceApprovalInput{
		BudgetAmount:   100,
		RolloutPercent: 20,
		Cohort: service.PlayGrowthCohortMetrics{
			WindowStart:              now.Add(-45 * 24 * time.Hour),
			WindowEnd:                now.Add(-31 * 24 * time.Hour),
			MetricsAvailable:         true,
			ParticipationUsers:       100,
			RealCall7dUsers:          40,
			RealCall7dRatio:          0.4,
			RealCall30dUsers:         60,
			RealCall30dRatio:         0.6,
			FirstRechargeUsers:       12,
			FirstRechargeRatio:       0.12,
			CouponsIssued:            100,
			CouponsRedeemed:          20,
			CouponRedemptionRatio:    0.2,
			ActualRewardCost:         25,
			D7RetainedUsers:          35,
			D7RetentionRatio:         0.35,
			AbnormalRedemptionUsers:  2,
			AbnormalRedemptionRatio:  &abnormalRatio,
			AppealCount:              4,
			FalsePositiveAppeals:     1,
			AppealFalsePositiveRatio: &appealRatio,
		},
		RuleVersion: service.PlayGrowthQualificationRuleVersion(),
		Reason:      "blindbox integration governance approval",
		ActorID:     actorID,
	}
	require.NoError(t, service.ValidatePlayGrowthGovernanceApproval(input, now))
	_, err := (&playRepository{client: testEntClient(t), sql: integrationDB}).CreateGrowthApproval(context.Background(), input)
	require.NoError(t, err)
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
	user := mustCreateBlindboxIntegrationUser(t, "blindbox-concurrent")
	approveBlindboxIntegrationGovernance(t, user.ID)
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
	user := mustCreateBlindboxIntegrationUser(t, "blindbox-same-key")
	approveBlindboxIntegrationGovernance(t, user.ID)
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
	user := mustCreateBlindboxIntegrationUser(t, "blindbox-rollback")
	approveBlindboxIntegrationGovernance(t, user.ID)
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
