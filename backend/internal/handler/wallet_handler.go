package handler

import (
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	walletService     *service.WalletService
	withdrawalService *service.WithdrawalService
}

type withdrawalAccountRequest struct {
	Method        string            `json:"method"`
	Currency      string            `json:"currency"`
	RecipientName string            `json:"recipient_name"`
	Details       map[string]string `json:"details"`
}

type withdrawalCreateRequest struct {
	Amount string `json:"amount"`
}

type withdrawalCancelRequest struct {
	Note string `json:"note"`
}

type withdrawalPayoutAccountDTO struct {
	ID                int64  `json:"id"`
	Method            string `json:"method"`
	Currency          string `json:"currency"`
	RecipientNameMask string `json:"recipient_name_mask"`
	AccountMask       string `json:"account_mask"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type withdrawalAvailabilityDTO struct {
	GlobalEnabled        bool   `json:"global_enabled"`
	UserEnabled          bool   `json:"user_enabled"`
	CanApply             bool   `json:"can_apply"`
	DisabledReason       string `json:"disabled_reason,omitempty"`
	RecalcStatus         string `json:"recalc_status"`
	MinimumAmount        string `json:"minimum_amount"`
	DailyLimitAmount     string `json:"daily_limit_amount"`
	DailyUsedAmount      string `json:"daily_used_amount"`
	RemainingDailyAmount string `json:"remaining_daily_amount"`
}

type withdrawalRequestDTO struct {
	ID                      int64                      `json:"id"`
	RequestNo               string                     `json:"request_no"`
	UserID                  int64                      `json:"user_id,omitempty"`
	UserEmail               string                     `json:"user_email,omitempty"`
	Amount                  string                     `json:"amount"`
	Currency                string                     `json:"currency"`
	Status                  string                     `json:"status"`
	PayoutMethod            string                     `json:"payout_method"`
	PayoutCurrency          string                     `json:"payout_currency"`
	PayoutAccountMask       string                     `json:"payout_account_mask"`
	PayoutRecipientNameMask string                     `json:"payout_recipient_name_mask"`
	FirstApprovedBy         *int64                     `json:"first_approved_by,omitempty"`
	FirstApprovedAt         *string                    `json:"first_approved_at,omitempty"`
	SecondApprovedBy        *int64                     `json:"second_approved_by,omitempty"`
	SecondApprovedAt        *string                    `json:"second_approved_at,omitempty"`
	RejectedBy              *int64                     `json:"rejected_by,omitempty"`
	RejectedAt              *string                    `json:"rejected_at,omitempty"`
	RejectedReason          string                     `json:"rejected_reason,omitempty"`
	CanceledAt              *string                    `json:"canceled_at,omitempty"`
	PaidBy                  *int64                     `json:"paid_by,omitempty"`
	PaidAt                  *string                    `json:"paid_at,omitempty"`
	PaidAmount              *string                    `json:"paid_amount,omitempty"`
	PaidCurrency            string                     `json:"paid_currency,omitempty"`
	PayoutFXRate            *string                    `json:"payout_fx_rate,omitempty"`
	ExternalTxnID           string                     `json:"external_txn_id,omitempty"`
	ExternalFeeAmount       string                     `json:"external_fee_amount"`
	PayoutNote              string                     `json:"payout_note,omitempty"`
	CreatedAt               string                     `json:"created_at"`
	UpdatedAt               string                     `json:"updated_at"`
	Events                  []withdrawalStatusEventDTO `json:"events,omitempty"`
}

type withdrawalStatusEventDTO struct {
	ID          int64          `json:"id"`
	Status      string         `json:"status"`
	ActorType   string         `json:"actor_type"`
	ActorUserID *int64         `json:"actor_user_id,omitempty"`
	Note        string         `json:"note,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   string         `json:"created_at"`
}

