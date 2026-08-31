package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetUserSubscriptionProgressRequiresSubscriptionOwner(t *testing.T) {
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(&UserSubscription{
		ID:        42,
		UserID:    1001,
		GroupID:   9,
		Group:     &Group{ID: 9, Name: "Pro"},
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)

	progress, err := svc.GetUserSubscriptionProgress(context.Background(), 1001, 42)
	require.NoError(t, err)
	require.Equal(t, int64(42), progress.ID)
	require.Equal(t, "Pro", progress.GroupName)

	_, err = svc.GetUserSubscriptionProgress(context.Background(), 2002, 42)
	require.ErrorIs(t, err, ErrSubscriptionNotFound)
}
