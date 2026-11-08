package repository

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestAffiliateExactFiltersAreBoundAndSharedByReports(t *testing.T) {
	filter := service.AffiliateRecordFilter{InviterID: 42, InviteeID: 7, SourceOrderID: 81, CampaignID: 3, SourceType: "campaign", Status: "frozen"}
	where, args := affiliateExactRecordWhere(filter, "rebates", "", nil)
	require.Contains(t, where, "ual.user_id = $1")
	require.Contains(t, where, "ual.source_user_id = $2")
	require.Contains(t, where, "ual.source_order_id = $3")
	require.Contains(t, where, "rr.campaign_id = $4")
	require.Contains(t, where, "ual.frozen_until > NOW()")
	require.Equal(t, []any{int64(42), int64(7), int64(81), int64(3)}, args)
	where, args = affiliateExactRecordWhere(service.AffiliateRecordFilter{UserID: 5, Action: "withdraw' OR true --"}, "transfers", "", nil)
	require.Contains(t, where, "ual.action = $2")
	require.NotContains(t, where, "OR true")
	require.Equal(t, "withdraw' OR true --", args[1])
	require.False(t, strings.Contains(where, "LIMIT"))
	where, args = affiliateExactRecordWhere(service.AffiliateRecordFilter{Currency: "CNY", SourceType: "other"}, "rebates", "", nil)
	require.Contains(t, where, "COALESCE(rr.currency,'USD') = $1")
	require.Contains(t, where, "ual.referral_reward_id IS NULL AND ual.source_order_id IS NULL")
	require.Equal(t, []any{"CNY"}, args)
}
