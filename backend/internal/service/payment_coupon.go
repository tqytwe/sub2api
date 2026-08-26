package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

// paymentCouponOrderService is the small coupon-domain boundary required by
// checkout. Keeping it narrow makes payment behavior testable without letting
// the payment service manage coupon persistence itself.
type paymentCouponOrderService interface {
	GetUserCouponForUser(context.Context, int64, int64) (*UserCoupon, error)
	QuoteUserCoupon(context.Context, int64, CouponOrderContext) (*CouponQuote, error)
	LockUserCouponForOrder(context.Context, CouponLockRequest) (*CouponLockResult, error)
	ReleaseUserCouponOrderLock(context.Context, int64, int64) (*UserCoupon, error)
	ConsumeUserCouponOrderLock(context.Context, int64, int64) (*UserCoupon, error)
}

const couponLockReleaseBatchSize = 100

// CouponPaymentQuoteRequest is the authenticated checkout context used to
// preview a selected coupon. It deliberately mirrors the relevant fields of
// CreateOrderRequest so quote and order creation share the same calculation.
type CouponPaymentQuoteRequest struct {
	UserID      int64
	CouponID    int64
	Amount      float64
	PaymentType string
	OrderType   string
	PlanID      int64
}

// CouponPaymentQuoteResponse contains the final payable quote in the payment
// gateway currency. The coupon stays unlocked until CreateOrder succeeds.
type CouponPaymentQuoteResponse struct {
	UserCouponID             int64   `json:"user_coupon_id"`
	TemplateID               int64   `json:"template_id"`
	ListAmount               float64 `json:"list_amount"`
	GatewayBaseAmount        float64 `json:"gateway_base_amount"`
	DiscountAmount           float64 `json:"discount_amount"`
	FeeAmount                float64 `json:"fee_amount"`
	PayAmount                float64 `json:"pay_amount"`
	PaymentCurrency          string  `json:"payment_currency"`
	QualifyingRechargeAmount float64 `json:"qualifying_recharge_amount"`
}

type paymentCouponSettlementInput struct {
	ListAmount  float64
	FeeRate     float64
	Currency    string
	CouponQuote *CouponQuote
}

// paymentCouponSettlement stores the exact cash calculation that is copied to
// the payment order. ListAmount is the gateway-currency price before a coupon;
// GatewayBaseAmount is the discounted amount before payment fees.
type paymentCouponSettlement struct {
	ListAmount               float64
	GatewayBaseAmount        float64
	DiscountAmount           float64
	FeeAmount                float64
	PayAmount                float64
	PayAmountText            string
	Currency                 string
	QualifyingRechargeAmount float64
	CouponQuote              *CouponQuote
}

func calculateCouponAdjustedPaymentSettlement(input paymentCouponSettlementInput) (*paymentCouponSettlement, error) {
	currency, err := payment.NormalizePaymentCurrency(input.Currency)
	if err != nil {
		return nil, fmt.Errorf("normalize payment currency: %w", err)
	}
	if !couponPaymentFiniteNonNegative(input.ListAmount) || input.ListAmount <= 0 {
		return nil, fmt.Errorf("payment list amount must be positive and finite")
	}
	if math.IsNaN(input.FeeRate) || math.IsInf(input.FeeRate, 0) || input.FeeRate < 0 {
		return nil, fmt.Errorf("payment fee rate must be non-negative and finite")
	}

	precision := int32(payment.CurrencyMaxFractionDigits(currency))
	list := decimal.NewFromFloat(input.ListAmount).Round(precision)
	if list.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("payment list amount must be positive after currency rounding")
	}

	discount := decimal.Zero
	if input.CouponQuote != nil {
		quote := input.CouponQuote
		if quote.UserCouponID <= 0 || quote.TemplateID <= 0 || !couponPaymentFiniteNonNegative(quote.DiscountAmount) || !couponPaymentFiniteNonNegative(quote.PayableAmount) {
			return nil, fmt.Errorf("coupon quote is invalid")
		}
		if quoteCurrency := strings.TrimSpace(quote.Currency); quoteCurrency != "" && !strings.EqualFold(quoteCurrency, currency) {
			return nil, fmt.Errorf("coupon quote currency does not match payment currency")
		}
		quotedOriginal := decimal.NewFromFloat(quote.OriginalAmount).Round(precision)
		if !quotedOriginal.Equal(list) {
			return nil, fmt.Errorf("coupon quote amount is stale")
		}
		discount = decimal.NewFromFloat(quote.DiscountAmount).Round(precision)
		if discount.GreaterThan(list) {
			discount = list
		}
	}

	base := list.Sub(discount).Round(precision)
	if base.IsNegative() {
		base = decimal.Zero
	}
	settlement := &paymentCouponSettlement{
		ListAmount:               list.InexactFloat64(),
		GatewayBaseAmount:        base.InexactFloat64(),
		DiscountAmount:           discount.InexactFloat64(),
		Currency:                 currency,
		QualifyingRechargeAmount: base.InexactFloat64(),
		CouponQuote:              cloneCouponQuote(input.CouponQuote),
	}
	if base.IsZero() {
		settlement.PayAmountText = payment.FormatAmountForCurrency(0, currency)
		return settlement, nil
	}
	paymentText, payAmount, err := calculateCreateOrderPayAmount(base.InexactFloat64(), input.FeeRate, currency)
	if err != nil {
		return nil, err
	}
	pay := decimal.NewFromFloat(payAmount).Round(precision)
	fee := pay.Sub(base).Round(precision)
	if fee.IsNegative() {
		return nil, fmt.Errorf("payment fee calculation is negative")
	}
	settlement.PayAmountText = paymentText
	settlement.PayAmount = pay.InexactFloat64()
	settlement.FeeAmount = fee.InexactFloat64()
	return settlement, nil
}

