package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type selectionGroupRepo struct {
	GroupRepository
	groups map[int64]*Group
}

type selectionSubscriptionRepo struct {
	UserSubscriptionRepository
	sub *UserSubscription
}

func (r selectionSubscriptionRepo) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	copy := *r.sub
	return &copy, nil
}

func TestAPIKeyRequestGroupSelectionUsesAuthoritativePackageEntitlement(t *testing.T) {
	for _, status := range []string{PackageEntitlementExhausted, PackageEntitlementExpired, "database error"} {
		t.Run(status, func(t *testing.T) {
			now := time.Now()
			primary := int64(1)
			subRepo := selectionSubscriptionRepo{sub: &UserSubscription{ID: 4, UserID: 8, GroupID: 1, Status: SubscriptionStatusActive, StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}}
			subscriptions, mock := newSubscriptionTermEntitlementTestService(t, subRepo, now)
			query := mock.ExpectQuery(`(?s)SELECT id, payment_order_id, source_type, granted_by,.*FROM subscription_package_entitlements.*ORDER BY expires_at ASC, id ASC`).WithArgs(int64(8), int64(1))
			failure := errors.New("package entitlement database unavailable")
			if status == "database error" {
				query.WillReturnError(failure)
			} else {
				expiry, used, reason := now.Add(time.Hour), int64(1), PackageExhaustedByRequest
				if status == PackageEntitlementExpired {
					expiry, used, reason = now.Add(-time.Hour), 0, ""
				}
				query.WillReturnRows(sqlmock.NewRows(packageEntitlementTermColumns).AddRow(11, 12, PackageEntitlementSourcePayment, nil, 13, 8, 1, now.Add(-2*time.Hour), expiry, status, reason, 1, used, nil, 0, nil, 0))
			}
			groups := map[int64]*Group{
				1: {ID: 1, Status: StatusActive, RateMultiplier: 0.1, SubscriptionType: SubscriptionTypeSubscription},
				2: {ID: 2, Status: StatusActive, RateMultiplier: 1},
			}
			key := &APIKey{ID: 7, UserID: 8, GroupID: &primary, GroupIDs: []int64{1, 2}, User: &User{ID: 8}}
			svc := &APIKeyService{userSubRepo: subRepo, groupRepo: selectionGroupRepo{groups: groups}, groupRouter: selectionRouter{states: map[int64]APIKeyGroupAvailability{1: {Available: true}, 2: {Available: true}}}}
			selected, err := svc.SelectRequestGroup(context.Background(), key, []string{"model"}, "token", subscriptions, nil, true)
			if status == "database error" {
				require.ErrorIs(t, err, failure, "infrastructure failures must not be hidden as candidate exhaustion")
			} else {
				require.NoError(t, err)
				require.EqualValues(t, 2, *selected.GroupID, "an active legacy subscription cannot override exhausted or expired package hard caps")
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func (r selectionGroupRepo) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	if g, ok := r.groups[id]; ok {
		return g, nil
	}
	return nil, ErrGroupNotFound
}

type selectionRouter struct {
	states map[int64]APIKeyGroupAvailability
	err    error
}

func (r selectionRouter) ProbeAPIKeyGroup(_ context.Context, g *Group, _, _ string) (APIKeyGroupAvailability, error) {
	return r.states[g.ID], r.err
}

func TestAPIKeyRequestGroupSelectionPreservesPermissionAndBillingIdentity(t *testing.T) {
	primary := int64(1)
	groups := map[int64]*Group{
		1: {ID: 1, Status: StatusActive, RateMultiplier: 0.1},
		2: {ID: 2, Status: StatusActive, RateMultiplier: 2},
		3: {ID: 3, Status: StatusActive, RateMultiplier: 3},
	}
	key := &APIKey{ID: 7, UserID: 8, GroupID: &primary, GroupIDs: []int64{1, 2, 3}, User: &User{ID: 8, RestrictPublicGroups: true, AllowedGroups: []int64{2, 3}}}
	svc := &APIKeyService{groupRepo: selectionGroupRepo{groups: groups}, groupRouter: selectionRouter{states: map[int64]APIKeyGroupAvailability{1: {Available: true}, 2: {Available: true}, 3: {Available: true}}}}
	selected, err := svc.SelectRequestGroup(context.Background(), key, []string{"model"}, "token", nil, nil, false)
	require.NoError(t, err)
	require.EqualValues(t, 2, *selected.GroupID, "a revoked cheaper primary must never win")
	require.EqualValues(t, 1, *key.GroupID, "selection must not mutate auth-cache snapshot")
	selected.User.Balance = 123
	require.Zero(t, key.User.Balance)
	key.User.AllowedGroups = nil
	_, err = svc.SelectRequestGroup(context.Background(), key, nil, "token", nil, nil, false)
	require.Error(t, err) // no candidate may bypass user grants
}

func TestAPIKeyRequestGroupSelectionPrefersIdleAndSurfacesProbeErrors(t *testing.T) {
	primary := int64(1)
	key := &APIKey{UserID: 8, GroupID: &primary, GroupIDs: []int64{1, 2}, User: &User{ID: 8}}
	svc := &APIKeyService{groupRepo: selectionGroupRepo{groups: map[int64]*Group{1: {ID: 1, Status: StatusActive, RateMultiplier: 1}, 2: {ID: 2, Status: StatusActive, RateMultiplier: 2}}}, groupRouter: selectionRouter{states: map[int64]APIKeyGroupAvailability{1: {Available: true, Busy: true, Load: 1}, 2: {Available: true, Load: 0.2}}}}
	selected, err := svc.SelectRequestGroup(context.Background(), key, nil, "token", nil, nil, false)
	require.NoError(t, err)
	require.EqualValues(t, 2, *selected.GroupID)
	failure := errors.New("scheduler unavailable")
	svc.groupRouter = selectionRouter{err: failure}
	_, err = svc.SelectRequestGroup(context.Background(), key, nil, "token", nil, nil, false)
	require.ErrorIs(t, err, failure)
}

func TestAPIKeyBillingGroupsOnlyExposesPermittedBindings(t *testing.T) {
	primary := int64(1)
	key := &APIKey{UserID: 8, GroupID: &primary, GroupIDs: []int64{1, 2, 3, 4}, User: &User{ID: 8, RestrictPublicGroups: true, AllowedGroups: []int64{2, 3}}}
	svc := &APIKeyService{groupRepo: selectionGroupRepo{groups: map[int64]*Group{1: {ID: 1, Status: StatusActive}, 2: {ID: 2, Status: StatusActive}, 3: {ID: 3, Status: StatusDisabled}}}}
	groups, err := svc.BillingGroups(context.Background(), key)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.EqualValues(t, 2, *groups[0].GroupID)
	require.EqualValues(t, 3, *groups[1].GroupID)
	require.Equal(t, StatusDisabled, groups[1].Group.Status)
	require.EqualValues(t, 1, *key.GroupID)
}
