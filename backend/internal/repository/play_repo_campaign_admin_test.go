package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreateAdminCampaignStoresStructuredRules(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &playRepository{sql: db}
	start := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	created := start.Add(-time.Hour)

	mock.ExpectQuery(`(?is)INSERT INTO play_campaigns \(name, start_at, end_at, rules_json, audience_json, enabled\).*RETURNING id, name, start_at, end_at, rules_json::text, audience_json::text, enabled, created_at`).
		WithArgs("开服福利周", start, end, `{"recharge_bonus_pct":10,"blindbox_extra_opens":2,"arena_score_multiplier":2,"name_i18n":{"en":"Launch week","zh":"开服福利周"}}`, `{}`, true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "start_at", "end_at", "rules_json", "audience_json", "enabled", "created_at"}).
			AddRow(int64(7), "开服福利周", start, end, `{"recharge_bonus_pct":10,"blindbox_extra_opens":2,"arena_score_multiplier":2,"name_i18n":{"en":"Launch week","zh":"开服福利周"}}`, `{}`, true, created))

	got, err := repo.CreateAdminCampaign(context.Background(), service.PlayCampaign{
		Name:    "开服福利周",
		StartAt: start,
		EndAt:   end,
		Enabled: true,
		Rules: service.PlayCampaignRules{
			RechargeBonusPct:     10,
			BlindboxExtraOpens:   2,
			ArenaScoreMultiplier: 2,
			NameI18n:             map[string]string{"en": "Launch week", "zh": "开服福利周"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), got.ID)
	require.Equal(t, 10.0, got.Rules.RechargeBonusPct)
	require.Equal(t, 2, got.Rules.BlindboxExtraOpens)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteAdminCampaignReturnsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &playRepository{sql: db}
	mock.ExpectExec(`DELETE FROM play_campaigns WHERE id = \$1`).
		WithArgs(int64(404)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.DeleteAdminCampaign(context.Background(), 404)

	require.Error(t, err)
	require.True(t, infraerrors.IsNotFound(err))
	require.Equal(t, "PLAY_CAMPAIGN_NOT_FOUND", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListNewUserGrowthCampaignsForUserKeepsRewardedEndedCampaigns(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	mock.ExpectQuery(`(?s)FROM play_campaigns p.*rules_json->>'campaign_type' IN \('new_user_growth','hybrid'\).*play_campaign_reward_snapshots`).
		WithArgs(int64(42), end.Add(time.Hour)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "start_at", "end_at", "rules_json", "audience_json", "enabled", "created_at"}).
			AddRow(int64(9), "新用户成长", start, end, `{"campaign_type":"new_user_growth","referral_campaign_id":7,"qualification_metric":"net_recharge","legacy_rebate_policy":"exclude","reward_tiers":[{"tier":1,"required_amount":50,"reward_amount":50,"currency":"CNY"}]}`, `{}`, true, start))

	repo := &playRepository{sql: db}
	items, err := repo.ListNewUserGrowthCampaignsForUser(context.Background(), 42, end.Add(time.Hour))

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(7), items[0].Rules.ReferralCampaignID)
	require.Equal(t, service.PlayCampaignLegacyRebateExclude, items[0].Rules.LegacyRebatePolicy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewUserGrowthMetricStartsAtCampaignStart(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	now := start.Add(48 * time.Hour)
	mock.ExpectQuery(`(?s)SUM\(m.net_amount\).*GREATEST\(a.registered_at,\$3\).*m.paid_at<LEAST\(\$4,a.qualification_to_snapshot\)`).
		WithArgs(int64(7), int64(42), start, now).
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(120.0))

	repo := &playRepository{sql: db}
	amount, err := repo.newUserGrowthMetric(context.Background(), service.PlayCampaign{
		StartAt: start,
		EndAt:   now.Add(24 * time.Hour),
		Rules:   service.PlayCampaignRules{ReferralCampaignID: 7, QualificationMetric: service.PlayCampaignMetricNetRecharge},
	}, 42, now)

	require.NoError(t, err)
	require.Equal(t, 120.0, amount)
	require.NoError(t, mock.ExpectationsWereMet())
}