func couponOrderScope(orderType string) (CouponScope, error) {
	switch strings.TrimSpace(orderType) {
	case payment.OrderTypeBalance:
		return CouponScopeBalance, nil
	case payment.OrderTypeSubscription:
		return CouponScopeSubscription, nil
	default:
		return "", fmt.Errorf("unsupported coupon order type: %s", orderType)
	}
}

func couponOrderContext(userID, planID int64, orderType string, listAmount float64, currency string, at time.Time) (CouponOrderContext, error) {
	scope, err := couponOrderScope(orderType)
	if err != nil {
		return CouponOrderContext{}, err
	}
	return CouponOrderContext{
		UserID:      userID,
		Scope:       scope,
		OrderAmount: listAmount,
		PlanID:      planID,
		Currency:    currency,
		At:          at,
	}, nil
}

func paymentGatewayListAmount(limitAmount float64, orderType, currency string, subscriptionUSDToCNYRate float64) float64 {
	if orderType == payment.OrderTypeSubscription {
		return calculateSubscriptionGatewayBaseAmount(limitAmount, subscriptionUSDToCNYRate, currency)
	}
	return limitAmount
}

func (s *PaymentService) quoteCouponForCheckout(
	ctx context.Context,
	couponID, userID, planID int64,
	orderType string,
	listAmount float64,
	currency string,
) (*CouponQuote, error) {
	if couponID <= 0 {
		return nil, nil
	}
	if s == nil || s.couponService == nil {
		return nil, infraerrors.ServiceUnavailable("COUPON_SERVICE_UNAVAILABLE", "coupon service is unavailable")
	}
	orderContext, err := couponOrderContext(userID, planID, orderType, listAmount, currency, time.Now().UTC())
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_COUPON_ORDER", err.Error())
	}
	return s.couponService.QuoteUserCoupon(ctx, couponID, orderContext)
}

