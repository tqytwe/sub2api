package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	// ManualPaymentConfirmedAuditAction is an append-only payment audit event.
	// The financial values are deliberately read only from the stored order.
	ManualPaymentConfirmedAuditAction = "MANUAL_PAYMENT_CONFIRMED"
	maxManualPaymentReferenceLength   = 128
)

// ManualConfirmPaymentInput contains the only operator-provided payment fact.
// Amounts, currency, customer and order snapshots are never client-controlled.
type ManualConfirmPaymentInput struct {
	GatewayTransactionReference string
	Operator                    string
}

// ManualConfirmPaymentResult reports a durable confirmation separately from a
// subsequent fulfillment failure. A confirmed payment with a failed fulfillment
// remains available to the existing RetryFulfillment workflow.
type ManualConfirmPaymentResult struct {
	FulfillmentPending bool `json:"fulfillment_pending"`
}

// ManualConfirmPayment records an externally verified payment for a recoverable
// unpaid order, then invokes the ordinary idempotent fulfillment state machine.
func (s *PaymentService) ManualConfirmPayment(ctx context.Context, orderID int64, input ManualConfirmPaymentInput) (*ManualConfirmPaymentResult, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PAYMENT_SERVICE_UNAVAILABLE", "payment service is unavailable")
	}
	reference := strings.TrimSpace(input.GatewayTransactionReference)
	if reference == "" || len(reference) > maxManualPaymentReferenceLength {
		return nil, infraerrors.BadRequest("INVALID_PAYMENT_REFERENCE", "gateway transaction reference is required and must be at most 128 characters")
	}
	operator := strings.TrimSpace(input.Operator)
	if operator == "" {
		return nil, infraerrors.Forbidden("FORBIDDEN", "authenticated administrator identity is required")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin manual payment confirmation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	// The guarded UPDATE below is the row lock and compare-and-set transition.
	// Avoid a standalone SELECT FOR UPDATE here because the unit-test dialect is
	// SQLite; PostgreSQL still locks the selected order through that UPDATE.
	order, err := tx.PaymentOrder.Query().Where(paymentorder.IDEQ(orderID)).Only(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
		}
		return nil, fmt.Errorf("lock payment order for manual confirmation: %w", err)
	}
	if err := validateManualPaymentConfirmationOrder(order, reference); err != nil {
		return nil, err
	}
	if order.CouponID != nil && s.couponService == nil {
		return nil, infraerrors.ServiceUnavailable("COUPON_SERVICE_UNAVAILABLE", "coupon service is unavailable")
	}
	if err := rejectDuplicateManualPaymentReference(txCtx, tx.Client(), order, reference); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	updated, err := tx.PaymentOrder.Update().
		Where(
			paymentorder.IDEQ(order.ID),
			paymentorder.PaidAtIsNil(),
			paymentorder.StatusIn(OrderStatusPending, OrderStatusFailed, OrderStatusExpired, OrderStatusCancelled),
		).
		SetStatus(OrderStatusPaid).
		SetPaymentTradeNo(reference).
		SetPaidAt(now).
		ClearFailedAt().
		ClearFailedReason().
		Save(txCtx)
	if err != nil {
		return nil, fmt.Errorf("persist manual payment confirmation: %w", err)
	}
	if updated != 1 {
		return nil, infraerrors.Conflict("CONFLICT", "order changed while confirming payment")
	}
	if order.CouponID != nil {
		if _, err := s.couponService.ConsumeUserCouponOrderLock(txCtx, *order.CouponID, order.ID); err != nil {
			return nil, fmt.Errorf("consume payment coupon during manual confirmation: %w", err)
		}
	}
	detail, err := json.Marshal(map[string]any{
		"source":                        "admin_manual_confirmation",
		"gateway_transaction_reference": reference,
		"previous_status":               order.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("encode manual payment audit: %w", err)
	}
	if _, err := tx.PaymentAuditLog.Create().
		SetOrderID(fmt.Sprintf("%d", order.ID)).
		SetAction(ManualPaymentConfirmedAuditAction).
		SetOperator(operator).
		SetDetail(string(detail)).
		Save(txCtx); err != nil {
		return nil, fmt.Errorf("write manual payment audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit manual payment confirmation: %w", err)
	}

	result := &ManualConfirmPaymentResult{}
	if !s.manualConfirmationCanExecuteFulfillment(order) {
		// Normal server wiring always provides these services. Keeping this guard
		// makes a partially initialized process fail safely after the durable paid
		// transition instead of panicking and obscuring the retryable order.
		result.FulfillmentPending = true
		return result, nil
	}
	if err := s.executeFulfillment(ctx, order.ID); err != nil {
		// The paid transition is already durable. executeFulfillment marks a
		// failed lease as retryable, so the caller must not be told to confirm
		// the same receipt a second time.
		result.FulfillmentPending = true
	}
	return result, nil
}

func (s *PaymentService) manualConfirmationCanExecuteFulfillment(order *dbent.PaymentOrder) bool {
	if order == nil {
		return false
	}
	if order.OrderType == "subscription" {
		return s.subscriptionSvc != nil
	}
	return s.redeemService != nil
}

func validateManualPaymentConfirmationOrder(order *dbent.PaymentOrder, reference string) error {
	if order == nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if order.PaidAt != nil || !manualPaymentConfirmableStatus(order.Status) || manualPaymentHasRefundState(order) {
		return infraerrors.BadRequest("INVALID_STATUS", "only unpaid pending, failed, expired, or cancelled orders can be manually confirmed")
	}
	if saved := strings.TrimSpace(order.PaymentTradeNo); saved != "" && saved != reference {
		return infraerrors.Conflict("PAYMENT_REFERENCE_MISMATCH", "gateway transaction reference conflicts with the stored order reference")
	}
	return nil
}

func manualPaymentConfirmableStatus(status string) bool {
	switch status {
	case OrderStatusPending, OrderStatusFailed, OrderStatusExpired, OrderStatusCancelled:
		return true
	default:
		return false
	}
}

func manualPaymentHasRefundState(order *dbent.PaymentOrder) bool {
	return psIsRefundStatus(order.Status) || order.RefundAt != nil || order.RefundRequestedAt != nil || order.RefundAmount > 0 || order.ForceRefund
}

func rejectDuplicateManualPaymentReference(ctx context.Context, client *dbent.Client, order *dbent.PaymentOrder, reference string) error {
	matches, err := client.PaymentOrder.Query().
		Where(paymentorder.PaymentTradeNoEQ(reference), paymentorder.IDNEQ(order.ID)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("check duplicate payment reference: %w", err)
	}
	providerIdentity := manualPaymentProviderIdentity(order)
	for _, match := range matches {
		if manualPaymentProviderIdentity(match) == providerIdentity {
			return infraerrors.Conflict("DUPLICATE_PAYMENT_REFERENCE", "gateway transaction reference is already recorded for this payment provider")
		}
	}
	return nil
}

// manualPaymentProviderIdentity mirrors the migration's unique-index scope.
func manualPaymentProviderIdentity(order *dbent.PaymentOrder) string {
	if order == nil {
		return ""
	}
	if instanceID := strings.TrimSpace(psStringValue(order.ProviderInstanceID)); instanceID != "" {
		return "instance:" + instanceID
	}
	if providerKey := strings.ToLower(strings.TrimSpace(psStringValue(order.ProviderKey))); providerKey != "" {
		return "provider:" + providerKey
	}
	return "type:" + strings.ToLower(strings.TrimSpace(order.PaymentType))
}
