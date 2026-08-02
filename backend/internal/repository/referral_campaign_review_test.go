package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestReviewReferralCampaignAllowsCreatorAndReviewerReusingAnotherRole(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := &affiliateRepository{client: client}

	expectReview := func(reviewType string, approvals int) {
		campaignTime := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT snapshot,base_rules_version FROM referral_campaign_financial_versions`).
			WithArgs(int64(7), int64(3)).
			WillReturnRows(sqlmock.NewRows([]string{"snapshot", "base_rules_version"}))
		mock.ExpectQuery(`SELECT version FROM referral_campaigns`).
			WithArgs(int64(7), int64(3)).
			WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(int64(3)))
		mock.ExpectExec(`INSERT INTO referral_campaign_approvals`).
			WithArgs(int64(7), int64(3), reviewType, "approved", int64(42), "review passed").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(`SELECT COUNT\(DISTINCT review_type\) FROM referral_campaign_approvals`).
			WithArgs(int64(7), int64(3)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(approvals))
		mock.ExpectCommit()
		mock.ExpectQuery(`SELECT id, campaign_key, name, status, version, registration_from`).
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "campaign_key", "name", "status", "version", "registration_from", "registration_to", "starts_at", "ends_at", "qualification_to", "claim_deadline", "risk_hold_hours", "pay_threshold", "usage_threshold", "max_enrollments", "budget_total", "budget_reserved", "budget_paid", "reward_mode", "public_rules_md", "invitee_notice_md", "legacy_rebate_policy", "rules_version", "rules_updated_at", "rank_rewards_json", "created_by", "approved_by",
			}).AddRow(
				int64(7), "campaign-7", "Campaign 7", "review", int64(3), campaignTime, campaignTime.Add(24*time.Hour), campaignTime, campaignTime.Add(7*24*time.Hour), campaignTime.Add(8*24*time.Hour), campaignTime.Add(14*24*time.Hour), 168, 10.0, 1.0, 100, 1000.0, 0.0, 0.0, "additive", "rules", "invitee notice", "exclude", int64(1), campaignTime, []byte("{}"), int64(11), nil,
			))
	}

	expectReview("ops", 1)
	_, err = repo.ReviewReferralCampaign(context.Background(), 7, 3, "ops", "approved", 42, "review passed")
	require.NoError(t, err)
	expectReview("finance", 2)
	_, err = repo.ReviewReferralCampaign(context.Background(), 7, 3, "finance", "approved", 42, "review passed")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
