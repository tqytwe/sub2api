package handler

import (
	"context"
	"errors"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const mobileAccountSummaryPageSize = 30

type mobileAccountSummaryError struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

func mobileAccountSummarySafeError(source string, _ error) mobileAccountSummaryError {
	messages := map[string]string{
		"wallet":        "账户余额暂时无法同步",
		"transactions":  "余额明细暂时无法同步",
		"orders":        "支付订单暂时无法同步",
		"plans":         "套餐信息暂时无法同步",
		"payment":       "支付信息暂时无法同步",
		"subscriptions": "订阅信息暂时无法同步",
	}
	message, ok := messages[source]
	if !ok {
		message = "部分账户信息暂时无法同步"
	}
	return mobileAccountSummaryError{Source: source, Message: message}
}

type mobileSubscriptionResult struct {
	dto.UserSubscription
	Progress *service.SubscriptionProgress `json:"progress,omitempty"`
}

type mobileAccountSummaryResult struct {
	Wallet        *walletSummaryDTO           `json:"wallet,omitempty"`
	Transactions  []walletTransactionDTO      `json:"transactions"`
	Orders        []PaymentOrderResult        `json:"orders"`
	Plans         []paymentPlanResult         `json:"plans"`
	Subscriptions []mobileSubscriptionResult  `json:"subscriptions"`
	PartialErrors []mobileAccountSummaryError `json:"partial_errors,omitempty"`
}

// MobileAccountSummary returns the account-page data in one authenticated request.
// Independent sub-source failures are reported without discarding successful data.
func MobileAccountSummary(wallet *WalletHandler, payment *PaymentHandler, subscription *SubscriptionHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := middleware2.GetAuthSubjectFromContext(c)
		if !ok {
			response.Unauthorized(c, "用户未登录")
			return
		}

		result := mobileAccountSummaryResult{
			Transactions:  []walletTransactionDTO{},
			Orders:        []PaymentOrderResult{},
			Plans:         []paymentPlanResult{},
			Subscriptions: []mobileSubscriptionResult{},
		}
		var mu sync.Mutex
		var wg sync.WaitGroup
		addError := func(source string, err error) {
			mu.Lock()
			result.PartialErrors = append(result.PartialErrors, mobileAccountSummarySafeError(source, err))
			mu.Unlock()
		}
		run := func(source string, fn func(context.Context) error) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := fn(c.Request.Context()); err != nil {
					addError(source, err)
				}
			}()
		}

		if wallet == nil || wallet.walletService == nil {
			addError("wallet", service.ErrWalletUnavailable)
		} else {
			run("wallet", func(ctx context.Context) error {
				summary, err := wallet.walletService.GetSummary(ctx, subject.UserID)
				if err != nil {
					return err
				}
				dtoValue := toWalletSummaryDTO(summary)
				mu.Lock()
				result.Wallet = &dtoValue
				mu.Unlock()
				return nil
			})
			run("transactions", func(ctx context.Context) error {
				page, err := wallet.walletService.ListTransactions(ctx, subject.UserID, service.WalletTransactionQuery{Page: 1, PageSize: mobileAccountSummaryPageSize})
				if err != nil {
					return err
				}
				items := toWalletTransactionPageDTO(page).Items
				mu.Lock()
				result.Transactions = items
				mu.Unlock()
				return nil
			})
		}

		if payment == nil || payment.paymentService == nil || payment.configService == nil {
			addError("payment", errors.New("payment service is unavailable"))
		} else {
			run("orders", func(ctx context.Context) error {
				orders, _, err := payment.paymentService.GetUserOrders(ctx, subject.UserID, service.OrderListParams{Page: 1, PageSize: mobileAccountSummaryPageSize})
				if err != nil {
					return err
				}
				items := sanitizePaymentOrdersForResponse(orders)
				mu.Lock()
				result.Orders = items
				mu.Unlock()
				return nil
			})
			run("plans", func(ctx context.Context) error {
				plans, err := payment.configService.ListPlansForSale(ctx)
				if err != nil {
					return err
				}
				items := buildPaymentPlansForResponse(ctx, payment.configService, plans)
				mu.Lock()
				result.Plans = items
				mu.Unlock()
				return nil
			})
		}

		if subscription == nil || subscription.subscriptionService == nil {
			addError("subscriptions", service.ErrSubscriptionNotFound)
		} else {
			run("subscriptions", func(ctx context.Context) error {
				subs, err := subscription.subscriptionService.ListUserSubscriptions(ctx, subject.UserID)
				if err != nil {
					return err
				}
				items := make([]mobileSubscriptionResult, 0, len(subs))
				for i := range subs {
					item := mobileSubscriptionResult{UserSubscription: *dto.UserSubscriptionFromService(&subs[i])}
					if progress, progressErr := subscription.subscriptionService.GetSubscriptionProgress(ctx, subs[i].ID); progressErr == nil {
						item.Progress = progress
					}
					items = append(items, item)
				}
				mu.Lock()
				result.Subscriptions = items
				mu.Unlock()
				return nil
			})
		}

		wg.Wait()
		response.Success(c, result)
	}
}
