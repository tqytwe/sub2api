package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type listingSubscriptionRepo struct {
	UserSubscriptionRepository
	subscriptions map[int64]*UserSubscription
	err           error
}

func (r listingSubscriptionRepo) GetActiveByUserIDAndGroupID(_ context.Context, _ int64, id int64) (*UserSubscription, error) {
	if r.err != nil {
		return nil, r.err
	}
	if subscription := r.subscriptions[id]; subscription != nil {
		return subscription, nil
	}
	return nil, ErrSubscriptionNotFound
}

func TestAPIKeyModelListingReadPermissionsAndSnapshots(t *testing.T) {
	primary := int64(4)
	key := &APIKey{UserID: 8, GroupID: &primary, GroupIDs: []int64{4, 3, 2, 1, 5}, User: &User{ID: 8, RestrictPublicGroups: true, AllowedGroups: []int64{1, 2}}}
	service := &APIKeyService{groupRepo: selectionGroupRepo{groups: map[int64]*Group{
		1: {ID: 1, Status: StatusActive}, 2: {ID: 2, Status: StatusActive}, 3: {ID: 3, Status: StatusActive, IsExclusive: true}, 4: {ID: 4, Status: StatusDisabled},
	}}}
	groups, err := service.ModelListingGroups(context.Background(), key)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.EqualValues(t, 1, *groups[0].GroupID)
	require.EqualValues(t, 2, *groups[1].GroupID)
	require.EqualValues(t, 4, *key.GroupID)
	groups[0].User.Balance = 19
	require.Zero(t, key.User.Balance)
	require.Zero(t, groups[1].User.Balance)
	key.User.AllowedGroups = nil
	_, err = service.ModelListingGroups(context.Background(), key)
	require.Equal(t, http.StatusForbidden, infraerrors.Code(err))
}

func TestAPIKeyModelListingSubscriptionEntitlementAndFailures(t *testing.T) {
	now := time.Now()
	primary := int64(1)
	key := &APIKey{UserID: 8, GroupID: &primary, GroupIDs: []int64{1, 2, 3}, User: &User{ID: 8}}
	groups := selectionGroupRepo{groups: map[int64]*Group{
		1: {ID: 1, Status: StatusActive},
		2: {ID: 2, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
		3: {ID: 3, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}}
	sub := &UserSubscription{ID: 9, UserID: 8, GroupID: 2, Status: SubscriptionStatusActive, StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}
	service := &APIKeyService{groupRepo: groups, userSubRepo: listingSubscriptionRepo{subscriptions: map[int64]*UserSubscription{2: sub}}}
	listed, err := service.ModelListingGroups(context.Background(), key)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.EqualValues(t, 2, *listed[1].GroupID)
	sub.ExpiresAt = now.Add(-time.Minute)
	listed, err = service.ModelListingGroups(context.Background(), key)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	service.userSubRepo = listingSubscriptionRepo{err: errors.New("private database failure")}
	_, err = service.ModelListingGroups(context.Background(), key)
	require.Equal(t, http.StatusServiceUnavailable, infraerrors.Code(err))
	require.NotContains(t, err.Error(), "private database failure")
	service.userSubRepo = nil
	_, err = service.ModelListingGroups(context.Background(), key)
	require.Equal(t, http.StatusServiceUnavailable, infraerrors.Code(err))
}
