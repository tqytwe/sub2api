package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	WithdrawalStatusPendingReview = "pending_review"
	WithdrawalStatusSecondReview  = "second_review"
	WithdrawalStatusPayoutPending = "payout_pending"
	WithdrawalStatusPaid          = "paid"
	WithdrawalStatusRejected      = "rejected"
	WithdrawalStatusCanceled      = "canceled"

	WithdrawalPayoutMethodAlipay       = "alipay"
	WithdrawalPayoutMethodBankTransfer = "bank_transfer"
	WithdrawalPayoutMethodOther        = "other"

	WithdrawalCurrencyUSD = "USD"
	WithdrawalCurrencyCNY = "CNY"

	WithdrawalLedgerSourceSubmit = "withdrawal_submit"
	WithdrawalLedgerSourceCancel = "withdrawal_cancel"
	WithdrawalLedgerSourceReject = "withdrawal_reject"
	WithdrawalLedgerSourcePaid   = "withdrawal_paid"

	withdrawalActorUser  = "user"
	withdrawalActorAdmin = "admin"
)

var (
	ErrWithdrawalUnavailable              = infraerrors.InternalServer("WITHDRAWAL_UNAVAILABLE", "withdrawal service is unavailable")
	ErrWithdrawalInvalidAmount            = infraerrors.BadRequest("WITHDRAWAL_INVALID_AMOUNT", "invalid withdrawal amount")
	ErrWithdrawalInvalidInput             = infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal request")
	ErrWithdrawalDisabled                 = infraerrors.Forbidden("WITHDRAWAL_DISABLED", "withdrawals are disabled for this user")
	ErrWithdrawalInsufficientWithdrawable = infraerrors.BadRequest("WITHDRAWAL_INSUFFICIENT_WITHDRAWABLE", "insufficient withdrawable balance")
	ErrWithdrawalDailyLimitExceeded       = infraerrors.BadRequest("WITHDRAWAL_DAILY_LIMIT_EXCEEDED", "daily withdrawal limit exceeded")
	ErrWithdrawalMinimumAmount            = infraerrors.BadRequest("WITHDRAWAL_MINIMUM_AMOUNT", "withdrawal amount is below the minimum")
	ErrWithdrawalInProgress               = infraerrors.Conflict("WITHDRAWAL_IN_PROGRESS", "another withdrawal request is already in progress")
	ErrWithdrawalAccountRequired          = infraerrors.BadRequest("WITHDRAWAL_ACCOUNT_REQUIRED", "payout account is required")
	ErrWithdrawalNotFound                 = infraerrors.NotFound("WITHDRAWAL_NOT_FOUND", "withdrawal request not found")
	ErrWithdrawalInvalidStatus            = infraerrors.BadRequest("WITHDRAWAL_INVALID_STATUS", "withdrawal request status does not allow this operation")
	ErrWithdrawalSelfReviewForbidden      = infraerrors.Forbidden("WITHDRAWAL_SELF_REVIEW_FORBIDDEN", "admins cannot review their own withdrawal")
	ErrWithdrawalUserNotReady             = infraerrors.BadRequest("WITHDRAWAL_USER_NOT_READY", "withdrawal recomputation is not ready for this user")
	ErrWithdrawalAccountEncryption        = infraerrors.InternalServer("WITHDRAWAL_ACCOUNT_ENCRYPTION", "withdrawal account encryption failed")
)

var withdrawalAmountPattern = regexp.MustCompile(`^[0-9]+(\.0+)?$`)

type WithdrawalService struct {
	db           *sql.DB
	ledger       *BalanceLedgerService
	encryptor    SecretEncryptor
	notification *NotificationEmailService
	now          func() time.Time
}

type WithdrawalSystemSettings struct {
	GlobalEnabled         bool            `json:"global_enabled"`
	MinimumAmount         decimal.Decimal `json:"minimum_amount"`
	DailyLimitAmount      decimal.Decimal `json:"daily_limit_amount"`
	DoubleReviewThreshold decimal.Decimal `json:"double_review_threshold"`
	RewardMaturityHours   int             `json:"reward_maturity_hours"`
	UpdatedBy             *int64          `json:"updated_by,omitempty"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

type WithdrawalSystemSettingsUpdate struct {
	GlobalEnabled         *bool
	MinimumAmount         *string
	DailyLimitAmount      *string
	DoubleReviewThreshold *string
	ActorUserID           int64
}

type UserWithdrawalSettings struct {
	UserID                   int64            `json:"user_id"`
	Enabled                  bool             `json:"enabled"`
	MinimumAmountOverride    *decimal.Decimal `json:"minimum_amount_override,omitempty"`
	DailyLimitAmountOverride *decimal.Decimal `json:"daily_limit_amount_override,omitempty"`
	DisabledReason           string           `json:"disabled_reason"`
	UpdatedBy                *int64           `json:"updated_by,omitempty"`
	UpdatedAt                time.Time        `json:"updated_at"`
	RecalcStatus             string           `json:"recalc_status,omitempty"`
}

type UserWithdrawalSettingsUpdate struct {
	UserID                   int64
	Enabled                  *bool
	MinimumAmountOverride    *string
	DailyLimitAmountOverride *string
	DisabledReason           *string
	ActorUserID              int64
}

type BatchUserWithdrawalSettingsUpdate struct {
	UserIDs                  []int64
	Enabled                  *bool
	MinimumAmountOverride    *string
	DailyLimitAmountOverride *string
	DisabledReason           *string
	ActorUserID              int64
}

type WithdrawalAvailability struct {
	GlobalEnabled         bool            `json:"global_enabled"`
	UserEnabled           bool            `json:"user_enabled"`
	CanApply              bool            `json:"can_apply"`
	DisabledReason        string          `json:"disabled_reason,omitempty"`
	RecalcStatus          string          `json:"recalc_status"`
	MinimumAmount         decimal.Decimal `json:"minimum_amount"`
	DailyLimitAmount      decimal.Decimal `json:"daily_limit_amount"`
	DailyUsedAmount       decimal.Decimal `json:"daily_used_amount"`
	RemainingDailyAmount  decimal.Decimal `json:"remaining_daily_amount"`
	DoubleReviewThreshold decimal.Decimal `json:"double_review_threshold"`
}

type WithdrawalPayoutAccount struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"user_id"`
	Method            string    `json:"method"`
	Currency          string    `json:"currency"`
	RecipientNameMask string    `json:"recipient_name_mask"`
	AccountMask       string    `json:"account_mask"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	accountEncrypted  string
}

type WithdrawalPayoutAccountInput struct {
	UserID        int64
	Method        string
	Currency      string
	RecipientName string
	Details       map[string]string
}

type withdrawalPayoutMask struct {
	RecipientNameMask string
	AccountMask       string
}

type WithdrawalCreateInput struct {
	UserID int64
	Amount string
	Locale string
}

type WithdrawalActionInput struct {
	RequestID   int64
	UserID      int64
	ActorUserID int64
	Note        string
	Reason      string
	Locale      string
}

type WithdrawalMarkPaidInput struct {
	RequestID     int64
	ActorUserID   int64
	PaidAmount    string
	PaidCurrency  string
	FXRate        string
	ExternalTxnID string
	PaidAt        *time.Time
	Note          string
	Locale        string
}

type WithdrawalRequest struct {
	ID                       int64                       `json:"id"`
	RequestNo                string                      `json:"request_no"`
	UserID                   int64                       `json:"user_id"`
	UserEmail                string                      `json:"user_email,omitempty"`
	Amount                   decimal.Decimal             `json:"amount"`
	Currency                 string                      `json:"currency"`
	Status                   string                      `json:"status"`
	PayoutMethod             string                      `json:"payout_method"`
	PayoutCurrency           string                      `json:"payout_currency"`
	PayoutAccountMask        string                      `json:"payout_account_mask"`
	PayoutRecipientNameMask  string                      `json:"payout_recipient_name_mask"`
	FirstApprovedBy          *int64                      `json:"first_approved_by,omitempty"`
	FirstApprovedAt          *time.Time                  `json:"first_approved_at,omitempty"`
	SecondApprovedBy         *int64                      `json:"second_approved_by,omitempty"`
	SecondApprovedAt         *time.Time                  `json:"second_approved_at,omitempty"`
	RejectedBy               *int64                      `json:"rejected_by,omitempty"`
	RejectedAt               *time.Time                  `json:"rejected_at,omitempty"`
	RejectedReason           string                      `json:"rejected_reason,omitempty"`
	CanceledAt               *time.Time                  `json:"canceled_at,omitempty"`
	PaidBy                   *int64                      `json:"paid_by,omitempty"`
	PaidAt                   *time.Time                  `json:"paid_at,omitempty"`
	PaidAmount               *decimal.Decimal            `json:"paid_amount,omitempty"`
	PaidCurrency             string                      `json:"paid_currency,omitempty"`
	PayoutFXRate             *decimal.Decimal            `json:"payout_fx_rate,omitempty"`
	ExternalTxnID            string                      `json:"external_txn_id,omitempty"`
	ExternalFeeAmount        decimal.Decimal             `json:"external_fee_amount"`
	PayoutNote               string                      `json:"payout_note,omitempty"`
	CreatedAt                time.Time                   `json:"created_at"`
	UpdatedAt                time.Time                   `json:"updated_at"`
	Events                   []WithdrawalStatusEvent     `json:"events,omitempty"`
	Entitlements             []WithdrawalEntitlementLock `json:"entitlements,omitempty"`
	accountSnapshotEncrypted string
}

type WithdrawalStatusEvent struct {
	ID          int64          `json:"id"`
	Status      string         `json:"status"`
	ActorType   string         `json:"actor_type"`
	ActorUserID *int64         `json:"actor_user_id,omitempty"`
	Note        string         `json:"note,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type WithdrawalEntitlementLock struct {
	EntitlementID int64           `json:"entitlement_id"`
	Amount        decimal.Decimal `json:"amount"`
	AvailableAt   time.Time       `json:"available_at"`
}

