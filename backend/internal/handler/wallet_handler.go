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
	walletService *service.WalletService
}

type walletSummaryDTO struct {
	AvailableBalance    string  `json:"available_balance"`
	TaskReservedBalance string  `json:"task_reserved_balance"`
	TotalCredits        string  `json:"total_credits"`
	TotalDebits         string  `json:"total_debits"`
	TransactionCount    int64   `json:"transaction_count"`
	LastTransactionAt   *string `json:"last_transaction_at,omitempty"`
}

type walletTransactionDTO struct {
	ID           int64  `json:"id"`
	Source       string `json:"source"`
	Direction    string `json:"direction"`
	BalanceDelta string `json:"balance_delta"`
	FrozenDelta  string `json:"frozen_delta"`
	BalanceAfter string `json:"balance_after"`
	FrozenAfter  string `json:"frozen_after"`
	CreatedAt    string `json:"created_at"`
}

type walletTransactionPageDTO struct {
	Items    []walletTransactionDTO `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Pages    int                    `json:"pages"`
}

func NewWalletHandler(walletService *service.WalletService) *WalletHandler {
	return &WalletHandler{walletService: walletService}
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

func parseWalletPositiveInt(raw string, field string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, infraerrors.BadRequest("WALLET_INVALID_INPUT", "invalid wallet request").
			WithMetadata(map[string]string{"field": field})
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
		AvailableBalance:    summary.AvailableBalance.StringFixed(8),
		TaskReservedBalance: summary.TaskReservedBalance.StringFixed(8),
		TotalCredits:        summary.TotalCredits.StringFixed(8),
		TotalDebits:         summary.TotalDebits.StringFixed(8),
		TransactionCount:    summary.TransactionCount,
		LastTransactionAt:   last,
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
			ID:           item.ID,
			Source:       item.Source,
			Direction:    item.Direction,
			BalanceDelta: item.BalanceDelta.StringFixed(8),
			FrozenDelta:  item.FrozenDelta.StringFixed(8),
			BalanceAfter: item.BalanceAfter.StringFixed(8),
			FrozenAfter:  item.FrozenAfter.StringFixed(8),
			CreatedAt:    item.CreatedAt.Format(time.RFC3339),
		})
	}
	return out
}