// QuoteCouponPayment previews a coupon-adjusted payment without creating an
// order or locking the coupon. CreateOrder repeats the quote under its order
// transaction before taking the lock, so this endpoint is informational only.
func (s *PaymentService) QuoteCouponPayment(ctx context.Context, req CouponPaymentQuoteRequest) (*CouponPaymentQuoteResponse, error) {
	if s == nil || s.configService == nil || s.userRepo == nil {
		return nil, infraerrors.ServiceUnavailable("PAYMENT_SERVICE_UNAVAILABLE", "payment service is unavailable")
	}
	if req.CouponID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER_COUPON_ID", "coupon id is required")
	}
	if req.OrderType == "" {
		req.OrderType = payment.OrderTypeBalance
	}
	if normalized := NormalizeVisibleMethod(req.PaymentType); normalized != "" {
		req.PaymentType = normalized
	}

	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment config: %w", err)
	}
	if !cfg.Enabled {
		return nil, infraerrors.Forbidden("PAYMENT_DISABLED", "payment system is disabled")
	}
	plan, err := s.validateOrderInput(ctx, CreateOrderRequest{
		Amount:      req.Amount,
		PaymentType: req.PaymentType,
		OrderType:   req.OrderType,
		PlanID:      req.PlanID,
	}, cfg)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}

	limitAmount := req.Amount
	if plan != nil {
		limitAmount = plan.Price
		req.PlanID = plan.ID
	}
	methodCurrency, err := s.configService.ValidateMethodCurrencyConsistency(ctx, req.PaymentType)
	if err != nil {
		return nil, err
	}
	listAmount := paymentGatewayListAmount(limitAmount, req.OrderType, methodCurrency, cfg.SubscriptionUSDToCNYRate)
	couponQuote, err := s.quoteCouponForCheckout(ctx, req.CouponID, req.UserID, req.PlanID, req.OrderType, listAmount, methodCurrency)
	if err != nil {
		return nil, err
	}
	settlement, err := calculateCouponAdjustedPaymentSettlement(paymentCouponSettlementInput{
		ListAmount:  listAmount,
		FeeRate:     cfg.RechargeFeeRate,
		Currency:    methodCurrency,
		CouponQuote: couponQuote,
	})
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_COUPON_SETTLEMENT", err.Error())
	}

	// QuoteUserCoupon already asserts ownership. Read the owned wallet item only
	// to make the response self-consistent for clients that keep the selection.
	coupon, err := s.couponService.GetUserCouponForUser(ctx, req.CouponID, req.UserID)
	if err != nil {
		return nil, err
	}
	return &CouponPaymentQuoteResponse{
		UserCouponID:             coupon.ID,
		TemplateID:               coupon.TemplateID,
		ListAmount:               settlement.ListAmount,
		GatewayBaseAmount:        settlement.GatewayBaseAmount,
		DiscountAmount:           settlement.DiscountAmount,
		FeeAmount:                settlement.FeeAmount,
		PayAmount:                settlement.PayAmount,
		PaymentCurrency:          settlement.Currency,
		QualifyingRechargeAmount: settlement.QualifyingRechargeAmount,
	}, nil
}

func couponSettlementsMatch(left, right *paymentCouponSettlement) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Currency == right.Currency &&
		moneyEqual(left.ListAmount, right.ListAmount) &&
		moneyEqual(left.GatewayBaseAmount, right.GatewayBaseAmount) &&
		moneyEqual(left.DiscountAmount, right.DiscountAmount) &&
		moneyEqual(left.FeeAmount, right.FeeAmount) &&
		moneyEqual(left.PayAmount, right.PayAmount) &&
		left.PayAmountText == right.PayAmountText
}

func moneyEqual(left, right float64) bool {
	return math.Abs(left-right) < 0.000001
}

func paymentCouponSnapshot(coupon UserCoupon, quote CouponQuote) map[string]any {
	return map[string]any{
		"schema_version":   1,
		"user_coupon_id":   coupon.ID,
		"template_id":      coupon.TemplateID,
		"template_version": coupon.TermsSnapshot.TemplateVersion,
		"template_key":     coupon.TermsSnapshot.TemplateKey,
		"name":             coupon.TermsSnapshot.Name,
		"terms":            coupon.TermsSnapshot,
		"valid_from":       coupon.ValidFrom,
		"expires_at":       coupon.ExpiresAt,
		"original_amount":  quote.OriginalAmount,
		"discount_amount":  quote.DiscountAmount,
		"payable_amount":   quote.PayableAmount,
		"currency":         quote.Currency,
	}
}

