package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	FundOperationKindOfflineRecharge   = "offline_recharge"
	FundOperationKindOpsGift           = "ops_gift"
	FundOperationKindCompensation      = "compensation"
	FundOperationKindRefund            = "refund"
	FundOperationKindReversal          = "reversal"
	FundOperationKindAccountCorrection = "account_correction"

	FundOperationStatusCompleted                  = "completed"
	FundOperationStatusPending                    = "pending"
	FundOperationStatusCanceled                   = "canceled"
	FundOperationStatusPendingInsufficientBalance = "pending_insufficient_balance"
)

var (
	ErrFundAccountNotFound             = infraerrors.NotFound("FUND_ACCOUNT_NOT_FOUND", "account was not found")
	ErrFundAccountInactive             = infraerrors.BadRequest("FUND_ACCOUNT_INACTIVE", "account is not active")
	ErrFundCorrectionNotFound          = infraerrors.NotFound("FUND_OPERATION_NOT_FOUND", "fund operation was not found")
	ErrFundCorrectionInvalid           = infraerrors.BadRequest("FUND_CORRECTION_INVALID", "fund operation cannot be corrected")
	ErrFundCorrectionOfflineRecharge   = infraerrors.BadRequest("FUND_CORRECTION_OFFLINE_RECHARGE_REQUIRES_MANUAL_REVIEW", "offline recharge account corrections require manual financial review")
	ErrFundExternalRefUsed             = infraerrors.Conflict("FUND_EXTERNAL_REF_ALREADY_USED", "offline recharge reference was already used")
	ErrFundCorrectionAlreadyActive     = infraerrors.Conflict("FUND_CORRECTION_ALREADY_ACTIVE", "a correction is already pending or completed for this operation")
	ErrFundCorrectionNotPending        = infraerrors.BadRequest("FUND_CORRECTION_NOT_PENDING", "fund correction is not pending")
	ErrFundCorrectionStillInsufficient = infraerrors.BadRequest("FUND_CORRECTION_SOURCE_BALANCE_INSUFFICIENT", "source account still has insufficient balance")
)

// FundAccount is deliberately presentation-safe. Internal IDs remain only in
// the service layer and never appear in fund-management JSON responses.
type FundAccount struct {
	Email          string          `json:"email"`
	Username       string          `json:"username,omitempty"`
	Status         string          `json:"status"`
	CurrentBalance decimal.Decimal `json:"current_balance"`
}

type FundOperationRecord struct {
	OperationNo         string           `json:"operation_no"`
	OperationKind       string           `json:"operation_kind"`
	Status              string           `json:"status"`
	AccountEmail        string           `json:"account_email"`
	AccountUsername     string           `json:"account_username,omitempty"`
	ActorAccountEmail   string           `json:"actor_account_email,omitempty"`
	Amount              decimal.Decimal  `json:"amount"`
	Currency            string           `json:"currency"`
	Reason              string           `json:"reason,omitempty"`
	Note                string           `json:"note,omitempty"`
	ExternalRefMasked   string           `json:"external_ref_masked,omitempty"`
	ExternalRef         string           `json:"external_ref,omitempty"`
	BalanceBefore       *decimal.Decimal `json:"balance_before,omitempty"`
	BalanceAfter        *decimal.Decimal `json:"balance_after,omitempty"`
	MembershipEffect    string           `json:"membership_effect,omitempty"`
	OriginalOperationNo string           `json:"original_operation_no,omitempty"`
	RelatedOperationNo  string           `json:"related_operation_no,omitempty"`
	CreatedAt           time.Time        `json:"created_at"`
	CompletedAt         *time.Time       `json:"completed_at,omitempty"`
}

type FundOperationListQuery struct {
	Kind            string
	Status          string
	AccountKeyword  string
	OperatorKeyword string
	Keyword         string
	StartAt         *time.Time
	EndAt           *time.Time
	Page            int
	PageSize        int
}

