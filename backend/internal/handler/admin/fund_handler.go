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

type FundHandler struct {
	fundService *service.FundManagementService
}

type adminFundRefundActionRequest struct {
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

type adminFundRefundPaidRequest struct {
	PaidAmount    string  `json:"paid_amount"`
	PaidCurrency  string  `json:"paid_currency"`
	FXRate        string  `json:"payout_fx_rate"`
	ExternalTxnID string  `json:"external_txn_id"`
	PaidAt        *string `json:"paid_at"`
	Note          string  `json:"note"`
}

type adminFundGrantRequest struct {
	AccountEmail string `json:"account_email"`
	Amount       string `json:"amount"`
	Reason       string `json:"reason"`
}

type adminOfflineRechargeRequest struct {
	AccountEmail string `json:"account_email"`
	Amount       string `json:"amount"`
	ExternalRef  string `json:"external_ref"`
	Reason       string `json:"reason"`
}

type adminFundCompensationRequest struct {
	AccountEmail string `json:"account_email"`
	Amount       string `json:"amount"`
	Reason       string `json:"reason"`
}

type adminFundCorrectionRequest struct {
	CorrectAccountEmail string `json:"correct_account_email"`
	Reason              string `json:"reason"`
}

type adminFundRefundRequestDTO struct {
	RequestNo               string  `json:"request_no"`
	UserEmail               string  `json:"user_email"`
	RequestType             string  `json:"request_type"`
	Amount                  string  `json:"amount"`
	Currency                string  `json:"currency"`
	Status                  string  `json:"status"`
	Reason                  string  `json:"reason,omitempty"`
	AdminNote               string  `json:"admin_note,omitempty"`
	PayoutMethod            string  `json:"payout_method,omitempty"`
	PayoutCurrency          string  `json:"payout_currency,omitempty"`
	PayoutAccountMask       string  `json:"payout_account_mask,omitempty"`
	PayoutRecipientNameMask string  `json:"payout_recipient_name_mask,omitempty"`
	ApprovedAt              *string `json:"approved_at,omitempty"`
	RejectedAt              *string `json:"rejected_at,omitempty"`
	RejectedReason          string  `json:"rejected_reason,omitempty"`
	CanceledAt              *string `json:"canceled_at,omitempty"`
	PaidAt                  *string `json:"paid_at,omitempty"`
	PaidAmount              *string `json:"paid_amount,omitempty"`
	PaidCurrency            string  `json:"paid_currency,omitempty"`
	PayoutFXRate            *string `json:"payout_fx_rate,omitempty"`
	ExternalTxnID           string  `json:"external_txn_id,omitempty"`
	CreatedAt               string  `json:"created_at"`
	UpdatedAt               string  `json:"updated_at"`
}

type adminFundRefundPageDTO struct {
	Items    []adminFundRefundRequestDTO `json:"items"`
	Total    int64                       `json:"total"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"page_size"`
	Pages    int                         `json:"pages"`
}

func NewFundHandler(fundService *service.FundManagementService) *FundHandler {
	return &FundHandler{fundService: fundService}
}

func (h *FundHandler) ListRefunds(c *gin.Context) {
	page, err := parseAdminFundPositiveInt(c.DefaultQuery("page", "1"), "page")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pageSize, err := parseAdminFundPositiveInt(c.DefaultQuery("page_size", "20"), "page_size")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.fundService.AdminListRefundRequests(c.Request.Context(), service.FundRefundListQuery{
		Status:         strings.TrimSpace(c.Query("status")),
		AccountKeyword: strings.TrimSpace(c.Query("account")),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toAdminFundRefundPageDTO(result))
}

func (h *FundHandler) GetRefund(c *gin.Context) {
	id, err := h.fundService.AdminRefundRequestIDByNo(c.Request.Context(), c.Param("request_no"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.fundService.GetRefundRequest(c.Request.Context(), id, 0, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toAdminFundRefundDTO(result))
}

func (h *FundHandler) ApproveRefund(c *gin.Context) {
	id, actorID, ok := h.actionContext(c)
	if !ok {
		return
	}
	var req adminFundRefundActionRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorFrom(c, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund refund approval request"))
			return
		}
	}
	result, err := h.fundService.AdminApproveRefundRequest(c.Request.Context(), service.FundRefundActionInput{
		RequestID:   id,
		ActorUserID: actorID,
		Note:        req.Note,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toAdminFundRefundDTO(result))
}

func (h *FundHandler) RejectRefund(c *gin.Context) {
	id, actorID, ok := h.actionContext(c)
	if !ok {
		return
	}
	var req adminFundRefundActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund refund reject request"))
		return
	}
	result, err := h.fundService.AdminRejectRefundRequest(c.Request.Context(), service.FundRefundActionInput{
		RequestID:   id,
		ActorUserID: actorID,
		Reason:      req.Reason,
		Note:        req.Note,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toAdminFundRefundDTO(result))
}

func (h *FundHandler) MarkRefundPaid(c *gin.Context) {
	id, actorID, ok := h.actionContext(c)
	if !ok {
		return
	}
	var req adminFundRefundPaidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund refund paid request"))
		return
	}
	paidAt, err := parseAdminFundOptionalTime(req.PaidAt)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.fundService.AdminMarkRefundPaid(c.Request.Context(), service.FundRefundMarkPaidInput{
		RequestID:     id,
		ActorUserID:   actorID,
		PaidAmount:    req.PaidAmount,
		PaidCurrency:  req.PaidCurrency,
		FXRate:        req.FXRate,
		ExternalTxnID: req.ExternalTxnID,
		PaidAt:        paidAt,
		Note:          req.Note,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toAdminFundRefundDTO(result))
}

func (h *FundHandler) GetRefundPayoutSensitive(c *gin.Context) {
	id, err := h.fundService.AdminRefundRequestIDByNo(c.Request.Context(), c.Param("request_no"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.fundService.AdminGetRefundPayoutSnapshot(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) GrantGift(c *gin.Context) {
	var req adminFundGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid gift request"))
		return
	}
	result, err := h.fundService.GrantFundCreditByAccount(c.Request.Context(), service.FundAccountCreditInput{
		AccountEmail: req.AccountEmail,
		Amount:       req.Amount,
		Reason:       req.Reason,
		Kind:         service.FundOperationKindOpsGift,
		ActorUserID:  currentAdminFundActorID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) GrantOfflineRecharge(c *gin.Context) {
	var req adminOfflineRechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid offline recharge request"))
		return
	}
	result, err := h.fundService.GrantFundCreditByAccount(c.Request.Context(), service.FundAccountCreditInput{
		AccountEmail: req.AccountEmail,
		Amount:       req.Amount,
		ExternalRef:  req.ExternalRef,
		Reason:       req.Reason,
		Kind:         service.FundOperationKindOfflineRecharge,
		ActorUserID:  currentAdminFundActorID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) GrantCompensation(c *gin.Context) {
	var req adminFundCompensationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid compensation request"))
		return
	}
	result, err := h.fundService.GrantFundCreditByAccount(c.Request.Context(), service.FundAccountCreditInput{
		AccountEmail: req.AccountEmail, Amount: req.Amount, Reason: req.Reason,
		Kind: service.FundOperationKindCompensation, ActorUserID: currentAdminFundActorID(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) SearchAccounts(c *gin.Context) {
	result, err := h.fundService.SearchFundAccounts(c.Request.Context(), c.Query("q"), 10)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) ListOperations(c *gin.Context) {
	page, err := parseAdminFundPositiveInt(c.DefaultQuery("page", "1"), "page")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pageSize, err := parseAdminFundPositiveInt(c.DefaultQuery("page_size", "20"), "page_size")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.fundService.ListFundOperations(c.Request.Context(), service.FundOperationListQuery{
		Kind: strings.TrimSpace(c.Query("kind")), Status: strings.TrimSpace(c.Query("status")),
		AccountKeyword: strings.TrimSpace(c.Query("account")), OperatorKeyword: strings.TrimSpace(c.Query("operator")),
		Keyword: strings.TrimSpace(c.Query("q")), Page: page, PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) GetOperation(c *gin.Context) {
	result, err := h.fundService.GetFundOperation(c.Request.Context(), c.Param("operation_no"), false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) GetOperationSensitive(c *gin.Context) {
	result, err := h.fundService.GetFundOperation(c.Request.Context(), c.Param("operation_no"), true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) CorrectOperation(c *gin.Context) {
	actorID := currentAdminFundActorID(c)
	if actorID <= 0 {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "admin JWT session required"))
		return
	}
	var req adminFundCorrectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid account correction request"))
		return
	}
	result, err := h.fundService.CorrectFundOperationAccount(c.Request.Context(), service.FundOperationCorrectionInput{OperationNo: c.Param("operation_no"), CorrectAccountEmail: req.CorrectAccountEmail, Reason: req.Reason, ActorUserID: actorID})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) RetryOperationCorrection(c *gin.Context) {
	actorID := currentAdminFundActorID(c)
	if actorID <= 0 {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "admin JWT session required"))
		return
	}
	result, err := h.fundService.RetryPendingFundCorrection(c.Request.Context(), c.Param("operation_no"), actorID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) CancelOperationCorrection(c *gin.Context) {
	actorID := currentAdminFundActorID(c)
	if actorID <= 0 {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "admin JWT session required"))
		return
	}
	result, err := h.fundService.CancelPendingFundCorrection(c.Request.Context(), c.Param("operation_no"), actorID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *FundHandler) actionContext(c *gin.Context) (int64, int64, bool) {
	id, err := h.fundService.AdminRefundRequestIDByNo(c.Request.Context(), c.Param("request_no"))
	if err != nil {
		response.ErrorFrom(c, err)
		return 0, 0, false
	}
	actorID := currentAdminFundActorID(c)
	if actorID <= 0 {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "admin JWT session required"))
		return 0, 0, false
	}
	return id, actorID, true
}

func currentAdminFundActorID(c *gin.Context) int64 {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		return 0
	}
	return subject.UserID
}

func parseAdminFundIDParam(c *gin.Context, name string) (int64, error) {
	return parseAdminFundRequiredInt(c.Param(name), name)
}

func parseAdminFundRequiredInt(raw string, field string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund request").
			WithMetadata(map[string]string{"field": field})
	}
	return value, nil
}

func parseAdminFundOptionalInt(raw string, field string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return parseAdminFundRequiredInt(raw, field)
}

func parseAdminFundPositiveInt(raw string, field string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund request").
			WithMetadata(map[string]string{"field": field})
	}
	return value, nil
}

func parseAdminFundOptionalTime(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		return nil, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund paid_at").
			WithMetadata(map[string]string{"field": "paid_at"})
	}
	return &value, nil
}

func toAdminFundRefundPageDTO(page *service.FundRefundRequestPage) adminFundRefundPageDTO {
	out := adminFundRefundPageDTO{
		Items:    make([]adminFundRefundRequestDTO, 0, len(page.Items)),
		Total:    page.Total,
		Page:     page.Page,
		PageSize: page.PageSize,
		Pages:    page.Pages,
	}
	for i := range page.Items {
		out.Items = append(out.Items, toAdminFundRefundDTO(&page.Items[i]))
	}
	return out
}

func toAdminFundRefundDTO(req *service.FundRefundRequest) adminFundRefundRequestDTO {
	dto := adminFundRefundRequestDTO{
		RequestNo:               req.RequestNo,
		UserEmail:               req.UserEmail,
		RequestType:             req.RequestType,
		Amount:                  req.Amount.StringFixed(8),
		Currency:                req.Currency,
		Status:                  req.Status,
		Reason:                  req.Reason,
		AdminNote:               req.AdminNote,
		PayoutMethod:            req.PayoutMethod,
		PayoutCurrency:          req.PayoutCurrency,
		PayoutAccountMask:       req.PayoutAccountMask,
		PayoutRecipientNameMask: req.PayoutRecipientNameMask,
		RejectedReason:          req.RejectedReason,
		PaidCurrency:            req.PaidCurrency,
		ExternalTxnID:           req.ExternalTxnID,
		CreatedAt:               req.CreatedAt.Format(time.RFC3339),
		UpdatedAt:               req.UpdatedAt.Format(time.RFC3339),
	}
	dto.ApprovedAt = adminFundOptionalTime(req.ApprovedAt)
	dto.RejectedAt = adminFundOptionalTime(req.RejectedAt)
	dto.CanceledAt = adminFundOptionalTime(req.CanceledAt)
	dto.PaidAt = adminFundOptionalTime(req.PaidAt)
	if req.PaidAmount != nil {
		value := req.PaidAmount.StringFixed(8)
		dto.PaidAmount = &value
	}
	if req.PayoutFXRate != nil {
		value := req.PayoutFXRate.StringFixed(8)
		dto.PayoutFXRate = &value
	}
	return dto
}

func adminFundOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	out := value.Format(time.RFC3339)
	return &out
}
