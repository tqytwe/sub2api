package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scheduledPersistencePlans struct {
	ScheduledTestPlanRepository
	finished int
}

func (r *scheduledPersistencePlans) UpdateAfterRun(context.Context, int64, time.Time, time.Time) error {
	r.finished++
	return nil
}

type scheduledPersistenceResults struct {
	ScheduledTestResultRepository
	insertErr, pruneErr error
	pruned              int
}

func (r *scheduledPersistenceResults) Create(_ context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error) {
	return result, r.insertErr
}

func (r *scheduledPersistenceResults) PruneOldResults(context.Context, int64, int) error {
	r.pruned++
	return r.pruneErr
}

type scheduledPersistenceAccounts struct{ AccountRepository }

func (scheduledPersistenceAccounts) GetByID(context.Context, int64) (*Account, error) {
	return nil, errors.New("test account unavailable")
}

func TestScheduledProbeOnlyAdvancesAfterDurableResult(t *testing.T) {
	failure := errors.New("storage unavailable")
	for _, tc := range []struct {
		name                string
		insertErr, pruneErr error
		finished, pruned    int
	}{
		{name: "insert failure", insertErr: failure},
		{name: "prune failure", pruneErr: failure, finished: 1, pruned: 1},
		{name: "success", finished: 1, pruned: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plans := &scheduledPersistencePlans{}
			results := &scheduledPersistenceResults{insertErr: tc.insertErr, pruneErr: tc.pruneErr}
			runner := &ScheduledTestRunnerService{
				planRepo:       plans,
				scheduledSvc:   &ScheduledTestService{resultRepo: results},
				accountTestSvc: &AccountTestService{accountRepo: scheduledPersistenceAccounts{}},
			}
			runner.runOnePlan(context.Background(), &ScheduledTestPlan{ID: 1, AccountID: 2, CronExpression: "* * * * *", MaxResults: 10})
			require.Equal(t, tc.finished, plans.finished)
			require.Equal(t, tc.pruned, results.pruned)
		})
	}
}
