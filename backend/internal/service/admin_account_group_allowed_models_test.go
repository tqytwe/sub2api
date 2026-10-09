//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// groupAllowedModelsRepoStub 记录 BindGroups 与 SetGroupAllowedModels 的调用顺序。
type groupAllowedModelsRepoStub struct {
	accountRepoStubForBulkUpdate
	calls              []string
	groupAllowedModels map[int64][]string
}

func (s *groupAllowedModelsRepoStub) BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error {
	s.calls = append(s.calls, "bind_groups")
	return s.accountRepoStubForBulkUpdate.BindGroups(ctx, accountID, groupIDs)
}

func (s *groupAllowedModelsRepoStub) SetGroupAllowedModels(_ context.Context, _ int64, allowed map[int64][]string) error {
	s.calls = append(s.calls, "set_group_allowed_models")
	s.groupAllowedModels = allowed
	return nil
}

func newGroupAllowedModelsAdminService() (*adminServiceImpl, *groupAllowedModelsRepoStub) {
	repo := &groupAllowedModelsRepoStub{
		accountRepoStubForBulkUpdate: accountRepoStubForBulkUpdate{
			getByIDAccounts: map[int64]*Account{
				7: {ID: 7, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Extra: map[string]any{}},
			},
		},
	}
	groupRepo := &groupRepoStubForAdmin{
		getByIDByID: map[int64]*Group{
			1: {ID: 1, Platform: PlatformOpenAI},
			2: {ID: 2, Platform: PlatformOpenAI},
		},
	}
	return &adminServiceImpl{accountRepo: repo, groupRepo: groupRepo}, repo
}

func TestAdminService_UpdateAccountWritesGroupAllowedModelsAtomically(t *testing.T) {
	svc, repo := newGroupAllowedModelsAdminService()
	groupIDs := []int64{1, 2}

	input := &UpdateAccountInput{GroupIDs: &groupIDs, SkipMixedChannelCheck: true}
	require.NoError(t, json.Unmarshal([]byte(`{"GroupAllowedModels":{"2":["gpt-5.5"]}}`), input))
	_, err := svc.UpdateAccount(context.Background(), 7, input)

	require.NoError(t, err)
	require.Equal(t, []string{"bind_groups_with_allowed_models"}, repo.calls, "bindings and policy must commit atomically")
	require.Equal(t, map[int64][]string{2: {"gpt-5.5"}}, repo.groupAllowedModels)
}

func TestAdminService_UpdateAccountLeavesGroupAllowedModelsWhenOmitted(t *testing.T) {
	svc, repo := newGroupAllowedModelsAdminService()
	groupIDs := []int64{1}

	_, err := svc.UpdateAccount(context.Background(), 7, &UpdateAccountInput{
		GroupIDs:              &groupIDs,
		SkipMixedChannelCheck: true,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"bind_groups"}, repo.calls, "请求没带该字段时不能动已有的限制")
}

func TestAdminService_UpdateAccountRejectsInvalidGroupAllowedModelsBeforeWriting(t *testing.T) {
	svc, repo := newGroupAllowedModelsAdminService()
	groupIDs := []int64{1}

	input := &UpdateAccountInput{GroupIDs: &groupIDs, SkipMixedChannelCheck: true}
	require.NoError(t, json.Unmarshal([]byte(`{"GroupAllowedModels":{"-1":["gpt-5.5"]}}`), input))
	_, err := svc.UpdateAccount(context.Background(), 7, input)

	require.Error(t, err)
	require.Empty(t, repo.calls)
	require.Empty(t, repo.updatedAccounts, "校验失败时不应写入任何账号数据")
}

func TestAdminService_UpdateAccountClearsGroupAllowedModels(t *testing.T) {
	svc, repo := newGroupAllowedModelsAdminService()
	_, err := svc.UpdateAccount(context.Background(), 7, &UpdateAccountInput{GroupAllowedModels: map[int64][]string{}})
	require.NoError(t, err)
	require.Equal(t, []string{"set_group_allowed_models"}, repo.calls)
	require.NotNil(t, repo.groupAllowedModels)
	require.Empty(t, repo.groupAllowedModels)
}

func TestAdminService_UpdateAccountGroupAllowedModelsUnsupportedFailsBeforeWrite(t *testing.T) {
	repo := &accountRepoStubForBulkUpdate{}
	svc := &adminServiceImpl{accountRepo: repo}
	_, err := svc.UpdateAccount(context.Background(), 7, &UpdateAccountInput{GroupAllowedModels: map[int64][]string{}})
	require.ErrorContains(t, err, "not supported")
	require.Empty(t, repo.getByIDCalled)
	require.Empty(t, repo.updatedAccounts)
}

func (s *groupAllowedModelsRepoStub) BindGroupsWithAllowedModels(ctx context.Context, accountID int64, groups []int64, allowed map[int64][]string) error {
	s.calls = append(s.calls, "bind_groups_with_allowed_models")
	s.groupAllowedModels = allowed
	return s.accountRepoStubForBulkUpdate.BindGroups(ctx, accountID, groups)
}

type groupAllowedModelsSetterOnlyRepo struct{ accountRepoStubForBulkUpdate }

func (*groupAllowedModelsSetterOnlyRepo) SetGroupAllowedModels(context.Context, int64, map[int64][]string) error {
	return nil
}

func TestAdminService_UpdateAccountCombinedGroupAllowedModelsUnsupportedFailsBeforeWrite(t *testing.T) {
	repo := &groupAllowedModelsSetterOnlyRepo{}
	svc := &adminServiceImpl{accountRepo: repo}
	ids := []int64{1}
	_, err := svc.UpdateAccount(context.Background(), 7, &UpdateAccountInput{GroupIDs: &ids, GroupAllowedModels: map[int64][]string{1: {"gpt-5.5"}}})
	require.ErrorContains(t, err, "not supported")
	require.Empty(t, repo.getByIDCalled)
	require.Empty(t, repo.updatedAccounts)
}
