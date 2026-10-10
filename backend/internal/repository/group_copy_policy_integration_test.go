//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Exercise the actual admin operation, not only a repository helper: the old
// delete-then-insert path silently removed policies on surviving memberships.
func TestGroupCopyAccountsPreservesAndInheritsPolicies(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	groups := newGroupRepositoryWithSQL(client, tx)
	accounts := newAccountRepositoryWithSQL(client, tx, nil)
	target := mustCreateGroup(t, client, &service.Group{Name: "copy-policy-target", Platform: service.PlatformOpenAI})
	source := mustCreateGroup(t, client, &service.Group{Name: "copy-policy-source", Platform: service.PlatformOpenAI})
	second := mustCreateGroup(t, client, &service.Group{Name: "copy-policy-second", Platform: service.PlatformOpenAI})
	survivor := mustCreateAccount(t, client, &service.Account{Name: "copy-survivor"})
	added := mustCreateAccount(t, client, &service.Account{Name: "copy-added"})
	removed := mustCreateAccount(t, client, &service.Account{Name: "copy-removed"})
	unrestricted := mustCreateAccount(t, client, &service.Account{Name: "copy-unrestricted"})
	for _, row := range []struct {
		account, group int64
		models         string
	}{
		{survivor.ID, target.ID, `["target-only"]`}, {survivor.ID, source.ID, `["source-only"]`},
		{added.ID, source.ID, `["gpt-5.5"]`}, {added.ID, second.ID, `["gpt-5.3-*"]`},
		{removed.ID, target.ID, `["removed-only"]`}, {unrestricted.ID, source.ID, `null`},
	} {
		_, err := tx.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id,priority,allowed_models) VALUES($1,$2,37,NULLIF($3::jsonb,'null'::jsonb))`, row.account, row.group, row.models)
		require.NoError(t, err)
	}
	svc := service.NewAdminService(nil, nil, groups, accounts, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	_, err := svc.UpdateGroup(ctx, target.ID, &service.UpdateGroupInput{CopyAccountsFromGroupIDs: []int64{source.ID, second.ID}})
	require.NoError(t, err)
	for _, check := range []struct {
		id     int64
		models []string
		bound  bool
	}{
		{survivor.ID, []string{"target-only"}, true}, {added.ID, []string{"gpt-5.3-*", "gpt-5.5"}, true},
		{removed.ID, nil, false}, {unrestricted.ID, nil, true},
	} {
		got, err := accounts.GetByID(ctx, check.id)
		require.NoError(t, err)
		require.ElementsMatch(t, check.models, got.GroupAllowedModels(target.ID))
		require.Equal(t, check.bound, containsGroupID(got.GroupIDs, target.ID))
	}
	got, err := accounts.GetByID(ctx, survivor.ID)
	require.NoError(t, err)
	for _, binding := range got.AccountGroups {
		if binding.GroupID == target.ID {
			require.Equal(t, 37, binding.Priority)
		}
	}
}

func containsGroupID(ids []int64, id int64) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func TestGroupCopyAccountsEmptySourceRemovesBindings(t *testing.T) {
	ctx, accounts, accountID, groupID := accountGroupPolicyConcurrencyFixture(t)
	groups := newGroupRepositoryWithSQL(integrationEntClient, integrationDB)
	source := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "copy-empty-source"})
	t.Cleanup(func() { _, _ = groups.DeleteCascade(ctx, source.ID) })
	require.NoError(t, accounts.SetGroupAllowedModels(ctx, accountID, map[int64][]string{groupID: {"old"}}))
	require.NoError(t, groups.ReplaceAccountsFromGroups(ctx, groupID, []int64{source.ID}, nil))
	got, err := accounts.GetByID(ctx, accountID)
	require.NoError(t, err)
	require.NotContains(t, got.GroupIDs, groupID)
}

func TestGroupCopyAccountsOutboxFailureRollsBack(t *testing.T) {
	ctx, accounts, accountID, groupID := accountGroupPolicyConcurrencyFixture(t)
	groups := newGroupRepositoryWithSQL(integrationEntClient, integrationDB)
	source := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "copy-rollback-source"})
	t.Cleanup(func() { _, _ = groups.DeleteCascade(ctx, source.ID) })
	require.NoError(t, accounts.SetGroupAllowedModels(ctx, accountID, map[int64][]string{groupID: {"old"}}))
	constraint := fmt.Sprintf("copy_outbox_%d", groupID)
	_, err := integrationDB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE scheduler_outbox ADD CONSTRAINT %s CHECK(group_id IS DISTINCT FROM %d) NOT VALID", constraint, groupID))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("ALTER TABLE scheduler_outbox DROP CONSTRAINT IF EXISTS " + constraint)
	})
	require.ErrorContains(t, groups.ReplaceAccountsFromGroups(ctx, groupID, []int64{source.ID}, nil), constraint)
	got, err := accounts.GetByID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, []string{"old"}, got.GroupAllowedModels(groupID))
	_, err = integrationDB.Exec("ALTER TABLE scheduler_outbox DROP CONSTRAINT " + constraint)
	require.NoError(t, err)
	require.NoError(t, groups.ReplaceAccountsFromGroups(ctx, groupID, []int64{source.ID}, nil))
	got, err = accounts.GetByID(ctx, accountID)
	require.NoError(t, err)
	require.NotContains(t, got.GroupIDs, groupID)
}

func TestGroupCopyAccountsWaitsForLatestPolicy(t *testing.T) {
	ctx, accounts, accountID, groupID := accountGroupPolicyConcurrencyFixture(t)
	groups := newGroupRepositoryWithSQL(integrationEntClient, integrationDB)
	source := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "copy-concurrent-source"})
	t.Cleanup(func() { _, _ = groups.DeleteCascade(ctx, source.ID) })
	require.NoError(t, accounts.BindGroups(ctx, accountID, []int64{groupID, source.ID}))
	tx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, lockAccountGroupPolicyAccount(ctx, tx.Client(), accountID))
	done := make(chan error, 1)
	go func() { done <- groups.ReplaceAccountsFromGroups(ctx, groupID, []int64{source.ID}, []int64{accountID}) }()
	waitForGroupPolicyBlockedTransaction(t)
	_, err = tx.ExecContext(ctx, `UPDATE account_groups SET allowed_models='["latest-policy"]'::jsonb WHERE account_id=$1 AND group_id=$2`, accountID, groupID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, <-done)
	got, err := accounts.GetByID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, []string{"latest-policy"}, got.GroupAllowedModels(groupID))
}

func TestGroupCopyAccountsOversizeUnionRollsBack(t *testing.T) {
	ctx, accounts, existingID, targetID := accountGroupPolicyConcurrencyFixture(t)
	groups := newGroupRepositoryWithSQL(integrationEntClient, integrationDB)
	first := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "copy-limit-first"})
	second := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "copy-limit-second"})
	added := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "copy-limit-account"})
	t.Cleanup(func() {
		_ = accounts.Delete(ctx, added.ID)
		_, _ = groups.DeleteCascade(ctx, first.ID)
		_, _ = groups.DeleteCascade(ctx, second.ID)
	})
	require.NoError(t, accounts.BindGroups(ctx, added.ID, []int64{first.ID, second.ID}))
	policies := map[int64][]string{}
	for i := 0; i < 300; i++ {
		policies[first.ID] = append(policies[first.ID], fmt.Sprintf("first-%d", i))
		policies[second.ID] = append(policies[second.ID], fmt.Sprintf("second-%d", i))
	}
	require.NoError(t, accounts.SetGroupAllowedModels(ctx, added.ID, policies))
	require.NoError(t, accounts.SetGroupAllowedModels(ctx, existingID, map[int64][]string{targetID: {"must-survive-failure"}}))
	require.ErrorContains(t, groups.ReplaceAccountsFromGroups(ctx, targetID, []int64{first.ID, second.ID}, []int64{added.ID}), "500")
	got, err := accounts.GetByID(ctx, existingID)
	require.NoError(t, err)
	require.Equal(t, []string{"must-survive-failure"}, got.GroupAllowedModels(targetID))
	got, err = accounts.GetByID(ctx, added.ID)
	require.NoError(t, err)
	require.NotContains(t, got.GroupIDs, targetID)

	// Exercise service ordering too: a failed copy cannot leave an orphan group
	// or persist unrelated group fields while the UI reports failure.
	svc := service.NewAdminService(nil, nil, groups, accounts, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	name := fmt.Sprintf("copy-rejected-%d", added.ID)
	_, err = svc.CreateGroup(ctx, &service.CreateGroupInput{Name: name, Platform: service.PlatformAnthropic, RateMultiplier: 1, CopyAccountsFromGroupIDs: []int64{first.ID, second.ID}})
	require.Error(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM groups WHERE name=$1", name).Scan(&count))
	require.Zero(t, count, "failed create+copy must not leave an orphan group")
	before, err := groups.GetByID(ctx, targetID)
	require.NoError(t, err)
	newRate := 2.5
	_, err = svc.UpdateGroup(ctx, targetID, &service.UpdateGroupInput{RateMultiplier: &newRate, CopyAccountsFromGroupIDs: []int64{first.ID, second.ID}})
	require.Error(t, err)
	after, err := groups.GetByID(ctx, targetID)
	require.NoError(t, err)
	require.Equal(t, before.RateMultiplier, after.RateMultiplier, "failed update+copy must not partially save group attributes")
}

func TestGroupCopyAccountsReciprocalEmptyCopiesDoNotDeadlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	groups := newGroupRepositoryWithSQL(integrationEntClient, integrationDB)
	first := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "reciprocal-copy-first"})
	second := mustCreateGroup(t, integrationEntClient, &service.Group{Name: "reciprocal-copy-second"})
	t.Cleanup(func() {
		_, _ = groups.DeleteCascade(context.Background(), first.ID)
		_, _ = groups.DeleteCascade(context.Background(), second.ID)
	})
	start := make(chan struct{})
	done := make(chan error, 2)
	go func() { <-start; done <- groups.UpdateWithCopiedAccounts(ctx, first, []int64{second.ID}, nil) }()
	go func() { <-start; done <- groups.UpdateWithCopiedAccounts(ctx, second, []int64{first.ID}, nil) }()
	close(start)
	require.NoError(t, <-done)
	require.NoError(t, <-done)
}
