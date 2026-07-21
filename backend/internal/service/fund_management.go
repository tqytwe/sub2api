package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	FundRefundTypeOnlineRecharge  = "online_recharge_refund"
	FundRefundTypeOfflineRecharge = "offline_recharge_refund"

	FundRefundStatusPendingReview = "pending_review"
	FundRefundStatusPayoutPending = "payout_pending"
	FundRefundStatusPaid          = "paid"
	FundRefundStatusRejected      = "rejected"
	FundRefundStatusCanceled      = "canceled"
)

var (
	ErrFundManagementUnavailable       = infraerrors.InternalServer("FUND_MANAGEMENT_UNAVAILABLE", "fund management service is unavailable")
	ErrFundInvalidInput                = infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund request")
	ErrFundInvalidAmount               = infraerrors.BadRequest("FUND_INVALID_AMOUNT", "invalid fund amount")
	ErrFundInsufficientRefundable      = infraerrors.BadRequest("FUND_INSUFFICIENT_REFUNDABLE", "insufficient refundable recharge balance")
	ErrFundRefundInProgress            = infraerrors.Conflict("FUND_REFUND_IN_PROGRESS", "another refund request is already in progress")
	ErrFundRefundAccountRequired       = infraerrors.BadRequest("FUND_REFUND_ACCOUNT_REQUIRED", "payout account is required")
	ErrFundRefundNotFound              = infraerrors.NotFound("FUND_REFUND_NOT_FOUND", "fund refund request not found")
	ErrFundRefundInvalidStatus         = infraerrors.BadRequest("FUND_REFUND_INVALID_STATUS", "fund refund request status does not allow this operation")
	ErrFundClassificationNothingToDo   = infraerrors.BadRequest("FUND_CLASSIFICATION_NOTHING_TO_DO", "no fund classification candidates selected")
	ErrFundClassificationReasonInvalid = infraerrors.BadRequest("FUND_CLASSIFICATION_REASON_INVALID", "classification reason is invalid")
)

type FundManagementService struct {
	db         *sql.DB
	ledger     *BalanceLedgerService
	withdrawal *WithdrawalService
	now        func() time.Time
}

type WalletFundBreakdown struct {
	RefundableRechargeBalance decimal.Decimal `json:"refundable_recharge_balance"`
	OnlineRechargeBalance     decimal.Decimal `json:"online_recharge_balance"`
	OfflineRechargeBalance    decimal.Decimal `json:"offline_recharge_balance"`
	GiftBalance               decimal.Decimal `json:"gift_balance"`
	SignupGiftBalance         decimal.Decimal `json:"signup_gift_balance"`
	OpsGiftBalance            decimal.Decimal `json:"ops_gift_balance"`
	RefundFrozenBalance       decimal.Decimal `json:"refund_frozen_balance"`
	UnclassifiedBalance       decimal.Decimal `json:"unclassified_balance"`
}

type FundRefundCreateInput struct {
	UserID      int64
	RequestType string
	Amount      string
	Reason      string
}

type FundRefundActionInput struct {
	RequestID   int64
	UserID      int64
	ActorUserID int64
	Reason      string
	Note        string
}

type FundRefundMarkPaidInput struct {
	RequestID     int64
	ActorUserID   int64
	PaidAmount    string
	PaidCurrency  string
	FXRate        string
	ExternalTxnID string
	PaidAt        *time.Time
	Note          string
}

type FundRefundRequest struct {
	ID                             int64            `json:"id"`
	RequestNo                      string           `json:"request_no"`
	UserID                         int64            `json:"user_id"`
	UserEmail                      string           `json:"user_email,omitempty"`
	RequestType                    string           `json:"request_type"`
	Amount                         decimal.Decimal  `json:"amount"`
	Currency                       string           `json:"currency"`
	Status                         string           `json:"status"`
	Reason                         string           `json:"reason,omitempty"`
	AdminNote                      string           `json:"admin_note,omitempty"`
	PayoutMethod                   string           `json:"payout_method,omitempty"`
	PayoutCurrency                 string           `json:"payout_currency,omitempty"`
	PayoutAccountMask              string           `json:"payout_account_mask,omitempty"`
	PayoutRecipientNameMask        string           `json:"payout_recipient_name_mask,omitempty"`
	SubmitBalanceTransactionID     *int64           `json:"submit_balance_transaction_id,omitempty"`
	CloseBalanceTransactionID      *int64           `json:"close_balance_transaction_id,omitempty"`
	ApprovedBy                     *int64           `json:"approved_by,omitempty"`
	ApprovedAt                     *time.Time       `json:"approved_at,omitempty"`
	RejectedBy                     *int64           `json:"rejected_by,omitempty"`
	RejectedAt                     *time.Time       `json:"rejected_at,omitempty"`
	RejectedReason                 string           `json:"rejected_reason,omitempty"`
	CanceledAt                     *time.Time       `json:"canceled_at,omitempty"`
	PaidBy                         *int64           `json:"paid_by,omitempty"`
	PaidAt                         *time.Time       `json:"paid_at,omitempty"`
	PaidAmount                     *decimal.Decimal `json:"paid_amount,omitempty"`
	PaidCurrency                   string           `json:"paid_currency,omitempty"`
	PayoutFXRate                   *decimal.Decimal `json:"payout_fx_rate,omitempty"`
	ExternalTxnID                  string           `json:"external_txn_id,omitempty"`
	CreatedAt                      time.Time        `json:"created_at"`
	UpdatedAt                      time.Time        `json:"updated_at"`
	payoutAccountSnapshotEncrypted string
}

type FundRefundListQuery struct {
	Status   string
	UserID   int64
	Page     int
	PageSize int
}