type WithdrawalListQuery struct {
	Status   string
	UserID   int64
	Page     int
	PageSize int
}

type WithdrawalRequestPage struct {
	Items    []WithdrawalRequest `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Pages    int                 `json:"pages"`
}

type withdrawalEntitlementFreezePlan struct {
	Total       decimal.Decimal
	Allocations []withdrawableAllocationPlan
}

func NewWithdrawalService(db *sql.DB, ledger *BalanceLedgerService, encryptor SecretEncryptor, notification *NotificationEmailService) *WithdrawalService {
	return &WithdrawalService{
		db:           db,
		ledger:       ledger,
		encryptor:    encryptor,
		notification: notification,
		now:          time.Now,
	}
}

func parseWithdrawalAmount(raw string) (decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if !withdrawalAmountPattern.MatchString(raw) {
		return decimal.Zero, ErrWithdrawalInvalidAmount
	}
	amount, err := decimal.NewFromString(raw)
	if err != nil || !amount.IsPositive() {
		return decimal.Zero, ErrWithdrawalInvalidAmount
	}
	if !amount.Equal(amount.Truncate(0)) {
		return decimal.Zero, ErrWithdrawalInvalidAmount
	}
	return amount.Round(8), nil
}

func parseOptionalWithdrawalAmount(raw *string, field string) (*decimal.Decimal, error) {
	if raw == nil {
		return nil, nil
	}
	amount, err := parseWithdrawalAmount(*raw)
	if err != nil {
		return nil, ErrWithdrawalInvalidAmount.WithMetadata(map[string]string{"field": field})
	}
	return &amount, nil
}

func parseWithdrawalPositiveDecimal(raw, field string) (decimal.Decimal, error) {
	value, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil || !value.IsPositive() {
		return decimal.Zero, ErrWithdrawalInvalidInput.WithMetadata(map[string]string{"field": field})
	}
	return value.Round(8), nil
}

func planWithdrawalEntitlementFreeze(amount decimal.Decimal, entitlements []withdrawableEntitlementSnapshot, asOf time.Time) (withdrawalEntitlementFreezePlan, error) {
	amount = clampDecimalScale(amount)
	remaining := amount
	asOf = asOf.UTC()
	plan := withdrawalEntitlementFreezePlan{Allocations: make([]withdrawableAllocationPlan, 0)}
	if !remaining.IsPositive() {
		return plan, ErrWithdrawalInvalidAmount
	}
	for _, entitlement := range entitlements {
		if !remaining.IsPositive() {
			break
		}
		if entitlement.AvailableAt.After(asOf) || !entitlement.Remaining.IsPositive() {
			continue
		}
		next := decimalMin(entitlement.Remaining, remaining)
		next = clampDecimalScale(next)
		plan.Allocations = append(plan.Allocations, withdrawableAllocationPlan{
			EntitlementID: entitlement.ID,
			Amount:        next,
			AvailableAt:   entitlement.AvailableAt.UTC(),
		})
		plan.Total = plan.Total.Add(next)
		remaining = remaining.Sub(next)
	}
	plan.Total = clampDecimalScale(plan.Total)
	if remaining.GreaterThan(decimal.RequireFromString("0.00000001")) {
		return plan, ErrWithdrawalInsufficientWithdrawable
	}
	return plan, nil
}

func withdrawalStatusAfterApproval(current string, amount, threshold decimal.Decimal, actorUserID int64, firstApprovedBy *int64) (string, error) {
	if actorUserID <= 0 {
		return "", ErrWithdrawalInvalidInput.WithMetadata(map[string]string{"field": "actor_user_id"})
	}
	switch current {
	case WithdrawalStatusPendingReview:
		if amount.GreaterThanOrEqual(threshold) {
			return WithdrawalStatusSecondReview, nil
		}
		return WithdrawalStatusPayoutPending, nil
	case WithdrawalStatusSecondReview:
		if firstApprovedBy == nil {
			return "", ErrWithdrawalInvalidStatus
		}
		if *firstApprovedBy == actorUserID {
			return "", ErrWithdrawalSelfReviewForbidden
		}
		return WithdrawalStatusPayoutPending, nil
	default:
		return "", ErrWithdrawalInvalidStatus
	}
}

func maskWithdrawalPayoutAccount(method string, details map[string]string) withdrawalPayoutMask {
	account := firstNonEmpty(
		details["account"],
		details["account_no"],
		details["card_number"],
		details["iban"],
		details["wallet"],
		details["address"],
	)
	recipient := firstNonEmpty(details["recipient_name"], details["name"], details["holder_name"])
	method = strings.ToLower(strings.TrimSpace(method))
	prefix := method
	if method == WithdrawalPayoutMethodAlipay {
		prefix = "alipay"
	}
	if method == WithdrawalPayoutMethodBankTransfer {
		prefix = firstNonEmpty(details["bank_name"], "bank")
	}
	if account == "" {
		account = prefix
	}
	return withdrawalPayoutMask{
		AccountMask:       prefix + ":" + maskSensitiveText(account),
		RecipientNameMask: maskSensitiveText(recipient),
	}
}

func maskSensitiveText(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	switch len(runes) {
	case 0:
		return ""
	case 1:
		return "*"
	case 2:
		return string(runes[:1]) + "*"
	default:
		head := string(runes[:1])
		tail := string(runes[len(runes)-1:])
		if len(runes) >= 8 {
			head = string(runes[:3])
			tail = string(runes[len(runes)-3:])
		}
		return head + "***" + tail
	}
}

func (s *WithdrawalService) GetAvailability(ctx context.Context, userID int64) (*WithdrawalAvailability, error) {
	if s == nil || s.db == nil {
		return nil, ErrWithdrawalUnavailable
	}
	settings, err := s.getSystemSettings(ctx, s.db)
	if err != nil {
		return nil, err
	}
	userSettings, err := s.getUserSettings(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	dailyUsed, err := s.getDailyUsed(ctx, s.db, userID, s.now().UTC())
	if err != nil {
		return nil, err
	}
	minimum := settings.MinimumAmount
	if userSettings.MinimumAmountOverride != nil {
		minimum = *userSettings.MinimumAmountOverride
	}
	dailyLimit := settings.DailyLimitAmount
	if userSettings.DailyLimitAmountOverride != nil {
		dailyLimit = *userSettings.DailyLimitAmountOverride
	}
	remaining := dailyLimit.Sub(dailyUsed)
	if remaining.IsNegative() {
		remaining = decimal.Zero
	}
	disabledReason := ""
	canApply := true
	if !settings.GlobalEnabled {
		canApply = false
		disabledReason = "global_disabled"
	} else if !userSettings.Enabled {
		canApply = false
		disabledReason = "user_disabled"
	} else if userSettings.RecalcStatus != WithdrawableRecomputeStatusReady {
		canApply = false
		disabledReason = "needs_review"
	} else if userSettings.DisabledReason != "" {
		canApply = false
		disabledReason = userSettings.DisabledReason
	}
	return &WithdrawalAvailability{
		GlobalEnabled:         settings.GlobalEnabled,
		UserEnabled:           userSettings.Enabled,
		CanApply:              canApply,
		DisabledReason:        disabledReason,
		RecalcStatus:          userSettings.RecalcStatus,
		MinimumAmount:         minimum,
		DailyLimitAmount:      dailyLimit,
		DailyUsedAmount:       dailyUsed,
		RemainingDailyAmount:  remaining,
		DoubleReviewThreshold: settings.DoubleReviewThreshold,
	}, nil
}

func (s *WithdrawalService) GetCurrentPayoutAccount(ctx context.Context, userID int64) (*WithdrawalPayoutAccount, error) {
	if s == nil || s.db == nil {
		return nil, ErrWithdrawalUnavailable
	}
	return s.getCurrentPayoutAccount(ctx, s.db, userID)
}

func (s *WithdrawalService) UpsertPayoutAccount(ctx context.Context, input WithdrawalPayoutAccountInput) (*WithdrawalPayoutAccount, error) {
	if s == nil || s.db == nil || s.encryptor == nil {
		return nil, ErrWithdrawalUnavailable
	}
	input.Method = strings.ToLower(strings.TrimSpace(input.Method))
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.RecipientName = strings.TrimSpace(input.RecipientName)
	if input.UserID <= 0 || !isValidWithdrawalPayoutMethod(input.Method) || !isValidWithdrawalCurrency(input.Currency) || len(input.Details) == 0 {
		return nil, ErrWithdrawalInvalidInput
	}
	if input.RecipientName != "" {
		input.Details["recipient_name"] = input.RecipientName
	}
	mask := maskWithdrawalPayoutAccount(input.Method, input.Details)
	if strings.TrimSpace(mask.AccountMask) == "" {
		return nil, ErrWithdrawalInvalidInput.WithMetadata(map[string]string{"field": "details"})
	}
	plain, err := json.Marshal(input.Details)
	if err != nil {
		return nil, ErrWithdrawalInvalidInput.WithCause(err)
	}
	encrypted, err := s.encryptor.Encrypt(string(plain))
	if err != nil {
		return nil, ErrWithdrawalAccountEncryption.WithCause(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin withdrawal account tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `
UPDATE withdrawal_payout_accounts
SET is_current = FALSE, updated_at = NOW()
WHERE user_id = $1 AND is_current`, input.UserID); err != nil {
		return nil, fmt.Errorf("disable old withdrawal payout account: %w", err)
	}
	account, err := queryWithdrawalPayoutAccount(ctx, tx, `
INSERT INTO withdrawal_payout_accounts (
	user_id, method, currency, recipient_name_mask, account_mask, account_encrypted, is_current, created_at, updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, TRUE, NOW(), NOW()
)
RETURNING id, user_id, method, currency, recipient_name_mask, account_mask, account_encrypted, created_at, updated_at`,
		input.UserID, input.Method, input.Currency, mask.RecipientNameMask, mask.AccountMask, encrypted)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit withdrawal payout account tx: %w", err)
	}
	committed = true
	return account, nil
}

func (s *WithdrawalService) CreateWithdrawal(ctx context.Context, input WithdrawalCreateInput) (*WithdrawalRequest, error) {
	if s == nil || s.db == nil || s.ledger == nil || s.encryptor == nil {
		return nil, ErrWithdrawalUnavailable
	}
	amount, err := parseWithdrawalAmount(input.Amount)
	if err != nil {
		return nil, err
	}
	availability, err := s.GetAvailability(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if !availability.CanApply {
		return nil, ErrWithdrawalDisabled.WithMetadata(map[string]string{"reason": availability.DisabledReason})
	}
	if amount.LessThan(availability.MinimumAmount) {
		return nil, ErrWithdrawalMinimumAmount
	}
	if amount.GreaterThan(availability.RemainingDailyAmount) {
		return nil, ErrWithdrawalDailyLimitExceeded
	}
	account, err := s.getCurrentPayoutAccount(ctx, s.db, input.UserID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrWithdrawalAccountRequired
	}
	snapshotEncrypted, err := s.buildAccountSnapshot(ctx, account)
	if err != nil {
		return nil, err
	}
	requestNo, err := newWithdrawalRequestNo()
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin withdrawal create tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if inProgress, err := s.hasInProgressWithdrawal(ctx, tx, input.UserID); err != nil {
		return nil, err
	} else if inProgress {
		return nil, ErrWithdrawalInProgress
	}
	now := s.now().UTC()
	req, err := queryWithdrawalRequest(ctx, tx, `
INSERT INTO withdrawal_requests (
	request_no, user_id, amount, currency, status, payout_method, payout_currency,
	payout_account_mask, payout_recipient_name_mask, account_snapshot_encrypted, created_at, updated_at
) VALUES (
	$1, $2, $3, 'USD', 'pending_review', $4, $5, $6, $7, $8, $9, NOW()
)
RETURNING `+withdrawalRequestSelectColumns+`
`, requestNo, input.UserID, decimalString(amount), account.Method, account.Currency, account.AccountMask, account.RecipientNameMask, snapshotEncrypted, now)
	if err != nil {
		return nil, err
	}
	txID, err := s.freezeWithdrawalFunds(ctx, tx, req, amount, now)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE withdrawal_requests
SET submit_balance_transaction_id = $1, updated_at = NOW()
WHERE id = $2`, txID, req.ID); err != nil {
		return nil, fmt.Errorf("update withdrawal submit transaction: %w", err)
	}
	if err := insertWithdrawalStatusEvent(ctx, tx, req.ID, WithdrawalStatusPendingReview, withdrawalActorUser, &input.UserID, "", nil, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit withdrawal create tx: %w", err)
	}
	committed = true
	req, _ = s.GetWithdrawal(ctx, req.ID, input.UserID, false)
	s.notifyWithdrawalAsync(ctx, NotificationEmailEventWithdrawalSubmitted, req, input.Locale)
	return req, nil
}

