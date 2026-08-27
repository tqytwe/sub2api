package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMobileVideoExecutionKeyNameIsDistinctFromInteractiveManagedKeys(t *testing.T) {
	name := mobileVideoExecutionKeyName(42)
	if !IsMobileVideoExecutionAPIKeyName(name) {
		t.Fatalf("expected execution key name to be recognized: %q", name)
	}
	if IsMobileVideoExecutionAPIKeyName(NextChatManagedVideoAPIKeyName) {
		t.Fatal("interactive video workspace key must not be accepted by worker")
	}
	if IsMobileVideoExecutionAPIKeyName(NextChatManagedAPIKeyName) {
		t.Fatal("chat workspace key must not be accepted by worker")
	}
}

func TestGetMobileVideoExecutionKeyRechecksCurrentGroupPermission(t *testing.T) {
	groupID := int64(42)
	executionKey := APIKey{
		ID: 7, UserID: 9, Name: mobileVideoExecutionKeyName(groupID), Status: StatusAPIKeyActive, GroupID: &groupID,
	}
	svc := NewAPIKeyService(
		&nextChatAPIKeyRepoStub{keys: []APIKey{executionKey}},
		&nextChatUserRepoStub{user: &User{ID: 9, Status: StatusActive}},
		&nextChatGroupRepoStub{groups: []Group{{ID: groupID, Status: StatusActive}}},
		&nextChatSubscriptionRepoStub{}, nil, nil, nil,
	)

	resolved, err := svc.GetMobileVideoExecutionKey(context.Background(), 9, groupID, executionKey.ID)
	require.NoError(t, err)
	require.Equal(t, executionKey.ID, resolved.ID)

	denied := NewAPIKeyService(
		&nextChatAPIKeyRepoStub{keys: []APIKey{executionKey}},
		&nextChatUserRepoStub{user: &User{ID: 9, Status: StatusActive}},
		&nextChatGroupRepoStub{}, &nextChatSubscriptionRepoStub{}, nil, nil, nil,
	)
	_, err = denied.GetMobileVideoExecutionKey(context.Background(), 9, groupID, executionKey.ID)
	require.ErrorIs(t, err, ErrGroupNotAllowed)
}
