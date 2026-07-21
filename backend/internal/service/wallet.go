package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	WalletDirectionCredit  = "credit"
	WalletDirectionDebit   = "debit"
	WalletDirectionNeutral = "neutral"

	WalletPublicSourceRecharge        = "recharge"
	WalletPublicSourceRedeem          = "redeem"
	WalletPublicSourceAffiliate       = "affiliate"
	WalletPublicSourceTeamReward      = "team_reward"
	WalletPublicSourceArenaReward     = "arena_reward"
	WalletPublicSourceCheckin         = "checkin"
	WalletPublicSourceQuiz            = "quiz"
	WalletPublicSourceBlindBox        = "blind_box"
	WalletPublicSourceUsage           = "usage"
	WalletPublicSourceImageTask       = "image_task"
	WalletPublicSourceRefund          = "refund"
	WalletPublicSourceAdminAdjustment = "admin_adjustment"
	WalletPublicSourcePromotion       = "promotion"
	WalletPublicSourceSubscription    = "subscription"
	WalletPublicSourceWithdrawal      = "withdrawal"
	WalletPublicSourceGift            = "gift"
	WalletPublicSourceOther           = "other"
)

var (
	ErrWalletUnavailable  = infraerrors.InternalServer("WALLET_UNAVAILABLE", "wallet service is unavailable")
	ErrWalletInvalidInput = infraerrors.BadRequest("WALLET_INVALID_INPUT", "invalid wallet request")
)

type WalletService struct {
	db *sql.DB
}

type WalletSummary struct {
	AvailableBalance           decimal.Decimal `json:"available_balance"`
	WithdrawableBalance        decimal.Decimal `json:"withdrawable_balance"`
	PendingWithdrawableBalance decimal.Decimal `json:"pending_withdrawable_balance"`
	RefundableRechargeBalance  decimal.Decimal `json:"refundable_recharge_balance"`
	OnlineRechargeBalance      decimal.Decimal `json:"online_recharge_balance"`
	OfflineRechargeBalance     decimal.Decimal `json:"offline_recharge_balance"`
	GiftBalance                decimal.Decimal `json:"gift_balance"`
	SignupGiftBalance          decimal.Decimal `json:"signup_gift_balance"`
	OpsGiftBalance             decimal.Decimal `json:"ops_gift_balance"`
	RefundFrozenBalance        decimal.Decimal `json:"refund_frozen_balance"`
	UnclassifiedBalance        decimal.Decimal `json:"unclassified_balance"`
	WithdrawalFrozenBalance    decimal.Decimal `json:"withdrawal_frozen_balance"`
	TaskReservedBalance        decimal.Decimal `json:"task_reserved_balance"`
	TotalCredits               decimal.Decimal `json:"total_credits"`
	TotalDebits                decimal.Decimal `json:"total_debits"`
	TransactionCount           int64           `json:"transaction_count"`
	LastTransactionAt          *time.Time      `json:"last_transaction_at,omitempty"`
}

type WalletTransaction struct {
	ID                    int64           `json:"id"`
	Source                string          `json:"source"`
	Direction             string          `json:"direction"`
	BalanceDelta          decimal.Decimal `json:"balance_delta"`
	FrozenDelta           decimal.Decimal `json:"frozen_delta"`
	WithdrawableDelta     decimal.Decimal `json:"withdrawable_delta"`
	WithdrawalFrozenDelta decimal.Decimal `json:"withdrawal_frozen_delta"`
	BalanceAfter          decimal.Decimal `json:"balance_after"`
	FrozenAfter           decimal.Decimal `json:"frozen_after"`
	WithdrawableAfter     decimal.Decimal `json:"withdrawable_after"`
	WithdrawalFrozenAfter decimal.Decimal `json:"withdrawal_frozen_after"`
	CreatedAt             time.Time       `json:"created_at"`
}

type WalletTransactionQuery struct {
	Source   string
	Page     int
	PageSize int
}