type FundRefundRequestPage struct {
	Items    []FundRefundRequest `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Pages    int                 `json:"pages"`
}

type FundGrantInput struct {
	UserID      int64
	Amount      string
	Reason      string
	ActorUserID int64
}

type OfflineRechargeInput struct {
	UserID      int64
	Amount      string
	ExternalRef string
	Reason      string
	ActorUserID int64
}

type FundClassificationCandidate struct {
	UserID                  int64           `json:"user_id"`
	UserEmail               string          `json:"user_email"`
	TransactionID           int64           `json:"transaction_id"`
	Amount                  decimal.Decimal `json:"amount"`
	CreatedAt               time.Time       `json:"created_at"`
	RecommendedKind         string          `json:"recommended_kind"`
	EstimatedRemaining      decimal.Decimal `json:"estimated_remaining"`
	EstimatedConsumed       decimal.Decimal `json:"estimated_consumed"`
	CurrentBalance          decimal.Decimal `json:"current_balance"`
	ExistingClassifiedFunds decimal.Decimal `json:"existing_classified_funds"`
}

type FundClassificationPreview struct {
	Mode           string                        `json:"mode"`
	GeneratedAt    time.Time                     `json:"generated_at"`
	CandidateCount int                           `json:"candidate_count"`
	Candidates     []FundClassificationCandidate `json:"candidates"`
}

type FundClassificationExecuteInput struct {
	TransactionIDs []int64
	Reason         string
	ActorUserID    int64
}

type FundClassificationExecuteResult struct {
	Mode          string                        `json:"mode"`
	GeneratedAt   time.Time                     `json:"generated_at"`
	AffectedCount int                           `json:"affected_count"`
	Candidates    []FundClassificationCandidate `json:"candidates"`
}

type refundBatchPlan struct {
	BatchID int64
	Amount  decimal.Decimal
}

type fundSQLRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func NewFundManagementService(db *sql.DB, ledger *BalanceLedgerService, withdrawal *WithdrawalService) *FundManagementService {
	return &FundManagementService{db: db, ledger: ledger, withdrawal: withdrawal, now: time.Now}
}

func (s *FundManagementService) GetWalletBreakdown(ctx context.Context, userID int64) (*WalletFundBreakdown, error) {
	if s == nil || s.db == nil {
		return nil, ErrFundManagementUnavailable
	}
	if userID <= 0 {
		return nil, ErrFundInvalidInput
	}
	var availableRaw, onlineRaw, offlineRaw, signupRaw, opsRaw, giftRaw, frozenRaw, classifiedRaw string
	if err := queryOneBalanceLedger(ctx, s.db, `
SELECT
	COALESCE(u.balance, 0)::text,
	COALESCE(SUM(CASE WHEN bfb.source_kind = 'online_recharge' AND bfb.status = 'active' THEN bfb.remaining_amount ELSE 0 END), 0)::text,
	COALESCE(SUM(CASE WHEN bfb.source_kind = 'offline_recharge' AND bfb.status = 'active' THEN bfb.remaining_amount ELSE 0 END), 0)::text,
	COALESCE(SUM(CASE WHEN bfb.source_kind = 'signup_gift' AND bfb.status = 'active' THEN bfb.remaining_amount ELSE 0 END), 0)::text,
	COALESCE(SUM(CASE WHEN bfb.source_kind IN ('ops_gift', 'compensation') AND bfb.status = 'active' THEN bfb.remaining_amount ELSE 0 END), 0)::text,
	COALESCE(SUM(CASE WHEN bfb.source_kind IN ('signup_gift', 'ops_gift', 'compensation', 'redeem_gift', 'promotion_gift', 'unknown') AND bfb.status = 'active' THEN bfb.remaining_amount ELSE 0 END), 0)::text,
	COALESCE(SUM(CASE WHEN bfb.status = 'active' THEN bfb.refund_frozen_amount ELSE 0 END), 0)::text,
	COALESCE(SUM(CASE WHEN bfb.status = 'active' THEN bfb.remaining_amount ELSE 0 END), 0)::text
FROM users u
LEFT JOIN balance_fund_batches bfb ON bfb.user_id = u.id
WHERE u.id = $1 AND u.deleted_at IS NULL
GROUP BY u.id, u.balance`, []any{userID}, &availableRaw, &onlineRaw, &offlineRaw, &signupRaw, &opsRaw, &giftRaw, &frozenRaw, &classifiedRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get wallet fund breakdown: %w", err)
	}
	available, err := parseLedgerDecimal(availableRaw, "available balance")
	if err != nil {
		return nil, err
	}
	online, err := parseLedgerDecimal(onlineRaw, "online recharge balance")
	if err != nil {
		return nil, err
	}
	offline, err := parseLedgerDecimal(offlineRaw, "offline recharge balance")
	if err != nil {
		return nil, err
	}
	signup, err := parseLedgerDecimal(signupRaw, "signup gift balance")
	if err != nil {
		return nil, err
	}
	ops, err := parseLedgerDecimal(opsRaw, "ops gift balance")
	if err != nil {
		return nil, err
	}
	gift, err := parseLedgerDecimal(giftRaw, "gift balance")
	if err != nil {
		return nil, err
	}
	frozen, err := parseLedgerDecimal(frozenRaw, "refund frozen balance")
	if err != nil {
		return nil, err
	}
	classified, err := parseLedgerDecimal(classifiedRaw, "classified fund balance")
	if err != nil {
		return nil, err
	}
	unclassified := available.Sub(classified)
	if unclassified.IsNegative() {
		unclassified = decimal.Zero
	}
	return &WalletFundBreakdown{
		RefundableRechargeBalance: online.Add(offline),
		OnlineRechargeBalance:     online,
		OfflineRechargeBalance:    offline,
		GiftBalance:               gift,
		SignupGiftBalance:         signup,
		OpsGiftBalance:            ops,
		RefundFrozenBalance:       frozen,
		UnclassifiedBalance:       unclassified,
	}, nil
}

func (s *FundManagementService) CreateRefundRequest(ctx context.Context, input FundRefundCreateInput) (*FundRefundRequest, error) {
	if s == nil || s.db == nil || s.ledger == nil || s.withdrawal == nil {
		return nil, ErrFundManagementUnavailable
	}
	amount, err := parseFundWholeAmount(input.Amount)
	if err != nil {
		return nil, err
	}
	requestType := normalizeFundRefundType(input.RequestType)
	if requestType == "" || input.UserID <= 0 {
		return nil, ErrFundInvalidInput
	}
	account, err := s.withdrawal.GetCurrentPayoutAccount(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrFundRefundAccountRequired
	}
	snapshot, err := s.withdrawal.buildAccountSnapshot(ctx, account)
	if err != nil {
		return nil, err
	}
	requestNo, err := newFundRefundRequestNo()
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin fund refund tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if inProgress, err := hasInProgressFundRefund(ctx, tx, input.UserID); err != nil {
		return nil, err
	} else if inProgress {
		return nil, ErrFundRefundInProgress
	}
	now := s.now().UTC()
	plans, err := lockRefundableFundBatches(ctx, tx, input.UserID, requestType, amount)
	if err != nil {
		return nil, err
	}
	var requestID int64
	if err := queryOneBalanceLedger(ctx, tx, `
INSERT INTO fund_refund_requests (
	request_no, user_id, request_type, amount, currency, status, reason,
	payout_method, payout_currency, payout_account_mask, payout_recipient_name_mask,
	payout_account_snapshot_encrypted, created_at, updated_at
) VALUES (
	$1, $2, $3, $4, 'USD', 'pending_review', $5,
	$6, $7, $8, $9, $10, $11, NOW()
)
RETURNING id`, []any{requestNo, input.UserID, requestType, decimalString(amount), strings.TrimSpace(input.Reason),
		account.Method, account.Currency, account.AccountMask, account.RecipientNameMask, snapshot, now,
	}, &requestID); err != nil {
		return nil, fmt.Errorf("insert fund refund request: %w", err)
	}
	for _, plan := range plans {
		if err := freezeFundRefundBatch(ctx, tx, input.UserID, requestID, 0, plan, FundRefundLedgerSourceSubmit, requestNo, now); err != nil {
			return nil, err
		}
	}
	ledgerTx, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{
		UserID:                             input.UserID,
		BalanceDelta:                       -decimalToLedgerFloat(amount),
		SourceType:                         FundRefundLedgerSourceSubmit,
		SourceID:                           requestNo,
		IdempotencyKey:                     "fund_refund_submit:" + requestNo,
		ActorType:                          BalanceLedgerActorUser,
		ActorUserID:                        &input.UserID,
		Description:                        "recharge refund request submitted",
		Metadata:                           map[string]any{"request_no": requestNo, "request_type": requestType},
		SkipWithdrawableEntitlementEffects: true,
		SkipFundBatchEffects:               true,
	})
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE fund_refund_requests
SET submit_balance_transaction_id = $1, updated_at = NOW()
WHERE id = $2`, ledgerTx.ID, requestID); err != nil {
		return nil, fmt.Errorf("update fund refund submit tx: %w", err)
	}
	for _, plan := range plans {
		if err := setFundRefundBatchTransaction(ctx, tx, input.UserID, requestID, plan.BatchID, ledgerTx.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit fund refund tx: %w", err)
	}
	committed = true
	return s.GetRefundRequest(ctx, requestID, input.UserID, false)
}

