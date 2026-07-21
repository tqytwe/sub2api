package admin

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

type WithdrawalHandler struct {
	withdrawalService            *service.WithdrawalService
	withdrawableRecomputeService *service.WithdrawableRecomputeService
}

type adminWithdrawalSystemSettingsRequest struct {
	GlobalEnabled         *bool   `json:"global_enabled"`
	MinimumAmount         *string `json:"minimum_amount"`
	DailyLimitAmount      *string `json:"daily_limit_amount"`
	DoubleReviewThreshold *string `json:"double_review_threshold"`
}

type adminWithdrawalUserSettingsRequest struct {
	Enabled                  *bool   `json:"enabled"`
	MinimumAmountOverride    *string `json:"minimum_amount_override"`
	MinimumOverride          *string `json:"minimum_override"`
	DailyLimitAmountOverride *string `json:"daily_limit_amount_override"`
	DailyLimitOverride       *string `json:"daily_limit_override"`
	DisabledReason           *string `json:"disabled_reason"`
}

type adminWithdrawalBatchUserSettingsRequest struct {
	UserIDs                  []int64 `json:"user_ids"`
	Enabled                  *bool   `json:"enabled"`
	MinimumAmountOverride    *string `json:"minimum_amount_override"`
	MinimumOverride          *string `json:"minimum_override"`
	DailyLimitAmountOverride *string `json:"daily_limit_amount_override"`
	DailyLimitOverride       *string `json:"daily_limit_override"`
	DisabledReason           *string `json:"disabled_reason"`
}

type adminWithdrawalActionRequest struct {
	Note   string `json:"note"`
	Reason string `json:"reason"`
}

type adminWithdrawalMarkPaidRequest struct {
	PaidAmount    string  `json:"paid_amount"`
	PaidCurrency  string  `json:"paid_currency"`
	FXRate        string  `json:"payout_fx_rate"`
	ExternalTxnID string  `json:"external_txn_id"`
	PaidAt        *string `json:"paid_at"`
	Note          string  `json:"note"`
}

type adminWithdrawalBatchUserSettingsResponse struct {
	Affected int `json:"affected"`
}

type adminWithdrawalRecomputeResponse struct {
	Mode        string                               `json:"mode"`
	GeneratedAt time.Time                            `json:"generated_at"`
	User        adminWithdrawalRecomputeUserResponse `json:"user"`
}

type adminWithdrawalRecomputeUserResponse struct {
	UserID                      int64                                     `json:"user_id"`
	Status                      string                                    `json:"status"`
	LedgerBalance               string                                    `json:"ledger_balance"`
	ComputedWithdrawableBalance string                                    `json:"computed_withdrawable_balance"`
	ComputedPendingBalance      string                                    `json:"computed_pending_balance"`
	ComputedEntitlementBalance  string                                    `json:"computed_entitlement_balance"`
	TransactionCount            int                                       `json:"transaction_count"`
	EligibleGrantCount          int                                       `json:"eligible_grant_count"`
	Anomalies                   []adminWithdrawalRecomputeAnomalyResponse `json:"anomalies"`
	Batches                     []adminWithdrawalRecomputeBatchResponse   `json:"batches,omitempty"`
}

type adminWithdrawalRecomputeBatchResponse struct {
	SourceTransactionID int64     `json:"source_transaction_id"`
	SourceType          string    `json:"source_type"`
	SourceID            string    `json:"source_id"`
	OriginalAmount      string    `json:"original_amount"`
	RemainingAmount     string    `json:"remaining_amount"`
	ConsumedAmount      string    `json:"consumed_amount"`
	AvailableAt         time.Time `json:"available_at"`
}

type adminWithdrawalRecomputeAnomalyResponse struct {
	Code    string            `json:"code"`
	Details map[string]string `json:"details,omitempty"`
}

func NewWithdrawalHandler(withdrawalService *service.WithdrawalService, withdrawableRecomputeService *service.WithdrawableRecomputeService) *WithdrawalHandler {
	return &WithdrawalHandler{
		withdrawalService:            withdrawalService,
		withdrawableRecomputeService: withdrawableRecomputeService,
	}
}

