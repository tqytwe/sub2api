//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func (s *APIKeyRepoSuite) TestMultiGroupRepeatedRemovalPromotesRemainingGroup() {
	user := s.mustCreateUser("group-removal@test.invalid")
	a := s.mustCreateGroup("remove-primary")
	b := s.mustCreateGroup("remove-promoted")
	c := s.mustCreateGroup("remaining")
	key := &service.APIKey{UserID: user.ID, Key: "sk-multi-group-remove-test", Name: "remove", Status: service.StatusActive, GroupID: &a.ID, GroupIDs: []int64{a.ID, b.ID, c.ID}}
	s.Require().NoError(s.repo.Create(s.ctx, key))
	for _, removed := range []int64{a.ID, b.ID} {
		n, err := s.repo.ClearGroupIDByGroupID(s.ctx, removed)
		s.Require().NoError(err)
		s.Require().EqualValues(1, n)
		got, err := s.repo.GetByID(s.ctx, key.ID)
		s.Require().NoError(err)
		s.Require().NotNil(got.GroupID)
		s.Require().NotEqual(removed, *got.GroupID, "the CTE update must exclude the row deleted in the same statement")
	}
	got, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err)
	s.Require().Equal(c.ID, *got.GroupID)
	s.Require().Equal([]int64{c.ID}, got.GroupIDs)
}

func (s *APIKeyRepoSuite) TestMultiGroupMigrationIncludesAdditionalBindings() {
	user := s.mustCreateUser("group-migration@test.invalid")
	otherUser := s.mustCreateUser("group-migration-other@test.invalid")
	a := s.mustCreateGroup("migration-old")
	b := s.mustCreateGroup("migration-stable")
	c := s.mustCreateGroup("migration-new")
	for i, owner := range []int64{user.ID, otherUser.ID} {
		key := &service.APIKey{UserID: owner, Key: fmt.Sprintf("sk-multi-migration-%d", i), Name: "additional", Status: service.StatusActive, GroupID: &b.ID, GroupIDs: []int64{b.ID, a.ID}}
		s.Require().NoError(s.repo.Create(s.ctx, key))
	}
	n, err := s.repo.UpdateGroupIDByUserAndGroup(s.ctx, user.ID, a.ID, c.ID)
	s.Require().NoError(err)
	s.Require().EqualValues(1, n)
	got, _, err := s.repo.ListByUserID(s.ctx, user.ID, pagination.PaginationParams{Page: 1, PageSize: 10}, service.APIKeyListFilters{})
	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Require().Equal([]int64{b.ID, c.ID}, got[0].GroupIDs)
	other, _, err := s.repo.ListByUserID(s.ctx, otherUser.ID, pagination.PaginationParams{Page: 1, PageSize: 10}, service.APIKeyListFilters{})
	s.Require().NoError(err)
	s.Require().Len(other, 1)
	s.Require().Equal([]int64{b.ID, a.ID}, other[0].GroupIDs)
}

func TestAPIKeyGroupsConcurrentReplacementIsAtomic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	client := testEntClient(t)
	suffix := time.Now().UnixNano()
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("groups-concurrent-%d@test.invalid", suffix)})
	groups := make([]*service.Group, 4)
	for i := range groups {
		groups[i] = mustCreateGroup(t, client, &service.Group{Name: fmt.Sprintf("groups-concurrent-%d-%d", suffix, i)})
	}
	repo := newAPIKeyRepositoryWithSQL(client, integrationDB)
	key := &service.APIKey{UserID: user.ID, Key: fmt.Sprintf("sk-multi-concurrent-%d", suffix), Name: "concurrent", Status: service.StatusActive, GroupID: &groups[0].ID, GroupIDs: []int64{groups[0].ID, groups[1].ID}}
	require.NoError(t, repo.Create(ctx, key))
	t.Cleanup(func() {
		_, _ = integrationDB.Exec("DELETE FROM api_keys WHERE id = $1", key.ID)
		for _, g := range groups {
			_, _ = integrationDB.Exec("DELETE FROM groups WHERE id = $1", g.ID)
		}
		_, _ = integrationDB.Exec("DELETE FROM users WHERE id = $1", user.ID)
	})
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()
	first := newAPIKeyRepositoryWithSQL(tx.Client(), tx)
	key.GroupID, key.GroupIDs = &groups[1].ID, []int64{groups[1].ID, groups[2].ID}
	require.NoError(t, first.Update(dbent.NewTxContext(ctx, tx), key, service.APIKeyUpdateFields{GroupID: true}))
	secondKey := *key
	secondKey.GroupID, secondKey.GroupIDs = &groups[2].ID, []int64{groups[2].ID, groups[3].ID}
	done := make(chan error, 1)
	go func() { done <- repo.Update(ctx, &secondKey, service.APIKeyUpdateFields{GroupID: true}) }()
	waitForGroupPolicyBlockedTransaction(t)
	before, err := repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{groups[0].ID, groups[1].ID}, before.GroupIDs, "uncommitted bindings must not be visible")
	require.NoError(t, tx.Commit())
	require.NoError(t, <-done)
	after, err := repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{groups[2].ID, groups[3].ID}, after.GroupIDs, "a competing replacement must acquire the API key row lock before replacing its extra bindings")
}