func (s *WithdrawalService) CancelWithdrawal(ctx context.Context, input WithdrawalActionInput) (*WithdrawalRequest, error) {
	if s == nil || s.db == nil || s.ledger == nil {
		return nil, ErrWithdrawalUnavailable
	}
	tx, req, err := s.beginAndLockWithdrawal(ctx, input.RequestID, input.UserID, true)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if req.Status != WithdrawalStatusPendingReview && req.Status != WithdrawalStatusSecondReview {
		return nil, ErrWithdrawalInvalidStatus
	}
	now := s.now().UTC()
	txID, err := s.restoreWithdrawalFunds(ctx, tx, req, WithdrawalLedgerSourceCancel, BalanceLedgerActorUser, &input.UserID, now)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE withdrawal_requests
SET status = 'canceled', close_balance_transaction_id = $1, canceled_at = $2, updated_at = NOW()
WHERE id = $3`, txID, now, req.ID); err != nil {
		return nil, fmt.Errorf("cancel withdrawal request: %w", err)
	}
	if err := insertWithdrawalStatusEvent(ctx, tx, req.ID, WithdrawalStatusCanceled, withdrawalActorUser, &input.UserID, input.Note, nil, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit withdrawal cancel tx: %w", err)
	}
	committed = true
	req, _ = s.GetWithdrawal(ctx, req.ID, input.UserID, false)
	s.notifyWithdrawalAsync(ctx, NotificationEmailEventWithdrawalCanceled, req, input.Locale)
	return req, nil
}

func (s *WithdrawalService) GetWithdrawal(ctx context.Context, requestID int64, requesterUserID int64, adminView bool) (*WithdrawalRequest, error) {
	if s == nil || s.db == nil {
		return nil, ErrWithdrawalUnavailable
	}
	query := `SELECT ` + withdrawalRequestSelectColumns + `
FROM withdrawal_requests wr
JOIN users u ON u.id = wr.user_id
WHERE wr.id = $1`
	args := []any{requestID}
	if !adminView {
		query += ` AND wr.user_id = $2`
		args = append(args, requesterUserID)
	}
	req, err := queryWithdrawalRequest(ctx, s.db, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWithdrawalNotFound
	}
	if err != nil {
		return nil, err
	}
	req.Events, _ = listWithdrawalStatusEvents(ctx, s.db, req.ID)
	req.Entitlements, _ = listWithdrawalEntitlementLocks(ctx, s.db, req.ID)
	return req, nil
}

func (s *WithdrawalService) ListUserWithdrawals(ctx context.Context, userID int64, query WithdrawalListQuery) (*WithdrawalRequestPage, error) {
	query.UserID = userID
	return s.listWithdrawals(ctx, query, false)
}

func (s *WithdrawalService) AdminListWithdrawals(ctx context.Context, query WithdrawalListQuery) (*WithdrawalRequestPage, error) {
	return s.listWithdrawals(ctx, query, true)
}

func (s *WithdrawalService) AdminApprove(ctx context.Context, input WithdrawalActionInput) (*WithdrawalRequest, error) {
	tx, req, err := s.beginAndLockWithdrawal(ctx, input.RequestID, 0, false)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if input.ActorUserID == req.UserID {
		return nil, ErrWithdrawalSelfReviewForbidden
	}
	settings, err := s.getSystemSettings(ctx, tx)
	if err != nil {
		return nil, err
	}
	nextStatus, err := withdrawalStatusAfterApproval(req.Status, req.Amount, settings.DoubleReviewThreshold, input.ActorUserID, req.FirstApprovedBy)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	switch req.Status {
	case WithdrawalStatusPendingReview:
		if nextStatus == WithdrawalStatusSecondReview {
			_, err = tx.ExecContext(ctx, `
UPDATE withdrawal_requests
SET status = $1, first_approved_by = $2, first_approved_at = $3, updated_at = NOW()
WHERE id = $4`, nextStatus, input.ActorUserID, now, req.ID)
		} else {
			_, err = tx.ExecContext(ctx, `
UPDATE withdrawal_requests
SET status = $1, first_approved_by = $2, first_approved_at = $3, updated_at = NOW()
WHERE id = $4`, nextStatus, input.ActorUserID, now, req.ID)
		}
	case WithdrawalStatusSecondReview:
		_, err = tx.ExecContext(ctx, `
UPDATE withdrawal_requests
SET status = $1, second_approved_by = $2, second_approved_at = $3, updated_at = NOW()
WHERE id = $4`, nextStatus, input.ActorUserID, now, req.ID)
	default:
		err = ErrWithdrawalInvalidStatus
	}
	if err != nil {
		return nil, fmt.Errorf("approve withdrawal request: %w", err)
	}
	if err := insertWithdrawalStatusEvent(ctx, tx, req.ID, nextStatus, withdrawalActorAdmin, &input.ActorUserID, input.Note, nil, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit withdrawal approve tx: %w", err)
	}
	committed = true
	req, _ = s.GetWithdrawal(ctx, req.ID, 0, true)
	if req.Status == WithdrawalStatusPayoutPending {
		s.notifyWithdrawalAsync(ctx, NotificationEmailEventWithdrawalApproved, req, input.Locale)
	}
	return req, nil
}

func (s *WithdrawalService) AdminReject(ctx context.Context, input WithdrawalActionInput) (*WithdrawalRequest, error) {
	if strings.TrimSpace(input.Reason) == "" || len([]rune(input.Reason)) > 500 {
		return nil, ErrWithdrawalInvalidInput.WithMetadata(map[string]string{"field": "reason"})
	}
	tx, req, err := s.beginAndLockWithdrawal(ctx, input.RequestID, 0, false)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if input.ActorUserID == req.UserID {
		return nil, ErrWithdrawalSelfReviewForbidden
	}
	if req.Status != WithdrawalStatusPendingReview && req.Status != WithdrawalStatusSecondReview {
		return nil, ErrWithdrawalInvalidStatus
	}
	now := s.now().UTC()
	txID, err := s.restoreWithdrawalFunds(ctx, tx, req, WithdrawalLedgerSourceReject, BalanceLedgerActorAdmin, &input.ActorUserID, now)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE withdrawal_requests
SET status = 'rejected', close_balance_transaction_id = $1, rejected_by = $2, rejected_at = $3,
    rejected_reason = $4, updated_at = NOW()
WHERE id = $5`, txID, input.ActorUserID, now, strings.TrimSpace(input.Reason), req.ID); err != nil {
		return nil, fmt.Errorf("reject withdrawal request: %w", err)
	}
	if err := insertWithdrawalStatusEvent(ctx, tx, req.ID, WithdrawalStatusRejected, withdrawalActorAdmin, &input.ActorUserID, input.Reason, nil, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit withdrawal reject tx: %w", err)
	}
	committed = true
	req, _ = s.GetWithdrawal(ctx, req.ID, 0, true)
	s.notifyWithdrawalAsync(ctx, NotificationEmailEventWithdrawalRejected, req, input.Locale)
	return req, nil
}

