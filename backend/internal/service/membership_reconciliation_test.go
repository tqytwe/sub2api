package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestMembershipReconciliationPreviewDoesNotWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`SELECT c\.order_id, p\.status`).
		WithArgs(int64(0), 10).
		WillReturnRows(sqlmock.NewRows([]string{"order_id", "status", "order_type", "payment_currency", "list_amount", "gateway_base_amount", "qualifying_recharge_amount", "amount", "refund_amount", "subscription_snapshot"}).
			AddRow(int64(11), "COMPLETED", "balance", "CNY", 100.0, 90.0, 90.0, 100.0, 0.0, nil).
			AddRow(int64(12), "REFUND_REQUESTED", "balance", "CNY", 100.0, 90.0, 90.0, 100.0, 10.0, nil).
			AddRow(int64(13), "COMPLETED", "subscription", "CNY", 100.0, 90.0, 90.0, 100.0, 0.0, []byte(`{"plan_id":1}`)))
	report, err := NewMembershipReconciliationService(db).Reconcile(context.Background(), MembershipReconciliationOptions{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 3, report.Scanned)
	require.Equal(t, 2, report.Verified)
	require.Equal(t, 1, report.PendingReview)
	require.Equal(t, "refund_or_payment_not_final", report.Items[1].Reason)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMembershipReviewReasonRequiresSubscriptionSnapshot(t *testing.T) {
	require.Equal(t, "missing_subscription_snapshot", membershipReviewReason(membershipReviewRow{
		status: "COMPLETED", orderType: "subscription",
		currency:           sql.NullString{String: "CNY", Valid: true},
		listAmount:         sql.NullFloat64{Float64: 100, Valid: true},
		gatewayBaseAmount:  sql.NullFloat64{Float64: 90, Valid: true},
		qualifyingRecharge: sql.NullFloat64{Float64: 90, Valid: true},
	}))
}
