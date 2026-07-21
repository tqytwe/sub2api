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
	AvailableBalance    decimal.Decimal `json:"available_balance"`
	TaskReservedBalance decimal.Decimal `json:"task_reserved_balance"`
	TotalCredits        decimal.Decimal `json:"total_credits"`
	TotalDebits         decimal.Decimal `json:"total_debits"`
	TransactionCount    int64           `json:"transaction_count"`
	LastTransactionAt   *time.Time      `json:"last_transaction_at,omitempty"`
}

type WalletTransaction struct {
	ID           int64           `json:"id"`
	Source       string          `json:"source"`
	Direction    string          `json:"direction"`
	BalanceDelta decimal.Decimal `json:"balance_delta"`
	FrozenDelta  decimal.Decimal `json:"frozen_delta"`
	BalanceAfter decimal.Decimal `json:"balance_after"`
	FrozenAfter  decimal.Decimal `json:"frozen_after"`
	CreatedAt    time.Time       `json:"created_at"`
}

type WalletTransactionQuery struct {
	Source   string
	Page     int
	PageSize int
}

type walletSourceFilter struct {
	rawTypes         []string
	containsPatterns []string
	exclude          bool
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
	COALESCE(u.frozen_balance, 0)::text AS task_reserved_balance,
	COALESCE(SUM(CASE WHEN bt.balance_delta > 0 THEN bt.balance_delta ELSE 0 END), 0)::text AS total_credits,
	COALESCE(SUM(CASE WHEN bt.balance_delta < 0 THEN -bt.balance_delta ELSE 0 END), 0)::text AS total_debits,
	COUNT(bt.id)::bigint AS transaction_count,
	MAX(bt.created_at) AS last_transaction_at
FROM users u
LEFT JOIN balance_transactions bt ON bt.user_id = u.id
WHERE u.id = $1
  AND u.deleted_at IS NULL
GROUP BY u.id, u.balance, u.frozen_balance`, userID)

	var summary WalletSummary
	var available, reserved, credits, debits string
	var last sql.NullTime
	if err := rows.Scan(&available, &reserved, &credits, &debits, &summary.TransactionCount, &last); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get wallet summary: %w", err)
	}
	var err error
	if summary.AvailableBalance, err = decimal.NewFromString(available); err != nil {
		return nil, fmt.Errorf("parse wallet available balance: %w", err)
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
	return &summary, nil
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
FROM balance_transactions
WHERE user_id = $1`
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
SELECT id, source_type, balance_delta::text, frozen_delta::text,
       COALESCE(balance_after, 0)::text, COALESCE(frozen_after, 0)::text, created_at
FROM balance_transactions
WHERE user_id = $1
ORDER BY created_at DESC, id DESC`
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
	if len(sourceFilter.rawTypes) == 0 && len(sourceFilter.containsPatterns) == 0 {
		return "", nil
	}
	if len(sourceFilter.rawTypes) == 1 && len(sourceFilter.containsPatterns) == 0 && !sourceFilter.exclude {
		return fmt.Sprintf("source_type = $%d", start), []any{sourceFilter.rawTypes[0]}
	}
	parts := make([]string, 0, 1+len(sourceFilter.containsPatterns))
	args := make([]any, 0, len(sourceFilter.rawTypes)+len(sourceFilter.containsPatterns))
	if len(sourceFilter.rawTypes) > 0 {
		filter, filterArgs := walletSourceFilterSQL(start, sourceFilter.rawTypes)
		operator := "IN"
		if sourceFilter.exclude {
			operator = "NOT IN"
		}
		parts = append(parts, "source_type "+operator+" ("+filter+")")
		args = append(args, filterArgs...)
	}
	for _, pattern := range sourceFilter.containsPatterns {
		operator := "LIKE"
		if sourceFilter.exclude {
			operator = "NOT LIKE"
		}
		parts = append(parts, fmt.Sprintf("LOWER(source_type) %s $%d", operator, start+len(args)))
		args = append(args, "%"+strings.ToLower(pattern)+"%")
	}
	joiner := " OR "
	if sourceFilter.exclude {
		joiner = " AND "
	}
	if len(parts) == 1 {
		return parts[0], args
	}
	return "(" + strings.Join(parts, joiner) + ")", args
}

func scanWalletTransaction(rows *sql.Rows) (WalletTransaction, error) {
	var item WalletTransaction
	var rawSource, balanceDelta, frozenDelta, balanceAfter, frozenAfter string
	if err := rows.Scan(
		&item.ID,
		&rawSource,
		&balanceDelta,
		&frozenDelta,
		&balanceAfter,
		&frozenAfter,
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
	if item.BalanceAfter, err = decimal.NewFromString(balanceAfter); err != nil {
		return WalletTransaction{}, fmt.Errorf("parse wallet balance after: %w", err)
	}
	if item.FrozenAfter, err = decimal.NewFromString(frozenAfter); err != nil {
		return WalletTransaction{}, fmt.Errorf("parse wallet frozen after: %w", err)
	}
	item.Source = WalletPublicSourceForRaw(rawSource)
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
	source := strings.ToLower(strings.TrimSpace(raw))
	switch source {
	case "payment_recharge", "recharge", "payment_balance":
		return WalletPublicSourceRecharge
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
	case "refund", "payment_refund", "reversal":
		return WalletPublicSourceRefund
	case "admin_balance", "admin_adjustment":
		return WalletPublicSourceAdminAdjustment
	case "promo_bonus", "promo_code", "promotion", "auth_first_bind_grant":
		return WalletPublicSourcePromotion
	case "subscription", "subscription_refund", "user_subscription":
		return WalletPublicSourceSubscription
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
		return walletIncludeRawSources("payment_recharge", "payment_balance", "recharge"), nil
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
		return walletIncludeRawSources("refund", "payment_refund", "reversal"), nil
	case WalletPublicSourceAdminAdjustment:
		return walletIncludeRawSources("admin_balance", "admin_adjustment"), nil
	case WalletPublicSourcePromotion:
		return walletIncludeRawSources("promo_bonus", "promo_code", "promotion", "auth_first_bind_grant"), nil
	case WalletPublicSourceSubscription:
		return walletIncludeRawSources("subscription", "subscription_refund", "user_subscription"), nil
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

func walletKnownRawSourceTypes() []string {
	return []string{
		"payment_recharge",
		"payment_balance",
		"recharge",
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
		"admin_balance",
		"admin_adjustment",
		"promo_bonus",
		"promo_code",
		"promotion",
		"auth_first_bind_grant",
		"subscription",
		"subscription_refund",
		"user_subscription",
	}
}