type walletSourceFilter struct {
	rawTypes         []string
	fundKinds        []string
	containsPatterns []string
	exclude          bool
	excludeFundKinds []string
}

type WalletTransactionPage struct {
	Items    []WalletTransaction `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Pages    int                 `json:"pages"`
}

func NewWalletService(db *sql.DB) *WalletService {
	return &WalletService{db: db}
}

func (s *WalletService) GetSummary(ctx context.Context, userID int64) (*WalletSummary, error) {
	if s == nil || s.db == nil {
		return nil, ErrWalletUnavailable
	}
	if userID <= 0 {
		return nil, ErrWalletInvalidInput
	}
	rows := s.db.QueryRowContext(ctx, `
SELECT
	COALESCE(u.balance, 0)::text AS available_balance,
	LEAST(
		COALESCE(u.balance, 0),
		GREATEST(
			COALESCE((
				SELECT SUM(we.remaining_amount)
				FROM withdrawable_entitlements we
				WHERE we.user_id = u.id
				  AND we.status = 'active'
				  AND we.remaining_amount > 0
				  AND we.available_at <= NOW()
				), 0),
				0
			)
		)::text AS withdrawable_balance,
	COALESCE((
		SELECT SUM(we.remaining_amount)
		FROM withdrawable_entitlements we
		WHERE we.user_id = u.id
		  AND we.status = 'active'
		  AND we.remaining_amount > 0
		  AND we.available_at > NOW()
	), 0)::text AS pending_withdrawable_balance,
	COALESCE(u.withdrawal_frozen_balance, 0)::text AS withdrawal_frozen_balance,
	COALESCE(u.frozen_balance, 0)::text AS task_reserved_balance,
	COALESCE(SUM(CASE WHEN bt.balance_delta > 0 THEN bt.balance_delta ELSE 0 END), 0)::text AS total_credits,
	COALESCE(SUM(CASE WHEN bt.balance_delta < 0 THEN -bt.balance_delta ELSE 0 END), 0)::text AS total_debits,
	COUNT(bt.id)::bigint AS transaction_count,
	MAX(bt.created_at) AS last_transaction_at
FROM users u
LEFT JOIN balance_transactions bt ON bt.user_id = u.id
WHERE u.id = $1
  AND u.deleted_at IS NULL
GROUP BY u.id, u.balance, u.frozen_balance, u.withdrawal_frozen_balance`, userID)

	var summary WalletSummary
	var available, withdrawable, pendingWithdrawable, withdrawalFrozen, reserved, credits, debits string
	var last sql.NullTime
	if err := rows.Scan(&available, &withdrawable, &pendingWithdrawable, &withdrawalFrozen, &reserved, &credits, &debits, &summary.TransactionCount, &last); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get wallet summary: %w", err)
	}
	var err error
	if summary.AvailableBalance, err = decimal.NewFromString(available); err != nil {
		return nil, fmt.Errorf("parse wallet available balance: %w", err)
	}
	if summary.WithdrawableBalance, err = decimal.NewFromString(withdrawable); err != nil {
		return nil, fmt.Errorf("parse wallet withdrawable balance: %w", err)
	}
	if summary.PendingWithdrawableBalance, err = decimal.NewFromString(pendingWithdrawable); err != nil {
		return nil, fmt.Errorf("parse wallet pending withdrawable balance: %w", err)
	}
	if summary.WithdrawalFrozenBalance, err = decimal.NewFromString(withdrawalFrozen); err != nil {
		return nil, fmt.Errorf("parse wallet withdrawal frozen balance: %w", err)
	}
	if summary.TaskReservedBalance, err = decimal.NewFromString(reserved); err != nil {
		return nil, fmt.Errorf("parse wallet task reserved balance: %w", err)
	}
	if summary.TotalCredits, err = decimal.NewFromString(credits); err != nil {
		return nil, fmt.Errorf("parse wallet total credits: %w", err)
	}
	if summary.TotalDebits, err = decimal.NewFromString(debits); err != nil {
		return nil, fmt.Errorf("parse wallet total debits: %w", err)
	}
	if last.Valid {
		summary.LastTransactionAt = &last.Time
	}
	if err := s.attachFundBreakdown(ctx, userID, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func (s *WalletService) attachFundBreakdown(ctx context.Context, userID int64, summary *WalletSummary) error {
	breakdown, err := NewFundManagementService(s.db, nil, nil).GetWalletBreakdown(ctx, userID)
	if err != nil {
		return err
	}
	summary.RefundableRechargeBalance = breakdown.RefundableRechargeBalance
	summary.OnlineRechargeBalance = breakdown.OnlineRechargeBalance
	summary.OfflineRechargeBalance = breakdown.OfflineRechargeBalance
	summary.GiftBalance = breakdown.GiftBalance
	summary.SignupGiftBalance = breakdown.SignupGiftBalance
	summary.OpsGiftBalance = breakdown.OpsGiftBalance
	summary.RefundFrozenBalance = breakdown.RefundFrozenBalance
	summary.UnclassifiedBalance = breakdown.UnclassifiedBalance
	return nil
}

func (s *WalletService) ListTransactions(ctx context.Context, userID int64, query WalletTransactionQuery) (*WalletTransactionPage, error) {
	if s == nil || s.db == nil {
		return nil, ErrWalletUnavailable
	}
	if userID <= 0 {
		return nil, ErrWalletInvalidInput
	}
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	sourceFilter, err := walletRawSourceFilter(query.Source)
	if err != nil {
		return nil, err
	}
	total, err := s.countTransactions(ctx, userID, sourceFilter)
	if err != nil {
		return nil, err
	}
	items, err := s.listTransactions(ctx, userID, sourceFilter, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	pages := int(math.Ceil(float64(total) / float64(pageSize)))
	if pages < 1 {
		pages = 1
	}
	return &WalletTransactionPage{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	}, nil
}

func (s *WalletService) countTransactions(ctx context.Context, userID int64, sourceFilter walletSourceFilter) (int64, error) {
	var total int64
	condition, filterArgs := walletSourceConditionSQL(2, sourceFilter)
	args := append([]any{userID}, filterArgs...)
	query := `
SELECT COUNT(*)::bigint
FROM balance_transactions bt
LEFT JOIN balance_fund_batches bfb
  ON bfb.user_id = bt.user_id
 AND bfb.balance_transaction_id = bt.id
WHERE bt.user_id = $1`
	if condition != "" {
		query += "\n  AND " + condition
	}
	row := s.db.QueryRowContext(ctx, query, args...)
	if err := row.Scan(&total); err != nil {
		return 0, fmt.Errorf("count wallet transactions: %w", err)
	}
	return total, nil
}

func (s *WalletService) listTransactions(ctx context.Context, userID int64, sourceFilter walletSourceFilter, limit, offset int) ([]WalletTransaction, error) {
	condition, filterArgs := walletSourceConditionSQL(2, sourceFilter)
	args := append([]any{userID}, filterArgs...)
	args = append(args, limit, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)
	query := `
	SELECT bt.id, bt.source_type, COALESCE(bfb.source_kind, '') AS fund_source_kind,
	       bt.balance_delta::text, bt.frozen_delta::text,
	       withdrawable_delta::text, withdrawal_frozen_delta::text,
	       COALESCE(bt.balance_after, 0)::text, COALESCE(bt.frozen_after, 0)::text,
	       COALESCE(bt.withdrawable_after, 0)::text, COALESCE(bt.withdrawal_frozen_after, 0)::text,
	       bt.created_at
FROM balance_transactions bt
LEFT JOIN balance_fund_batches bfb
  ON bfb.user_id = bt.user_id
 AND bfb.balance_transaction_id = bt.id
WHERE bt.user_id = $1
ORDER BY bt.created_at DESC, bt.id DESC`
	if condition != "" {
		query = strings.Replace(query, "\nORDER BY", "\n  AND "+condition+"\nORDER BY", 1)
	}
	query += "\nLIMIT $" + strconv.Itoa(limitArg) + " OFFSET $" + strconv.Itoa(offsetArg)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list wallet transactions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]WalletTransaction, 0, limit)
	for rows.Next() {
		item, scanErr := scanWalletTransaction(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wallet transactions: %w", err)
	}
	return items, nil
}

func walletSourceFilterSQL(start int, sourceTypes []string) (string, []any) {
	placeholders := make([]string, 0, len(sourceTypes))
	args := make([]any, 0, len(sourceTypes))
	for i, sourceType := range sourceTypes {
		placeholders = append(placeholders, fmt.Sprintf("$%d", start+i))
		args = append(args, sourceType)
	}
	return strings.Join(placeholders, ", "), args
}

func walletSourceConditionSQL(start int, sourceFilter walletSourceFilter) (string, []any) {
	if len(sourceFilter.rawTypes) == 0 && len(sourceFilter.fundKinds) == 0 && len(sourceFilter.containsPatterns) == 0 && len(sourceFilter.excludeFundKinds) == 0 {
		return "", nil
	}
	includeParts := make([]string, 0, 2+len(sourceFilter.containsPatterns))
	excludeParts := make([]string, 0, 2+len(sourceFilter.containsPatterns))
	args := make([]any, 0, len(sourceFilter.rawTypes)+len(sourceFilter.containsPatterns))
	if len(sourceFilter.rawTypes) > 0 {
		filter, filterArgs := walletSourceFilterSQL(start, sourceFilter.rawTypes)
		if sourceFilter.exclude {
			excludeParts = append(excludeParts, "bt.source_type NOT IN ("+filter+")")
		} else if len(sourceFilter.rawTypes) == 1 {
			includeParts = append(includeParts, "bt.source_type = "+filter)
		} else {
			includeParts = append(includeParts, "bt.source_type IN ("+filter+")")
		}
		args = append(args, filterArgs...)
	}
	if len(sourceFilter.fundKinds) > 0 {
		filter, filterArgs := walletSourceFilterSQL(start+len(args), sourceFilter.fundKinds)
		if sourceFilter.exclude {
			excludeParts = append(excludeParts, "(bfb.source_kind IS NULL OR bfb.source_kind NOT IN ("+filter+"))")
		} else if len(sourceFilter.fundKinds) == 1 {
			includeParts = append(includeParts, "bfb.source_kind = "+filter)
		} else {
			includeParts = append(includeParts, "bfb.source_kind IN ("+filter+")")
		}
		args = append(args, filterArgs...)
	}
	for _, pattern := range sourceFilter.containsPatterns {
		if sourceFilter.exclude {
			excludeParts = append(excludeParts, fmt.Sprintf("LOWER(bt.source_type) NOT LIKE $%d", start+len(args)))
		} else {
			includeParts = append(includeParts, fmt.Sprintf("LOWER(bt.source_type) LIKE $%d", start+len(args)))
		}
		args = append(args, "%"+strings.ToLower(pattern)+"%")
	}
	if len(sourceFilter.excludeFundKinds) > 0 {
		filter, filterArgs := walletSourceFilterSQL(start+len(args), sourceFilter.excludeFundKinds)
		excludeParts = append(excludeParts, "(bfb.source_kind IS NULL OR bfb.source_kind NOT IN ("+filter+"))")
		args = append(args, filterArgs...)
	}
	conditions := make([]string, 0, 2)
	if len(includeParts) > 0 {
		if len(includeParts) == 1 {
			conditions = append(conditions, includeParts[0])
		} else {
			conditions = append(conditions, "("+strings.Join(includeParts, " OR ")+")")
		}
	}
	if len(excludeParts) > 0 {
		conditions = append(conditions, strings.Join(excludeParts, " AND "))
	}
	if len(conditions) == 0 {
		return "", args
	}
	if len(conditions) == 1 {
		return conditions[0], args
	}
	return "(" + strings.Join(conditions, " AND ") + ")", args
}

func scanWalletTransaction(rows *sql.Rows) (WalletTransaction, error) {
	var item WalletTransaction
	var rawSource, fundSourceKind, balanceDelta, frozenDelta, withdrawableDelta, withdrawalFrozenDelta, balanceAfter, frozenAfter, withdrawableAfter, withdrawalFrozenAfter string
	if err := rows.Scan(
		&item.ID,
		&rawSource,
		&fundSourceKind,
		&balanceDelta,
		&frozenDelta,
		&withdrawableDelta,
		&withdrawalFrozenDelta,
		&balanceAfter,
		&frozenAfter,
		&withdrawableAfter,
		&withdrawalFrozenAfter,
		&item.CreatedAt,
	); err != nil {
		return WalletTransaction{}, fmt.Errorf("scan wallet transaction: %w", err)
	}
	var err error
	if item.BalanceDelta, err = decimal.NewFromString(balanceDelta); err != nil {
		return WalletTransaction{}, fmt.Errorf("parse wallet balance delta: %w", err)
	}
	if item.FrozenDelta, err = decimal.NewFromString(frozenDelta); err != nil {
		return WalletTransaction{}, fmt.Errorf("parse wallet frozen delta: %w", err)
	}
	if item.WithdrawableDelta, err = decimal.NewFromString(withdrawableDelta); err != nil {
		return WalletTransaction{}, fmt.Errorf("parse wallet withdrawable delta: %w", err)
	}
	if item.WithdrawalFrozenDelta, err = decimal.NewFromString(withdrawalFrozenDelta); err != nil {
		return WalletTransaction{}, fmt.Errorf("parse wallet withdrawal frozen delta: %w", err)
	}
	if item.BalanceAfter, err = decimal.NewFromString(balanceAfter); err != nil {
		return WalletTransaction{}, fmt.Errorf("parse wallet balance after: %w", err)
	}
	if item.FrozenAfter, err = decimal.NewFromString(frozenAfter); err != nil {
		return WalletTransaction{}, fmt.Errorf("parse wallet frozen after: %w", err)
	}
	if item.WithdrawableAfter, err = decimal.NewFromString(withdrawableAfter); err != nil {
		return WalletTransaction{}, fmt.Errorf("parse wallet withdrawable after: %w", err)
	}
	if item.WithdrawalFrozenAfter, err = decimal.NewFromString(withdrawalFrozenAfter); err != nil {
		return WalletTransaction{}, fmt.Errorf("parse wallet withdrawal frozen after: %w", err)
	}
	item.Source = WalletPublicSourceForRawWithFundKind(rawSource, fundSourceKind)
	item.Direction = walletDirection(item.BalanceDelta, item.FrozenDelta)
	return item, nil
}

func walletDirection(balanceDelta, frozenDelta decimal.Decimal) string {
	net := balanceDelta.Add(frozenDelta)
	switch {
	case net.IsPositive():
		return WalletDirectionCredit
	case net.IsNegative():
		return WalletDirectionDebit
	default:
		return WalletDirectionNeutral
	}
}

func WalletPublicSourceForRaw(raw string) string {
	return WalletPublicSourceForRawWithFundKind(raw, "")
}

func WalletPublicSourceForRawWithFundKind(raw string, fundKind string) string {
	switch strings.ToLower(strings.TrimSpace(fundKind)) {
	case FundSourceKindOnlineRecharge, FundSourceKindOfflineRecharge:
		return WalletPublicSourceRecharge
	case FundSourceKindSignupGift, FundSourceKindOpsGift, FundSourceKindCompensation:
		return WalletPublicSourceGift
	}
	source := strings.ToLower(strings.TrimSpace(raw))
	switch source {
	case "payment_recharge", "recharge", "payment_balance", FundLedgerSourceOfflineRecharge:
		return WalletPublicSourceRecharge
	case FundLedgerSourceOpsGift, FundLedgerSourceSignupGift, FundLedgerSourceCompensation, "auth_first_bind_grant":
		return WalletPublicSourceGift
	case "balance", "redeem", "redeem_code":
		return WalletPublicSourceRedeem
	case "affiliate_balance", "affiliate_transfer", "user_affiliate_ledger":
		return WalletPublicSourceAffiliate
	case PlayRewardSourceTeamSharedReward:
		return WalletPublicSourceTeamReward
	case PlayRewardSourceArenaSettlement, PlayRewardSourceArenaDaily:
		return WalletPublicSourceArenaReward
	case PlayRewardSourceCheckin, PlayRewardSourceCheckinMilestone, PlayRewardSourceCheckinMakeup:
		return WalletPublicSourceCheckin
	case PlayRewardSourceQuiz:
		return WalletPublicSourceQuiz
	case PlayRewardSourceBlindbox:
		return WalletPublicSourceBlindBox
	case "usage_charge", "usage_log", "api_usage":
		return WalletPublicSourceUsage
	case "refund", "payment_refund", "reversal", FundRefundLedgerSourceSubmit, FundRefundLedgerSourceCancel, FundRefundLedgerSourceReject, FundRefundLedgerSourcePaid:
		return WalletPublicSourceRefund
	case "admin_balance", "admin_adjustment":
		return WalletPublicSourceAdminAdjustment
	case "promo_bonus", "promo_code", "promotion":
		return WalletPublicSourcePromotion
	case "subscription", "subscription_refund", "user_subscription":
		return WalletPublicSourceSubscription
	case WithdrawalLedgerSourceSubmit, WithdrawalLedgerSourceCancel, WithdrawalLedgerSourceReject, WithdrawalLedgerSourcePaid:
		return WalletPublicSourceWithdrawal
	default:
		if strings.Contains(source, "image") || strings.Contains(source, "batch_image") {
			return WalletPublicSourceImageTask
		}
		return WalletPublicSourceOther
	}
}

func walletRawSourceFilter(publicSource string) (walletSourceFilter, error) {
	switch strings.ToLower(strings.TrimSpace(publicSource)) {
	case "", "all":
		return walletSourceFilter{}, nil
	case WalletPublicSourceRecharge:
		return walletIncludeRawSourcesAndFundKinds(
			[]string{"payment_recharge", "payment_balance", "recharge", FundLedgerSourceOfflineRecharge},
			[]string{FundSourceKindOnlineRecharge, FundSourceKindOfflineRecharge},
		), nil
	case WalletPublicSourceGift:
		return walletIncludeRawSourcesAndFundKinds(
			[]string{FundLedgerSourceOpsGift, FundLedgerSourceSignupGift, FundLedgerSourceCompensation, "auth_first_bind_grant"},
			[]string{FundSourceKindSignupGift, FundSourceKindOpsGift, FundSourceKindCompensation},
		), nil
	case WalletPublicSourceRedeem:
		return walletIncludeRawSources("balance", "redeem", "redeem_code"), nil
	case WalletPublicSourceAffiliate:
		return walletIncludeRawSources("affiliate_balance", "affiliate_transfer", "user_affiliate_ledger"), nil
	case WalletPublicSourceTeamReward:
		return walletIncludeRawSources(PlayRewardSourceTeamSharedReward), nil
	case WalletPublicSourceArenaReward:
		return walletIncludeRawSources(PlayRewardSourceArenaSettlement, PlayRewardSourceArenaDaily), nil
	case WalletPublicSourceCheckin:
		return walletIncludeRawSources(PlayRewardSourceCheckin, PlayRewardSourceCheckinMilestone, PlayRewardSourceCheckinMakeup), nil
	case WalletPublicSourceQuiz:
		return walletIncludeRawSources(PlayRewardSourceQuiz), nil
	case WalletPublicSourceBlindBox:
		return walletIncludeRawSources(PlayRewardSourceBlindbox), nil
	case WalletPublicSourceUsage:
		return walletIncludeRawSources("usage_charge", "usage_log", "api_usage"), nil
	case WalletPublicSourceImageTask:
		return walletSourceFilter{
			rawTypes:         []string{"image_task", "batch_image_task", "batch_image", "image_balance_hold", "image_balance_capture", "image_balance_release"},
			containsPatterns: []string{"image"},
		}, nil
	case WalletPublicSourceRefund:
		return walletIncludeRawSources("refund", "payment_refund", "reversal", FundRefundLedgerSourceSubmit, FundRefundLedgerSourceCancel, FundRefundLedgerSourceReject, FundRefundLedgerSourcePaid), nil
	case WalletPublicSourceAdminAdjustment:
		filter := walletIncludeRawSources("admin_balance", "admin_adjustment")
		filter.excludeFundKinds = []string{FundSourceKindSignupGift, FundSourceKindOpsGift, FundSourceKindCompensation}
		return filter, nil
	case WalletPublicSourcePromotion:
		return walletIncludeRawSources("promo_bonus", "promo_code", "promotion"), nil
	case WalletPublicSourceSubscription:
		return walletIncludeRawSources("subscription", "subscription_refund", "user_subscription"), nil
	case WalletPublicSourceWithdrawal:
		return walletIncludeRawSources(WithdrawalLedgerSourceSubmit, WithdrawalLedgerSourceCancel, WithdrawalLedgerSourceReject, WithdrawalLedgerSourcePaid), nil
	case WalletPublicSourceOther:
		return walletSourceFilter{
			rawTypes:         walletKnownRawSourceTypes(),
			containsPatterns: []string{"image"},
			exclude:          true,
		}, nil
	default:
		return walletSourceFilter{}, ErrWalletInvalidInput.WithMetadata(map[string]string{"source": "invalid"})
	}
}

func walletIncludeRawSources(rawTypes ...string) walletSourceFilter {
	return walletSourceFilter{rawTypes: rawTypes}
}

func walletIncludeRawSourcesAndFundKinds(rawTypes []string, fundKinds []string) walletSourceFilter {
	return walletSourceFilter{rawTypes: rawTypes, fundKinds: fundKinds}
}

func walletKnownRawSourceTypes() []string {
	return []string{
		"payment_recharge",
		"payment_balance",
		"recharge",
		FundLedgerSourceOfflineRecharge,
		FundLedgerSourceOpsGift,
		FundLedgerSourceSignupGift,
		FundLedgerSourceCompensation,
		"auth_first_bind_grant",
		"balance",
		"redeem",
		"redeem_code",
		"affiliate_balance",
		"affiliate_transfer",
		"user_affiliate_ledger",
		PlayRewardSourceTeamSharedReward,
		PlayRewardSourceArenaSettlement,
		PlayRewardSourceArenaDaily,
		PlayRewardSourceCheckin,
		PlayRewardSourceCheckinMilestone,
		PlayRewardSourceCheckinMakeup,
		PlayRewardSourceQuiz,
		PlayRewardSourceBlindbox,
		"usage_charge",
		"usage_log",
		"api_usage",
		"image_task",
		"batch_image_task",
		"batch_image",
		"image_balance_hold",
		"image_balance_capture",
		"image_balance_release",
		"refund",
		"payment_refund",
		"reversal",
		FundRefundLedgerSourceSubmit,
		FundRefundLedgerSourceCancel,
		FundRefundLedgerSourceReject,
		FundRefundLedgerSourcePaid,
		"admin_balance",
		"admin_adjustment",
		"promo_bonus",
		"promo_code",
		"promotion",
		"subscription",
		"subscription_refund",
		"user_subscription",
		WithdrawalLedgerSourceSubmit,
		WithdrawalLedgerSourceCancel,
		WithdrawalLedgerSourceReject,
		WithdrawalLedgerSourcePaid,
	}
}