func (s *WithdrawalService) AdminMarkPaid(ctx context.Context, input WithdrawalMarkPaidInput) (*WithdrawalRequest, error) {
	if s == nil || s.db == nil || s.ledger == nil {
		return nil, ErrWithdrawalUnavailable
	}
	paidAmount, err := parseWithdrawalAmount(input.PaidAmount)
	if err != nil {
		return nil, err
	}
	fxRate, err := parseWithdrawalPositiveDecimal(input.FXRate, "payout_fx_rate")
	if err != nil {
		return nil, err
	}
	input.PaidCurrency = strings.ToUpper(strings.TrimSpace(input.PaidCurrency))
	if !isValidWithdrawalCurrency(input.PaidCurrency) {
		return nil, ErrWithdrawalInvalidInput.WithMetadata(map[string]string{"field": "paid_currency"})
	}
	tx, req, err := s.beginAndLockWithdrawal(ctx, input.RequestID, 0, false)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if input.ActorUserID == req.UserID {
		return nil, ErrWithdrawalSelfReviewForbidden
	}
	if req.Status != WithdrawalStatusPayoutPending {
		return nil, ErrWithdrawalInvalidStatus
	}
	now := s.now().UTC()
	paidAt := now
	if input.PaidAt != nil {
		paidAt = input.PaidAt.UTC()
	}
	ledgerTx, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{
		UserID:                             req.UserID,
		WithdrawalFrozenDelta:              -decimalToLedgerFloat(req.Amount),
		SourceType:                         WithdrawalLedgerSourcePaid,
		SourceID:                           req.RequestNo,
		IdempotencyKey:                     "withdrawal_paid:" + req.RequestNo,
		ActorType:                          BalanceLedgerActorAdmin,
		ActorUserID:                        &input.ActorUserID,
		Description:                        "withdrawal marked paid",
		SkipWithdrawableEntitlementEffects: true,
	})
	if err != nil {
		return nil, err
	}
	if err := applyWithdrawalEntitlementPayoutConsume(ctx, tx, req, ledgerTx.ID, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE withdrawal_requests
SET status = 'paid', close_balance_transaction_id = $1, paid_by = $2, paid_at = $3,
    paid_amount = $4, paid_currency = $5, payout_fx_rate = $6, external_txn_id = $7,
    payout_note = $8, updated_at = NOW()
WHERE id = $9`, ledgerTx.ID, input.ActorUserID, paidAt, decimalString(paidAmount), input.PaidCurrency, decimalString(fxRate), strings.TrimSpace(input.ExternalTxnID), strings.TrimSpace(input.Note), req.ID); err != nil {
		return nil, fmt.Errorf("mark withdrawal paid: %w", err)
	}
	if err := insertWithdrawalStatusEvent(ctx, tx, req.ID, WithdrawalStatusPaid, withdrawalActorAdmin, &input.ActorUserID, input.Note, map[string]any{
		"external_txn_id": strings.TrimSpace(input.ExternalTxnID),
		"paid_currency":   input.PaidCurrency,
	}, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit withdrawal paid tx: %w", err)
	}
	committed = true
	req, _ = s.GetWithdrawal(ctx, req.ID, 0, true)
	s.notifyWithdrawalAsync(ctx, NotificationEmailEventWithdrawalPaid, req, input.Locale)
	return req, nil
}

func (s *WithdrawalService) AdminGetSensitivePayoutSnapshot(ctx context.Context, requestID int64) (map[string]any, error) {
	if s == nil || s.db == nil || s.encryptor == nil {
		return nil, ErrWithdrawalUnavailable
	}
	req, err := s.GetWithdrawal(ctx, requestID, 0, true)
	if err != nil {
		return nil, err
	}
	plain, err := s.encryptor.Decrypt(req.accountSnapshotEncrypted)
	if err != nil {
		return nil, ErrWithdrawalAccountEncryption.WithCause(err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return nil, ErrWithdrawalAccountEncryption.WithCause(err)
	}
	return out, nil
}

func (s *WithdrawalService) GetSystemSettings(ctx context.Context) (*WithdrawalSystemSettings, error) {
	return s.getSystemSettings(ctx, s.db)
}

func (s *WithdrawalService) UpdateSystemSettings(ctx context.Context, input WithdrawalSystemSettingsUpdate) (*WithdrawalSystemSettings, error) {
	if s == nil || s.db == nil {
		return nil, ErrWithdrawalUnavailable
	}
	current, err := s.getSystemSettings(ctx, s.db)
	if err != nil {
		return nil, err
	}
	enabled := current.GlobalEnabled
	if input.GlobalEnabled != nil {
		enabled = *input.GlobalEnabled
	}
	minimum := current.MinimumAmount
	if input.MinimumAmount != nil {
		next, err := parseWithdrawalAmount(*input.MinimumAmount)
		if err != nil {
			return nil, err
		}
		minimum = next
	}
	daily := current.DailyLimitAmount
	if input.DailyLimitAmount != nil {
		next, err := parseWithdrawalAmount(*input.DailyLimitAmount)
		if err != nil {
			return nil, err
		}
		daily = next
	}
	threshold := current.DoubleReviewThreshold
	if input.DoubleReviewThreshold != nil {
		next, err := parseWithdrawalAmount(*input.DoubleReviewThreshold)
		if err != nil {
			return nil, err
		}
		threshold = next
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO withdrawal_system_settings (
	id, global_enabled, minimum_amount, daily_limit_amount, double_review_threshold, reward_maturity_hours, updated_by, updated_at
) VALUES (1, $1, $2, $3, $4, 72, $5, NOW())
ON CONFLICT (id) DO UPDATE
SET global_enabled = EXCLUDED.global_enabled,
    minimum_amount = EXCLUDED.minimum_amount,
    daily_limit_amount = EXCLUDED.daily_limit_amount,
    double_review_threshold = EXCLUDED.double_review_threshold,
    reward_maturity_hours = 72,
    updated_by = EXCLUDED.updated_by,
    updated_at = NOW()`, enabled, decimalString(minimum), decimalString(daily), decimalString(threshold), nullableInt64(input.ActorUserID)); err != nil {
		return nil, fmt.Errorf("update withdrawal system settings: %w", err)
	}
	return s.getSystemSettings(ctx, s.db)
}