func (s *FundManagementService) CancelRefundRequest(ctx context.Context, input FundRefundActionInput) (*FundRefundRequest, error) {
	if input.UserID <= 0 {
		return nil, ErrFundInvalidInput
	}
	return s.restoreRefundRequest(ctx, input, FundRefundStatusCanceled, FundRefundLedgerSourceCancel, BalanceLedgerActorUser)
}

func (s *FundManagementService) AdminRejectRefundRequest(ctx context.Context, input FundRefundActionInput) (*FundRefundRequest, error) {
	if strings.TrimSpace(input.Reason) == "" || len([]rune(input.Reason)) > 500 {
		return nil, ErrFundInvalidInput.WithMetadata(map[string]string{"field": "reason"})
	}
	return s.restoreRefundRequest(ctx, input, FundRefundStatusRejected, FundRefundLedgerSourceReject, BalanceLedgerActorAdmin)
}

func (s *FundManagementService) AdminApproveRefundRequest(ctx context.Context, input FundRefundActionInput) (*FundRefundRequest, error) {
	if s == nil || s.db == nil {
		return nil, ErrFundManagementUnavailable
	}
	tx, req, err := s.beginAndLockFundRefund(ctx, input.RequestID, 0, false)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if req.Status != FundRefundStatusPendingReview {
		return nil, ErrFundRefundInvalidStatus
	}
	now := s.now().UTC()
	if _, err := tx.ExecContext(ctx, `
UPDATE fund_refund_requests
SET status = 'payout_pending', approved_by = $1, approved_at = $2, admin_note = $3, updated_at = NOW()
WHERE id = $4`, input.ActorUserID, now, strings.TrimSpace(input.Note), req.ID); err != nil {
		return nil, fmt.Errorf("approve fund refund: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit fund refund approve tx: %w", err)
	}
	committed = true
	return s.GetRefundRequest(ctx, req.ID, 0, true)
}