func cloneCouponQuote(in *CouponQuote) *CouponQuote {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func couponPaymentFiniteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func applyCouponSettlementToOrderResponse(response *CreateOrderResponse, settlement *paymentCouponSettlement) {
	if response == nil || settlement == nil {
		return
	}
	response.ListAmount = settlement.ListAmount
	response.GatewayBaseAmount = settlement.GatewayBaseAmount
	response.DiscountAmount = settlement.DiscountAmount
	response.FeeAmount = settlement.FeeAmount
	response.QualifyingRechargeAmount = settlement.QualifyingRechargeAmount
	response.PaymentCurrency = settlement.Currency
}

func buildZeroCouponOrderResponse(order *dbent.PaymentOrder, req CreateOrderRequest) *CreateOrderResponse {
	if order == nil {
		return nil
	}
	return &CreateOrderResponse{
		OrderID:                  order.ID,
		Amount:                   order.Amount,
		ListAmount:               order.ListAmount,
		GatewayBaseAmount:        order.GatewayBaseAmount,
		DiscountAmount:           order.DiscountAmount,
		FeeAmount:                order.FeeAmount,
		QualifyingRechargeAmount: order.QualifyingRechargeAmount,
		PayAmount:                order.PayAmount,
		FeeRate:                  order.FeeRate,
		Status:                   order.Status,
		ResultType:               payment.CreatePaymentResultOrderCreated,
		PaymentType:              req.PaymentType,
		PaymentCurrency:          order.PaymentCurrency,
		OutTradeNo:               order.OutTradeNo,
		RechargeSnapshot:         paymentOrderRechargeSnapshot(order),
		ExpiresAt:                order.ExpiresAt,
	}
}

// completeZeroCouponOrder records an all-coupon checkout as paid without
// calling a payment provider. The order status and user-coupon status are
// committed together, exactly like a successful payment callback.
func (s *PaymentService) completeZeroCouponOrder(ctx context.Context, order *dbent.PaymentOrder) error {
	if order == nil || order.ID <= 0 || order.CouponID == nil || order.PayAmount != 0 {
		return infraerrors.BadRequest("INVALID_ZERO_PAYMENT_ORDER", "only a coupon-backed zero-payment order can be completed directly")
	}
	updated, err := s.markOrderPaidAndConsumeCoupon(ctx, order, "", 0, false)
	if err != nil {
		return err
	}
	if !updated {
		return infraerrors.Conflict("CONFLICT", "zero-payment order was already processed")
	}
	s.writeAuditLog(ctx, order.ID, "ORDER_PAID", "coupon", map[string]any{
		"paidAmount":      0,
		"discountAmount":  order.DiscountAmount,
		"couponID":        *order.CouponID,
		"paymentCurrency": order.PaymentCurrency,
	})
	return s.executeFulfillment(ctx, order.ID)
}

// markOrderPaidAndConsumeCoupon is the single transition used by both gateway
// callbacks and fully discounted orders. A failed coupon consumption rolls
// back the PAID transition instead of leaving a paid order with a reusable
// coupon.
func (s *PaymentService) markOrderPaidAndConsumeCoupon(ctx context.Context, order *dbent.PaymentOrder, tradeNo string, paid float64, allowLatePayment bool) (bool, error) {
	if s == nil || s.entClient == nil || order == nil {
		return false, errors.New("payment coupon transition requires an order and ent client")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return false, fmt.Errorf("begin paid coupon transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	statusPredicate := paymentorder.StatusEQ(OrderStatusPending)
	if allowLatePayment {
		grace := now.Add(-paymentGraceMinutes * time.Minute)
		lateCancelled := paymentorder.And(
			paymentorder.StatusEQ(OrderStatusCancelled),
			paymentorder.CouponLockReleaseProcessedAtIsNil(),
			paymentorder.UpdatedAtGTE(grace),
		)
		lateExpired := paymentorder.And(
			paymentorder.StatusEQ(OrderStatusExpired),
			paymentorder.CouponLockReleaseProcessedAtIsNil(),
			paymentorder.UpdatedAtGTE(grace),
		)
		lateFailed := paymentorder.And(
			paymentorder.StatusEQ(OrderStatusFailed),
			paymentorder.PaidAtIsNil(),
			paymentorder.CouponLockReleaseProcessedAtIsNil(),
			paymentorder.UpdatedAtGTE(grace),
		)
		if order.CouponID == nil {
			// Preserve the historical recovery behavior for orders that never
			// locked a coupon. Coupon orders must remain inside the release grace.
			lateCancelled = paymentorder.StatusEQ(OrderStatusCancelled)
		}
		statusPredicate = paymentorder.Or(statusPredicate, lateCancelled, lateExpired, lateFailed)
	}

	txCtx := dbent.NewTxContext(ctx, tx)
	updated, err := tx.PaymentOrder.Update().Where(paymentorder.IDEQ(order.ID), statusPredicate).
		SetStatus(OrderStatusPaid).
		SetPayAmount(paid).
		SetPaymentTradeNo(tradeNo).
		SetPaidAt(now).
		ClearFailedAt().
		ClearFailedReason().
		Save(txCtx)
	if err != nil {
		return false, fmt.Errorf("update order to paid: %w", err)
	}
	if updated == 0 {
		return false, nil
	}
	if order.CouponID != nil {
		if s.couponService == nil {
			return false, infraerrors.ServiceUnavailable("COUPON_SERVICE_UNAVAILABLE", "coupon service is unavailable")
		}
		if _, err := s.couponService.ConsumeUserCouponOrderLock(txCtx, *order.CouponID, order.ID); err != nil {
			return false, fmt.Errorf("consume payment coupon: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit paid coupon transaction: %w", err)
	}
	return true, nil
}

func (s *PaymentService) releaseCouponAfterGatewayCreateFailure(ctx context.Context, order *dbent.PaymentOrder) {
	if s == nil || order == nil || order.CouponID == nil {
		return
	}
	// A provider create error can be ambiguous: it may have accepted the charge
	// before the client observed the failure. Keep the order's coupon lock until
	// the late-webhook grace window closes so a successful callback consumes the
	// original coupon instead of fulfilling an order with a reusable coupon.
	slog.Info("retain coupon lock after payment gateway creation failure", "order_id", order.ID, "coupon_id", *order.CouponID)
}

// ReleaseCouponLocksAfterLatePaymentGrace only releases locks from unpaid
// cancelled, expired, or gateway-create-failed orders after the webhook
// recovery window. It is called by the periodic order expiry sweep; paid
// fulfillment failures never qualify.
func (s *PaymentService) ReleaseCouponLocksAfterLatePaymentGrace(ctx context.Context) (int, error) {
	if s == nil || s.entClient == nil || s.couponService == nil {
		return 0, nil
	}
	graceCutoff := time.Now().UTC().Add(-paymentGraceMinutes * time.Minute)
	orders, err := s.entClient.PaymentOrder.Query().Where(
		paymentorder.CouponIDNotNil(),
		paymentorder.StatusIn(OrderStatusCancelled, OrderStatusExpired, OrderStatusFailed),
		paymentorder.PaidAtIsNil(),
		paymentorder.CouponLockReleaseProcessedAtIsNil(),
		paymentorder.UpdatedAtLT(graceCutoff),
	).
		Order(paymentorder.ByUpdatedAt(), paymentorder.ByID()).
		Limit(couponLockReleaseBatchSize).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("list coupon locks eligible for release: %w", err)
	}
	released := 0
	for _, order := range orders {
		if order.CouponID == nil {
			continue
		}
		wasReleased, reconciliationReason, err := s.releaseCouponOrderLockAfterLatePaymentGrace(ctx, order, graceCutoff)
		if err != nil {
			slog.Warn("release expired payment coupon lock failed", "order_id", order.ID, "coupon_id", *order.CouponID, "error", err)
			continue
		}
		if reconciliationReason != "" {
			slog.Info("reconciled terminal payment coupon lock",
				"order_id", order.ID,
				"coupon_id", *order.CouponID,
				"order_status", order.Status,
				"reason", reconciliationReason,
			)
		}
		if wasReleased {
			released++
		}
	}
	return released, nil
}

// releaseCouponOrderLockAfterLatePaymentGrace holds the terminal payment
// order row and its coupon transition in one transaction. A release scan sees
// a snapshot of the order before it acts; without this conditional claim, a
// late success callback could mark the order paid while a second transaction
// returns the same coupon to the user's wallet.
func (s *PaymentService) releaseCouponOrderLockAfterLatePaymentGrace(ctx context.Context, order *dbent.PaymentOrder, graceCutoff time.Time) (bool, string, error) {
	if s == nil || s.entClient == nil || s.couponService == nil || order == nil || order.CouponID == nil {
		return false, "", nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return false, "", fmt.Errorf("begin coupon lock release transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Set updated_at to its current value. PostgreSQL still obtains an UPDATE
	// row lock, but preserving the old timestamp means a callback that arrives
	// after this release cannot re-enter the five-minute recovery window.
	txCtx := dbent.NewTxContext(ctx, tx)
	claimed, err := tx.PaymentOrder.Update().Where(
		paymentorder.IDEQ(order.ID),
		paymentorder.CouponIDNotNil(),
		paymentorder.StatusIn(OrderStatusCancelled, OrderStatusExpired, OrderStatusFailed),
		paymentorder.PaidAtIsNil(),
		paymentorder.CouponLockReleaseProcessedAtIsNil(),
		paymentorder.UpdatedAtEQ(order.UpdatedAt),
		paymentorder.UpdatedAtLT(graceCutoff),
	).
		SetCouponLockReleaseProcessedAt(time.Now().UTC()).
		SetUpdatedAt(order.UpdatedAt).
		Save(txCtx)
	if err != nil {
		return false, "", fmt.Errorf("claim coupon lock release: %w", err)
	}
	if claimed == 0 {
		return false, "", nil
	}
	if _, err := s.couponService.ReleaseUserCouponOrderLock(txCtx, *order.CouponID, order.ID); err != nil {
		reason := infraerrors.Reason(err)
		switch reason {
		case "COUPON_LOCK_MISMATCH", "COUPON_ALREADY_USED", "COUPON_NOT_FOUND":
			if commitErr := tx.Commit(); commitErr != nil {
				return false, "", fmt.Errorf("commit terminal coupon lock reconciliation: %w", commitErr)
			}
			return false, reason, nil
		default:
			return false, "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, "", fmt.Errorf("commit coupon lock release: %w", err)
	}
	return true, "", nil
}