type withdrawalRequestPageDTO struct {
	Items    []withdrawalRequestDTO `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Pages    int                    `json:"pages"`
}

type walletSummaryDTO struct {
	AvailableBalance           string  `json:"available_balance"`
	WithdrawableBalance        string  `json:"withdrawable_balance"`
	PendingWithdrawableBalance string  `json:"pending_withdrawable_balance"`
	WithdrawalFrozenBalance    string  `json:"withdrawal_frozen_balance"`
	TaskReservedBalance        string  `json:"task_reserved_balance"`
	TotalCredits               string  `json:"total_credits"`
	TotalDebits                string  `json:"total_debits"`
	TransactionCount           int64   `json:"transaction_count"`
	LastTransactionAt          *string `json:"last_transaction_at,omitempty"`
}

type walletTransactionDTO struct {
	ID                    int64  `json:"id"`
	Source                string `json:"source"`
	Direction             string `json:"direction"`
	BalanceDelta          string `json:"balance_delta"`
	FrozenDelta           string `json:"frozen_delta"`
	WithdrawableDelta     string `json:"withdrawable_delta"`
	WithdrawalFrozenDelta string `json:"withdrawal_frozen_delta"`
	BalanceAfter          string `json:"balance_after"`
	FrozenAfter           string `json:"frozen_after"`
	WithdrawableAfter     string `json:"withdrawable_after"`
	WithdrawalFrozenAfter string `json:"withdrawal_frozen_after"`
	CreatedAt             string `json:"created_at"`
}

type walletTransactionPageDTO struct {
	Items    []walletTransactionDTO `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Pages    int                    `json:"pages"`
}

func NewWalletHandler(walletService *service.WalletService, withdrawalService ...*service.WithdrawalService) *WalletHandler {
	var withdrawals *service.WithdrawalService
	if len(withdrawalService) > 0 {
		withdrawals = withdrawalService[0]
	}
	return &WalletHandler{walletService: walletService, withdrawalService: withdrawals}
}

func (h *WalletHandler) Summary(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	summary, err := h.walletService.GetSummary(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toWalletSummaryDTO(summary))
}

