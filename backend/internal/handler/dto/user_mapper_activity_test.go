package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserFromServiceAdmin_MapsActivityTimestamps(t *testing.T) {
	t.Parallel()

	lastLoginAt := time.Date(2026, time.April, 20, 10, 0, 0, 0, time.UTC)
	lastActiveAt := lastLoginAt.Add(15 * time.Minute)
	lastUsedAt := lastLoginAt.Add(45 * time.Minute)

	out := UserFromServiceAdmin(&service.User{
		ID:           42,
		Email:        "admin@example.com",
		Username:     "admin",
		Role:         service.RoleAdmin,
		Status:       service.StatusActive,
		LastActiveAt: &lastActiveAt,
		LastUsedAt:   &lastUsedAt,
	})

	require.NotNil(t, out)
	require.NotNil(t, out.LastActiveAt)
	require.NotNil(t, out.LastUsedAt)
	require.WithinDuration(t, lastActiveAt, *out.LastActiveAt, time.Second)
	require.WithinDuration(t, lastUsedAt, *out.LastUsedAt, time.Second)
}

func TestUserFromServiceAdmin_MapsMembershipProjection(t *testing.T) {
	t.Parallel()

	out := UserFromServiceAdmin(&service.User{
		ID:                   42,
		Email:                "member@example.com",
		MembershipPaidAmount: 618.25,
		VIPTier:              2,
		VIPLabel:             "V2",
		MembershipDataState:  "verified",
	})

	require.NotNil(t, out)
	require.InDelta(t, 618.25, out.MembershipPaidAmount, 1e-9)
	require.Equal(t, 2, out.VIPTier)
	require.Equal(t, "V2", out.VIPLabel)
	require.Equal(t, "verified", out.MembershipDataState)
}

func TestUserFromServiceAdmin_MapsVerifiedV0MembershipProjection(t *testing.T) {
	t.Parallel()

	out := UserFromServiceAdmin(&service.User{
		ID:                  43,
		Email:               "ordinary@example.com",
		VIPLabel:            "V0",
		MembershipDataState: "verified",
	})

	require.NotNil(t, out)
	require.Zero(t, out.MembershipPaidAmount)
	require.Zero(t, out.VIPTier)
	require.Equal(t, "V0", out.VIPLabel)
	require.Equal(t, "verified", out.MembershipDataState)
}
