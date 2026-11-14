package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyPolicyContractRejectsLegacyV25AuthSnapshot(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{ID: 10, UserID: 20, Key: "synthetic-policy-fixture", Status: StatusActive, User: &User{ID: 20, Status: StatusActive}}
	snapshot := svc.snapshotFromAPIKey(t.Context(), key)
	snapshot.Version = 25
	got, hit, err := svc.applyAuthCacheEntry(key.Key, &APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	require.False(t, hit, "v25 omits administrator model and context restrictions and must be reloaded")
	require.Nil(t, got)
}
