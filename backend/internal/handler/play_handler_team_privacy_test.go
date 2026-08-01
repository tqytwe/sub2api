package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPlayTeamSummaryDTOForActorRedactsSensitiveMemberData(t *testing.T) {
	team := &service.PlayTeamSummary{
		ID:         11,
		Name:       "Private Team",
		InviteCode: "private-invite-code",
		CaptainID:  7,
		TokenSum:   12345,
		Members: []service.PlayTeamMember{{
			UserID:          7,
			DisplayName:     "Captain",
			AvatarURL:       "https://example.test/avatar.png",
			JoinedAt:        time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
			TokenSum:        9000,
			TokenPct:        72,
			Spend:           decimal.RequireFromString("12.50000000"),
			SpendPct:        71,
			EstimatedReward: decimal.RequireFromString("0.75000000"),
		}},
	}

	captain := toPlayTeamSummaryDTOForActor(team, 7)
	require.True(t, captain.IsCaptain)
	require.True(t, captain.CanManage)
	require.Equal(t, "private-invite-code", captain.InviteCode)

	member := toPlayTeamSummaryDTOForActor(team, 8)
	require.False(t, member.IsCaptain)
	require.False(t, member.CanManage)
	require.Empty(t, member.InviteCode)

	payload, err := json.Marshal(member)
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(payload, &document))
	require.NotContains(t, document, "captain_id")
	require.NotContains(t, document, "invite_code")
	members, ok := document["members"].([]any)
	require.True(t, ok)
	require.Len(t, members, 1)
	memberDocument, ok := members[0].(map[string]any)
	require.True(t, ok)
	for _, field := range []string{
		"user_id",
		"joined_at",
		"token_sum",
		"token_pct",
		"spend",
		"spend_pct",
		"estimated_reward",
	} {
		require.NotContains(t, memberDocument, field)
	}
}

func TestPlayHubSummaryDTOForActorUsesTheSameTeamPrivacyContract(t *testing.T) {
	team := &service.PlayTeamSummary{
		ID:         11,
		Name:       "Private Team",
		InviteCode: "private-invite-code",
		CaptainID:  7,
		Members:    []service.PlayTeamMember{{UserID: 7, DisplayName: "Captain"}},
	}
	hub := &service.PlayHubSummary{Team: &service.PlayTeamMe{Enabled: true, Team: team}}

	captain := toPlayHubSummaryDTOForActor(hub, 7)
	require.NotNil(t, captain.Team)
	require.NotNil(t, captain.Team.Team)
	require.True(t, captain.Team.Team.IsCaptain)
	require.Equal(t, "private-invite-code", captain.Team.Team.InviteCode)

	member := toPlayHubSummaryDTOForActor(hub, 8)
	require.NotNil(t, member.Team)
	require.NotNil(t, member.Team.Team)
	require.False(t, member.Team.Team.IsCaptain)
	require.Empty(t, member.Team.Team.InviteCode)
}
