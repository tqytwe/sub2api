//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyMultiGroupAuthDoesNotRequirePrimaryGrant(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(map[bool]string{false: "repository", true: "cache"}[cached], func(t *testing.T) {
			primary := int64(9)
			key := &APIKey{ID: 1, UserID: 2, GroupID: &primary, GroupIDs: []int64{9, 10}, Status: StatusActive,
				User:  &User{ID: 2, Status: StatusActive, RestrictPublicGroups: true, AllowedGroups: []int64{10}},
				Group: &Group{ID: 9, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}}
			repo := &authRepoStub{getByKeyForAuth: func(context.Context, string) (*APIKey, error) { return key, nil }}
			cache := &authCacheStub{}
			svc := NewAPIKeyService(repo, nil, nil, nil, nil, cache, &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60}})
			if cached {
				entry := &APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)}
				cache.getAuthCache = func(context.Context, string) (*APIKeyAuthCacheEntry, error) { return entry, nil }
			}
			got, err := svc.GetByKey(context.Background(), "fixture-key")
			require.NoError(t, err)
			require.Equal(t, []int64{9, 10}, got.ConfiguredGroupIDs())
		})
	}
}
