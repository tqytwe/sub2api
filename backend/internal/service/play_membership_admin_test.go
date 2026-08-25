//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type membershipAdminMissingUserRepo struct {
	PlayRepository
}

func (membershipAdminMissingUserRepo) MembershipAdminOverview(context.Context, float64) (int, decimal.Decimal, error) {
	return 0, decimal.Zero, nil
}

func (membershipAdminMissingUserRepo) ListMembershipAdminRows(context.Context, string, *bool, float64, int, int) ([]PlayMembershipAdminRow, int, error) {
	return nil, 0, nil
}

func (membershipAdminMissingUserRepo) ListMembershipPaidTotals(context.Context) (map[int64]decimal.Decimal, error) {
	return nil, nil
}

func (membershipAdminMissingUserRepo) GetMembershipAdminRow(context.Context, int64) (*PlayMembershipAdminRow, error) {
	return nil, nil
}

func (membershipAdminMissingUserRepo) ListMembershipContributions(context.Context, int64, int) ([]PlayMembershipContribution, error) {
	return nil, nil
}

func (membershipAdminMissingUserRepo) ListMembershipTierHistory(context.Context, int64, int) ([]PlayMembershipTierChange, error) {
	return nil, nil
}

func (membershipAdminMissingUserRepo) CountRecentMembershipTierChanges(context.Context, time.Time) (int, int, error) {
	return 0, 0, nil
}

func (membershipAdminMissingUserRepo) RecordMembershipTierChange(context.Context, PlayMembershipTierChange) error {
	return nil
}

func (membershipAdminMissingUserRepo) GetVIPConfigVersion(context.Context) (int64, error) {
	return 0, nil
}

func (membershipAdminMissingUserRepo) PublishVIPConfig(context.Context, int64, int64, string, string, int, int, int, []PlayMembershipTierChange) (int64, error) {
	return 0, nil
}

func TestGetMembershipAdminUserReturnsNotFoundForMissingUser(t *testing.T) {
	svc := &PlayService{repo: membershipAdminMissingUserRepo{}}

	detail, err := svc.GetMembershipAdminUser(context.Background(), 264)

	require.Nil(t, detail)
	require.Error(t, err)
	require.True(t, infraerrors.IsNotFound(err))
	require.Equal(t, "PLAY_MEMBERSHIP_USER_NOT_FOUND", infraerrors.Reason(err))
}
