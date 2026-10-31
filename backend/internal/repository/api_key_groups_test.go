package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyGroupsRepositoryRoundTripAndLegacy(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "multi-groups@test.invalid")
	primary, err := client.Group.Create().SetName("primary").SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	additional, err := client.Group.Create().SetName("additional").SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	legacy := &service.APIKey{UserID: user.ID, Key: "sk-groups-legacy-test", Name: "legacy", Status: service.StatusActive, GroupID: &primary.ID}
	require.NoError(t, repo.Create(ctx, legacy))
	key := &service.APIKey{UserID: user.ID, Key: "sk-groups-multiple-test", Name: "multiple", Status: service.StatusActive, GroupID: &primary.ID, GroupIDs: []int64{primary.ID, additional.ID}}
	require.NoError(t, repo.Create(ctx, key))
	for _, getter := range []func() (*service.APIKey, error){
		func() (*service.APIKey, error) { return repo.GetByID(ctx, key.ID) },
		func() (*service.APIKey, error) { return repo.GetByKey(ctx, key.Key) },
		func() (*service.APIKey, error) { return repo.GetByKeyForAuth(ctx, key.Key) },
	} {
		got, err := getter()
		require.NoError(t, err)
		require.Equal(t, key.GroupIDs, got.GroupIDs)
		require.Equal(t, primary.ID, *got.GroupID)
	}
	got, err := repo.GetByID(ctx, legacy.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{primary.ID}, got.GroupIDs)
	listed, page, err := repo.ListByUserID(ctx, user.ID, pagination.PaginationParams{Page: 1, PageSize: 10}, service.APIKeyListFilters{GroupID: &additional.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Len(t, listed, 1)
	require.Equal(t, key.GroupIDs, listed[0].GroupIDs)
	count, err := repo.CountByGroupID(ctx, additional.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	credentials, err := repo.ListKeysByGroupID(ctx, additional.ID)
	require.NoError(t, err)
	require.Equal(t, []string{key.Key}, credentials)
	key.GroupID, key.GroupIDs = &additional.ID, []int64{additional.ID}
	require.NoError(t, repo.Update(ctx, key, service.APIKeyUpdateFields{GroupID: true}))
	got, err = repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{additional.ID}, got.GroupIDs)
	key.GroupID, key.GroupIDs = nil, nil
	require.NoError(t, repo.Update(ctx, key, service.APIKeyUpdateFields{GroupID: true}))
	got, err = repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Nil(t, got.GroupID)
	require.Empty(t, got.GroupIDs)
}

func TestAPIKeyGroupsRepositoryMissingSchemaFails(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "missing-groups@test.invalid")
	key := &service.APIKey{UserID: user.ID, Key: "sk-missing-groups-table-test", Name: "missing schema", Status: service.StatusActive}
	require.NoError(t, repo.Create(ctx, key))
	_, err := client.ExecContext(ctx, `DROP TABLE api_key_groups`)
	require.NoError(t, err)
	_, err = repo.GetByID(ctx, key.ID)
	require.Error(t, err, "missing schema must not silently hide configured groups")
}

func TestAPIKeyGroupsRepositoryCreateAndUpdateRollback(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "groups-rollback@test.invalid")
	primary, err := client.Group.Create().SetName("primary").SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	additional, err := client.Group.Create().SetName("additional").SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	key := &service.APIKey{UserID: user.ID, Key: "sk-rollback-groups-test", Name: "rollback", Status: service.StatusActive, GroupID: &primary.ID, GroupIDs: []int64{primary.ID, additional.ID}}
	_, err = client.ExecContext(ctx, `CREATE TRIGGER reject_extra_group BEFORE INSERT ON api_key_groups BEGIN SELECT RAISE(ABORT, 'test rejected extra group'); END`)
	require.NoError(t, err)
	require.Error(t, repo.Create(ctx, key))
	exists, err := repo.ExistsByKey(ctx, key.Key)
	require.NoError(t, err)
	require.False(t, exists, "failure must roll back the API key, not leave an orphan")
	_, err = client.ExecContext(ctx, `DROP TRIGGER reject_extra_group`)
	require.NoError(t, err)
	key.ID = 0
	require.NoError(t, repo.Create(ctx, key))
	_, err = client.ExecContext(ctx, `CREATE TRIGGER reject_extra_group BEFORE INSERT ON api_key_groups BEGIN SELECT RAISE(ABORT, 'test rejected extra group'); END`)
	require.NoError(t, err)
	key.GroupID, key.GroupIDs = &additional.ID, []int64{additional.ID, primary.ID}
	require.Error(t, repo.Update(ctx, key, service.APIKeyUpdateFields{GroupID: true}))
	got, err := repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, primary.ID, *got.GroupID)
	require.Equal(t, []int64{primary.ID, additional.ID}, got.GroupIDs)
}