func (s *WithdrawalService) GetUserSettings(ctx context.Context, userID int64) (*UserWithdrawalSettings, error) {
	return s.getUserSettings(ctx, s.db, userID)
}

func (s *WithdrawalService) UpdateUserSettings(ctx context.Context, input UserWithdrawalSettingsUpdate) (*UserWithdrawalSettings, error) {
	if s == nil || s.db == nil {
		return nil, ErrWithdrawalUnavailable
	}
	if input.UserID <= 0 {
		return nil, ErrWithdrawalInvalidInput
	}
	if input.Enabled != nil && *input.Enabled {
		ready, err := s.userWithdrawalReady(ctx, s.db, input.UserID)
		if err != nil {
			return nil, err
		}
		if !ready {
			return nil, ErrWithdrawalUserNotReady
		}
	}
	minimum, err := parseOptionalWithdrawalAmount(input.MinimumAmountOverride, "minimum_amount_override")
	if err != nil {
		return nil, err
	}
	daily, err := parseOptionalWithdrawalAmount(input.DailyLimitAmountOverride, "daily_limit_amount_override")
	if err != nil {
		return nil, err
	}
	current, err := s.getUserSettings(ctx, s.db, input.UserID)
	if err != nil {
		return nil, err
	}
	enabled := current.Enabled
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	disabledReason := current.DisabledReason
	if input.DisabledReason != nil {
		disabledReason = strings.TrimSpace(*input.DisabledReason)
	}
	if input.MinimumAmountOverride == nil {
		minimum = current.MinimumAmountOverride
	}
	if input.DailyLimitAmountOverride == nil {
		daily = current.DailyLimitAmountOverride
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO user_withdrawal_settings (
	user_id, enabled, minimum_amount_override, daily_limit_amount_override, disabled_reason, updated_by, updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, NOW()
)
ON CONFLICT (user_id) DO UPDATE
SET enabled = EXCLUDED.enabled,
    minimum_amount_override = EXCLUDED.minimum_amount_override,
    daily_limit_amount_override = EXCLUDED.daily_limit_amount_override,
    disabled_reason = EXCLUDED.disabled_reason,
    updated_by = EXCLUDED.updated_by,
    updated_at = NOW()`, input.UserID, enabled, nullableDecimal(minimum), nullableDecimal(daily), disabledReason, nullableInt64(input.ActorUserID)); err != nil {
		return nil, fmt.Errorf("update user withdrawal settings: %w", err)
	}
	return s.getUserSettings(ctx, s.db, input.UserID)
}

func (s *WithdrawalService) BatchUpdateUserSettings(ctx context.Context, input BatchUserWithdrawalSettingsUpdate) (int, error) {
	if s == nil || s.db == nil {
		return 0, ErrWithdrawalUnavailable
	}
	count := 0
	for _, userID := range input.UserIDs {
		if userID <= 0 {
			continue
		}
		_, err := s.UpdateUserSettings(ctx, UserWithdrawalSettingsUpdate{
			UserID:                   userID,
			Enabled:                  input.Enabled,
			MinimumAmountOverride:    input.MinimumAmountOverride,
			DailyLimitAmountOverride: input.DailyLimitAmountOverride,
			DisabledReason:           input.DisabledReason,
			ActorUserID:              input.ActorUserID,
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *WithdrawalService) listWithdrawals(ctx context.Context, query WithdrawalListQuery, adminView bool) (*WithdrawalRequestPage, error) {
	if s == nil || s.db == nil {
		return nil, ErrWithdrawalUnavailable
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
	status := strings.TrimSpace(query.Status)
	where := []string{"1=1"}
	args := []any{}
	if !adminView || query.UserID > 0 {
		args = append(args, query.UserID)
		where = append(where, fmt.Sprintf("wr.user_id = $%d", len(args)))
	}
	if status != "" && status != "all" {
		if !isValidWithdrawalStatus(status) {
			return nil, ErrWithdrawalInvalidInput.WithMetadata(map[string]string{"field": "status"})
		}
		args = append(args, status)
		where = append(where, fmt.Sprintf("wr.status = $%d", len(args)))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)::bigint FROM withdrawal_requests wr WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count withdrawals: %w", err)
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT `+withdrawalRequestSelectColumns+`
FROM withdrawal_requests wr
JOIN users u ON u.id = wr.user_id
WHERE `+whereSQL+`
ORDER BY wr.created_at DESC, wr.id DESC
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]WithdrawalRequest, 0, pageSize)
	for rows.Next() {
		item, err := scanWithdrawalRequest(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return &WithdrawalRequestPage{Items: items, Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func (s *WithdrawalService) getSystemSettings(ctx context.Context, runner withdrawalSQLRunner) (*WithdrawalSystemSettings, error) {
	if s == nil || runner == nil {
		return nil, ErrWithdrawalUnavailable
	}
	row := runner.QueryRowContext(ctx, `
SELECT global_enabled, minimum_amount::text, daily_limit_amount::text, double_review_threshold::text,
       reward_maturity_hours, updated_by, updated_at
FROM withdrawal_system_settings
WHERE id = 1`)
	var settings WithdrawalSystemSettings
	var minRaw, dailyRaw, thresholdRaw string
	var updatedBy sql.NullInt64
	if err := row.Scan(&settings.GlobalEnabled, &minRaw, &dailyRaw, &thresholdRaw, &settings.RewardMaturityHours, &updatedBy, &settings.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &WithdrawalSystemSettings{
				GlobalEnabled:         false,
				MinimumAmount:         decimal.RequireFromString("10.00000000"),
				DailyLimitAmount:      decimal.RequireFromString("500.00000000"),
				DoubleReviewThreshold: decimal.RequireFromString("100.00000000"),
				RewardMaturityHours:   72,
				UpdatedAt:             time.Time{},
			}, nil
		}
		return nil, fmt.Errorf("get withdrawal system settings: %w", err)
	}
	var err error
	if settings.MinimumAmount, err = parseLedgerDecimal(minRaw, "withdrawal minimum amount"); err != nil {
		return nil, err
	}
	if settings.DailyLimitAmount, err = parseLedgerDecimal(dailyRaw, "withdrawal daily limit"); err != nil {
		return nil, err
	}
	if settings.DoubleReviewThreshold, err = parseLedgerDecimal(thresholdRaw, "withdrawal double review threshold"); err != nil {
		return nil, err
	}
	if updatedBy.Valid {
		settings.UpdatedBy = &updatedBy.Int64
	}
	return &settings, nil
}

func (s *WithdrawalService) getUserSettings(ctx context.Context, runner withdrawalSQLRunner, userID int64) (*UserWithdrawalSettings, error) {
	row := runner.QueryRowContext(ctx, `
SELECT COALESCE(uws.enabled, FALSE),
       uws.minimum_amount_override::text,
       uws.daily_limit_amount_override::text,
       COALESCE(uws.disabled_reason, ''),
       uws.updated_by,
       COALESCE(uws.updated_at, u.updated_at),
       COALESCE(u.withdrawal_recalc_status, 'needs_review')
FROM users u
LEFT JOIN user_withdrawal_settings uws ON uws.user_id = u.id
WHERE u.id = $1 AND u.deleted_at IS NULL`, userID)
	var settings UserWithdrawalSettings
	var minRaw, dailyRaw sql.NullString
	var updatedBy sql.NullInt64
	if err := row.Scan(&settings.Enabled, &minRaw, &dailyRaw, &settings.DisabledReason, &updatedBy, &settings.UpdatedAt, &settings.RecalcStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user withdrawal settings: %w", err)
	}
	settings.UserID = userID
	if minRaw.Valid && strings.TrimSpace(minRaw.String) != "" {
		value, err := parseLedgerDecimal(minRaw.String, "minimum override")
		if err != nil {
			return nil, err
		}
		settings.MinimumAmountOverride = &value
	}
	if dailyRaw.Valid && strings.TrimSpace(dailyRaw.String) != "" {
		value, err := parseLedgerDecimal(dailyRaw.String, "daily override")
		if err != nil {
			return nil, err
		}
		settings.DailyLimitAmountOverride = &value
	}
	if updatedBy.Valid {
		settings.UpdatedBy = &updatedBy.Int64
	}
	return &settings, nil
}

func (s *WithdrawalService) getDailyUsed(ctx context.Context, runner withdrawalSQLRunner, userID int64, now time.Time) (decimal.Decimal, error) {
	loc := shanghaiLocation()
	localNow := now.In(loc)
	startLocal := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
	endLocal := startLocal.Add(24 * time.Hour)
	var raw string
	if err := runner.QueryRowContext(ctx, `
SELECT COALESCE(SUM(amount), 0)::text
FROM withdrawal_requests
WHERE user_id = $1
  AND status NOT IN ('rejected', 'canceled')
  AND created_at >= $2
  AND created_at < $3`, userID, startLocal.UTC(), endLocal.UTC()).Scan(&raw); err != nil {
		return decimal.Zero, fmt.Errorf("get withdrawal daily used: %w", err)
	}
	return parseLedgerDecimal(raw, "withdrawal daily used")
}

func (s *WithdrawalService) getCurrentPayoutAccount(ctx context.Context, runner withdrawalSQLRunner, userID int64) (*WithdrawalPayoutAccount, error) {
	account, err := queryWithdrawalPayoutAccount(ctx, runner, `
SELECT id, user_id, method, currency, recipient_name_mask, account_mask, account_encrypted, created_at, updated_at
FROM withdrawal_payout_accounts
WHERE user_id = $1 AND is_current
ORDER BY id DESC
LIMIT 1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return account, err
}

func (s *WithdrawalService) buildAccountSnapshot(ctx context.Context, account *WithdrawalPayoutAccount) (string, error) {
	plainDetails, err := s.encryptor.Decrypt(account.accountEncrypted)
	if err != nil {
		return "", ErrWithdrawalAccountEncryption.WithCause(err)
	}
	snapshot := map[string]any{
		"account_id":          account.ID,
		"method":              account.Method,
		"currency":            account.Currency,
		"recipient_name_mask": account.RecipientNameMask,
		"account_mask":        account.AccountMask,
		"details":             json.RawMessage(plainDetails),
		"snapshot_at":         s.now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return "", ErrWithdrawalInvalidInput.WithCause(err)
	}
	encrypted, err := s.encryptor.Encrypt(string(raw))
	if err != nil {
		return "", ErrWithdrawalAccountEncryption.WithCause(err)
	}
	_ = ctx
	return encrypted, nil
}

func (s *WithdrawalService) hasInProgressWithdrawal(ctx context.Context, runner withdrawalSQLRunner, userID int64) (bool, error) {
	rows, err := runner.QueryContext(ctx, `
SELECT id
FROM withdrawal_requests
WHERE user_id = $1 AND status IN ('pending_review', 'second_review', 'payout_pending')
LIMIT 1
FOR UPDATE`, userID)
	if err != nil {
		return false, fmt.Errorf("lock in-progress withdrawal: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return rows.Next(), rows.Err()
}

func (s *WithdrawalService) freezeWithdrawalFunds(ctx context.Context, tx *sql.Tx, req *WithdrawalRequest, amount decimal.Decimal, createdAt time.Time) (int64, error) {
	ledgerTx, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{
		UserID:                             req.UserID,
		BalanceDelta:                       -decimalToLedgerFloat(amount),
		WithdrawableDelta:                  -decimalToLedgerFloat(amount),
		WithdrawalFrozenDelta:              decimalToLedgerFloat(amount),
		SourceType:                         WithdrawalLedgerSourceSubmit,
		SourceID:                           req.RequestNo,
		IdempotencyKey:                     "withdrawal_submit:" + req.RequestNo,
		ActorType:                          BalanceLedgerActorUser,
		ActorUserID:                        &req.UserID,
		Description:                        "withdrawal request submitted",
		SkipWithdrawableEntitlementEffects: true,
		CreatedAt:                          &createdAt,
	})
	if err != nil {
		if errors.Is(err, ErrBalanceLedgerInsufficientBalance) {
			return 0, ErrWithdrawalInsufficientWithdrawable
		}
		return 0, err
	}
	entitlements, err := selectWithdrawableEntitlementSnapshots(ctx, tx, req.UserID)
	if err != nil {
		return 0, fmt.Errorf("select withdrawal freeze entitlements: %w", err)
	}
	plan, err := planWithdrawalEntitlementFreeze(amount, entitlements, createdAt)
	if err != nil {
		return 0, err
	}
	if err := applyWithdrawalEntitlementFreeze(ctx, tx, req, ledgerTx.ID, plan, createdAt); err != nil {
		return 0, err
	}
	return ledgerTx.ID, nil
}

func (s *WithdrawalService) restoreWithdrawalFunds(ctx context.Context, tx *sql.Tx, req *WithdrawalRequest, sourceType, actorType string, actorUserID *int64, createdAt time.Time) (int64, error) {
	ledgerTx, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{
		UserID:                             req.UserID,
		BalanceDelta:                       decimalToLedgerFloat(req.Amount),
		WithdrawableDelta:                  decimalToLedgerFloat(req.Amount),
		WithdrawalFrozenDelta:              -decimalToLedgerFloat(req.Amount),
		SourceType:                         sourceType,
		SourceID:                           req.RequestNo,
		IdempotencyKey:                     sourceType + ":" + req.RequestNo,
		ActorType:                          actorType,
		ActorUserID:                        actorUserID,
		Description:                        "withdrawal request restored",
		SkipWithdrawableEntitlementEffects: true,
		CreatedAt:                          &createdAt,
	})
	if err != nil {
		return 0, err
	}
	if err := applyWithdrawalEntitlementUnfreeze(ctx, tx, req, ledgerTx.ID, sourceType, createdAt); err != nil {
		return 0, err
	}
	return ledgerTx.ID, nil
}

func (s *WithdrawalService) beginAndLockWithdrawal(ctx context.Context, requestID int64, userID int64, requireUser bool) (*sql.Tx, *WithdrawalRequest, error) {
	if s == nil || s.db == nil {
		return nil, nil, ErrWithdrawalUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin withdrawal tx: %w", err)
	}
	query := `SELECT ` + withdrawalRequestSelectColumns + `
FROM withdrawal_requests wr
JOIN users u ON u.id = wr.user_id
WHERE wr.id = $1`
	args := []any{requestID}
	if requireUser {
		query += ` AND wr.user_id = $2`
		args = append(args, userID)
	}
	query += ` FOR UPDATE OF wr`
	req, err := queryWithdrawalRequest(ctx, tx, query, args...)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrWithdrawalNotFound
		}
		return nil, nil, err
	}
	return tx, req, nil
}

