//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type accountAdminUpdateCache struct {
	service.SchedulerCache
	onSet func(context.Context, *service.Account) error
}

func (c *accountAdminUpdateCache) SetAccount(ctx context.Context, account *service.Account) error {
	return c.onSet(ctx, account)
}

func TestAccountAdminUpdateShadowProxyAndCacheCommitTogether(t *testing.T) {
	ctx, accounts, id, groupID := accountGroupPolicyConcurrencyFixture(t)
	oldProxy := mustCreateProxy(t, integrationEntClient, &service.Proxy{Name: "atomic-old-proxy"})
	newProxy := mustCreateProxy(t, integrationEntClient, &service.Proxy{Name: "atomic-new-proxy"})
	parent, err := accounts.GetByID(ctx, id)
	require.NoError(t, err)
	parent.ProxyID = &oldProxy.ID
	require.NoError(t, accounts.Update(ctx, parent))
	shadow := mustCreateAccount(t, integrationEntClient, &service.Account{
		Name: "atomic-shadow", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
		ParentAccountID: &id, QuotaDimension: service.QuotaDimensionSpark, ProxyID: &oldProxy.ID,
	})
	t.Cleanup(func() {
		// Run before the parent fixture's cleanup: its parent FK is RESTRICT.
		for _, statement := range []string{
			"DELETE FROM scheduler_outbox WHERE account_id=$1",
			"DELETE FROM account_groups WHERE account_id=$1",
			"DELETE FROM accounts WHERE id=$1",
		} {
			_, err := integrationDB.Exec(statement, shadow.ID)
			require.NoError(t, err)
		}
		_, err := integrationDB.Exec("UPDATE accounts SET proxy_id=NULL WHERE id=$1", id)
		require.NoError(t, err)
		_, err = integrationDB.Exec("DELETE FROM proxies WHERE id IN ($1,$2)", oldProxy.ID, newProxy.ID)
		require.NoError(t, err)
	})
	var snapshots []int64
	accounts.schedulerCache = &accountAdminUpdateCache{onSet: func(ctx context.Context, snapshot *service.Account) error {
		var committedProxy int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT proxy_id FROM accounts WHERE id=$1", snapshot.ID).Scan(&committedProxy))
		require.Equal(t, newProxy.ID, committedProxy, "cache publication must follow commit")
		require.Equal(t, &newProxy.ID, snapshot.ProxyID)
		snapshots = append(snapshots, snapshot.ID)
		return nil
	}}
	groups := newGroupRepositoryWithSQL(integrationEntClient, integrationDB)
	svc := service.NewAdminService(nil, nil, groups, accounts, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	constraint := fmt.Sprintf("account_shadow_failure_%d", id)
	_, err = integrationDB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE scheduler_outbox ADD CONSTRAINT %s CHECK(account_id IS DISTINCT FROM %d OR event_type<>'account_groups_changed') NOT VALID", constraint, id))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("ALTER TABLE scheduler_outbox DROP CONSTRAINT IF EXISTS " + constraint)
	})
	input := &service.UpdateAccountInput{Name: "atomic-parent-updated", ProxyID: &newProxy.ID, GroupAllowedModels: map[int64][]string{groupID: {"restricted-model"}}, SkipMixedChannelCheck: true}
	_, err = svc.UpdateAccount(ctx, id, input)
	require.ErrorContains(t, err, constraint)
	require.Empty(t, snapshots, "failed transaction must not publish parent or shadow snapshots")
	for _, accountID := range []int64{id, shadow.ID} {
		got, err := accounts.GetByID(ctx, accountID)
		require.NoError(t, err)
		require.Equal(t, &oldProxy.ID, got.ProxyID)
		if accountID == id {
			require.Equal(t, parent.Name, got.Name)
			require.Empty(t, got.GroupAllowedModels(groupID))
		} else {
			require.Equal(t, shadow.Name, got.Name)
		}
	}
	_, err = integrationDB.ExecContext(ctx, "ALTER TABLE scheduler_outbox DROP CONSTRAINT "+constraint)
	require.NoError(t, err)
	updated, err := svc.UpdateAccount(ctx, id, input)
	require.NoError(t, err)
	require.Equal(t, input.Name, updated.Name)
	require.Equal(t, []string{"restricted-model"}, updated.GroupAllowedModels(groupID))
	require.Equal(t, []int64{id, shadow.ID}, snapshots)
}

