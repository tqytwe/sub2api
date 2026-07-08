package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnrichTeamAffiliateProgress(t *testing.T) {
	s := &PlayService{}
	rt := PlayRuntime{
		AgentTeamEnabled:            true,
		TeamAffiliateEnabled:        true,
		TeamAffiliateTokenThreshold: 1_000_000,
		TeamAffiliateCaptainBonus:   5,
	}

	info, err := s.enrichTeamAffiliate(t.Context(), 1, 10, 500_000, rt)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.False(t, info.MilestoneReached)
	require.Equal(t, int64(500_000), info.TokensToMilestone)

	info, err = s.enrichTeamAffiliate(t.Context(), 1, 10, 1_500_000, rt)
	require.NoError(t, err)
	require.True(t, info.MilestoneReached)
	require.Equal(t, int64(0), info.TokensToMilestone)
}
