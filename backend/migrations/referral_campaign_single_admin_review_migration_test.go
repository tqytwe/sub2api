package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReferralCampaignSingleAdminReviewMigrationDropsReviewerUniqueness(t *testing.T) {
	raw, err := FS.ReadFile("241_referral_campaign_single_admin_review.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))
	require.Contains(t, sql, "alter table referral_campaign_approvals")
	require.Contains(t, sql, "drop constraint if exists referral_campaign_approvals_campaign_id_version_reviewer_id_key")
	require.NotContains(t, sql, "drop constraint if exists referral_campaign_approvals_campaign_id_version_review_type_key")
}
