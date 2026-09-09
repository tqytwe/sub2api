//go:build unit

package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// This contract test guards the transaction's lock ordering and financial
// invariants without requiring a PostgreSQL container. PostgreSQL performs the
// actual row locking and statements in the repository transaction.
func TestEarlyCloseReferralCampaignSQLPreservesProcessedRewardsAndAuditsRelease(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	start := strings.Index(string(source), "func (r *affiliateRepository) EarlyCloseReferralCampaign")
	require.NotEqual(t, -1, start)
	query := string(source[start:])

	require.Contains(t, query, "SELECT version,status,claim_deadline FROM referral_campaigns WHERE id=$1 FOR UPDATE")
	require.Contains(t, query, "status='claimable' FOR UPDATE")
	require.Contains(t, query, "SET status='expired'")
	require.Contains(t, query, "budget_reserved=GREATEST(budget_reserved-$1,0)")
	require.Contains(t, query, "referral:reward:%d:early-close-expire")
	require.Contains(t, query, "'early_closed'")
	require.NotContains(t, query[strings.Index(query, "func (r *affiliateRepository) EarlyCloseReferralCampaign"):strings.Index(query, "func (r *affiliateRepository) ReviewReferralCampaign")], "WITH changed AS")
	for _, preserved := range []string{"claimed_frozen", "available", "debt_review", "resolved"} {
		require.NotContains(t, query[strings.Index(query, "func (r *affiliateRepository) EarlyCloseReferralCampaign"):strings.Index(query, "func (r *affiliateRepository) ReviewReferralCampaign")], "r.status='"+preserved+"'")
	}
}

func TestReferralCampaignClaimLocksTheCampaignBeforeRewardAndJoinsItsState(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	start := strings.Index(string(source), "func (r *affiliateRepository) ClaimReferralReward")
	end := strings.Index(string(source), "func (r *affiliateRepository) ResolveReferralRewardDebt")
	require.NotEqual(t, -1, start)
	require.NotEqual(t, -1, end)
	query := string(source)[start:end]

	require.Contains(t, query, "SELECT id FROM referral_campaigns WHERE id=$1 FOR UPDATE")
	require.Contains(t, query, "c.id=r.campaign_id")
}

func TestReferralQualificationAndNewUserGrowthDoNotCreateRewardsInClaimWindow(t *testing.T) {
	affiliateSource, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	affiliate := string(affiliateSource)
	updateStart := strings.Index(affiliate, "func (r *affiliateRepository) UpdateReferralQualification")
	updateEnd := strings.Index(affiliate, "func (r *affiliateRepository) ClaimReferralReward")
	require.NotEqual(t, -1, updateStart)
	require.NotEqual(t, -1, updateEnd)
	update := affiliate[updateStart:updateEnd]
	require.Contains(t, update, "SELECT reward_mode,status FROM referral_campaigns WHERE id=$1 FOR UPDATE")
	require.Contains(t, update, "campaignStatus != service.ReferralCampaignStatusScheduled && campaignStatus != service.ReferralCampaignStatusRunning")

	playSource, err := os.ReadFile("play_repo_campaign.go")
	require.NoError(t, err)
	play := string(playSource)
	ensureStart := strings.Index(play, "func (r *playRepository) ensureNewUserGrowthReward")
	ensureEnd := strings.Index(play, "func (r *playRepository) revokeUnqualifiedNewUserGrowthRewards")
	require.NotEqual(t, -1, ensureStart)
	require.NotEqual(t, -1, ensureEnd)
	ensure := play[ensureStart:ensureEnd]
	require.Contains(t, ensure, "SELECT id,status FROM referral_campaigns WHERE id=$1 FOR UPDATE")
	require.Contains(t, ensure, "referralStatus != service.ReferralCampaignStatusScheduled && referralStatus != service.ReferralCampaignStatusRunning")
}
