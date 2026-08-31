//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newGrowthGovernanceIntegrationRepository(t *testing.T) *playRepository {
	t.Helper()
	return &playRepository{client: testEntClient(t), sql: integrationDB}
}

func newGrowthGovernanceIntegrationUser(t *testing.T) int64 {
	t.Helper()
	user, err := testEntClient(t).User.Create().
		SetEmail(fmt.Sprintf("growth-governance-%d@example.test", time.Now().UnixNano())).
		SetPasswordHash("growth-governance-test").
		Save(context.Background())
	require.NoError(t, err)
	return user.ID
}

func newGrowthGovernanceIntegrationApproval(t *testing.T, repo *playRepository, actorID int64, budget float64) int64 {
	t.Helper()
	now := time.Now().UTC()
	state, err := repo.CreateGrowthApproval(context.Background(), service.PlayGrowthGovernanceApprovalInput{
		BudgetAmount:   budget,
		RolloutPercent: 10,
		Cohort: service.PlayGrowthCohortMetrics{
			WindowStart: now.Add(-30 * 24 * time.Hour),
			WindowEnd:   now.Add(-16 * 24 * time.Hour),
		},
		RuleVersion: service.PlayGrowthQualificationRuleVersion(),
		Reason:      "integration governance approval",
		ActorID:     actorID,
	})
	require.NoError(t, err)
	return state.ID
}

func TestGrowthGovernanceBudgetReservationSerializesConcurrentConnections(t *testing.T) {
	repo := newGrowthGovernanceIntegrationRepository(t)
	userID := newGrowthGovernanceIntegrationUser(t)
	approvalID := newGrowthGovernanceIntegrationApproval(t, repo, userID, 1)

	type reservationResult struct {
		reserved bool
		err      error
	}
	start := make(chan struct{})
	results := make(chan reservationResult, 2)
	var wg sync.WaitGroup
	for _, actionID := range []string{"checkin:concurrent:a", "checkin:concurrent:b"} {
		wg.Add(1)
		go func(actionID string) {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			reserved, err := repo.ReserveGrowthRewardBudget(ctx, approvalID, userID, service.PlayRewardSourceCheckin, actionID, 1)
			results <- reservationResult{reserved: reserved, err: err}
		}(actionID)
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	for result := range results {
		require.NoError(t, result.err)
		if result.reserved {
			successes++
		}
	}
	require.Equal(t, 1, successes, "a budget of one must admit exactly one concurrent reservation")

	var spent float64
	var entries int
	err := integrationDB.QueryRowContext(context.Background(), `
		SELECT COALESCE(SUM(amount), 0)::double precision, COUNT(*)
		FROM play_growth_reward_budget_ledger
		WHERE approval_id = $1`, approvalID).Scan(&spent, &entries)
	require.NoError(t, err)
	require.InDelta(t, 1, spent, 0.000001)
	require.Equal(t, 1, entries)
}

func TestGrowthGovernanceRevocationRejectsSubsequentReservation(t *testing.T) {
	repo := newGrowthGovernanceIntegrationRepository(t)
	userID := newGrowthGovernanceIntegrationUser(t)
	approvalID := newGrowthGovernanceIntegrationApproval(t, repo, userID, 1)

	revoked, err := repo.RevokeGrowthApproval(context.Background(), userID, "integration revocation after review")
	require.NoError(t, err)
	require.Greater(t, revoked.ID, approvalID)

	reserved, err := repo.ReserveGrowthRewardBudget(context.Background(), approvalID, userID, service.PlayRewardSourceCheckin, "checkin:revoked", 1)
	require.NoError(t, err)
	require.False(t, reserved, "the post-revocation current decision must reject an older approval")
}

func TestGrowthGovernanceBudgetLedgerBlocksHardDeletingItsUser(t *testing.T) {
	repo := newGrowthGovernanceIntegrationRepository(t)
	userID := newGrowthGovernanceIntegrationUser(t)
	approvalID := newGrowthGovernanceIntegrationApproval(t, repo, userID, 2)

	reserved, err := repo.ReserveGrowthRewardBudget(context.Background(), approvalID, userID, service.PlayRewardSourceCheckin, "checkin:retention", 1)
	require.NoError(t, err)
	require.True(t, reserved)

	_, err = integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	require.Error(t, err, "a budget reservation must retain its immutable subject reference")

	var entries int
	err = integrationDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM play_growth_reward_budget_ledger
		WHERE approval_id = $1 AND user_id = $2`, approvalID, userID).Scan(&entries)
	require.NoError(t, err)
	require.Equal(t, 1, entries)
}
