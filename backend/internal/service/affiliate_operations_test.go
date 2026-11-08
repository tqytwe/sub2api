package service

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAffiliateOperationsMonthUsesShanghaiCalendar(t *testing.T) {
	now := time.Date(2026, 1, 31, 16, 0, 0, 0, time.UTC)
	q, err := NormalizeAffiliateOperationsQuery(AffiliateOperationsQuery{}, now)
	require.NoError(t, err)
	require.Equal(t, "2026-02", q.Month)
	require.Equal(t, 1, q.Page)
	require.Equal(t, 20, q.PageSize)
	_, err = NormalizeAffiliateOperationsQuery(AffiliateOperationsQuery{Month: "2026-13"}, now)
	require.Error(t, err)
	_, err = NormalizeAffiliateOperationsQuery(AffiliateOperationsQuery{Month: "2026-2"}, now)
	require.Error(t, err)
}

func TestAffiliateOperationsOwnershipIsAuthenticatedSubject(t *testing.T) {
	q, err := AffiliateOperationsQueryFromValues(url.Values{"user_id": {"999"}, "invitee_id": {"123"}, "source_order_id": {"456"}, "month": {"2026-01"}}, 42, false)
	require.NoError(t, err)
	require.EqualValues(t, 42, q.UserID)
	require.Zero(t, q.InviteeID)
	require.Zero(t, q.SourceOrderID)
	require.False(t, q.Admin)
	q, err = AffiliateOperationsQueryFromValues(url.Values{"user_id": {"999"}, "invitee_id": {"123"}}, 0, true)
	require.NoError(t, err)
	require.EqualValues(t, 999, q.UserID)
	require.EqualValues(t, 123, q.InviteeID)
}

func TestAffiliateOperationsDateRangeCoversWholeShanghaiDay(t *testing.T) {
	q, err := NormalizeAffiliateOperationsQuery(AffiliateOperationsQuery{StartDate: "2026-01-31", EndDate: "2026-01-31"}, time.Now())
	require.NoError(t, err)
	require.Equal(t, "2026-01-30T16:00:00Z", q.StartAt.UTC().Format(time.RFC3339))
	require.Equal(t, "2026-01-31T16:00:00Z", q.EndAt.UTC().Format(time.RFC3339))
	_, err = NormalizeAffiliateOperationsQuery(AffiliateOperationsQuery{StartDate: "2026-02-01", EndDate: "2026-01-31"}, time.Now())
	require.Error(t, err)
}

type affiliateCampaignFilterStub struct {
	ReferralCampaignRepository
	called bool
	risk   string
}

func (s *affiliateCampaignFilterStub) ListReferralCampaignInvites(_ context.Context, _ int64, _, _ int, _, _ string) (*ReferralCampaignInvitePage, error) {
	s.called = true
	return &ReferralCampaignInvitePage{}, nil
}
func (s *affiliateCampaignFilterStub) ListReferralCampaignInvitesFiltered(_ context.Context, _ int64, _, _ int, _, _, risk string) (*ReferralCampaignInvitePage, error) {
	s.called = true
	s.risk = risk
	return &ReferralCampaignInvitePage{}, nil
}

func TestAffiliateOperationsActivityFiltersAreIndependent(t *testing.T) {
	r := &affiliateCampaignFilterStub{}
	s := &ReferralCampaignService{repo: r}
	_, err := s.Invites(context.Background(), 1, 1, 20, "", "qualified", "rejected")
	require.NoError(t, err)
	require.Equal(t, "rejected", r.risk)
	r.called = false
	_, err = s.Invites(context.Background(), 1, 1, 20, "", "approved", "pending")
	require.Error(t, err)
	require.False(t, r.called)
}

func TestAffiliateOperationsRejectsAmbiguousFilters(t *testing.T) {
	for _, q := range []AffiliateOperationsQuery{
		{Status: "not-real"}, {Action: "delete"}, {UserID: -1}, {InviteeID: -2}, {SourceOrderID: -3},
	} {
		_, err := NormalizeAffiliateOperationsQuery(q, time.Now())
		require.Error(t, err)
	}
}

func TestAffiliateOperationsMaskDoesNotRevealEmailDomain(t *testing.T) {
	for _, input := range []string{"alice@private-company.example", "张三@company.cn", "x", ""} {
		masked := MaskAffiliateOperationsName(input)
		require.NotContains(t, masked, "@")
		require.NotContains(t, masked, "company")
		require.NotEmpty(t, masked)
	}
}
