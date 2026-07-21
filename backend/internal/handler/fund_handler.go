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

type FundHandler struct {
	fundService *service.FundManagementService
}

type fundRefundCreateRequest struct {
	RequestType string `json:"request_type"`
	Amount      string `json:"amount"`
	Reason      string `json:"reason"`
}

type fundRefundCancelRequest struct {
	Note string `json:"note"`
}

type fundRefundRequestDTO struct {
	ID                      int64   `json:"id"`
	RequestNo               string  `json:"request_no"`
	UserID                  int64   `json:"user_id,omitempty"`
	UserEmail               string  `json:"user_email,omitempty"`
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
	ApprovedBy              *int64  `json:"approved_by,omitempty"`
	ApprovedAt              *string `json:"approved_at,omitempty"`
	RejectedBy              *int64  `json:"rejected_by,omitempty"`
	RejectedAt              *string `json:"rejected_at,omitempty"`
	RejectedReason          string  `json:"rejected_reason,omitempty"`
	CanceledAt              *string `json:"canceled_at,omitempty"`
	PaidBy                  *int64  `json:"paid_by,omitempty"`
	PaidAt                  *string `json:"paid_at,omitempty"`
	PaidAmount              *string `json:"paid_amount,omitempty"`
	PaidCurrency            string  `json:"paid_currency,omitempty"`
	PayoutFXRate            *string `json:"payout_fx_rate,omitempty"`
	ExternalTxnID           string  `json:"external_txn_id,omitempty"`
	CreatedAt               string  `json:"created_at"`
	UpdatedAt               string  `json:"updated_at"`
}

type fundRefundPageDTO struct {
	Items    []fundRefundRequestDTO `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Pages    int                    `json:"pages"`
}

func NewFundHandler(fundService *service.FundManagementService) *FundHandler {
	return &FundHandler{fundService: fundService}
}

func (h *FundHandler) CreateRefund(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req fundRefundCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund refund request"))
		return
	}
	result, err := h.fundService.CreateRefundRequest(c.Request.Context(), service.FundRefundCreateInput{
		UserID:      subject.UserID,
		RequestType: req.RequestType,
		Amount:      req.Amount,
		Reason:      req.Reason,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toFundRefundDTO(result, false))
}

func (h *FundHandler) ListRefunds(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, err := parseFundPositiveInt(c.DefaultQuery("page", "1"), "page")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pageSize, err := parseFundPositiveInt(c.DefaultQuery("page_size", "20"), "page_size")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.fundService.ListUserRefundRequests(c.Request.Context(), subject.UserID, service.FundRefundListQuery{
		Status:   strings.TrimSpace(c.Query("status")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toFundRefundPageDTO(result, false))
}

func (h *FundHandler) GetRefund(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := parseFundIDParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.fundService.GetRefundRequest(c.Request.Context(), id, subject.UserID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toFundRefundDTO(result, false))
}

func (h *FundHandler) CancelRefund(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := parseFundIDParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req fundRefundCancelRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorFrom(c, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund refund cancel request"))
			return
		}
	}
	result, err := h.fundService.CancelRefundRequest(c.Request.Context(), service.FundRefundActionInput{
		RequestID: id,
		UserID:    subject.UserID,
		Note:      req.Note,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toFundRefundDTO(result, false))
}

func parseFundPositiveInt(raw string, field string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund request").
			WithMetadata(map[string]string{"field": field})
	}
	return value, nil
}

func parseFundIDParam(c *gin.Context, name string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || value <= 0 {
		return 0, infraerrors.BadRequest("FUND_INVALID_INPUT", "invalid fund request").
			WithMetadata(map[string]string{"field": name})
	}
	return value, nil
}

func toFundRefundPageDTO(page *service.FundRefundRequestPage, adminView bool) fundRefundPageDTO {
	out := fundRefundPageDTO{
		Items:    make([]fundRefundRequestDTO, 0, len(page.Items)),
		Total:    page.Total,
		Page:     page.Page,
		PageSize: page.PageSize,
		Pages:    page.Pages,
	}
	for i := range page.Items {
		out.Items = append(out.Items, toFundRefundDTO(&page.Items[i], adminView))
	}
	return out
}

func toFundRefundDTO(req *service.FundRefundRequest, adminView bool) fundRefundRequestDTO {
	dto := fundRefundRequestDTO{
		ID:                      req.ID,
		RequestNo:               req.RequestNo,
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
		ApprovedBy:              req.ApprovedBy,
		RejectedBy:              req.RejectedBy,
		RejectedReason:          req.RejectedReason,
		PaidBy:                  req.PaidBy,
		PaidCurrency:            req.PaidCurrency,
		ExternalTxnID:           req.ExternalTxnID,
		CreatedAt:               req.CreatedAt.Format(time.RFC3339),
		UpdatedAt:               req.UpdatedAt.Format(time.RFC3339),
	}
	if adminView {
		dto.UserID = req.UserID
		dto.UserEmail = req.UserEmail
	}
	dto.ApprovedAt = formatOptionalTime(req.ApprovedAt)
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
	return dto
}
