package service

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// MembershipPaidTotal returns the authoritative net paid amount. The legacy
// users.total_recharged field remains available for balance and first-charge
// behavior, but is deliberately not used for VIP qualification.
func (s *PlayService) MembershipPaidTotal(ctx context.Context, userID int64) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if repo, ok := s.repo.(interface {
		GetMembershipPaidTotal(context.Context, int64) (float64, error)
	}); ok {
		return repo.GetMembershipPaidTotal(ctx, userID)
	}
	return 0, nil
}

func (s *PlayService) SyncMembershipOrder(ctx context.Context, orderID, userID int64, orderType string, paidAmount, refundAmount float64, paidAt *time.Time, status string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if repo, ok := s.repo.(PlayMembershipRepository); ok {
		before, _ := repo.GetMembershipPaidTotal(ctx, userID)
		if err := repo.SyncMembershipOrderContribution(ctx, orderID, userID, orderType, paidAmount, refundAmount, paidAt, status); err != nil {
			return err
		}
		after, err := repo.GetMembershipPaidTotal(ctx, userID)
		if err != nil {
			return err
		}
		if adminRepo, ok := s.repo.(PlayMembershipAdminRepository); ok {
			tiers := s.GetRuntime(ctx).VIPTiers
			fromTier := resolveVIPStatus(before, tiers).Tier
			toTier := resolveVIPStatus(after, tiers).Tier
			if fromTier != toTier {
				reason := "payment"
				if after < before || refundAmount > 0 {
					reason = "refund"
				}
				orderIDCopy := orderID
				if err := adminRepo.RecordMembershipTierChange(ctx, PlayMembershipTierChange{UserID: userID, OrderID: &orderIDCopy, FromTier: fromTier, ToTier: toTier, NetPaidBefore: decimal.NewFromFloat(before), NetPaidAfter: decimal.NewFromFloat(after), Reason: reason}); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return nil
}

func (s *PlayService) TeamLeaderboard(ctx context.Context, userID int64, limit int) (*PlayTeamLeaderboard, error) {
	now := s.serverNow()
	out := &PlayTeamLeaderboard{Month: now.Format("2006-01")}
	repo, ok := s.repo.(PlayMembershipRepository)
	if !ok {
		return out, nil
	}
	loc := now.Location()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)
	rows, err := repo.ListTeamLeaderboardBase(ctx, start, end, limit)
	if err != nil {
		return nil, err
	}
	// Keep the pool estimate aligned with the settlement tier resolver.
	rt := s.GetRuntime(ctx)
	cfg := TeamRewardConfig{Enabled: rt.TeamSharedRewardEnabled, Cap: rt.TeamSharedRewardCap, Tiers: rt.TeamSharedRewardTiers}
	for i, row := range rows {
		entry := PlayTeamLeaderboardEntry{Rank: i + 1, TeamID: row.TeamID, TeamName: row.TeamName, MemberCount: row.MemberCount, Spend: row.Spend, EstimatedPool: resolveTeamRewardPool(row.Spend, cfg)}
		if team, err := s.repo.GetUserTeam(ctx, userID); err == nil && team != nil {
			entry.IsMine = team.ID == row.TeamID
		}
		out.Rows = append(out.Rows, entry)
	}
	if team, err := s.repo.GetUserTeam(ctx, userID); err == nil && team != nil {
		found := false
		for _, row := range out.Rows {
			if row.TeamID == team.ID {
				found = true
				break
			}
		}
		if !found {
			if spend, err := s.repo.GetAdminTeamSpend(ctx, team.ID, start, end); err == nil {
				members, _ := s.repo.CountActiveTeamMembers(ctx, team.ID)
				rank, total, previousSpend, rankErr := repo.GetTeamLeaderboardRank(ctx, team.ID, start, end)
				if rankErr == nil {
					out.TotalTeams = total
				}
				gap := decimal.Zero
				if previousSpend.GreaterThan(spend) {
					gap = previousSpend.Sub(spend)
				}
				out.Rows = append(out.Rows, PlayTeamLeaderboardEntry{TeamID: team.ID, TeamName: team.Name, MemberCount: members, Spend: spend, EstimatedPool: resolveTeamRewardPool(spend, cfg), GapToPrevious: gap, IsMine: true, Rank: rank})
			}
		}
	}
	if out.TotalTeams == 0 {
		if _, active, err := s.repo.CountAdminTeams(ctx); err == nil {
			out.TotalTeams = active
		}
	}
	for i := range out.Rows {
		if i > 0 && out.Rows[i-1].Rank+1 == out.Rows[i].Rank {
			out.Rows[i].GapToPrevious = out.Rows[i-1].Spend.Sub(out.Rows[i].Spend)
		}
	}
	return out, nil
}

type PlayTeamLeaderboardEntry struct {
	Rank          int             `json:"rank"`
	TeamID        int64           `json:"team_id"`
	TeamName      string          `json:"team_name"`
	MemberCount   int             `json:"member_count"`
	Spend         decimal.Decimal `json:"monthly_spend"`
	EstimatedPool decimal.Decimal `json:"estimated_pool"`
	GapToPrevious decimal.Decimal `json:"gap_to_previous"`
	IsMine        bool            `json:"is_mine"`
}
type PlayTeamLeaderboard struct {
	Rows       []PlayTeamLeaderboardEntry `json:"rows"`
	Month      string                     `json:"month"`
	TotalTeams int                        `json:"total_teams"`
}