func (h *WithdrawalHandler) List(c *gin.Context) {
	page, err := parseAdminWithdrawalPositiveInt(c.DefaultQuery("page", "1"), "page")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pageSize, err := parseAdminWithdrawalPositiveInt(c.DefaultQuery("page_size", "20"), "page_size")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	userID, err := parseAdminWithdrawalOptionalInt(c.Query("user_id"), "user_id")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.withdrawalService.AdminListWithdrawals(c.Request.Context(), service.WithdrawalListQuery{
		Status:   strings.TrimSpace(c.Query("status")),
		UserID:   userID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *WithdrawalHandler) Get(c *gin.Context) {
	id, err := parseAdminWithdrawalIDParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	req, err := h.withdrawalService.GetWithdrawal(c.Request.Context(), id, 0, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, req)
}

func (h *WithdrawalHandler) GetSettings(c *gin.Context) {
	settings, err := h.withdrawalService.GetSystemSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *WithdrawalHandler) UpdateSettings(c *gin.Context) {
	var req adminWithdrawalSystemSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal settings request"))
		return
	}
	actorID := currentAdminActorID(c)
	settings, err := h.withdrawalService.UpdateSystemSettings(c.Request.Context(), service.WithdrawalSystemSettingsUpdate{
		GlobalEnabled:         req.GlobalEnabled,
		MinimumAmount:         req.MinimumAmount,
		DailyLimitAmount:      req.DailyLimitAmount,
		DoubleReviewThreshold: req.DoubleReviewThreshold,
		ActorUserID:           actorID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{
		"result":  "updated",
		"enabled": settings.GlobalEnabled,
	})
	response.Success(c, settings)
}

func (h *WithdrawalHandler) GetUserSettings(c *gin.Context) {
	userID, err := parseAdminWithdrawalIDParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	settings, err := h.withdrawalService.GetUserSettings(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *WithdrawalHandler) UpdateUserSettings(c *gin.Context) {
	userID, err := parseAdminWithdrawalIDParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req adminWithdrawalUserSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid user withdrawal settings request"))
		return
	}
	settings, err := h.withdrawalService.UpdateUserSettings(c.Request.Context(), service.UserWithdrawalSettingsUpdate{
		UserID:                   userID,
		Enabled:                  req.Enabled,
		MinimumAmountOverride:    firstStringPtr(req.MinimumAmountOverride, req.MinimumOverride),
		DailyLimitAmountOverride: firstStringPtr(req.DailyLimitAmountOverride, req.DailyLimitOverride),
		DisabledReason:           req.DisabledReason,
		ActorUserID:              currentAdminActorID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{
		"result":         "updated",
		"target_user_id": userID,
		"enabled":        settings.Enabled,
	})
	response.Success(c, settings)
}

func (h *WithdrawalHandler) DryRunUserRecompute(c *gin.Context) {
	h.recomputeUser(c, false)
}

func (h *WithdrawalHandler) ExecuteUserRecompute(c *gin.Context) {
	h.recomputeUser(c, true)
}

func (h *WithdrawalHandler) recomputeUser(c *gin.Context, execute bool) {
	userID, err := parseAdminWithdrawalIDParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.withdrawableRecomputeService == nil {
		response.ErrorFrom(c, service.ErrWithdrawalUnavailable)
		return
	}
	report, err := h.withdrawableRecomputeService.Recompute(c.Request.Context(), service.WithdrawableRecomputeOptions{
		Execute: execute,
		UserID:  userID,
		Limit:   1,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if len(report.Users) == 0 {
		response.ErrorFrom(c, service.ErrUserNotFound)
		return
	}
	userReport := report.Users[0]
	middleware.SetAuditExtra(c, map[string]any{
		"result":               report.Mode,
		"target_user_id":       userID,
		"recompute_status":     userReport.Status,
		"anomaly_count":        len(userReport.Anomalies),
		"eligible_grant_count": userReport.EligibleGrantCount,
	})
	response.Success(c, adminWithdrawalRecomputeResponseFromReport(report, userReport))
}

func (h *WithdrawalHandler) BatchUpdateUserSettings(c *gin.Context) {
	var req adminWithdrawalBatchUserSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid batch user withdrawal settings request"))
		return
	}
	affected, err := h.withdrawalService.BatchUpdateUserSettings(c.Request.Context(), service.BatchUserWithdrawalSettingsUpdate{
		UserIDs:                  req.UserIDs,
		Enabled:                  req.Enabled,
		MinimumAmountOverride:    firstStringPtr(req.MinimumAmountOverride, req.MinimumOverride),
		DailyLimitAmountOverride: firstStringPtr(req.DailyLimitAmountOverride, req.DailyLimitOverride),
		DisabledReason:           req.DisabledReason,
		ActorUserID:              currentAdminActorID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{
		"result":          "updated",
		"requested_count": len(req.UserIDs),
		"matched_count":   affected,
	})
	response.Success(c, adminWithdrawalBatchUserSettingsResponse{Affected: affected})
}

func (h *WithdrawalHandler) Approve(c *gin.Context) {
	id, actorID, ok := h.actionContext(c)
	if !ok {
		return
	}
	var req adminWithdrawalActionRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorFrom(c, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal approval request"))
			return
		}
	}
	result, err := h.withdrawalService.AdminApprove(c.Request.Context(), service.WithdrawalActionInput{
		RequestID:   id,
		ActorUserID: actorID,
		Note:        req.Note,
		Locale:      c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{
		"result":                "approved",
		"withdrawal_request_id": result.ID,
		"withdrawal_status":     result.Status,
		"target_user_id":        result.UserID,
	})
	response.Success(c, result)
}

func (h *WithdrawalHandler) Reject(c *gin.Context) {
	id, actorID, ok := h.actionContext(c)
	if !ok {
		return
	}
	var req adminWithdrawalActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal reject request"))
		return
	}
	result, err := h.withdrawalService.AdminReject(c.Request.Context(), service.WithdrawalActionInput{
		RequestID:   id,
		ActorUserID: actorID,
		Reason:      req.Reason,
		Note:        req.Note,
		Locale:      c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{
		"result":                "rejected",
		"withdrawal_request_id": result.ID,
		"withdrawal_status":     result.Status,
		"target_user_id":        result.UserID,
	})
	response.Success(c, result)
}

func (h *WithdrawalHandler) GetSensitivePayout(c *gin.Context) {
	id, err := parseAdminWithdrawalIDParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	snapshot, err := h.withdrawalService.AdminGetSensitivePayoutSnapshot(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{
		"result":                "read",
		"withdrawal_request_id": id,
	})
	response.Success(c, snapshot)
}

func (h *WithdrawalHandler) MarkPaid(c *gin.Context) {
	id, actorID, ok := h.actionContext(c)
	if !ok {
		return
	}
	var req adminWithdrawalMarkPaidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal paid request"))
		return
	}
	paidAt, err := parseAdminWithdrawalOptionalTime(req.PaidAt)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	fxRate := strings.TrimSpace(req.FXRate)
	if fxRate == "" {
		fxRate = "1.00"
	}
	result, err := h.withdrawalService.AdminMarkPaid(c.Request.Context(), service.WithdrawalMarkPaidInput{
		RequestID:     id,
		ActorUserID:   actorID,
		PaidAmount:    req.PaidAmount,
		PaidCurrency:  req.PaidCurrency,
		FXRate:        fxRate,
		ExternalTxnID: req.ExternalTxnID,
		PaidAt:        paidAt,
		Note:          req.Note,
		Locale:        c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{
		"result":                "paid",
		"withdrawal_request_id": result.ID,
		"withdrawal_status":     result.Status,
		"target_user_id":        result.UserID,
		"paid_currency":         result.PaidCurrency,
	})
	response.Success(c, result)
}

func (h *WithdrawalHandler) actionContext(c *gin.Context) (int64, int64, bool) {
	id, err := parseAdminWithdrawalIDParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, err)
		return 0, 0, false
	}
	actorID := currentAdminActorID(c)
	if actorID <= 0 {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "admin JWT session required"))
		return 0, 0, false
	}
	return id, actorID, true
}

func currentAdminActorID(c *gin.Context) int64 {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		return 0
	}
	return subject.UserID
}

