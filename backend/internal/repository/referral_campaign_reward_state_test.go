package repository

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNextReferralRewardGenerationReunlocksOnlyAfterRevocation(t *testing.T) {
	generation, create, err := nextReferralRewardGeneration(service.ReferralRewardStatusRevoked, 3, nil)
	require.NoError(t, err)
	require.True(t, create)
	require.Equal(t, 4, generation)

	generation, create, err = nextReferralRewardGeneration(service.ReferralRewardStatusClaimable, 3, nil)
	require.NoError(t, err)
	require.False(t, create)
	require.Equal(t, 3, generation)

	generation, create, err = nextReferralRewardGeneration("", 0, sql.ErrNoRows)
	require.NoError(t, err)
	require.True(t, create)
	require.Equal(t, 1, generation)
}

func TestReferralRewardReservationReportsBudgetExhaustion(t *testing.T) {
	require.ErrorIs(t, translateReferralRewardReservationError(sql.ErrNoRows), service.ErrReferralCampaignBudgetExceeded)
	sentinel := errors.New("database unavailable")
	require.ErrorIs(t, translateReferralRewardReservationError(sentinel), sentinel)
}