func (s *WithdrawalService) userWithdrawalReady(ctx context.Context, runner withdrawalSQLRunner, userID int64) (bool, error) {
	var status string
	if err := runner.QueryRowContext(ctx, `
SELECT COALESCE(withdrawal_recalc_status, 'needs_review')
FROM users
WHERE id = $1 AND deleted_at IS NULL`, userID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrUserNotFound
		}
		return false, fmt.Errorf("check withdrawal user readiness: %w", err)
	}
	return status == WithdrawableRecomputeStatusReady, nil
}

func (s *WithdrawalService) notifyWithdrawalAsync(ctx context.Context, event string, req *WithdrawalRequest, locale string) {
	if s == nil || s.notification == nil || req == nil || strings.TrimSpace(req.UserEmail) == "" {
		return
	}
	input := NotificationEmailSendInput{
		Event:          event,
		Locale:         locale,
		RecipientEmail: req.UserEmail,
		RecipientName:  req.UserEmail,
		UserID:         req.UserID,
		SourceType:     "withdrawal_request",
		SourceID:       req.RequestNo,
		Variables: map[string]string{
			"withdrawal_id":     req.RequestNo,
			"withdrawal_amount": req.Amount.StringFixed(2),
			"withdrawal_status": withdrawalStatusEmailLabel(req.Status, locale),
			"payout_currency":   req.PayoutCurrency,
			"external_txn_id":   req.ExternalTxnID,
		},
	}
	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.notification.Send(sendCtx, input); err != nil {
			slog.Warn("withdrawal notification failed", "event", event, "request_no", req.RequestNo, "err", err)
		}
	}()
	_ = ctx
}

