package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestReviewReferralCampaignRejectsReviewerReusingAnotherRole(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := &affiliateRepository{client: client}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT COALESCE\(created_by,0\),version FROM referral_campaigns`).
		WithArgs(int64(7), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"created_by", "version"}).AddRow(int64(11), int64(3)))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM referral_campaign_approvals`).
		WithArgs(int64(7), int64(3), int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err = repo.ReviewReferralCampaign(context.Background(), 7, 3, "risk", "approved", 42, "risk review passed")
	require.ErrorIs(t, err, service.ErrReferralCampaignReviewerConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
