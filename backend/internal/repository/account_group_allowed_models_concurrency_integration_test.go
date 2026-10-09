//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func accountGroupPolicyConcurrencyFixture(t *testing.T) (context.Context, *accountRepository, int64, int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	client := testEntClient(t)
	group := mustCreateGroup(t, client, &service.Group{Name: fmt.Sprintf("group-policy-%d", time.Now().UnixNano())})
	account := mustCreateAccount(t, client, &service.Account{Name: fmt.Sprintf("account-policy-%d", time.Now().UnixNano())})
	repo := newAccountRepositoryWithSQL(client, integrationDB, nil)
	require.NoError(t, repo.BindGroups(ctx, account.ID, []int64{group.ID}))
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("DELETE FROM scheduler_outbox WHERE account_id = $1 OR group_id = $2", account.ID, group.ID)
		_, _ = integrationDB.Exec("DELETE FROM account_groups WHERE account_id = $1", account.ID)
		_, _ = integrationDB.Exec("DELETE FROM accounts WHERE id = $1", account.ID)
		_, _ = integrationDB.Exec("DELETE FROM groups WHERE id = $1", group.ID)
	})
	return ctx, repo, account.ID, group.ID
}

func waitForGroupPolicyBlockedTransaction(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		var count int
		err := integrationDB.QueryRow("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND pid <> pg_backend_pid()").Scan(&count)
		return err == nil && count > 0
	}, 3*time.Second, 10*time.Millisecond, "concurrent mutation must reach its database lock before commit")
}

func TestAccountGroupAllowedModelsConcurrentRebindPreservesRestriction(t *testing.T) {
	ctx, repo, accountID, groupID := accountGroupPolicyConcurrencyFixture(t)
	tx, err := repo.client.Tx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()
	writer := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
	require.NoError(t, writer.SetGroupAllowedModels(ctx, accountID, map[int64][]string{groupID: {"gpt-5.5"}}))
	done := make(chan error, 1)
	go func() { done <- repo.BindGroups(ctx, accountID, []int64{groupID}) }()
	waitForGroupPolicyBlockedTransaction(t)
	require.NoError(t, tx.Commit())
	require.NoError(t, <-done)
	account, err := repo.GetByID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5.5"}, account.GroupAllowedModels(groupID), "rebind must read the committed restriction, not rebuild from a stale pre-lock snapshot")
}

func TestAccountGroupAllowedModelsConcurrentGroupDeletionRejectsRebind(t *testing.T) {
	ctx, repo, accountID, groupID := accountGroupPolicyConcurrencyFixture(t)
	require.NoError(t, repo.SetGroupAllowedModels(ctx, accountID, map[int64][]string{groupID: {"gpt-5.5"}}))
	tx, err := repo.client.Tx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()
	deleter := newGroupRepositoryWithSQL(tx.Client(), tx)
	_, err = deleter.DeleteCascade(ctx, groupID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- repo.BindGroups(ctx, accountID, []int64{groupID}) }()
	waitForGroupPolicyBlockedTransaction(t)
	require.NoError(t, tx.Commit(), "group deletion must not acquire an account lock in reverse order")
	require.ErrorIs(t, <-done, service.ErrGroupNotFound)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM account_groups WHERE account_id = $1 AND group_id = $2", accountID, groupID).Scan(&count))
	require.Zero(t, count, "a deleted group must not be resurrected by rebinding")
}

// Force a real PostgreSQL outbox constraint failure, not a mocked driver error.
// Policy changes must roll back together with their scheduler notification.
func (s *AccountRepoSuite) TestGroupAllowedModels_OutboxFailureRollsBack() {
	t := s.T()
	ctx, repo, accountID, groupID := accountGroupPolicyConcurrencyFixture(t)
	require.NoError(t, repo.SetGroupAllowedModels(ctx, accountID, map[int64][]string{groupID: {"old-model"}}))
	constraint := fmt.Sprintf("account_policy_outbox_%d", accountID)
	_, err := integrationDB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE scheduler_outbox ADD CONSTRAINT %s CHECK (account_id IS DISTINCT FROM %d) NOT VALID", constraint, accountID))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("ALTER TABLE scheduler_outbox DROP CONSTRAINT IF EXISTS " + constraint)
	})
	err = repo.SetGroupAllowedModels(ctx, accountID, map[int64][]string{groupID: {"gpt-5.5"}})
	require.ErrorContains(t, err, constraint)
	account, err := repo.GetByID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, []string{"old-model"}, account.GroupAllowedModels(groupID), "failed notification must roll back persisted membership restrictions")
}