func (s *FundManagementService) AdminMarkRefundPaid(ctx context.Context, input FundRefundMarkPaidInput) (*FundRefundRequest, error) {
	if s == nil || s.db == nil || s.ledger == nil {
		return nil, ErrFundManagementUnavailable
	}
	paidAmount, err := parseFundWholeAmount(input.PaidAmount)
	if err != nil {
		return nil, err
	}
	fxRate, err := parseWithdrawalPositiveDecimal(firstNonEmpty(input.FXRate, "1"), "payout_fx_rate")
	if err != nil {
		return nil, err
	}
	input.PaidCurrency = strings.ToUpper(strings.TrimSpace(input.PaidCurrency))
	if !isValidWithdrawalCurrency(input.PaidCurrency) {
		return nil, ErrFundInvalidInput.WithMetadata(map[string]string{"field": "paid_currency"})
	}
	tx, req, err := s.beginAndLockFundRefund(ctx, input.RequestID, 0, false)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if req.Status != FundRefundStatusPayoutPending {
		return nil, ErrFundRefundInvalidStatus
	}
	now := s.now().UTC()
	paidAt := now
	if input.PaidAt != nil {
		paidAt = input.PaidAt.UTC()
	}
	ledgerTx, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{
		UserID:                             req.UserID,
		SourceType:                         FundRefundLedgerSourcePaid,
		SourceID:                           req.RequestNo,
		IdempotencyKey:                     "fund_refund_paid:" + req.RequestNo,
		ActorType:                          BalanceLedgerActorAdmin,
		ActorUserID:                        &input.ActorUserID,
		Description:                        "recharge refund marked paid",
		Metadata:                           map[string]any{"request_no": req.RequestNo, "external_txn_id": strings.TrimSpace(input.ExternalTxnID)},
		SkipWithdrawableEntitlementEffects: true,
		SkipFundBatchEffects:               true,
	})
	if err != nil {
		return nil, err
	}
	if err := completeFundRefundBatches(ctx, tx, req, ledgerTx.ID, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE fund_refund_requests
SET status = 'paid', close_balance_transaction_id = $1, paid_by = $2, paid_at = $3,
    paid_amount = $4, paid_currency = $5, payout_fx_rate = $6, external_txn_id = $7,
    admin_note = $8, updated_at = NOW()
WHERE id = $9`, ledgerTx.ID, input.ActorUserID, paidAt, decimalString(paidAmount), input.PaidCurrency, decimalString(fxRate), strings.TrimSpace(input.ExternalTxnID), strings.TrimSpace(input.Note), req.ID); err != nil {
		return nil, fmt.Errorf("mark fund refund paid: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit fund refund paid tx: %w", err)
	}
	committed = true
	return s.GetRefundRequest(ctx, req.ID, 0, true)
}

func (s *FundManagementService) restoreRefundRequest(ctx context.Context, input FundRefundActionInput, nextStatus string, sourceType string, actorType string) (*FundRefundRequest, error) {
	if s == nil || s.db == nil || s.ledger == nil {
		return nil, ErrFundManagementUnavailable
	}
	requireUser := actorType == BalanceLedgerActorUser
	tx, req, err := s.beginAndLockFundRefund(ctx, input.RequestID, input.UserID, requireUser)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if req.Status != FundRefundStatusPendingReview {
		return nil, ErrFundRefundInvalidStatus
	}
	now := s.now().UTC()
	actorID := input.ActorUserID
	if actorType == BalanceLedgerActorUser {
		actorID = input.UserID
	}
	ledgerTx, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{
		UserID:                             req.UserID,
		BalanceDelta:                       decimalToLedgerFloat(req.Amount),
		SourceType:                         sourceType,
		SourceID:                           req.RequestNo,
		IdempotencyKey:                     sourceType + ":" + req.RequestNo,
		ActorType:                          actorType,
		ActorUserID:                        &actorID,
		Description:                        "recharge refund request restored",
		Metadata:                           map[string]any{"request_no": req.RequestNo, "reason": strings.TrimSpace(input.Reason)},
		SkipWithdrawableEntitlementEffects: true,
		SkipFundBatchEffects:               true,
	})
	if err != nil {
		return nil, err
	}
	if err := unfreezeFundRefundBatches(ctx, tx, req, ledgerTx.ID, sourceType, now); err != nil {
		return nil, err
	}
	switch nextStatus {
	case FundRefundStatusCanceled:
		_, err = tx.ExecContext(ctx, `
UPDATE fund_refund_requests
SET status = 'canceled', close_balance_transaction_id = $1, canceled_at = $2, admin_note = $3, updated_at = NOW()
WHERE id = $4`, ledgerTx.ID, now, strings.TrimSpace(input.Note), req.ID)
	case FundRefundStatusRejected:
		_, err = tx.ExecContext(ctx, `
UPDATE fund_refund_requests
SET status = 'rejected', close_balance_transaction_id = $1, rejected_by = $2, rejected_at = $3,
    rejected_reason = $4, admin_note = $5, updated_at = NOW()
WHERE id = $6`, ledgerTx.ID, input.ActorUserID, now, strings.TrimSpace(input.Reason), strings.TrimSpace(input.Note), req.ID)
	default:
		err = ErrFundRefundInvalidStatus
	}
	if err != nil {
		return nil, fmt.Errorf("restore fund refund request: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit fund refund restore tx: %w", err)
	}
	committed = true
	return s.GetRefundRequest(ctx, req.ID, 0, actorType == BalanceLedgerActorAdmin)
}

func (s *FundManagementService) GetRefundRequest(ctx context.Context, requestID int64, requesterUserID int64, adminView bool) (*FundRefundRequest, error) {
	if s == nil || s.db == nil {
		return nil, ErrFundManagementUnavailable
	}
	query := `SELECT ` + fundRefundSelectColumns + `
FROM fund_refund_requests frr
JOIN users u ON u.id = frr.user_id
WHERE frr.id = $1`
	args := []any{requestID}
	if !adminView {
		query += ` AND frr.user_id = $2`
		args = append(args, requesterUserID)
	}
	req, err := queryFundRefundRequest(ctx, s.db, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFundRefundNotFound
	}
	return req, err
}

func (s *FundManagementService) ListUserRefundRequests(ctx context.Context, userID int64, query FundRefundListQuery) (*FundRefundRequestPage, error) {
	query.UserID = userID
	return s.listRefundRequests(ctx, query, false)
}

func (s *FundManagementService) AdminListRefundRequests(ctx context.Context, query FundRefundListQuery) (*FundRefundRequestPage, error) {
	return s.listRefundRequests(ctx, query, true)
}

func (s *FundManagementService) listRefundRequests(ctx context.Context, query FundRefundListQuery, adminView bool) (*FundRefundRequestPage, error) {
	if s == nil || s.db == nil {
		return nil, ErrFundManagementUnavailable
	}
	page, pageSize := normalizeFundPagination(query.Page, query.PageSize)
	where, args := fundRefundListWhere(query, adminView)
	countQuery := `SELECT COUNT(*)::bigint FROM fund_refund_requests frr JOIN users u ON u.id = frr.user_id` + where
	var total int64
	if err := queryOneBalanceLedger(ctx, s.db, countQuery, args, &total); err != nil {
		return nil, fmt.Errorf("count fund refunds: %w", err)
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT `+fundRefundSelectColumns+`
FROM fund_refund_requests frr
JOIN users u ON u.id = frr.user_id`+where+`
ORDER BY frr.created_at DESC, frr.id DESC
LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("list fund refunds: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]FundRefundRequest, 0)
	for rows.Next() {
		item, err := scanFundRefundRequest(rows.Scan)
		if err != nil {
			return nil, err
		}
		if !adminView {
			item.UserEmail = ""
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pages := int(math.Ceil(float64(total) / float64(pageSize)))
	if pages < 1 {
		pages = 1
	}
	return &FundRefundRequestPage{Items: items, Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func (s *FundManagementService) GrantGift(ctx context.Context, input FundGrantInput) (*BalanceTransaction, error) {
	return s.adminCreditUser(ctx, input.UserID, input.Amount, FundLedgerSourceOpsGift, "administrator gift balance", input.Reason, input.ActorUserID, "")
}

func (s *FundManagementService) GrantOfflineRecharge(ctx context.Context, input OfflineRechargeInput) (*BalanceTransaction, error) {
	return s.adminCreditUser(ctx, input.UserID, input.Amount, FundLedgerSourceOfflineRecharge, "offline recharge confirmed", input.Reason, input.ActorUserID, input.ExternalRef)
}

func (s *FundManagementService) adminCreditUser(ctx context.Context, userID int64, rawAmount string, sourceType string, description string, reason string, actorUserID int64, externalRef string) (*BalanceTransaction, error) {
	if s == nil || s.ledger == nil {
		return nil, ErrFundManagementUnavailable
	}
	amount, err := parseFundWholeAmount(rawAmount)
	if err != nil {
		return nil, err
	}
	reason = strings.TrimSpace(reason)
	if userID <= 0 || actorUserID <= 0 || len([]rune(reason)) < 3 || len([]rune(reason)) > 500 {
		return nil, ErrFundInvalidInput
	}
	sourceID := strings.TrimSpace(externalRef)
	if sourceID == "" {
		sourceID = fmt.Sprintf("%d:%d", userID, s.now().UTC().UnixNano())
	}
	return s.ledger.ApplyDelta(ctx, BalanceLedgerApplyInput{
		UserID:         userID,
		BalanceDelta:   decimalToLedgerFloat(amount),
		SourceType:     sourceType,
		SourceID:       sourceID,
		IdempotencyKey: sourceType + ":" + sourceID,
		ActorType:      BalanceLedgerActorAdmin,
		ActorUserID:    &actorUserID,
		Description:    description,
		Metadata: map[string]any{
			"reason":       reason,
			"external_ref": strings.TrimSpace(externalRef),
		},
	})
}

func (s *FundManagementService) PreviewSignupGift30(ctx context.Context, limit int) (*FundClassificationPreview, error) {
	if s == nil || s.db == nil {
		return nil, ErrFundManagementUnavailable
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	candidates, err := selectSignupGift30Candidates(ctx, s.db, nil, limit, false)
	if err != nil {
		return nil, err
	}
	return &FundClassificationPreview{
		Mode:           "preview",
		GeneratedAt:    s.now().UTC(),
		CandidateCount: len(candidates),
		Candidates:     candidates,
	}, nil
}

func (s *FundManagementService) ExecuteSignupGift30(ctx context.Context, input FundClassificationExecuteInput) (*FundClassificationExecuteResult, error) {
	if s == nil || s.db == nil {
		return nil, ErrFundManagementUnavailable
	}
	reason := strings.TrimSpace(input.Reason)
	if len([]rune(reason)) < 10 || len([]rune(reason)) > 500 {
		return nil, ErrFundClassificationReasonInvalid
	}
	if input.ActorUserID <= 0 || len(input.TransactionIDs) == 0 {
		return nil, ErrFundClassificationNothingToDo
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin fund classification tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	candidates, err := selectSignupGift30Candidates(ctx, tx, input.TransactionIDs, len(input.TransactionIDs), true)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	for _, candidate := range candidates {
		if err := insertHistoricalSignupGiftBatch(ctx, tx, candidate, input.ActorUserID, reason, now); err != nil {
			return nil, err
		}
	}
	reportRaw, _ := json.Marshal(map[string]any{
		"transaction_ids": input.TransactionIDs,
		"candidates":      candidates,
	})
	if _, err := tx.ExecContext(ctx, `
INSERT INTO fund_classification_runs (
	mode, classification_kind, actor_user_id, reason, candidate_count, affected_count, report, created_at
) VALUES (
	'execute', 'signup_gift_30', $1, $2, $3, $4, $5::jsonb, $6
)`, input.ActorUserID, reason, len(input.TransactionIDs), len(candidates), string(reportRaw), now); err != nil {
		return nil, fmt.Errorf("insert classification run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit fund classification tx: %w", err)
	}
	committed = true
	return &FundClassificationExecuteResult{
		Mode:          "execute",
		GeneratedAt:   now,
		AffectedCount: len(candidates),
		Candidates:    candidates,
	}, nil
}

func (s *FundManagementService) AdminGetRefundPayoutSnapshot(ctx context.Context, requestID int64) (map[string]any, error) {
	if s == nil || s.withdrawal == nil || s.withdrawal.encryptor == nil {
		return nil, ErrFundManagementUnavailable
	}
	req, err := s.GetRefundRequest(ctx, requestID, 0, true)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.payoutAccountSnapshotEncrypted) == "" {
		return nil, ErrFundRefundAccountRequired
	}
	plain, err := s.withdrawal.encryptor.Decrypt(req.payoutAccountSnapshotEncrypted)
	if err != nil {
		return nil, ErrWithdrawalAccountEncryption.WithCause(err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return nil, ErrWithdrawalAccountEncryption.WithCause(err)
	}
	return out, nil
}

func parseFundWholeAmount(raw string) (decimal.Decimal, error) {
	amount, err := parseWithdrawalAmount(raw)
	if err != nil {
		return decimal.Zero, ErrFundInvalidAmount
	}
	return amount, nil
}

func normalizeFundRefundType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case FundRefundTypeOnlineRecharge, "online", "online_recharge":
		return FundRefundTypeOnlineRecharge
	case FundRefundTypeOfflineRecharge, "offline", "offline_recharge":
		return FundRefundTypeOfflineRecharge
	default:
		return ""
	}
}

func fundRefundSourceKinds(requestType string) []string {
	switch requestType {
	case FundRefundTypeOnlineRecharge:
		return []string{FundSourceKindOnlineRecharge}
	case FundRefundTypeOfflineRecharge:
		return []string{FundSourceKindOfflineRecharge}
	default:
		return nil
	}
}

func lockRefundableFundBatches(ctx context.Context, runner fundSQLRunner, userID int64, requestType string, amount decimal.Decimal) ([]refundBatchPlan, error) {
	kinds := fundRefundSourceKinds(requestType)
	if len(kinds) == 0 {
		return nil, ErrFundInvalidInput
	}
	rows, err := runner.QueryContext(ctx, `
SELECT id, remaining_amount::text
FROM balance_fund_batches
WHERE user_id = $1
  AND source_kind = ANY($2::text[])
  AND refundable = TRUE
  AND status = 'active'
  AND remaining_amount > 0
ORDER BY available_at ASC, id ASC
FOR UPDATE`, userID, pqStringArray(kinds))
	if err != nil {
		return nil, fmt.Errorf("lock refundable fund batches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	remaining := amount
	plans := make([]refundBatchPlan, 0)
	for rows.Next() {
		if !remaining.IsPositive() {
			break
		}
		var batchID int64
		var amountRaw string
		if err := rows.Scan(&batchID, &amountRaw); err != nil {
			return nil, err
		}
		available, err := parseLedgerDecimal(amountRaw, "refundable fund batch")
		if err != nil {
			return nil, err
		}
		next := decimalMin(available, remaining)
		next = clampDecimalScale(next)
		plans = append(plans, refundBatchPlan{BatchID: batchID, Amount: next})
		remaining = remaining.Sub(next)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if remaining.GreaterThan(decimal.RequireFromString("0.00000001")) {
		return nil, ErrFundInsufficientRefundable
	}
	return plans, nil
}

func freezeFundRefundBatch(ctx context.Context, runner fundSQLRunner, userID int64, requestID int64, transactionID int64, plan refundBatchPlan, sourceType string, sourceID string, createdAt time.Time) error {
	res, err := runner.ExecContext(ctx, `
UPDATE balance_fund_batches
SET remaining_amount = remaining_amount - $1::numeric,
    refund_frozen_amount = refund_frozen_amount + $1::numeric,
    status = 'active',
    updated_at = NOW()
WHERE id = $2
  AND user_id = $3
  AND refundable = TRUE
  AND remaining_amount >= $1::numeric`, decimalString(plan.Amount), plan.BatchID, userID)
	if err != nil {
		return fmt.Errorf("freeze fund refund batch: %w", err)
	}
	if err := requireRowsAffected(res, "freeze fund refund batch"); err != nil {
		return err
	}
	if _, err := runner.ExecContext(ctx, `
INSERT INTO fund_refund_request_batches (fund_refund_request_id, batch_id, amount, created_at)
VALUES ($1, $2, $3, $4)`, requestID, plan.BatchID, decimalString(plan.Amount), createdAt.UTC()); err != nil {
		return fmt.Errorf("insert fund refund batch lock: %w", err)
	}
	return insertFundBatchAllocation(ctx, runner, userID, plan.BatchID, transactionID, fundAllocationRefundFreeze, plan.Amount, sourceType, sourceID, map[string]any{"fund_refund_request_id": requestID}, createdAt)
}

func setFundRefundBatchTransaction(ctx context.Context, runner fundSQLRunner, userID int64, requestID int64, batchID int64, transactionID int64) error {
	_, err := runner.ExecContext(ctx, `
UPDATE balance_fund_allocations
SET balance_transaction_id = $1
WHERE user_id = $2
  AND batch_id = $3
  AND action = 'refund_freeze'
  AND metadata->>'fund_refund_request_id' = $4`, transactionID, userID, batchID, strconv.FormatInt(requestID, 10))
	return err
}

func unfreezeFundRefundBatches(ctx context.Context, runner fundSQLRunner, req *FundRefundRequest, transactionID int64, sourceType string, createdAt time.Time) error {
	locks, err := listFundRefundBatchLocks(ctx, runner, req.ID)
	if err != nil {
		return err
	}
	for _, lock := range locks {
		res, err := runner.ExecContext(ctx, `
UPDATE balance_fund_batches
SET remaining_amount = remaining_amount + $1::numeric,
    refund_frozen_amount = refund_frozen_amount - $1::numeric,
    status = 'active',
    updated_at = NOW()
WHERE id = $2
  AND user_id = $3
  AND refund_frozen_amount >= $1::numeric`, decimalString(lock.Amount), lock.BatchID, req.UserID)
		if err != nil {
			return fmt.Errorf("unfreeze fund refund batch: %w", err)
		}
		if err := requireRowsAffected(res, "unfreeze fund refund batch"); err != nil {
			return err
		}
		if err := insertFundBatchAllocation(ctx, runner, req.UserID, lock.BatchID, transactionID, fundAllocationRefundUnfreeze, lock.Amount, sourceType, req.RequestNo, map[string]any{"fund_refund_request_id": req.ID}, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func completeFundRefundBatches(ctx context.Context, runner fundSQLRunner, req *FundRefundRequest, transactionID int64, createdAt time.Time) error {
	locks, err := listFundRefundBatchLocks(ctx, runner, req.ID)
	if err != nil {
		return err
	}
	for _, lock := range locks {
		res, err := runner.ExecContext(ctx, `
UPDATE balance_fund_batches
SET refund_frozen_amount = refund_frozen_amount - $1::numeric,
    refunded_amount = refunded_amount + $1::numeric,
    status = CASE WHEN remaining_amount <= 0 AND refund_frozen_amount - $1::numeric <= 0 THEN 'consumed' ELSE status END,
    updated_at = NOW()
WHERE id = $2
  AND user_id = $3
  AND refund_frozen_amount >= $1::numeric`, decimalString(lock.Amount), lock.BatchID, req.UserID)
		if err != nil {
			return fmt.Errorf("complete fund refund batch: %w", err)
		}
		if err := requireRowsAffected(res, "complete fund refund batch"); err != nil {
			return err
		}
		if err := insertFundBatchAllocation(ctx, runner, req.UserID, lock.BatchID, transactionID, fundAllocationRefundComplete, lock.Amount, FundRefundLedgerSourcePaid, req.RequestNo, map[string]any{"fund_refund_request_id": req.ID}, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func listFundRefundBatchLocks(ctx context.Context, runner fundSQLRunner, requestID int64) ([]refundBatchPlan, error) {
	rows, err := runner.QueryContext(ctx, `
SELECT batch_id, amount::text
FROM fund_refund_request_batches
WHERE fund_refund_request_id = $1
ORDER BY id ASC`, requestID)
	if err != nil {
		return nil, fmt.Errorf("list fund refund batch locks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []refundBatchPlan
	for rows.Next() {
		var item refundBatchPlan
		var amountRaw string
		if err := rows.Scan(&item.BatchID, &amountRaw); err != nil {
			return nil, err
		}
		amount, err := parseLedgerDecimal(amountRaw, "fund refund batch amount")
		if err != nil {
			return nil, err
		}
		item.Amount = amount
		out = append(out, item)
	}
	return out, rows.Err()
}

func hasInProgressFundRefund(ctx context.Context, runner fundSQLRunner, userID int64) (bool, error) {
	var exists bool
	err := queryOneBalanceLedger(ctx, runner, `
SELECT EXISTS (
	SELECT 1
	FROM fund_refund_requests
	WHERE user_id = $1
	  AND status IN ('pending_review', 'payout_pending')
)`, []any{userID}, &exists)
	return exists, err
}

func (s *FundManagementService) beginAndLockFundRefund(ctx context.Context, requestID int64, userID int64, requireUser bool) (*sql.Tx, *FundRefundRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin fund refund lock tx: %w", err)
	}
	query := `SELECT ` + fundRefundSelectColumns + `
FROM fund_refund_requests frr
JOIN users u ON u.id = frr.user_id
WHERE frr.id = $1`
	args := []any{requestID}
	if requireUser {
		query += ` AND frr.user_id = $2`
		args = append(args, userID)
	}
	query += ` FOR UPDATE OF frr`
	req, err := queryFundRefundRequest(ctx, tx, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return nil, nil, ErrFundRefundNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return nil, nil, err
	}
	return tx, req, nil
}

const fundRefundSelectColumns = `
	frr.id,
	frr.request_no,
	frr.user_id,
	u.email,
	frr.request_type,
	frr.amount::text,
	frr.currency,
	frr.status,
	frr.reason,
	frr.admin_note,
	frr.payout_method,
	frr.payout_currency,
	frr.payout_account_mask,
	frr.payout_recipient_name_mask,
	frr.payout_account_snapshot_encrypted,
	frr.submit_balance_transaction_id,
	frr.close_balance_transaction_id,
	frr.approved_by,
	frr.approved_at,
	frr.rejected_by,
	frr.rejected_at,
	frr.rejected_reason,
	frr.canceled_at,
	frr.paid_by,
	frr.paid_at,
	frr.paid_amount::text,
	frr.paid_currency,
	frr.payout_fx_rate::text,
	frr.external_txn_id,
	frr.created_at,
	frr.updated_at`

func queryFundRefundRequest(ctx context.Context, runner fundSQLRunner, query string, args ...any) (*FundRefundRequest, error) {
	rows, err := runner.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	req, err := scanFundRefundRequest(rows.Scan)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		return nil, ErrFundInvalidInput
	}
	return req, rows.Err()
}

func scanFundRefundRequest(scan func(dest ...any) error) (*FundRefundRequest, error) {
	var req FundRefundRequest
	var amountRaw, paidAmountRaw, paidCurrency, fxRaw sql.NullString
	var submitTx, closeTx, approvedBy, rejectedBy, paidBy sql.NullInt64
	var approvedAt, rejectedAt, canceledAt, paidAt sql.NullTime
	if err := scan(
		&req.ID,
		&req.RequestNo,
		&req.UserID,
		&req.UserEmail,
		&req.RequestType,
		&amountRaw,
		&req.Currency,
		&req.Status,
		&req.Reason,
		&req.AdminNote,
		&req.PayoutMethod,
		&req.PayoutCurrency,
		&req.PayoutAccountMask,
		&req.PayoutRecipientNameMask,
		&req.payoutAccountSnapshotEncrypted,
		&submitTx,
		&closeTx,
		&approvedBy,
		&approvedAt,
		&rejectedBy,
		&rejectedAt,
		&req.RejectedReason,
		&canceledAt,
		&paidBy,
		&paidAt,
		&paidAmountRaw,
		&paidCurrency,
		&fxRaw,
		&req.ExternalTxnID,
		&req.CreatedAt,
		&req.UpdatedAt,
	); err != nil {
		return nil, err
	}
	amount, err := parseLedgerDecimal(amountRaw.String, "fund refund amount")
	if err != nil {
		return nil, err
	}
	req.Amount = amount
	req.SubmitBalanceTransactionID = int64PtrFromNull(submitTx)
	req.CloseBalanceTransactionID = int64PtrFromNull(closeTx)
	req.ApprovedBy = int64PtrFromNull(approvedBy)
	req.RejectedBy = int64PtrFromNull(rejectedBy)
	req.PaidBy = int64PtrFromNull(paidBy)
	req.ApprovedAt = timePtrFromNull(approvedAt)
	req.RejectedAt = timePtrFromNull(rejectedAt)
	req.CanceledAt = timePtrFromNull(canceledAt)
	req.PaidAt = timePtrFromNull(paidAt)
	if paidAmountRaw.Valid && strings.TrimSpace(paidAmountRaw.String) != "" {
		value, err := parseLedgerDecimal(paidAmountRaw.String, "fund refund paid amount")
		if err != nil {
			return nil, err
		}
		req.PaidAmount = &value
	}
	if paidCurrency.Valid {
		req.PaidCurrency = paidCurrency.String
	}
	if fxRaw.Valid && strings.TrimSpace(fxRaw.String) != "" {
		value, err := parseLedgerDecimal(fxRaw.String, "fund refund fx rate")
		if err != nil {
			return nil, err
		}
		req.PayoutFXRate = &value
	}
	return &req, nil
}

func normalizeFundPagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func fundRefundListWhere(query FundRefundListQuery, adminView bool) (string, []any) {
	parts := []string{"u.deleted_at IS NULL"}
	args := make([]any, 0)
	if !adminView || query.UserID > 0 {
		args = append(args, query.UserID)
		parts = append(parts, fmt.Sprintf("frr.user_id = $%d", len(args)))
	}
	status := strings.TrimSpace(query.Status)
	if status != "" && status != "all" {
		args = append(args, status)
		parts = append(parts, fmt.Sprintf("frr.status = $%d", len(args)))
	}
	if len(parts) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

func selectSignupGift30Candidates(ctx context.Context, runner fundSQLRunner, ids []int64, limit int, forUpdate bool) ([]FundClassificationCandidate, error) {
	args := []any{}
	filter := ""
	if len(ids) > 0 {
		args = append(args, pqInt64Array(ids))
		filter = "AND bt.id = ANY($1::bigint[])"
	}
	args = append(args, limit)
	limitParam := len(args)
	lockJoin := ""
	lockClause := ""
	if forUpdate {
		lockJoin = "JOIN balance_transactions lock_bt ON lock_bt.id = bt.id"
		lockClause = "\nFOR UPDATE OF lock_bt"
	}
	rows, err := runner.QueryContext(ctx, `
WITH admin_positive AS (
	SELECT
		bt.id,
		bt.user_id,
		bt.balance_delta,
		bt.created_at,
		ROW_NUMBER() OVER (PARTITION BY bt.user_id ORDER BY bt.created_at ASC, bt.id ASC) AS rn
	FROM balance_transactions bt
	WHERE bt.source_type = 'admin_balance'
	  AND bt.balance_delta > 0
),
classified AS (
	SELECT user_id, COALESCE(SUM(remaining_amount), 0) AS remaining_amount
	FROM balance_fund_batches
	WHERE status = 'active'
	GROUP BY user_id
)
SELECT
	bt.user_id,
	u.email,
	bt.id,
	bt.balance_delta::text,
	bt.created_at,
	COALESCE(u.balance, 0)::text,
	COALESCE(c.remaining_amount, 0)::text
FROM admin_positive bt
`+lockJoin+`
JOIN users u ON u.id = bt.user_id
LEFT JOIN classified c ON c.user_id = bt.user_id
WHERE bt.rn = 1
  AND bt.balance_delta = 30.00000000
  AND u.deleted_at IS NULL
  AND NOT EXISTS (
	  SELECT 1
	  FROM balance_fund_batches existing
	  WHERE existing.user_id = bt.user_id
	    AND existing.balance_transaction_id = bt.id
  )
`+filter+`
ORDER BY bt.created_at ASC, bt.id ASC
LIMIT $`+strconv.Itoa(limitParam)+lockClause, args...)
	if err != nil {
		return nil, fmt.Errorf("select signup gift candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	candidates := make([]FundClassificationCandidate, 0)
	for rows.Next() {
		var item FundClassificationCandidate
		var amountRaw, balanceRaw, classifiedRaw string
		if err := rows.Scan(&item.UserID, &item.UserEmail, &item.TransactionID, &amountRaw, &item.CreatedAt, &balanceRaw, &classifiedRaw); err != nil {
			return nil, err
		}
		amount, err := parseLedgerDecimal(amountRaw, "signup gift amount")
		if err != nil {
			return nil, err
		}
		balance, err := parseLedgerDecimal(balanceRaw, "signup gift user balance")
		if err != nil {
			return nil, err
		}
		classified, err := parseLedgerDecimal(classifiedRaw, "classified funds")
		if err != nil {
			return nil, err
		}
		unclassified := balance.Sub(classified)
		if unclassified.IsNegative() {
			unclassified = decimal.Zero
		}
		remaining := decimalMin(amount, unclassified)
		remaining = clampDecimalScale(remaining)
		item.Amount = amount
		item.CurrentBalance = balance
		item.ExistingClassifiedFunds = classified
		item.RecommendedKind = FundSourceKindSignupGift
		item.EstimatedRemaining = remaining
		item.EstimatedConsumed = amount.Sub(remaining)
		candidates = append(candidates, item)
	}
	return candidates, rows.Err()
}

func insertHistoricalSignupGiftBatch(ctx context.Context, runner fundSQLRunner, candidate FundClassificationCandidate, actorUserID int64, reason string, createdAt time.Time) error {
	metadata := map[string]any{
		"classified_by": actorUserID,
		"reason":        reason,
		"historical":    true,
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return ErrFundInvalidInput.WithCause(err)
	}
	status := "active"
	if !candidate.EstimatedRemaining.IsPositive() {
		status = "consumed"
	}
	var batchID int64
	if err := queryOneBalanceLedger(ctx, runner, `
INSERT INTO balance_fund_batches (
	user_id,
	balance_transaction_id,
	source_kind,
	source_type,
	source_id,
	original_amount,
	remaining_amount,
	consumed_amount,
	refunded_amount,
	refund_frozen_amount,
	refundable,
	available_at,
	status,
	metadata,
	created_at,
	updated_at
) VALUES (
	$1, $2, 'signup_gift', 'admin_balance', $3, $4, $5, $6, 0, 0, FALSE, $7, $8, $9::jsonb, $10, NOW()
)
ON CONFLICT (user_id, balance_transaction_id) WHERE balance_transaction_id IS NOT NULL DO NOTHING
RETURNING id`, []any{
		candidate.UserID,
		candidate.TransactionID,
		strconv.FormatInt(candidate.TransactionID, 10),
		decimalString(candidate.Amount),
		decimalString(candidate.EstimatedRemaining),
		decimalString(candidate.EstimatedConsumed),
		candidate.CreatedAt.UTC(),
		status,
		string(raw),
		createdAt.UTC(),
	}, &batchID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("insert historical signup gift batch: %w", err)
	}
	if err := insertFundBatchAllocation(ctx, runner, candidate.UserID, batchID, candidate.TransactionID, fundAllocationReclassify, candidate.Amount, "admin_balance", strconv.FormatInt(candidate.TransactionID, 10), metadata, createdAt); err != nil {
		return err
	}
	return nil
}

func newFundRefundRequestNo() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "FR" + time.Now().UTC().Format("20060102150405") + strings.ToUpper(hex.EncodeToString(buf[:])), nil
}

func pqStringArray(values []string) any {
	return "{" + strings.Join(values, ",") + "}"
}

func pqInt64Array(values []int64) any {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatInt(value, 10))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func timePtrFromNull(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	out := value.Time
	return &out
}