type FundOperationPage struct {
	Items    []FundOperationRecord `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Pages    int                   `json:"pages"`
}

type FundAccountCreditInput struct {
	AccountEmail string
	Amount       string
	Reason       string
	ExternalRef  string
	Kind         string
	ActorUserID  int64
}

type FundOperationCorrectionInput struct {
	OperationNo         string
	CorrectAccountEmail string
	Reason              string
	ActorUserID         int64
}

type FundPendingCorrectionActionInput struct {
	OperationNo string
	ActorUserID int64
}

func maskFundOperationReference(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) <= 3 {
		if raw == "" {
			return ""
		}
		return "***"
	}
	parts := strings.Split(raw, "-")
	if len(parts) >= 2 {
		return parts[0] + "-***-" + parts[len(parts)-1]
	}
	return raw[:1] + "***" + raw[len(raw)-3:]
}

func (s *FundManagementService) SearchFundAccounts(ctx context.Context, keyword string, limit int) (accounts []FundAccount, err error) {
	if s == nil || s.db == nil {
		return nil, ErrFundManagementUnavailable
	}
	keyword = strings.TrimSpace(keyword)
	if len([]rune(keyword)) < 2 {
		return []FundAccount{}, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT u.email, COALESCE(u.username,''), u.status, COALESCE(u.balance, 0)::text
FROM users u
WHERE u.deleted_at IS NULL AND (u.email ILIKE '%' || $1 || '%' OR COALESCE(u.username,'') ILIKE '%' || $1 || '%')
ORDER BY CASE WHEN lower(u.email) = lower($1) THEN 0 ELSE 1 END, u.id DESC
LIMIT $2`, keyword, limit)
	if err != nil {
		return nil, fmt.Errorf("search fund accounts: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close fund account search rows: %w", closeErr)
		}
	}()
	accounts = make([]FundAccount, 0)
	for rows.Next() {
		var account FundAccount
		var balance string
		if err := rows.Scan(&account.Email, &account.Username, &account.Status, &balance); err != nil {
			return nil, err
		}
		account.CurrentBalance, err = decimal.NewFromString(balance)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s *FundManagementService) GrantFundCreditByAccount(ctx context.Context, input FundAccountCreditInput) (*FundOperationRecord, error) {
	if s == nil || s.db == nil || s.ledger == nil {
		return nil, ErrFundManagementUnavailable
	}
	amount, err := parseFundCreditAmount(input.Amount)
	if err != nil {
		return nil, err
	}
	kind := strings.TrimSpace(input.Kind)
	if kind != FundOperationKindOpsGift && kind != FundOperationKindCompensation && kind != FundOperationKindOfflineRecharge {
		return nil, ErrFundInvalidInput
	}
	reason := strings.TrimSpace(input.Reason)
	email := strings.TrimSpace(input.AccountEmail)
	ref := strings.TrimSpace(input.ExternalRef)
	if input.ActorUserID <= 0 || email == "" || len([]rune(reason)) < 3 || len([]rune(reason)) > 500 || len([]rune(ref)) > 160 || (kind == FundOperationKindOfflineRecharge && ref == "") {
		return nil, ErrFundInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin fund credit: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if kind == FundOperationKindOfflineRecharge {
		var alreadyUsed bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
			SELECT 1 FROM fund_operation_records
			WHERE operation_kind='offline_recharge' AND root_external_ref=$1
		)`, ref).Scan(&alreadyUsed); err != nil {
			return nil, fmt.Errorf("check offline recharge reference: %w", err)
		}
		if alreadyUsed {
			return nil, ErrFundExternalRefUsed
		}
	}
	userID, account, err := lockFundAccountByEmail(ctx, tx, email)
	if err != nil {
		return nil, err
	}
	if account.Status != StatusActive {
		return nil, ErrFundAccountInactive
	}
	operationNo, err := newFundOperationNo(s.now().UTC())
	if err != nil {
		return nil, err
	}
	sourceType, description := FundLedgerSourceOpsGift, fundOpsGiftDescription
	if kind == FundOperationKindCompensation {
		sourceType, description = "compensation", "管理员补偿余额"
	}
	if kind == FundOperationKindOfflineRecharge {
		sourceType, description = FundLedgerSourceOfflineRecharge, fundOfflineRechargeDescription
	}
	sourceID := operationNo
	if ref != "" {
		sourceID = ref
	}
	ledgerTx, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{UserID: userID, BalanceDelta: decimalToLedgerFloat(amount), SourceType: sourceType, SourceID: sourceID, IdempotencyKey: sourceType + ":" + sourceID, ActorType: BalanceLedgerActorAdmin, ActorUserID: &input.ActorUserID, Description: description, Metadata: map[string]any{"reason": reason, "external_ref": ref, "fund_operation_no": operationNo}})
	if err != nil {
		return nil, err
	}
	if kind == FundOperationKindOfflineRecharge {
		if err := s.recordOfflineMembershipContribution(ctx, tx, userID, input.ActorUserID, ledgerTx.ID, amount, ref, reason); err != nil {
			return nil, err
		}
	}
	ledgerTransactionID := ledgerTx.ID
	record, _, err := insertFundOperationRecord(ctx, tx, fundOperationInsert{OperationNo: operationNo, Kind: kind, Status: FundOperationStatusCompleted, UserID: userID, ActorUserID: input.ActorUserID, Amount: amount, Reason: reason, ExternalRef: ref, RootExternalRef: ref, BalanceTransactionID: &ledgerTransactionID, CreatedAt: s.now().UTC()})
	if err != nil {
		if kind == FundOperationKindOfflineRecharge && isFundOperationUniqueViolation(err) {
			return nil, ErrFundExternalRefUsed
		}
		return nil, err
	}
	record.AccountEmail, record.AccountUsername = account.Email, account.Username
	if ledgerTx.BalanceBefore != nil {
		before := decimal.NewFromFloat(*ledgerTx.BalanceBefore)
		record.BalanceBefore = &before
	}
	if ledgerTx.BalanceAfter != nil {
		after := decimal.NewFromFloat(*ledgerTx.BalanceAfter)
		record.BalanceAfter = &after
	}
	if kind == FundOperationKindOfflineRecharge {
		record.MembershipEffect = "offline_recharge"
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit fund credit: %w", err)
	}
	committed = true
	s.ledger.InvalidateUserBalanceCaches(ctx, userID)
	return record, nil
}

func isFundOperationUniqueViolation(err error) bool {
	var stateErr interface{ SQLState() string }
	return errors.As(err, &stateErr) && stateErr.SQLState() == "23505"
}

func (s *FundManagementService) ListFundOperations(ctx context.Context, query FundOperationListQuery) (pageResult *FundOperationPage, err error) {
	if s == nil || s.db == nil {
		return nil, ErrFundManagementUnavailable
	}
	page, pageSize := query.Page, query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	where := []string{"1=1"}
	args := make([]any, 0)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if strings.TrimSpace(query.Kind) != "" && query.Kind != "all" {
		add("forr.operation_kind = $%d", query.Kind)
	}
	if strings.TrimSpace(query.Status) != "" && query.Status != "all" {
		add("forr.status = $%d", query.Status)
	}
	if strings.TrimSpace(query.AccountKeyword) != "" {
		add("(u.email ILIKE '%%' || $%d || '%%' OR COALESCE(u.username,'') ILIKE '%%' || $%d || '%%')", query.AccountKeyword)
	}
	if strings.TrimSpace(query.OperatorKeyword) != "" {
		add("actor.email ILIKE '%%' || $%d || '%%'", query.OperatorKeyword)
	}
	if strings.TrimSpace(query.Keyword) != "" {
		add("(forr.operation_no ILIKE '%%' || $%d || '%%' OR forr.external_ref ILIKE '%%' || $%d || '%%' OR forr.reason ILIKE '%%' || $%d || '%%' OR forr.note ILIKE '%%' || $%d || '%%')", query.Keyword)
	}
	if query.StartAt != nil {
		add("forr.created_at >= $%d", query.StartAt.UTC())
	}
	if query.EndAt != nil {
		add("forr.created_at <= $%d", query.EndAt.UTC())
	}
	whereSQL := strings.Join(where, " AND ")
	countArgs := append([]any{}, args...)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM fund_operation_records forr JOIN users u ON u.id=forr.target_user_id LEFT JOIN users actor ON actor.id=forr.actor_user_id WHERE `+whereSQL, countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count fund operations: %w", err)
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, fundOperationSelect+` WHERE `+whereSQL+fmt.Sprintf(` ORDER BY forr.created_at DESC, forr.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("list fund operations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close fund operation rows: %w", closeErr)
		}
	}()
	items := make([]FundOperationRecord, 0)
	for rows.Next() {
		record, err := scanFundOperationRecord(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return &FundOperationPage{Items: items, Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func (s *FundManagementService) GetFundOperation(ctx context.Context, operationNo string, includeSensitive bool) (record *FundOperationRecord, err error) {
	if s == nil || s.db == nil {
		return nil, ErrFundManagementUnavailable
	}
	rows, err := s.db.QueryContext(ctx, fundOperationSelect+` WHERE forr.operation_no=$1`, strings.TrimSpace(operationNo))
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close fund operation rows: %w", closeErr)
		}
	}()
	if !rows.Next() {
		return nil, ErrFundCorrectionNotFound
	}
	record, err = scanFundOperationRecord(rows.Scan)
	if err != nil {
		return nil, err
	}
	if !includeSensitive {
		record.ExternalRef = ""
	}
	return record, rows.Err()
}

func (s *FundManagementService) CorrectFundOperationAccount(ctx context.Context, input FundOperationCorrectionInput) (*FundOperationRecord, error) {
	if s == nil || s.db == nil || s.ledger == nil {
		return nil, ErrFundManagementUnavailable
	}
	if input.ActorUserID <= 0 || strings.TrimSpace(input.OperationNo) == "" || strings.TrimSpace(input.CorrectAccountEmail) == "" || len([]rune(strings.TrimSpace(input.Reason))) < 3 {
		return nil, ErrFundInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	var originalID, wrongUserID int64
	var kind, status, ref string
	var amountRaw string
	err = tx.QueryRowContext(ctx, `SELECT id,target_user_id,operation_kind,status,external_ref,amount::text FROM fund_operation_records WHERE operation_no=$1 FOR UPDATE`, input.OperationNo).Scan(&originalID, &wrongUserID, &kind, &status, &ref, &amountRaw)
	if err == sql.ErrNoRows {
		return nil, ErrFundCorrectionNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != FundOperationStatusCompleted || (kind != FundOperationKindOfflineRecharge && kind != FundOperationKindOpsGift && kind != FundOperationKindCompensation) {
		return nil, ErrFundCorrectionInvalid
	}
	// An offline recharge carries a refundable fund batch and a Play membership
	// contribution. Moving only the balance is insufficient and can make a later
	// refund or membership history inconsistent, so it stays a finance-review
	// operation until that transfer has its own complete transaction contract.
	if kind == FundOperationKindOfflineRecharge {
		return nil, ErrFundCorrectionOfflineRecharge
	}
	amount, err := decimal.NewFromString(amountRaw)
	if err != nil {
		return nil, err
	}
	wrongID, wrong, err := lockFundAccountByID(ctx, tx, wrongUserID)
	if err != nil {
		return nil, err
	}
	correctID, correct, err := lockFundAccountByEmail(ctx, tx, input.CorrectAccountEmail)
	if err != nil {
		return nil, err
	}
	if wrongID == correctID {
		return nil, ErrFundCorrectionInvalid
	}
	if correct.Status != StatusActive {
		return nil, ErrFundAccountInactive
	}
	var activeCorrection bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM fund_operation_corrections
		WHERE original_operation_id=$1 AND status IN ('pending_insufficient_balance','completed')
	)`, originalID).Scan(&activeCorrection); err != nil {
		return nil, err
	}
	if activeCorrection {
		return nil, ErrFundCorrectionAlreadyActive
	}
	correctionNo, err := newFundCorrectionNo(s.now().UTC())
	if err != nil {
		return nil, err
	}
	if wrong.CurrentBalance.LessThan(amount) {
		operationNo, err := newFundOperationNo(s.now().UTC())
		if err != nil {
			return nil, err
		}
		pending, pendingID, err := insertFundOperationRecord(ctx, tx, fundOperationInsert{OperationNo: operationNo, Kind: FundOperationKindAccountCorrection, Status: FundOperationStatusPendingInsufficientBalance, UserID: correctID, ActorUserID: input.ActorUserID, Amount: amount, Reason: strings.TrimSpace(input.Reason), ExternalRef: ref, RootExternalRef: ref, OriginalOperationID: &originalID, CreatedAt: s.now().UTC()})
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO fund_operation_corrections (correction_no,original_operation_id,corrected_user_id,actor_user_id,amount,status,reason,operation_record_id) VALUES ($1,$2,$3,$4,$5,'pending_insufficient_balance',$6,$7)`, correctionNo, originalID, correctID, input.ActorUserID, decimalString(amount), strings.TrimSpace(input.Reason), pendingID); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		committed = true
		pending.AccountEmail, pending.AccountUsername = correct.Email, correct.Username
		pending.OriginalOperationNo = input.OperationNo
		return pending, nil
	}
	debit, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{UserID: wrongID, BalanceDelta: -decimalToLedgerFloat(amount), SourceType: "fund_account_correction", SourceID: correctionNo + ":debit", IdempotencyKey: "fund_account_correction:" + correctionNo + ":debit", ActorType: BalanceLedgerActorAdmin, ActorUserID: &input.ActorUserID, Description: "错充账号纠正扣减", Metadata: map[string]any{"original_operation_no": input.OperationNo, "correction_no": correctionNo, "reason": strings.TrimSpace(input.Reason)}})
	if err != nil {
		return nil, err
	}
	credit, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{UserID: correctID, BalanceDelta: decimalToLedgerFloat(amount), SourceType: kind, SourceID: correctionNo + ":credit", IdempotencyKey: "fund_account_correction:" + correctionNo + ":credit", ActorType: BalanceLedgerActorAdmin, ActorUserID: &input.ActorUserID, Description: "错充账号纠正入账", Metadata: map[string]any{"original_operation_no": input.OperationNo, "correction_no": correctionNo, "reason": strings.TrimSpace(input.Reason)}})
	if err != nil {
		return nil, err
	}
	operationNo, err := newFundOperationNo(s.now().UTC())
	if err != nil {
		return nil, err
	}
	creditTransactionID := credit.ID
	record, recordID, err := insertFundOperationRecord(ctx, tx, fundOperationInsert{OperationNo: operationNo, Kind: FundOperationKindAccountCorrection, Status: FundOperationStatusCompleted, UserID: correctID, ActorUserID: input.ActorUserID, Amount: amount, Reason: strings.TrimSpace(input.Reason), ExternalRef: ref, RootExternalRef: ref, BalanceTransactionID: &creditTransactionID, OriginalOperationID: &originalID, CreatedAt: s.now().UTC()})
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO fund_operation_corrections (correction_no,original_operation_id,corrected_user_id,actor_user_id,amount,status,reason,debit_balance_transaction_id,credit_balance_transaction_id,operation_record_id,completed_at) VALUES ($1,$2,$3,$4,$5,'completed',$6,$7,$8,$9,NOW())`, correctionNo, originalID, correctID, input.ActorUserID, decimalString(amount), strings.TrimSpace(input.Reason), debit.ID, credit.ID, recordID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	s.ledger.InvalidateUserBalanceCaches(ctx, wrongID)
	s.ledger.InvalidateUserBalanceCaches(ctx, correctID)
	record.AccountEmail, record.AccountUsername = correct.Email, correct.Username
	before := decimal.NewFromFloat(*credit.BalanceBefore)
	after := decimal.NewFromFloat(*credit.BalanceAfter)
	record.BalanceBefore = &before
	record.BalanceAfter = &after
	record.OriginalOperationNo = input.OperationNo
	return record, nil
}

type pendingFundCorrection struct {
	ID                  int64
	CorrectionNo        string
	OriginalOperationID int64
	CorrectedUserID     int64
	Amount              decimal.Decimal
	Reason              string
	OperationRecordID   int64
	OperationNo         string
	WrongUserID         int64
	OriginalKind        string
	ExternalRef         string
	OriginalOperationNo string
}

func (s *FundManagementService) RetryPendingFundCorrection(ctx context.Context, operationNo string, actorUserID int64) (*FundOperationRecord, error) {
	if s == nil || s.db == nil || s.ledger == nil || actorUserID <= 0 || strings.TrimSpace(operationNo) == "" {
		return nil, ErrFundInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	pending, err := loadPendingFundCorrectionForUpdate(ctx, tx, operationNo)
	if err != nil {
		return nil, err
	}
	wrongID, wrong, err := lockFundAccountByID(ctx, tx, pending.WrongUserID)
	if err != nil {
		return nil, err
	}
	correctID, correct, err := lockFundAccountByID(ctx, tx, pending.CorrectedUserID)
	if err != nil {
		return nil, err
	}
	if wrongID == correctID || correct.Status != StatusActive {
		return nil, ErrFundCorrectionInvalid
	}
	if wrong.CurrentBalance.LessThan(pending.Amount) {
		return nil, ErrFundCorrectionStillInsufficient
	}
	actor := actorUserID
	debit, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{UserID: wrongID, BalanceDelta: -decimalToLedgerFloat(pending.Amount), SourceType: "fund_account_correction", SourceID: pending.CorrectionNo + ":debit", IdempotencyKey: "fund_account_correction:" + pending.CorrectionNo + ":debit", ActorType: BalanceLedgerActorAdmin, ActorUserID: &actor, Description: "错充账号纠正扣减", Metadata: map[string]any{"original_operation_no": pending.OriginalOperationNo, "correction_no": pending.CorrectionNo, "reason": pending.Reason}})
	if err != nil {
		return nil, err
	}
	credit, err := s.ledger.ApplyDeltaInSQLTx(ctx, tx, BalanceLedgerApplyInput{UserID: correctID, BalanceDelta: decimalToLedgerFloat(pending.Amount), SourceType: pending.OriginalKind, SourceID: pending.CorrectionNo + ":credit", IdempotencyKey: "fund_account_correction:" + pending.CorrectionNo + ":credit", ActorType: BalanceLedgerActorAdmin, ActorUserID: &actor, Description: "错充账号纠正入账", Metadata: map[string]any{"original_operation_no": pending.OriginalOperationNo, "correction_no": pending.CorrectionNo, "reason": pending.Reason}})
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE fund_operation_records SET status='completed', balance_transaction_id=$1, completed_at=NOW(), updated_at=NOW() WHERE id=$2 AND status='pending_insufficient_balance'`, credit.ID, pending.OperationRecordID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE fund_operation_corrections SET status='completed', actor_user_id=$1, debit_balance_transaction_id=$2, credit_balance_transaction_id=$3, completed_at=NOW(), updated_at=NOW() WHERE id=$4 AND status='pending_insufficient_balance'`, actorUserID, debit.ID, credit.ID, pending.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	s.ledger.InvalidateUserBalanceCaches(ctx, wrongID)
	s.ledger.InvalidateUserBalanceCaches(ctx, correctID)
	before := decimal.NewFromFloat(*credit.BalanceBefore)
	after := decimal.NewFromFloat(*credit.BalanceAfter)
	completedAt := s.now().UTC()
	return &FundOperationRecord{OperationNo: pending.OperationNo, OperationKind: FundOperationKindAccountCorrection, Status: FundOperationStatusCompleted, AccountEmail: correct.Email, AccountUsername: correct.Username, Amount: pending.Amount, Currency: "USD", Reason: pending.Reason, ExternalRefMasked: maskFundOperationReference(pending.ExternalRef), BalanceBefore: &before, BalanceAfter: &after, OriginalOperationNo: pending.OriginalOperationNo, CompletedAt: &completedAt, CreatedAt: completedAt}, nil
}

func (s *FundManagementService) CancelPendingFundCorrection(ctx context.Context, operationNo string, actorUserID int64) (*FundOperationRecord, error) {
	if s == nil || s.db == nil || actorUserID <= 0 || strings.TrimSpace(operationNo) == "" {
		return nil, ErrFundInvalidInput
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	pending, err := loadPendingFundCorrectionForUpdate(ctx, tx, operationNo)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE fund_operation_records SET status='canceled', updated_at=NOW() WHERE id=$1 AND status='pending_insufficient_balance'`, pending.OperationRecordID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE fund_operation_corrections SET status='canceled', actor_user_id=$1, updated_at=NOW() WHERE id=$2 AND status='pending_insufficient_balance'`, actorUserID, pending.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return &FundOperationRecord{OperationNo: pending.OperationNo, OperationKind: FundOperationKindAccountCorrection, Status: FundOperationStatusCanceled, Amount: pending.Amount, Currency: "USD", Reason: pending.Reason, ExternalRefMasked: maskFundOperationReference(pending.ExternalRef), OriginalOperationNo: pending.OriginalOperationNo, CreatedAt: s.now().UTC()}, nil
}

func loadPendingFundCorrectionForUpdate(ctx context.Context, tx *sql.Tx, operationNo string) (*pendingFundCorrection, error) {
	pending := &pendingFundCorrection{}
	var amount string
	err := tx.QueryRowContext(ctx, `SELECT c.id,c.correction_no,c.original_operation_id,c.corrected_user_id,c.amount::text,c.reason,pending.id,pending.operation_no,original.target_user_id,original.operation_kind,original.external_ref,original.operation_no
		FROM fund_operation_corrections c
		JOIN fund_operation_records pending ON pending.id=c.operation_record_id
		JOIN fund_operation_records original ON original.id=c.original_operation_id
		WHERE pending.operation_no=$1 AND c.status='pending_insufficient_balance' AND pending.status='pending_insufficient_balance'
		FOR UPDATE OF c,pending,original`, strings.TrimSpace(operationNo)).Scan(&pending.ID, &pending.CorrectionNo, &pending.OriginalOperationID, &pending.CorrectedUserID, &amount, &pending.Reason, &pending.OperationRecordID, &pending.OperationNo, &pending.WrongUserID, &pending.OriginalKind, &pending.ExternalRef, &pending.OriginalOperationNo)
	if err == sql.ErrNoRows {
		return nil, ErrFundCorrectionNotPending
	}
	if err != nil {
		return nil, err
	}
	parsed, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	pending.Amount = parsed
	if pending.OriginalKind != FundOperationKindOpsGift && pending.OriginalKind != FundOperationKindCompensation {
		return nil, ErrFundCorrectionInvalid
	}
	return pending, nil
}

type fundOperationInsert struct {
	OperationNo, Kind, Status            string
	UserID, ActorUserID                  int64
	Amount                               decimal.Decimal
	Reason, ExternalRef, RootExternalRef string
	BalanceTransactionID                 *int64
	OriginalOperationID                  *int64
	CreatedAt                            time.Time
}

func insertFundOperationRecord(ctx context.Context, tx *sql.Tx, input fundOperationInsert) (*FundOperationRecord, int64, error) {
	var id int64
	var completedAt any
	if input.Status == FundOperationStatusCompleted {
		completedAt = input.CreatedAt
	}
	err := tx.QueryRowContext(ctx, `INSERT INTO fund_operation_records (operation_no,operation_kind,status,target_user_id,actor_user_id,amount,currency,reason,external_ref,root_external_ref,balance_transaction_id,original_operation_id,created_at,completed_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,'USD',$7,$8,$9,$10,$11,$12,$13,$12) RETURNING id`, input.OperationNo, input.Kind, input.Status, input.UserID, input.ActorUserID, decimalString(input.Amount), input.Reason, input.ExternalRef, input.RootExternalRef, input.BalanceTransactionID, input.OriginalOperationID, input.CreatedAt, completedAt).Scan(&id)
	if err != nil {
		return nil, 0, fmt.Errorf("insert fund operation: %w", err)
	}
	record := &FundOperationRecord{OperationNo: input.OperationNo, OperationKind: input.Kind, Status: input.Status, Amount: input.Amount, Currency: "USD", Reason: input.Reason, ExternalRefMasked: maskFundOperationReference(input.ExternalRef), CreatedAt: input.CreatedAt}
	if input.Status == FundOperationStatusCompleted {
		completed := input.CreatedAt
		record.CompletedAt = &completed
	}
	return record, id, nil
}
func lockFundAccountByEmail(ctx context.Context, tx *sql.Tx, email string) (int64, FundAccount, error) {
	return lockFundAccount(ctx, tx, `WHERE lower(email)=lower($1)`, email)
}
func lockFundAccountByID(ctx context.Context, tx *sql.Tx, id int64) (int64, FundAccount, error) {
	return lockFundAccount(ctx, tx, `WHERE id=$1`, id)
}
func lockFundAccount(ctx context.Context, tx *sql.Tx, clause string, arg any) (int64, FundAccount, error) {
	var id int64
	var a FundAccount
	var raw string
	err := tx.QueryRowContext(ctx, `SELECT id,email,COALESCE(username,''),status,COALESCE(balance,0)::text FROM users `+clause+` AND deleted_at IS NULL FOR UPDATE`, arg).Scan(&id, &a.Email, &a.Username, &a.Status, &raw)
	if err == sql.ErrNoRows {
		return 0, a, ErrFundAccountNotFound
	}
	if err != nil {
		return 0, a, err
	}
	a.CurrentBalance, err = decimal.NewFromString(raw)
	return id, a, err
}
func newFundOperationNo(now time.Time) (string, error)  { return newFundPublicNo("FM", now) }
func newFundCorrectionNo(now time.Time) (string, error) { return newFundPublicNo("FMC", now) }
func newFundPublicNo(prefix string, now time.Time) (string, error) {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s", prefix, now.UTC().Format("20060102"), strings.ToUpper(hex.EncodeToString(raw))), nil
}

const fundOperationSelect = `SELECT forr.operation_no,forr.operation_kind,forr.status,u.email,COALESCE(u.username,''),COALESCE(actor.email,''),forr.amount::text,forr.currency,forr.reason,forr.note,forr.external_ref,bt.balance_before::text,bt.balance_after::text,CASE WHEN forr.operation_kind='offline_recharge' THEN 'offline_recharge' ELSE '' END,COALESCE(original.operation_no,''),COALESCE(related.operation_no,''),forr.created_at,forr.completed_at FROM fund_operation_records forr JOIN users u ON u.id=forr.target_user_id LEFT JOIN users actor ON actor.id=forr.actor_user_id LEFT JOIN balance_transactions bt ON bt.id=forr.balance_transaction_id LEFT JOIN fund_operation_records original ON original.id=forr.original_operation_id LEFT JOIN fund_operation_records related ON related.id=forr.related_operation_id`

func scanFundOperationRecord(scan func(...any) error) (*FundOperationRecord, error) {
	var r FundOperationRecord
	var amount, before, after sql.NullString
	var completed sql.NullTime
	var ref string
	if err := scan(&r.OperationNo, &r.OperationKind, &r.Status, &r.AccountEmail, &r.AccountUsername, &r.ActorAccountEmail, &amount, &r.Currency, &r.Reason, &r.Note, &ref, &before, &after, &r.MembershipEffect, &r.OriginalOperationNo, &r.RelatedOperationNo, &r.CreatedAt, &completed); err != nil {
		return nil, err
	}
	var err error
	r.Amount, err = decimal.NewFromString(amount.String)
	if err != nil {
		return nil, err
	}
	r.ExternalRefMasked = maskFundOperationReference(ref)
	if before.Valid {
		v, e := decimal.NewFromString(before.String)
		if e != nil {
			return nil, e
		}
		r.BalanceBefore = &v
	}
	if after.Valid {
		v, e := decimal.NewFromString(after.String)
		if e != nil {
			return nil, e
		}
		r.BalanceAfter = &v
	}
	if completed.Valid {
		v := completed.Time
		r.CompletedAt = &v
	}
	return &r, nil
}

func (s *FundManagementService) recordOfflineMembershipContribution(ctx context.Context, tx *sql.Tx, userID, actorID, ledgerTxID int64, amount decimal.Decimal, ref, reason string) error {
	before, err := membershipPaidTotalInTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO play_membership_manual_contributions (user_id,balance_transaction_id,source_type,external_ref,paid_amount,net_amount,currency,qualification_state,qualification_source,qualification_reason,reviewed_by,reviewed_at,paid_at) VALUES ($1,$2,$3,$4,$5,$5,'CNY','verified','manual_review',$6,$7,NOW(),NOW())`, userID, ledgerTxID, FundLedgerSourceOfflineRecharge, ref, decimalString(amount), reason, actorID); err != nil {
		return fmt.Errorf("record offline membership contribution: %w", err)
	}
	after, err := membershipPaidTotalInTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	from := resolveVIPStatus(before.InexactFloat64(), defaultPlayVIPTiers()).Tier
	to := resolveVIPStatus(after.InexactFloat64(), defaultPlayVIPTiers()).Tier
	if from != to {
		_, err = tx.ExecContext(ctx, `INSERT INTO play_membership_tier_history (user_id,from_tier,to_tier,net_paid_before,net_paid_after,reason) VALUES ($1,$2,$3,$4,$5,'offline_recharge')`, userID, from, to, before, after)
		if err != nil {
			return err
		}
	}
	return nil
}