func withdrawalStatusEmailLabel(status, locale string) string {
	zh := normalizeNotificationLocale(locale) == notificationEmailLocaleChinese
	switch status {
	case WithdrawalStatusPendingReview:
		if zh {
			return "待审核"
		}
		return "Pending review"
	case WithdrawalStatusSecondReview:
		if zh {
			return "等待二次审核"
		}
		return "Second review"
	case WithdrawalStatusPayoutPending:
		if zh {
			return "待线下打款"
		}
		return "Payout pending"
	case WithdrawalStatusPaid:
		if zh {
			return "已打款"
		}
		return "Paid"
	case WithdrawalStatusRejected:
		if zh {
			return "已拒绝"
		}
		return "Rejected"
	case WithdrawalStatusCanceled:
		if zh {
			return "已取消"
		}
		return "Canceled"
	default:
		if zh {
			return "状态更新"
		}
		return "Updated"
	}
}

type withdrawalSQLRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

const withdrawalRequestSelectColumns = `
wr.id, wr.request_no, wr.user_id, COALESCE(u.email, '') AS user_email, wr.amount::text, wr.currency, wr.status,
wr.payout_method, wr.payout_currency, wr.payout_account_mask, wr.payout_recipient_name_mask,
wr.account_snapshot_encrypted, wr.first_approved_by, wr.first_approved_at, wr.second_approved_by,
wr.second_approved_at, wr.rejected_by, wr.rejected_at, wr.rejected_reason, wr.canceled_at,
wr.paid_by, wr.paid_at, wr.paid_amount::text, wr.paid_currency, wr.payout_fx_rate::text,
wr.external_txn_id, wr.external_fee_amount::text, wr.payout_note, wr.created_at, wr.updated_at`

func queryWithdrawalRequest(ctx context.Context, runner withdrawalSQLRunner, query string, args ...any) (*WithdrawalRequest, error) {
	rows, err := runner.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query withdrawal request: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	req, err := scanWithdrawalRequest(rows)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		return nil, ErrWithdrawalInvalidInput
	}
	return req, rows.Err()
}

func scanWithdrawalRequest(rows *sql.Rows) (*WithdrawalRequest, error) {
	var req WithdrawalRequest
	var amountRaw, externalFeeRaw string
	var paidAmountRaw, fxRaw sql.NullString
	var firstBy, secondBy, rejectedBy, paidBy sql.NullInt64
	var firstAt, secondAt, rejectedAt, canceledAt, paidAt sql.NullTime
	if err := rows.Scan(
		&req.ID,
		&req.RequestNo,
		&req.UserID,
		&req.UserEmail,
		&amountRaw,
		&req.Currency,
		&req.Status,
		&req.PayoutMethod,
		&req.PayoutCurrency,
		&req.PayoutAccountMask,
		&req.PayoutRecipientNameMask,
		&req.accountSnapshotEncrypted,
		&firstBy,
		&firstAt,
		&secondBy,
		&secondAt,
		&rejectedBy,
		&rejectedAt,
		&req.RejectedReason,
		&canceledAt,
		&paidBy,
		&paidAt,
		&paidAmountRaw,
		&req.PaidCurrency,
		&fxRaw,
		&req.ExternalTxnID,
		&externalFeeRaw,
		&req.PayoutNote,
		&req.CreatedAt,
		&req.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan withdrawal request: %w", err)
	}
	var err error
	if req.Amount, err = parseLedgerDecimal(amountRaw, "withdrawal amount"); err != nil {
		return nil, err
	}
	if req.ExternalFeeAmount, err = parseLedgerDecimal(externalFeeRaw, "external fee amount"); err != nil {
		return nil, err
	}
	if firstBy.Valid {
		req.FirstApprovedBy = &firstBy.Int64
	}
	if firstAt.Valid {
		req.FirstApprovedAt = &firstAt.Time
	}
	if secondBy.Valid {
		req.SecondApprovedBy = &secondBy.Int64
	}
	if secondAt.Valid {
		req.SecondApprovedAt = &secondAt.Time
	}
	if rejectedBy.Valid {
		req.RejectedBy = &rejectedBy.Int64
	}
	if rejectedAt.Valid {
		req.RejectedAt = &rejectedAt.Time
	}
	if canceledAt.Valid {
		req.CanceledAt = &canceledAt.Time
	}
	if paidBy.Valid {
		req.PaidBy = &paidBy.Int64
	}
	if paidAt.Valid {
		req.PaidAt = &paidAt.Time
	}
	if paidAmountRaw.Valid && strings.TrimSpace(paidAmountRaw.String) != "" {
		value, err := parseLedgerDecimal(paidAmountRaw.String, "paid amount")
		if err != nil {
			return nil, err
		}
		req.PaidAmount = &value
	}
	if fxRaw.Valid && strings.TrimSpace(fxRaw.String) != "" {
		value, err := parseLedgerDecimal(fxRaw.String, "payout fx rate")
		if err != nil {
			return nil, err
		}
		req.PayoutFXRate = &value
	}
	return &req, nil
}

func queryWithdrawalPayoutAccount(ctx context.Context, runner withdrawalSQLRunner, query string, args ...any) (*WithdrawalPayoutAccount, error) {
	rows, err := runner.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query withdrawal payout account: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	var account WithdrawalPayoutAccount
	if err := rows.Scan(&account.ID, &account.UserID, &account.Method, &account.Currency, &account.RecipientNameMask, &account.AccountMask, &account.accountEncrypted, &account.CreatedAt, &account.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan withdrawal payout account: %w", err)
	}
	if rows.Next() {
		return nil, ErrWithdrawalInvalidInput
	}
	return &account, rows.Err()
}

