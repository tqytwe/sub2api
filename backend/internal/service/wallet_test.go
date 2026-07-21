package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestWalletSummaryReadsUserBalanceAndUnifiedLedgerTotals(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	createdAt := time.Date(2026, 7, 21, 9, 30, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)SELECT.*FROM users.*LEFT JOIN.*balance_transactions`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"available_balance",
			"withdrawable_balance",
			"pending_withdrawable_balance",
			"withdrawal_frozen_balance",
			"task_reserved_balance",
			"total_credits",
			"total_debits",
			"transaction_count",
			"last_transaction_at",
		}).AddRow("42.50000000", "12.00000000", "4.50000000", "1.25000000", "3.25000000", "100.00000000", "57.50000000", int64(5), createdAt))
	mock.ExpectQuery(`(?s)SELECT.*COALESCE\(u.balance.*FROM users u.*LEFT JOIN balance_fund_batches`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"available_balance",
			"online_recharge_balance",
			"offline_recharge_balance",
			"signup_gift_balance",
			"ops_gift_balance",
			"gift_balance",
			"refund_frozen_balance",
			"classified_fund_balance",
		}).AddRow("42.50000000", "20.00000000", "5.00000000", "10.00000000", "2.50000000", "12.50000000", "1.00000000", "37.50000000"))

	svc := NewWalletService(db)
	summary, err := svc.GetSummary(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, "42.50000000", summary.AvailableBalance.StringFixed(8))
	require.Equal(t, "12.00000000", summary.WithdrawableBalance.StringFixed(8))
	require.Equal(t, "4.50000000", summary.PendingWithdrawableBalance.StringFixed(8))
	require.Equal(t, "25.00000000", summary.RefundableRechargeBalance.StringFixed(8))
	require.Equal(t, "20.00000000", summary.OnlineRechargeBalance.StringFixed(8))
	require.Equal(t, "5.00000000", summary.OfflineRechargeBalance.StringFixed(8))
	require.Equal(t, "12.50000000", summary.GiftBalance.StringFixed(8))
	require.Equal(t, "10.00000000", summary.SignupGiftBalance.StringFixed(8))
	require.Equal(t, "2.50000000", summary.OpsGiftBalance.StringFixed(8))
	require.Equal(t, "1.00000000", summary.RefundFrozenBalance.StringFixed(8))
	require.Equal(t, "5.00000000", summary.UnclassifiedBalance.StringFixed(8))
	require.Equal(t, "1.25000000", summary.WithdrawalFrozenBalance.StringFixed(8))
	require.Equal(t, "3.25000000", summary.TaskReservedBalance.StringFixed(8))
	require.Equal(t, "100.00000000", summary.TotalCredits.StringFixed(8))
	require.Equal(t, "57.50000000", summary.TotalDebits.StringFixed(8))
	require.Equal(t, int64(5), summary.TransactionCount)
	require.Equal(t, createdAt, *summary.LastTransactionAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletTransactionsReturnSafePublicDTOAndSourceFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	createdAt := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM balance_transactions`).
		WithArgs(int64(7), "team_shared_reward").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)SELECT.*id.*source_type.*balance_delta.*frozen_delta.*withdrawable_delta.*withdrawal_frozen_delta.*balance_after.*frozen_after.*withdrawable_after.*withdrawal_frozen_after.*created_at.*FROM balance_transactions`).
		WithArgs(int64(7), "team_shared_reward", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"source_type",
			"fund_source_kind",
			"balance_delta",
			"frozen_delta",
			"withdrawable_delta",
			"withdrawal_frozen_delta",
			"balance_after",
			"frozen_after",
			"withdrawable_after",
			"withdrawal_frozen_after",
			"created_at",
		}).AddRow(int64(88), "team_shared_reward", "", "12.34000000", "0.00000000", "12.34000000", "0.00000000", "58.34000000", "0.00000000", "58.34000000", "0.00000000", createdAt))

	svc := NewWalletService(db)
	page, err := svc.ListTransactions(context.Background(), 7, WalletTransactionQuery{
		Source:   "team_reward",
		Page:     1,
		PageSize: 20,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, page.Items, 1)
	item := page.Items[0]
	require.Equal(t, int64(88), item.ID)
	require.Equal(t, WalletPublicSourceTeamReward, item.Source)
	require.Equal(t, WalletDirectionCredit, item.Direction)
	require.Equal(t, "12.34000000", item.BalanceDelta.StringFixed(8))
	require.Equal(t, "12.34000000", item.WithdrawableDelta.StringFixed(8))
	require.Equal(t, "58.34000000", item.BalanceAfter.StringFixed(8))

	raw, err := json.Marshal(page)
	require.NoError(t, err)
	jsonText := string(raw)
	require.NotContains(t, jsonText, "metadata")
	require.NotContains(t, jsonText, "description")
	require.NotContains(t, jsonText, "source_id")
	require.NotContains(t, jsonText, "idempotency")
	require.NotContains(t, jsonText, "actor")
	require.NotContains(t, jsonText, "admin")
	require.NotContains(t, jsonText, "email")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletSourceFiltersCoverImageAliasesAndOtherExcludesKnownSources(t *testing.T) {
	imageFilter, err := walletRawSourceFilter(WalletPublicSourceImageTask)
	require.NoError(t, err)
	imageCondition, imageArgs := walletSourceConditionSQL(2, imageFilter)
	require.Contains(t, imageCondition, "source_type IN")
	require.Contains(t, imageCondition, "LOWER(bt.source_type) LIKE")
	require.Contains(t, imageArgs, "image_balance_hold")
	require.Contains(t, imageArgs, "%image%")

	otherFilter, err := walletRawSourceFilter(WalletPublicSourceOther)
	require.NoError(t, err)
	otherCondition, otherArgs := walletSourceConditionSQL(2, otherFilter)
	require.Contains(t, otherCondition, "source_type NOT IN")
	require.Contains(t, otherCondition, "LOWER(bt.source_type) NOT LIKE")
	require.Contains(t, otherArgs, PlayRewardSourceTeamSharedReward)
	require.Contains(t, otherArgs, "image_balance_capture")
	require.Contains(t, otherArgs, "%image%")

	require.Equal(t, WalletPublicSourceImageTask, WalletPublicSourceForRaw("image_balance_release"))
	require.Equal(t, WalletPublicSourceRefund, WalletPublicSourceForRaw("reversal"))
	require.Equal(t, WalletPublicSourceRefund, WalletPublicSourceForRaw(FundRefundLedgerSourceSubmit))
	require.Equal(t, WalletPublicSourceRecharge, WalletPublicSourceForRaw(FundLedgerSourceOfflineRecharge))
	require.Equal(t, WalletPublicSourceGift, WalletPublicSourceForRaw("auth_first_bind_grant"))
	require.Equal(t, WalletPublicSourceGift, WalletPublicSourceForRaw(FundLedgerSourceOpsGift))
	require.Equal(t, WalletPublicSourceSubscription, WalletPublicSourceForRaw("user_subscription"))
	require.Equal(t, WalletPublicSourceWithdrawal, WalletPublicSourceForRaw(WithdrawalLedgerSourceSubmit))
	require.Equal(t, WalletPublicSourceWithdrawal, WalletPublicSourceForRaw(WithdrawalLedgerSourcePaid))
	require.Equal(t, WalletPublicSourceOther, WalletPublicSourceForRaw("legacy_manual_adjustment"))
}

func TestWalletSourceFiltersUseClassifiedFundBatchesForGifts(t *testing.T) {
	giftFilter, err := walletRawSourceFilter(WalletPublicSourceGift)
	require.NoError(t, err)
	giftCondition, giftArgs := walletSourceConditionSQL(2, giftFilter)
	require.Contains(t, giftCondition, "bt.source_type IN")
	require.Contains(t, giftCondition, "bfb.source_kind IN")
	require.Contains(t, giftArgs, "auth_first_bind_grant")
	require.Contains(t, giftArgs, FundSourceKindSignupGift)

	adminFilter, err := walletRawSourceFilter(WalletPublicSourceAdminAdjustment)
	require.NoError(t, err)
	adminCondition, adminArgs := walletSourceConditionSQL(2, adminFilter)
	require.Contains(t, adminCondition, "bt.source_type IN")
	require.Contains(t, adminCondition, "bfb.source_kind IS NULL OR bfb.source_kind NOT IN")
	require.Contains(t, adminArgs, "admin_balance")
	require.Contains(t, adminArgs, FundSourceKindOpsGift)

	require.Equal(t, WalletPublicSourceGift, WalletPublicSourceForRawWithFundKind("admin_balance", FundSourceKindSignupGift))
	require.Equal(t, WalletPublicSourceGift, WalletPublicSourceForRawWithFundKind("admin_adjustment", FundSourceKindOpsGift))
	require.Equal(t, WalletPublicSourceAdminAdjustment, WalletPublicSourceForRawWithFundKind("admin_adjustment", ""))
	require.Equal(t, WalletPublicSourceRecharge, WalletPublicSourceForRawWithFundKind("admin_balance", FundSourceKindOfflineRecharge))
}
