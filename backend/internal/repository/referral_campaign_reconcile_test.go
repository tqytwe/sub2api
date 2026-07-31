package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestProcessReferralReconcileQueueRetriesFailedRefund(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := &affiliateRepository{client: client}

	queueRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "source_order_id"}).AddRow(int64(9), int64(77))
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT id,source_order_id FROM referral_campaign_reconcile_queue.*status IN \('pending','failed','processing'\).*available_at<=NOW\(\)`).
		WillReturnRows(queueRows())
	mock.ExpectExec(`(?s)UPDATE referral_campaign_reconcile_queue SET status='processing'.*available_at=NOW\(\)\+INTERVAL '5 minutes'.*status IN \('pending','failed','processing'\)`).
		WithArgs(int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT DISTINCT a.campaign_id,a.inviter_id,a.invitee_id`).
		WithArgs(int64(77)).
		WillReturnError(errors.New("temporary refund reconciliation failure"))
	mock.ExpectRollback()
	mock.ExpectExec(`(?s)UPDATE referral_campaign_reconcile_queue SET status='failed'`).
		WithArgs(int64(77), "temporary refund reconciliation failure").
		WillReturnResult(sqlmock.NewResult(0, 1))

	processed, err := repo.ProcessReferralReconcileQueue(context.Background(), 1)
	require.Equal(t, 0, processed)
	require.ErrorContains(t, err, "temporary refund reconciliation failure")

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT id,source_order_id FROM referral_campaign_reconcile_queue.*status IN \('pending','failed','processing'\).*available_at<=NOW\(\)`).
		WillReturnRows(queueRows())
	mock.ExpectExec(`(?s)UPDATE referral_campaign_reconcile_queue SET status='processing'.*available_at=NOW\(\)\+INTERVAL '5 minutes'.*status IN \('pending','failed','processing'\)`).
		WithArgs(int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT DISTINCT a.campaign_id,a.inviter_id,a.invitee_id`).
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows([]string{"campaign_id", "inviter_id", "invitee_id"}))
	mock.ExpectCommit()
	mock.ExpectExec(`(?s)UPDATE referral_campaign_reconcile_queue SET status='done'`).
		WithArgs(int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	processed, err = repo.ProcessReferralReconcileQueue(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.NoError(t, mock.ExpectationsWereMet())
}
