//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUpstreamChannelEntitySoftDeleteRetainsOwnership(t *testing.T) {
	ctx := context.Background()
	db, client := integrationDB, integrationEntClient
	repo, ok := NewChannelRepository(db).(service.UpstreamChannelRepository)
	require.True(t, ok)
	fixture := "delete-" + uuid.NewString()
	channel, err := repo.CreateUpstreamChannel(ctx, service.UpstreamChannelMutation{Code: fixture, Name: "Delete fixture", Confirm: true})
	require.NoError(t, err)
	var id int64
	t.Cleanup(func() {
		_, err := db.Exec(`DELETE FROM accounts WHERE id=$1`, id)
		require.NoError(t, err)
		_, err = db.Exec(`DELETE FROM upstream_channels WHERE id=$1`, channel.ID)
		require.NoError(t, err)
	})
	require.NoError(t, db.QueryRow(`INSERT INTO accounts(name,platform,type,credentials) VALUES($1,'gemini','oauth','{}') RETURNING id`, fixture).Scan(&id))
	_, err = repo.BindUpstreamChannelAccounts(ctx, service.UpstreamChannelBindingsMutation{ChannelID: channel.ID, Bindings: []service.UpstreamChannelBinding{{AccountID: id}}, Confirm: true})
	require.NoError(t, err)
	before, err := repo.GetUpstreamChannel(ctx, channel.ID)
	require.NoError(t, err)
	require.NoError(t, NewAdminAccountRepository(client, db, nil).Delete(ctx, id))
	bindings, err := repo.ReadUpstreamChannelBindings(ctx, []int64{id})
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	require.Equal(t, channel.ID, bindings[0].ChannelID, "soft-deleted accounts must retain explicit analytics ownership")
	require.True(t, bindings[0].Deleted)
	after, err := repo.GetUpstreamChannel(ctx, channel.ID)
	require.NoError(t, err)
	require.Equal(t, before.Revision, after.Revision)
	require.ErrorIs(t, repo.DeleteUpstreamChannel(ctx, channel.ID, service.UpstreamChannelMutation{Revision: &after.Revision, Confirm: true}), service.ErrUpstreamChannelConflict)
	// Exercise true physical deletion separately, with no usage or billing rows.
	_, err = db.Exec(`DELETE FROM accounts WHERE id=$1`, id)
	require.NoError(t, err)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM upstream_channel_bindings WHERE account_id=$1`, id).Scan(&count))
	require.Zero(t, count)
	final, err := repo.GetUpstreamChannel(ctx, channel.ID)
	require.NoError(t, err)
	require.Equal(t, after.Revision+1, final.Revision)
	require.Zero(t, final.Members)
}

func TestUpstreamChannelEntityAccountCreationIsAtomic(t *testing.T) {
	ctx := context.Background()
	db := integrationDB
	repo := NewChannelRepository(db).(service.UpstreamChannelRepository)
	fixture := "create-" + uuid.NewString()
	channel, err := repo.CreateUpstreamChannel(ctx, service.UpstreamChannelMutation{Code: fixture, Name: fixture, Confirm: true})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec(`DELETE FROM accounts WHERE name IN ($1,$2)`, fixture, fixture+"-missing")
		require.NoError(t, err)
		_, err = db.Exec(`DELETE FROM upstream_channels WHERE id=$1`, channel.ID)
		require.NoError(t, err)
	})
	accounts := NewAdminAccountRepository(integrationEntClient, db, nil)
	account := &service.Account{Name: fixture, Platform: service.PlatformGemini, Type: service.AccountTypeOAuth, Status: service.StatusActive}
	require.NoError(t, accounts.Create(service.WithInitialUpstreamChannel(ctx, channel.ID), account))
	bindings, err := repo.ReadUpstreamChannelBindings(ctx, []int64{account.ID})
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	require.Equal(t, channel.ID, bindings[0].ChannelID)
	after, err := repo.GetUpstreamChannel(ctx, channel.ID)
	require.NoError(t, err)
	require.Equal(t, channel.Revision+1, after.Revision)

	missing := *account
	missing.ID, missing.Name = 0, fixture+"-missing"
	require.ErrorIs(t, accounts.Create(service.WithInitialUpstreamChannel(ctx, 1<<60), &missing), service.ErrUpstreamChannelNotFound)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE name=$1`, missing.Name).Scan(&count))
	require.Zero(t, count, "an invalid channel must not leave an account behind")
}