func (h *WalletHandler) Transactions(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, err := parseWalletPositiveInt(c.DefaultQuery("page", "1"), "page")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pageSize, err := parseWalletPositiveInt(c.DefaultQuery("page_size", "20"), "page_size")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.walletService.ListTransactions(c.Request.Context(), subject.UserID, service.WalletTransactionQuery{
		Source:   strings.TrimSpace(c.Query("source")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toWalletTransactionPageDTO(result))
}

func (h *WalletHandler) WithdrawalAvailability(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	availability, err := h.withdrawalService.GetAvailability(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toWithdrawalAvailabilityDTO(availability))
}

func (h *WalletHandler) GetWithdrawalAccount(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	account, err := h.withdrawalService.GetCurrentPayoutAccount(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if account == nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, toWithdrawalPayoutAccountDTO(account))
}

func (h *WalletHandler) UpdateWithdrawalAccount(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req withdrawalAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal account request"))
		return
	}
	account, err := h.withdrawalService.UpsertPayoutAccount(c.Request.Context(), service.WithdrawalPayoutAccountInput{
		UserID:        subject.UserID,
		Method:        req.Method,
		Currency:      req.Currency,
		RecipientName: req.RecipientName,
		Details:       req.Details,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, service.AuditActionWithdrawalAccountUpdate)
	middleware.SetAuditExtra(c, map[string]any{
		"result":    "updated",
		"operation": "withdrawal_account_update",
	})
	response.Success(c, toWithdrawalPayoutAccountDTO(account))
}

func (h *WalletHandler) ListWithdrawals(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, err := parseWalletPositiveInt(c.DefaultQuery("page", "1"), "page")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pageSize, err := parseWalletPositiveInt(c.DefaultQuery("page_size", "20"), "page_size")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.withdrawalService.ListUserWithdrawals(c.Request.Context(), subject.UserID, service.WithdrawalListQuery{
		Status:   strings.TrimSpace(c.Query("status")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toWithdrawalRequestPageDTO(result, false))
}

func (h *WalletHandler) GetWithdrawal(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := parseWalletIDParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	req, err := h.withdrawalService.GetWithdrawal(c.Request.Context(), id, subject.UserID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toWithdrawalRequestDTO(req, false))
}

func (h *WalletHandler) CreateWithdrawal(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req withdrawalCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal request"))
		return
	}
	result, err := h.withdrawalService.CreateWithdrawal(c.Request.Context(), service.WithdrawalCreateInput{
		UserID: subject.UserID,
		Amount: req.Amount,
		Locale: c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, service.AuditActionWithdrawalSubmit)
	middleware.SetAuditExtra(c, map[string]any{
		"result":                "submitted",
		"withdrawal_request_id": result.ID,
		"withdrawal_status":     result.Status,
	})
	response.Success(c, toWithdrawalRequestDTO(result, false))
}

func (h *WalletHandler) CancelWithdrawal(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := parseWalletIDParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req withdrawalCancelRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorFrom(c, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal cancel request"))
			return
		}
	}
	result, err := h.withdrawalService.CancelWithdrawal(c.Request.Context(), service.WithdrawalActionInput{
		RequestID: id,
		UserID:    subject.UserID,
		Note:      req.Note,
		Locale:    c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, service.AuditActionWithdrawalCancel)
	middleware.SetAuditExtra(c, map[string]any{
		"result":                "canceled",
		"withdrawal_request_id": result.ID,
		"withdrawal_status":     result.Status,
	})
	response.Success(c, toWithdrawalRequestDTO(result, false))
}

func parseWalletPositiveInt(raw string, field string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, infraerrors.BadRequest("WALLET_INVALID_INPUT", "invalid wallet request").
			WithMetadata(map[string]string{"field": field})
	}
	return value, nil
}

func parseWalletIDParam(c *gin.Context, name string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || value <= 0 {
		return 0, infraerrors.BadRequest("WALLET_INVALID_INPUT", "invalid wallet request").
			WithMetadata(map[string]string{"field": name})
	}
	return value, nil
}

func toWalletSummaryDTO(summary *service.WalletSummary) walletSummaryDTO {
	var last *string
	if summary.LastTransactionAt != nil {
		value := summary.LastTransactionAt.Format(time.RFC3339)
		last = &value
	}
	return walletSummaryDTO{
		AvailableBalance:           summary.AvailableBalance.StringFixed(8),
		WithdrawableBalance:        summary.WithdrawableBalance.StringFixed(8),
		PendingWithdrawableBalance: summary.PendingWithdrawableBalance.StringFixed(8),
		WithdrawalFrozenBalance:    summary.WithdrawalFrozenBalance.StringFixed(8),
		TaskReservedBalance:        summary.TaskReservedBalance.StringFixed(8),
		TotalCredits:               summary.TotalCredits.StringFixed(8),
		TotalDebits:                summary.TotalDebits.StringFixed(8),
		TransactionCount:           summary.TransactionCount,
		LastTransactionAt:          last,
	}
}

func toWalletTransactionPageDTO(page *service.WalletTransactionPage) walletTransactionPageDTO {
	out := walletTransactionPageDTO{
		Items:    make([]walletTransactionDTO, 0, len(page.Items)),
		Total:    page.Total,
		Page:     page.Page,
		PageSize: page.PageSize,
		Pages:    page.Pages,
	}
	for _, item := range page.Items {
		out.Items = append(out.Items, walletTransactionDTO{
			ID:                    item.ID,
			Source:                item.Source,
			Direction:             item.Direction,
			BalanceDelta:          item.BalanceDelta.StringFixed(8),
			FrozenDelta:           item.FrozenDelta.StringFixed(8),
			WithdrawableDelta:     item.WithdrawableDelta.StringFixed(8),
			WithdrawalFrozenDelta: item.WithdrawalFrozenDelta.StringFixed(8),
			BalanceAfter:          item.BalanceAfter.StringFixed(8),
			FrozenAfter:           item.FrozenAfter.StringFixed(8),
			WithdrawableAfter:     item.WithdrawableAfter.StringFixed(8),
			WithdrawalFrozenAfter: item.WithdrawalFrozenAfter.StringFixed(8),
			CreatedAt:             item.CreatedAt.Format(time.RFC3339),
		})
	}
	return out
}

func toWithdrawalAvailabilityDTO(availability *service.WithdrawalAvailability) withdrawalAvailabilityDTO {
	return withdrawalAvailabilityDTO{
		GlobalEnabled:        availability.GlobalEnabled,
		UserEnabled:          availability.UserEnabled,
		CanApply:             availability.CanApply,
		DisabledReason:       availability.DisabledReason,
		RecalcStatus:         availability.RecalcStatus,
		MinimumAmount:        availability.MinimumAmount.StringFixed(8),
		DailyLimitAmount:     availability.DailyLimitAmount.StringFixed(8),
		DailyUsedAmount:      availability.DailyUsedAmount.StringFixed(8),
		RemainingDailyAmount: availability.RemainingDailyAmount.StringFixed(8),
	}
}

func toWithdrawalPayoutAccountDTO(account *service.WithdrawalPayoutAccount) withdrawalPayoutAccountDTO {
	return withdrawalPayoutAccountDTO{
		ID:                account.ID,
		Method:            account.Method,
		Currency:          account.Currency,
		RecipientNameMask: account.RecipientNameMask,
		AccountMask:       account.AccountMask,
		CreatedAt:         account.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         account.UpdatedAt.Format(time.RFC3339),
	}
}

func toWithdrawalRequestPageDTO(page *service.WithdrawalRequestPage, adminView bool) withdrawalRequestPageDTO {
	out := withdrawalRequestPageDTO{
		Items:    make([]withdrawalRequestDTO, 0, len(page.Items)),
		Total:    page.Total,
		Page:     page.Page,
		PageSize: page.PageSize,
		Pages:    page.Pages,
	}
	for i := range page.Items {
		out.Items = append(out.Items, toWithdrawalRequestDTO(&page.Items[i], adminView))
	}
	return out
}

func toWithdrawalRequestDTO(req *service.WithdrawalRequest, adminView bool) withdrawalRequestDTO {
	dto := withdrawalRequestDTO{
		ID:                      req.ID,
		RequestNo:               req.RequestNo,
		Amount:                  req.Amount.StringFixed(8),
		Currency:                req.Currency,
		Status:                  req.Status,
		PayoutMethod:            req.PayoutMethod,
		PayoutCurrency:          req.PayoutCurrency,
		PayoutAccountMask:       req.PayoutAccountMask,
		PayoutRecipientNameMask: req.PayoutRecipientNameMask,
		FirstApprovedBy:         req.FirstApprovedBy,
		SecondApprovedBy:        req.SecondApprovedBy,
		RejectedBy:              req.RejectedBy,
		RejectedReason:          req.RejectedReason,
		PaidBy:                  req.PaidBy,
		PaidCurrency:            req.PaidCurrency,
		ExternalTxnID:           req.ExternalTxnID,
		ExternalFeeAmount:       req.ExternalFeeAmount.StringFixed(8),
		PayoutNote:              req.PayoutNote,
		CreatedAt:               req.CreatedAt.Format(time.RFC3339),
		UpdatedAt:               req.UpdatedAt.Format(time.RFC3339),
		Events:                  make([]withdrawalStatusEventDTO, 0, len(req.Events)),
	}
	if adminView {
		dto.UserID = req.UserID
		dto.UserEmail = req.UserEmail
	}
	dto.FirstApprovedAt = formatOptionalTime(req.FirstApprovedAt)
	dto.SecondApprovedAt = formatOptionalTime(req.SecondApprovedAt)
	dto.RejectedAt = formatOptionalTime(req.RejectedAt)
	dto.CanceledAt = formatOptionalTime(req.CanceledAt)
	dto.PaidAt = formatOptionalTime(req.PaidAt)
	if req.PaidAmount != nil {
		value := req.PaidAmount.StringFixed(8)
		dto.PaidAmount = &value
	}
	if req.PayoutFXRate != nil {
		value := req.PayoutFXRate.StringFixed(8)
		dto.PayoutFXRate = &value
	}
	for _, event := range req.Events {
		dto.Events = append(dto.Events, withdrawalStatusEventDTO{
			ID:          event.ID,
			Status:      event.Status,
			ActorType:   event.ActorType,
			ActorUserID: event.ActorUserID,
			Note:        event.Note,
			Metadata:    event.Metadata,
			CreatedAt:   event.CreatedAt.Format(time.RFC3339),
		})
	}
	return dto
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	out := value.Format(time.RFC3339)
	return &out
}