func applyWithdrawalEntitlementFreeze(ctx context.Context, runner withdrawalSQLRunner, req *WithdrawalRequest, transactionID int64, plan withdrawalEntitlementFreezePlan, createdAt time.Time) error {
	for _, allocation := range plan.Allocations {
		res, err := runner.ExecContext(ctx, `
UPDATE withdrawable_entitlements
SET remaining_amount = remaining_amount - $1::numeric,
    withdrawal_frozen_amount = withdrawal_frozen_amount + $1::numeric,
    status = 'active',
    updated_at = NOW()
WHERE id = $2
  AND user_id = $3
  AND status = 'active'
  AND remaining_amount >= $1::numeric
  AND available_at <= $4`, decimalString(allocation.Amount), allocation.EntitlementID, req.UserID, createdAt.UTC())
		if err != nil {
			return fmt.Errorf("freeze withdrawable entitlement: %w", err)
		}
		if err := requireRowsAffected(res, "freeze withdrawable entitlement"); err != nil {
			return err
		}
		if _, err := runner.ExecContext(ctx, `
INSERT INTO withdrawal_request_entitlements (
	withdrawal_request_id, entitlement_id, amount, available_at, created_at
) VALUES (
	$1, $2, $3, $4, $5
)`, req.ID, allocation.EntitlementID, decimalString(allocation.Amount), allocation.AvailableAt.UTC(), createdAt.UTC()); err != nil {
			return fmt.Errorf("insert withdrawal entitlement lock: %w", err)
		}
		if err := insertWithdrawableAllocation(ctx, runner, req.UserID, allocation.EntitlementID, transactionID, withdrawableAllocationFreeze, allocation.Amount, allocation.AvailableAt, WithdrawalLedgerSourceSubmit, req.RequestNo, map[string]any{
			"withdrawal_request_id": req.ID,
			"withdrawal_request_no": req.RequestNo,
		}, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func applyWithdrawalEntitlementUnfreeze(ctx context.Context, runner withdrawalSQLRunner, req *WithdrawalRequest, transactionID int64, sourceType string, createdAt time.Time) error {
	locks, err := listWithdrawalEntitlementLocks(ctx, runner, req.ID)
	if err != nil {
		return err
	}
	for _, allocation := range locks {
		res, err := runner.ExecContext(ctx, `
UPDATE withdrawable_entitlements
SET remaining_amount = remaining_amount + $1::numeric,
    withdrawal_frozen_amount = withdrawal_frozen_amount - $1::numeric,
    status = 'active',
    updated_at = NOW()
WHERE id = $2
  AND user_id = $3
  AND withdrawal_frozen_amount >= $1::numeric`, decimalString(allocation.Amount), allocation.EntitlementID, req.UserID)
		if err != nil {
			return fmt.Errorf("unfreeze withdrawable entitlement: %w", err)
		}
		if err := requireRowsAffected(res, "unfreeze withdrawable entitlement"); err != nil {
			return err
		}
		if err := insertWithdrawableAllocation(ctx, runner, req.UserID, allocation.EntitlementID, transactionID, withdrawableAllocationUnfreeze, allocation.Amount, allocation.AvailableAt, sourceType, req.RequestNo, map[string]any{
			"withdrawal_request_id": req.ID,
			"withdrawal_request_no": req.RequestNo,
		}, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func applyWithdrawalEntitlementPayoutConsume(ctx context.Context, runner withdrawalSQLRunner, req *WithdrawalRequest, transactionID int64, createdAt time.Time) error {
	locks, err := listWithdrawalEntitlementLocks(ctx, runner, req.ID)
	if err != nil {
		return err
	}
	for _, allocation := range locks {
		res, err := runner.ExecContext(ctx, `
UPDATE withdrawable_entitlements
SET withdrawal_frozen_amount = withdrawal_frozen_amount - $1::numeric,
    consumed_amount = consumed_amount + $1::numeric,
    status = CASE WHEN remaining_amount <= 0 AND withdrawal_frozen_amount - $1::numeric <= 0 THEN 'consumed' ELSE status END,
    updated_at = NOW()
WHERE id = $2
  AND user_id = $3
  AND withdrawal_frozen_amount >= $1::numeric`, decimalString(allocation.Amount), allocation.EntitlementID, req.UserID)
		if err != nil {
			return fmt.Errorf("consume paid withdrawal entitlement: %w", err)
		}
		if err := requireRowsAffected(res, "consume paid withdrawal entitlement"); err != nil {
			return err
		}
		if err := insertWithdrawableAllocation(ctx, runner, req.UserID, allocation.EntitlementID, transactionID, withdrawableAllocationConsume, allocation.Amount, allocation.AvailableAt, WithdrawalLedgerSourcePaid, req.RequestNo, map[string]any{
			"withdrawal_request_id": req.ID,
			"withdrawal_request_no": req.RequestNo,
		}, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func insertWithdrawalStatusEvent(ctx context.Context, runner withdrawalSQLRunner, requestID int64, status, actorType string, actorUserID *int64, note string, metadata map[string]any, createdAt time.Time) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return ErrWithdrawalInvalidInput.WithCause(err)
	}
	_, err = runner.ExecContext(ctx, `
INSERT INTO withdrawal_status_events (
	withdrawal_request_id, status, actor_type, actor_user_id, note, metadata, created_at
) VALUES (
	$1, $2, $3, $4, $5, $6::jsonb, $7
)`, requestID, status, actorType, actorUserID, strings.TrimSpace(note), string(raw), createdAt.UTC())
	if err != nil {
		return fmt.Errorf("insert withdrawal status event: %w", err)
	}
	return nil
}

func listWithdrawalStatusEvents(ctx context.Context, runner withdrawalSQLRunner, requestID int64) ([]WithdrawalStatusEvent, error) {
	rows, err := runner.QueryContext(ctx, `
SELECT id, status, actor_type, actor_user_id, note, metadata::text, created_at
FROM withdrawal_status_events
WHERE withdrawal_request_id = $1
ORDER BY created_at ASC, id ASC`, requestID)
	if err != nil {
		return nil, fmt.Errorf("list withdrawal status events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []WithdrawalStatusEvent
	for rows.Next() {
		var item WithdrawalStatusEvent
		var actor sql.NullInt64
		var raw string
		if err := rows.Scan(&item.ID, &item.Status, &item.ActorType, &actor, &item.Note, &raw, &item.CreatedAt); err != nil {
			return nil, err
		}
		if actor.Valid {
			item.ActorUserID = &actor.Int64
		}
		if strings.TrimSpace(raw) != "" {
			_ = json.Unmarshal([]byte(raw), &item.Metadata)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func listWithdrawalEntitlementLocks(ctx context.Context, runner withdrawalSQLRunner, requestID int64) ([]WithdrawalEntitlementLock, error) {
	rows, err := runner.QueryContext(ctx, `
SELECT entitlement_id, amount::text, available_at
FROM withdrawal_request_entitlements
WHERE withdrawal_request_id = $1
ORDER BY available_at ASC, entitlement_id ASC`, requestID)
	if err != nil {
		return nil, fmt.Errorf("list withdrawal entitlement locks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []WithdrawalEntitlementLock
	for rows.Next() {
		var item WithdrawalEntitlementLock
		var raw string
		if err := rows.Scan(&item.EntitlementID, &raw, &item.AvailableAt); err != nil {
			return nil, err
		}
		amount, err := parseLedgerDecimal(raw, "withdrawal entitlement lock")
		if err != nil {
			return nil, err
		}
		item.Amount = amount
		item.AvailableAt = item.AvailableAt.UTC()
		out = append(out, item)
	}
	return out, rows.Err()
}

func newWithdrawalRequestNo() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "wd_" + hex.EncodeToString(buf), nil
}

func isValidWithdrawalPayoutMethod(method string) bool {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case WithdrawalPayoutMethodAlipay, WithdrawalPayoutMethodBankTransfer, WithdrawalPayoutMethodOther:
		return true
	default:
		return false
	}
}

func isValidWithdrawalCurrency(currency string) bool {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case WithdrawalCurrencyCNY, WithdrawalCurrencyUSD:
		return true
	default:
		return false
	}
}

func isValidWithdrawalStatus(status string) bool {
	switch status {
	case WithdrawalStatusPendingReview, WithdrawalStatusSecondReview, WithdrawalStatusPayoutPending, WithdrawalStatusPaid, WithdrawalStatusRejected, WithdrawalStatusCanceled:
		return true
	default:
		return false
	}
}

func nullableDecimal(value *decimal.Decimal) any {
	if value == nil {
		return nil
	}
	return decimalString(*value)
}

func nullableInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func shanghaiLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}
