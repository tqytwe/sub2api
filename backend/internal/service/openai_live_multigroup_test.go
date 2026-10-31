package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetLiveCallForAPIKeyPreservesOwnerBoundaryBeforeGroupRestore(t *testing.T) {
	record := &LiveCallRecord{CallID: "call_multi_group", CallHash: hashLiveCallID("call_multi_group"), APIKeyID: 22, UserID: 33, GroupID: 44, Controller: LiveControllerPending}
	store := &liveTestStore{}
	require.NoError(t, store.SaveLiveCall(context.Background(), record, time.Hour))
	svc := &OpenAIGatewayService{cache: store}
	for _, tc := range []struct {
		name          string
		keyID, userID int64
	}{
		{"another key", 99, record.UserID},
		{"another user", record.APIKeyID, 99},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GetLiveCallForAPIKey(context.Background(), record.CallID, tc.keyID, tc.userID)
			require.ErrorIs(t, err, ErrLiveIdentityMismatch)
		})
	}
	loaded, err := svc.GetLiveCallForAPIKey(context.Background(), record.CallID, record.APIKeyID, record.UserID)
	require.NoError(t, err)
	require.EqualValues(t, 44, loaded.GroupID, "restore the original group before checking its current grant")
	record.Controller = LiveControllerClosed
	require.NoError(t, store.SaveLiveCall(context.Background(), record, time.Hour))
	_, err = svc.GetLiveCallForAPIKey(context.Background(), record.CallID, record.APIKeyID, record.UserID)
	require.ErrorIs(t, err, ErrLiveCallNotFound)
}

func TestPinAPIKeyGroupKeepsLiveIdentityAndRechecksCurrentPermission(t *testing.T) {
	primary := int64(1)
	groups := map[int64]*Group{
		1: {ID: 1, Status: StatusActive, RateMultiplier: 0.1},
		2: {ID: 2, Status: StatusActive, RateMultiplier: 10},
	}
	key := &APIKey{ID: 7, UserID: 8, GroupID: &primary, GroupIDs: []int64{1, 2}, User: &User{ID: 8, RestrictPublicGroups: true, AllowedGroups: []int64{1, 2}}}
	svc := &APIKeyService{groupRepo: selectionGroupRepo{groups: groups}}
	pinned, err := svc.PinAPIKeyGroup(context.Background(), key, 2)
	require.NoError(t, err)
	require.EqualValues(t, 2, *pinned.GroupID, "sideband must not reselect the cheaper group")
	require.EqualValues(t, 1, *key.GroupID, "auth snapshot must remain unchanged")
	key.User.AllowedGroups = []int64{1}
	_, err = svc.PinAPIKeyGroup(context.Background(), key, 2)
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	key.User.AllowedGroups = []int64{1, 2}
	groups[2].Status = StatusDisabled
	_, err = svc.PinAPIKeyGroup(context.Background(), key, 2)
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	groups[2].Status = StatusActive
	key.GroupIDs = []int64{1, 3}
	_, err = svc.PinAPIKeyGroup(context.Background(), key, 2)
	require.Error(t, err, "a removed binding must not be restored from the live record")
}