func TestAccountGroupAllowedModelsConcurrentAccountDeletionDoesNotDeadlock(t *testing.T) {
	ctx, repo, accountID, groupID := accountGroupPolicyConcurrencyFixture(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, integrationDB)))
	deletedMembership := make(chan struct{})
	continueDelete := make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(continueDelete) })
	client.AccountGroup.Use(func(next dbent.Mutator) dbent.Mutator {
		return dbent.MutateFunc(func(ctx context.Context, m dbent.Mutation) (dbent.Value, error) {
			value, err := next.Mutate(ctx, m)
			if err == nil && m.Op().Is(dbent.OpDelete|dbent.OpDeleteOne) {
				close(deletedMembership)
				select {
				case <-continueDelete:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return value, err
		})
	})
	deleter := newAccountRepositoryWithSQL(client, integrationDB, nil)
	deleteDone := make(chan error, 1)
	go func() { deleteDone <- deleter.Delete(ctx, accountID) }()
	select {
	case <-deletedMembership:
	case err := <-deleteDone:
		t.Fatalf("deletion stopped before membership mutation: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	policyDone := make(chan error, 1)
	go func() {
		policyDone <- repo.SetGroupAllowedModels(ctx, accountID, map[int64][]string{groupID: {"gpt-5.5"}})
	}()
	waitForGroupPolicyBlockedTransaction(t)
	release.Do(func() { close(continueDelete) })
	deleteErr, policyErr := <-deleteDone, <-policyDone
	t.Logf("delete result=%v; concurrent policy result=%v", deleteErr, policyErr)
	require.NoError(t, deleteErr, "account deletion and policy mutation must use account-first locks")
	require.ErrorIs(t, policyErr, service.ErrAccountNotFound)
}

type accountGroupBindingsWithModelsWriter interface {
	BindGroupsWithAllowedModels(context.Context, int64, []int64, map[int64][]string) error
}

func (s *AccountRepoSuite) TestGroupAllowedModels_CombinedFailureRestoresBindingsAndPolicy() {
	t := s.T()
	ctx, repo, accountID, oldGroupID := accountGroupPolicyConcurrencyFixture(t)
	newGroup := mustCreateGroup(t, repo.client, &service.Group{Name: fmt.Sprintf("policy-new-%d", time.Now().UnixNano())})
	t.Cleanup(func() { _, _ = integrationDB.Exec("DELETE FROM groups WHERE id = $1", newGroup.ID) })
	require.NoError(t, repo.SetGroupAllowedModels(ctx, accountID, map[int64][]string{oldGroupID: {"old-model"}}))
	writer, ok := any(repo).(accountGroupBindingsWithModelsWriter)
	require.True(t, ok, "combined binding and policy operation must be atomic")
	constraint := fmt.Sprintf("account_combined_outbox_%d", accountID)
	_, err := integrationDB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE scheduler_outbox ADD CONSTRAINT %s CHECK (account_id IS DISTINCT FROM %d) NOT VALID", constraint, accountID))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("ALTER TABLE scheduler_outbox DROP CONSTRAINT IF EXISTS " + constraint)
	})
	err = writer.BindGroupsWithAllowedModels(ctx, accountID, []int64{newGroup.ID}, map[int64][]string{newGroup.ID: {"gpt-5.5"}})
	require.ErrorContains(t, err, constraint)
	account, err := repo.GetByID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, []int64{oldGroupID}, account.GroupIDs)
	require.Equal(t, []string{"old-model"}, account.GroupAllowedModels(oldGroupID))
}

func TestAccountGroupAllowedModelsConcurrentCombinedUpdatesStayPaired(t *testing.T) {
	ctx, repo, accountID, oldGroupID := accountGroupPolicyConcurrencyFixture(t)
	firstGroup := mustCreateGroup(t, repo.client, &service.Group{Name: fmt.Sprintf("policy-first-%d", time.Now().UnixNano())})
	firstGroupID := firstGroup.ID
	secondGroup := mustCreateGroup(t, repo.client, &service.Group{Name: fmt.Sprintf("policy-concurrent-%d", time.Now().UnixNano())})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("DELETE FROM account_groups WHERE account_id = $1", accountID)
		_, _ = integrationDB.Exec("DELETE FROM groups WHERE id IN ($1,$2)", firstGroupID, secondGroup.ID)
	})
	writer, ok := any(repo).(accountGroupBindingsWithModelsWriter)
	require.True(t, ok, "combined binding and policy operation must be atomic")
	var initialEvents int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM scheduler_outbox WHERE account_id = $1 AND event_type = $2", accountID, service.SchedulerOutboxEventAccountGroupsChanged).Scan(&initialEvents))
	// Hold request A's commit after its complete mutation. Request B must wait
	// before reading memberships, then replace both bindings and policy together.
	tx, err := repo.client.Tx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()
	firstWriter := any(newAccountRepositoryWithSQL(tx.Client(), tx, nil)).(accountGroupBindingsWithModelsWriter)
	require.NoError(t, firstWriter.BindGroupsWithAllowedModels(ctx, accountID, []int64{firstGroupID}, map[int64][]string{firstGroupID: {"first-model"}}))
	done := make(chan error, 1)
	go func() {
		done <- writer.BindGroupsWithAllowedModels(ctx, accountID, []int64{secondGroup.ID}, map[int64][]string{secondGroup.ID: {"second-model"}})
	}()
	waitForGroupPolicyBlockedTransaction(t)
	// Other readers still see the prior committed state, never half of request A.
	before, err := repo.GetByID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, []int64{oldGroupID}, before.GroupIDs, "uncommitted new membership must not be published")
	require.Empty(t, before.GroupAllowedModels(firstGroupID))
	require.NoError(t, tx.Commit())
	require.NoError(t, <-done)
	after, err := repo.GetByID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, []int64{secondGroup.ID}, after.GroupIDs)
	require.Equal(t, []string{"second-model"}, after.GroupAllowedModels(secondGroup.ID))
	var events int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM scheduler_outbox WHERE account_id = $1 AND event_type = $2 AND payload @> $3::jsonb", accountID, service.SchedulerOutboxEventAccountGroupsChanged, fmt.Sprintf(`{"group_ids":[%d,%d]}`, firstGroupID, secondGroup.ID)).Scan(&events))
	require.Equal(t, 1, events, "combined change emits one union-scope notification")
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM scheduler_outbox WHERE account_id = $1 AND event_type = $2", accountID, service.SchedulerOutboxEventAccountGroupsChanged).Scan(&events))
	require.Equal(t, initialEvents+2, events, "each combined request must emit exactly one notification")
}

func (s *AccountRepoSuite) TestGroupAllowedModels_AccountDeletionRemainsIdempotent() {
	ctx, repo, accountID, _ := accountGroupPolicyConcurrencyFixture(s.T())
	s.Require().NoError(repo.Delete(ctx, accountID))
	s.Require().NoError(repo.Delete(ctx, accountID))
}