func parseAdminWithdrawalIDParam(c *gin.Context, name string) (int64, error) {
	return parseAdminWithdrawalRequiredInt(c.Param(name), name)
}

func parseAdminWithdrawalRequiredInt(raw, field string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal request").
			WithMetadata(map[string]string{"field": field})
	}
	return value, nil
}

func parseAdminWithdrawalOptionalInt(raw, field string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return parseAdminWithdrawalRequiredInt(raw, field)
}

func parseAdminWithdrawalPositiveInt(raw string, field string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal request").
			WithMetadata(map[string]string{"field": field})
	}
	return value, nil
}

func parseAdminWithdrawalOptionalTime(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		return nil, infraerrors.BadRequest("WITHDRAWAL_INVALID_INPUT", "invalid withdrawal paid_at").
			WithMetadata(map[string]string{"field": "paid_at"})
	}
	return &value, nil
}

func firstStringPtr(values ...*string) *string {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func adminWithdrawalRecomputeResponseFromReport(report *service.WithdrawableRecomputeReport, user service.WithdrawableRecomputeUserReport) adminWithdrawalRecomputeResponse {
	anomalies := make([]adminWithdrawalRecomputeAnomalyResponse, 0, len(user.Anomalies))
	for _, raw := range user.Anomalies {
		anomalies = append(anomalies, adminWithdrawalRecomputeAnomalyFromRaw(raw))
	}
	batches := make([]adminWithdrawalRecomputeBatchResponse, 0, len(user.Batches))
	for _, batch := range user.Batches {
		batches = append(batches, adminWithdrawalRecomputeBatchResponse{
			SourceTransactionID: batch.SourceTransactionID,
			SourceType:          batch.SourceType,
			SourceID:            batch.SourceID,
			OriginalAmount:      batch.OriginalAmount.StringFixed(8),
			RemainingAmount:     batch.RemainingAmount.StringFixed(8),
			ConsumedAmount:      batch.ConsumedAmount.StringFixed(8),
			AvailableAt:         batch.AvailableAt,
		})
	}
	return adminWithdrawalRecomputeResponse{
		Mode:        report.Mode,
		GeneratedAt: report.GeneratedAt,
		User: adminWithdrawalRecomputeUserResponse{
			UserID:                      user.UserID,
			Status:                      user.Status,
			LedgerBalance:               user.LedgerBalance.StringFixed(8),
			ComputedWithdrawableBalance: user.ComputedWithdrawableBalance.StringFixed(8),
			ComputedPendingBalance:      user.ComputedPendingBalance.StringFixed(8),
			ComputedEntitlementBalance:  user.ComputedEntitlementBalance.StringFixed(8),
			TransactionCount:            user.TransactionCount,
			EligibleGrantCount:          user.EligibleGrantCount,
			Anomalies:                   anomalies,
			Batches:                     batches,
		},
	}
}

func adminWithdrawalRecomputeAnomalyFromRaw(raw string) adminWithdrawalRecomputeAnomalyResponse {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "existing withdrawable entitlements require manual review before execute":
		return adminWithdrawalRecomputeAnomalyResponse{Code: "existing_entitlements"}
	case strings.HasPrefix(raw, "transaction ") && strings.Contains(raw, " confidence is "):
		txID, rest := splitAdminWithdrawalRecomputeTransaction(raw)
		return adminWithdrawalRecomputeAnomalyResponse{
			Code:    "transaction_confidence",
			Details: map[string]string{"transaction_id": txID, "confidence": strings.TrimSpace(strings.TrimPrefix(rest, "confidence is "))},
		}
	case strings.HasPrefix(raw, "transaction ") && strings.Contains(raw, " missing reliable balance_before"):
		txID, _ := splitAdminWithdrawalRecomputeTransaction(raw)
		return adminWithdrawalRecomputeAnomalyResponse{
			Code:    "missing_balance_before",
			Details: map[string]string{"transaction_id": txID},
		}
	case strings.HasPrefix(raw, "transaction ") && strings.Contains(raw, " could restore only "):
		txID, rest := splitAdminWithdrawalRecomputeTransaction(raw)
		return adminWithdrawalRecomputeAnomalyResponse{
			Code:    "restore_mismatch",
			Details: map[string]string{"transaction_id": txID, "detail": rest},
		}
	case raw == "computed entitlement balance exceeds current balance":
		return adminWithdrawalRecomputeAnomalyResponse{Code: "entitlement_exceeds_balance"}
	case strings.HasPrefix(raw, "ledger replay balance ") && strings.Contains(raw, " does not match users.balance "):
		details := strings.TrimPrefix(raw, "ledger replay balance ")
		parts := strings.SplitN(details, " does not match users.balance ", 2)
		if len(parts) == 2 {
			return adminWithdrawalRecomputeAnomalyResponse{
				Code:    "replay_balance_mismatch",
				Details: map[string]string{"ledger_balance": strings.TrimSpace(parts[0]), "user_balance": strings.TrimSpace(parts[1])},
			}
		}
		return adminWithdrawalRecomputeAnomalyResponse{Code: "replay_balance_mismatch"}
	default:
		return adminWithdrawalRecomputeAnomalyResponse{Code: "unknown"}
	}
}

func splitAdminWithdrawalRecomputeTransaction(raw string) (string, string) {
	rest := strings.TrimSpace(strings.TrimPrefix(raw, "transaction "))
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], strings.TrimSpace(parts[1])
}
