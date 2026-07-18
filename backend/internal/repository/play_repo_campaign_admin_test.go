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

	mock.ExpectQuery(`(?is)INSERT INTO play_campaigns \(name, start_at, end_at, rules_json, enabled\).*RETURNING id, name, start_at, end_at, rules_json::text, enabled, created_at`).
		WithArgs("开服福利周", start, end, `{"recharge_bonus_pct":10,"blindbox_extra_opens":2,"arena_score_multiplier":2,"name_i18n":{"en":"Launch week","zh":"开服福利周"}}`, true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "start_at", "end_at", "rules_json", "enabled", "created_at"}).
			AddRow(int64(7), "开服福利周", start, end, `{"recharge_bonus_pct":10,"blindbox_extra_opens":2,"arena_score_multiplier":2,"name_i18n":{"en":"Launch week","zh":"开服福利周"}}`, true, created))

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