func TestAccountAdminUpdatePolicyFailureRollsBackActivation(t *testing.T) {
	for _, rebind := range []bool{false, true} {
		t.Run(fmt.Sprintf("rebind_%t", rebind), func(t *testing.T) {
			ctx, accounts, id, groupID := accountGroupPolicyConcurrencyFixture(t)
			before, err := accounts.GetByID(ctx, id)
			require.NoError(t, err)
			before.Status = "inactive"
			require.NoError(t, accounts.Update(ctx, before))
			groups := newGroupRepositoryWithSQL(integrationEntClient, integrationDB)
			svc := service.NewAdminService(nil, nil, groups, accounts, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			constraint := fmt.Sprintf("account_edit_failure_%d", id)
			_, err = integrationDB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE scheduler_outbox ADD CONSTRAINT %s CHECK(account_id IS DISTINCT FROM %d OR event_type<>'account_groups_changed') NOT VALID", constraint, id))
			require.NoError(t, err)
			t.Cleanup(func() {
				_, _ = integrationDB.Exec("ALTER TABLE scheduler_outbox DROP CONSTRAINT IF EXISTS " + constraint)
			})
			var outboxBefore int
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM scheduler_outbox WHERE account_id=$1", id).Scan(&outboxBefore))
			input := &service.UpdateAccountInput{Name: "must-rollback", Status: service.StatusActive, Credentials: map[string]any{"model_mapping": map[string]any{"restricted-model": "restricted-model"}}, GroupAllowedModels: map[int64][]string{groupID: {"restricted-model"}}, SkipMixedChannelCheck: true}
			if rebind {
				ids := []int64{groupID}
				input.GroupIDs = &ids
			}
			_, err = svc.UpdateAccount(ctx, id, input)
			require.ErrorContains(t, err, constraint)
			got, err := accounts.GetByID(ctx, id)
			require.NoError(t, err)
			require.Equal(t, "inactive", got.Status, "failed policy write must not activate an unrestricted account")
			require.Equal(t, before.Name, got.Name)
			require.Equal(t, before.Credentials, got.Credentials)
			require.Equal(t, before.RateMultiplier, got.RateMultiplier)
			require.Equal(t, before.GroupIDs, got.GroupIDs)
			require.Empty(t, got.GroupAllowedModels(groupID))
			var outboxAfter int
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM scheduler_outbox WHERE account_id=$1", id).Scan(&outboxAfter))
			require.Equal(t, outboxBefore, outboxAfter)
			_, err = integrationDB.ExecContext(ctx, "ALTER TABLE scheduler_outbox DROP CONSTRAINT "+constraint)
			require.NoError(t, err)
			got, err = svc.UpdateAccount(ctx, id, input)
			require.NoError(t, err)
			require.Equal(t, service.StatusActive, got.Status)
			require.Equal(t, input.Name, got.Name)
			require.Equal(t, []string{"restricted-model"}, got.GroupAllowedModels(groupID))
		})
	}
}

func TestAccountAdminUpdatePreservesExplicitBillingIntent(t *testing.T) {
	ctx, accounts, id, groupID := accountGroupPolicyConcurrencyFixture(t)
	stale, err := accounts.GetByID(ctx, id)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "UPDATE accounts SET rate_multiplier=3 WHERE id=$1", id)
	require.NoError(t, err)
	stale.Name = "unrelated-edit-preserves-current-rate"
	ids := []int64{groupID}
	updated, err := accounts.UpdateAdminAccount(ctx, stale, service.AccountAdminUpdateOptions{GroupIDs: &ids})
	require.NoError(t, err)
	require.NotNil(t, updated.RateMultiplier)
	require.Equal(t, 3.0, *updated.RateMultiplier)
	zero := 0.0
	updated, err = accounts.UpdateAdminAccount(ctx, updated, service.AccountAdminUpdateOptions{RateMultiplier: &zero})
	require.NoError(t, err)
	require.Equal(t, 0.0, *updated.RateMultiplier)
}
