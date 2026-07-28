//go:build unit

package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type paymentCouponLifecycleStub struct {
	lockResult *CouponLockResult
	lockErr    error
	consumeErr error
	releaseErr error

	lockInTx    bool
	consumeInTx bool
	releaseInTx bool
	lockCalls   []CouponLockRequest
	consumeIDs  []int64
	releaseIDs  []int64
}

func (s *paymentCouponLifecycleStub) GetUserCouponForUser(context.Context, int64, int64) (*UserCoupon, error) {
	return nil, errors.New("unexpected user coupon lookup")
}

func (s *paymentCouponLifecycleStub) QuoteUserCoupon(context.Context, int64, CouponOrderContext) (*CouponQuote, error) {
	return nil, errors.New("unexpected coupon quote")
}

func (s *paymentCouponLifecycleStub) LockUserCouponForOrder(ctx context.Context, request CouponLockRequest) (*CouponLockResult, error) {
	s.lockInTx = dbent.TxFromContext(ctx) != nil
	s.lockCalls = append(s.lockCalls, request)
	if s.lockErr != nil {
		return nil, s.lockErr
	}
	return s.lockResult, nil
}

func (s *paymentCouponLifecycleStub) ReleaseUserCouponOrderLock(ctx context.Context, couponID, _ int64) (*UserCoupon, error) {
	s.releaseInTx = dbent.TxFromContext(ctx) != nil
	s.releaseIDs = append(s.releaseIDs, couponID)
	if s.releaseErr != nil {
		return nil, s.releaseErr
	}
	return &UserCoupon{ID: couponID}, nil
}

func (s *paymentCouponLifecycleStub) ConsumeUserCouponOrderLock(ctx context.Context, couponID, _ int64) (*UserCoupon, error) {
	s.consumeInTx = dbent.TxFromContext(ctx) != nil
	s.consumeIDs = append(s.consumeIDs, couponID)
	if s.consumeErr != nil {
		return nil, s.consumeErr
	}
	return &UserCoupon{ID: couponID, Status: UserCouponStatusUsed}, nil
}

func couponLifecycleSettlement() *paymentCouponSettlement {
	return &paymentCouponSettlement{
		ListAmount:               100,
		GatewayBaseAmount:        80,
		DiscountAmount:           20,
		FeeAmount:                2,
		PayAmount:                82,
		PayAmountText:            "82.00",
		Currency:                 payment.DefaultPaymentCurrency,
		QualifyingRechargeAmount: 80,
		CouponQuote: &CouponQuote{
			UserCouponID:   41,
			TemplateID:     9,
			OriginalAmount: 100,
			DiscountAmount: 20,
			PayableAmount:  80,
			Currency:       payment.DefaultPaymentCurrency,
		},
	}
}

func couponLifecycleLockResult() *CouponLockResult {
	return &CouponLockResult{
		Coupon: UserCoupon{
			ID:         41,
			TemplateID: 9,
			UserID:     7,
			Status:     UserCouponStatusLocked,
			TermsSnapshot: CouponTermsSnapshot{
				TemplateID:       9,
				TemplateVersion:  1,
				TemplateKey:      "payment-20-off",
				Name:             "20 off 100",
				BenefitType:      CouponBenefitTypeFixedAmount,
				BenefitValue:     20,
				Currency:         payment.DefaultPaymentCurrency,
				ApplicableScopes: []CouponScope{CouponScopeBalance},
			},
		},
		Quote: *couponLifecycleSettlement().CouponQuote,
	}
}

func createCouponLifecycleUser(t *testing.T, client *dbent.Client) *dbent.User {
	t.Helper()
	suffix := time.Now().UTC().Format("150405.000000000")
	user, err := client.User.Create().
		SetEmail("coupon-lifecycle-" + suffix + "@example.com").
		SetPasswordHash("coupon-lifecycle-test-password").
		SetUsername("coupon-lifecycle-" + suffix).
		Save(context.Background())
	require.NoError(t, err)
	return user
}

func createCouponLifecycleOrder(t *testing.T, client *dbent.Client, couponID int64, status string, updatedAt time.Time) *dbent.PaymentOrder {
	t.Helper()
	user := createCouponLifecycleUser(t, client)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail("coupon-lifecycle@example.com").
		SetUserName("coupon-lifecycle-user").
		SetAmount(100).
		SetListAmount(100).
		SetGatewayBaseAmount(80).
		SetDiscountAmount(20).
		SetFeeAmount(2).
		SetQualifyingRechargeAmount(80).
		SetPaymentCurrency(payment.DefaultPaymentCurrency).
		SetPayAmount(82).
		SetFeeRate(2.5).
		SetCouponID(couponID).
		SetCouponTemplateID(9).
		SetRechargeCode("COUPON-LIFECYCLE-" + time.Now().UTC().Format("150405.000000000")).
		SetOutTradeNo("coupon-lifecycle-" + time.Now().UTC().Format("150405.000000000")).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(status).
		SetExpiresAt(time.Now().UTC().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetUpdatedAt(updatedAt).
		Save(context.Background())
	require.NoError(t, err)
	return order
}

func TestCreateOrderInTxLocksCouponAndPersistsSettlementSnapshot(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	stub := &paymentCouponLifecycleStub{lockResult: couponLifecycleLockResult()}
	svc := &PaymentService{entClient: client, couponService: stub}
	user := createCouponLifecycleUser(t, client)

	order, err := svc.createOrderInTx(ctx, CreateOrderRequest{
		UserID:      user.ID,
		CouponID:    41,
		PaymentType: payment.TypeAlipay,
		OrderType:   payment.OrderTypeBalance,
		ClientIP:    "127.0.0.1",
		SrcHost:     "api.example.com",
	}, &User{ID: user.ID, Email: user.Email, Username: user.Username}, nil,
		&PaymentConfig{OrderTimeoutMin: 10}, 100, 100, 2.5, couponLifecycleSettlement(), nil, nil)

	require.NoError(t, err)
	require.True(t, stub.lockInTx)
	require.Len(t, stub.lockCalls, 1)
	require.Equal(t, int64(41), stub.lockCalls[0].UserCouponID)
	require.Equal(t, order.ID, stub.lockCalls[0].OrderID)
	require.NotNil(t, order.CouponID)
	require.Equal(t, int64(41), *order.CouponID)
	require.NotNil(t, order.CouponTemplateID)
	require.Equal(t, int64(9), *order.CouponTemplateID)
	require.Equal(t, 20.0, order.DiscountAmount)
	require.Equal(t, 82.0, order.PayAmount)
	require.Equal(t, 80.0, order.QualifyingRechargeAmount)
	require.EqualValues(t, 41, order.CouponSnapshot["user_coupon_id"])
}

func TestCreateOrderInTxRollsBackWhenCouponCannotBeLocked(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	stub := &paymentCouponLifecycleStub{lockErr: errors.New("coupon unavailable")}
	svc := &PaymentService{entClient: client, couponService: stub}
	user := createCouponLifecycleUser(t, client)

	_, err := svc.createOrderInTx(ctx, CreateOrderRequest{
		UserID:      user.ID,
		CouponID:    41,
		PaymentType: payment.TypeAlipay,
		OrderType:   payment.OrderTypeBalance,
		ClientIP:    "127.0.0.1",
		SrcHost:     "api.example.com",
	}, &User{ID: user.ID, Email: user.Email, Username: user.Username}, nil,
		&PaymentConfig{OrderTimeoutMin: 10}, 100, 100, 2.5, couponLifecycleSettlement(), nil, nil)

	require.ErrorContains(t, err, "coupon unavailable")
	count, countErr := client.PaymentOrder.Query().Count(ctx)
	require.NoError(t, countErr)
	require.Zero(t, count)
}

func TestPaidCouponOrderConsumesOnceAcrossDuplicatePaymentCallbacks(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	order := createCouponLifecycleOrder(t, client, 41, OrderStatusPending, time.Now().UTC())
	stub := &paymentCouponLifecycleStub{}
	svc := &PaymentService{entClient: client, couponService: stub}

	updated, err := svc.markOrderPaidAndConsumeCoupon(ctx, order, "trade-41", 82, true)
	require.NoError(t, err)
	require.True(t, updated)
	require.True(t, stub.consumeInTx)
	require.Equal(t, []int64{41}, stub.consumeIDs)

	updated, err = svc.markOrderPaidAndConsumeCoupon(ctx, order, "trade-41", 82, true)
	require.NoError(t, err)
	require.False(t, updated)
	require.Equal(t, []int64{41}, stub.consumeIDs)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, reloaded.Status)
	require.NotNil(t, reloaded.PaidAt)
}

func TestCouponLocksRetainGatewayFailureDuringGraceAndReleaseUnpaidTerminalOrders(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	stub := &paymentCouponLifecycleStub{}
	svc := &PaymentService{entClient: client, couponService: stub}

	gatewayFailure := createCouponLifecycleOrder(t, client, 41, OrderStatusFailed, time.Now().UTC())
	svc.releaseCouponAfterGatewayCreateFailure(ctx, gatewayFailure)
	require.Empty(t, stub.releaseIDs, "an ambiguous provider failure must retain the coupon through the late-webhook grace period")
	released, err := svc.ReleaseCouponLocksAfterLatePaymentGrace(ctx)
	require.NoError(t, err)
	require.Zero(t, released, "recent failed orders remain locked during the late-webhook grace period")

	updated, err := svc.markOrderPaidAndConsumeCoupon(ctx, gatewayFailure, "late-trade-41", 82, true)
	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, []int64{41}, stub.consumeIDs)
	paidAfterLateCallback, err := client.PaymentOrder.Get(ctx, gatewayFailure.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, paidAfterLateCallback.Status)
	require.NotNil(t, paidAfterLateCallback.PaidAt)

	old := time.Now().UTC().Add(-(paymentGraceMinutes + 1) * time.Minute)
	createCouponLifecycleOrder(t, client, 42, OrderStatusCancelled, old)
	createCouponLifecycleOrder(t, client, 43, OrderStatusExpired, old)
	createCouponLifecycleOrder(t, client, 44, OrderStatusFailed, old)

	released, err = svc.ReleaseCouponLocksAfterLatePaymentGrace(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, released)
	require.True(t, stub.releaseInTx, "coupon release must share the conditional payment-order claim transaction")
	sort.Slice(stub.releaseIDs, func(i, j int) bool { return stub.releaseIDs[i] < stub.releaseIDs[j] })
	require.Equal(t, []int64{42, 43, 44}, stub.releaseIDs)
}

func TestGatewayCreateFailureDoesNotOverwritePaidOrCompletedCouponOrders(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	stub := &paymentCouponLifecycleStub{}
	svc := &PaymentService{entClient: client, couponService: stub}

	pending := createCouponLifecycleOrder(t, client, 41, OrderStatusPending, time.Now().UTC())
	paid := createCouponLifecycleOrder(t, client, 42, OrderStatusPaid, time.Now().UTC())
	completed := createCouponLifecycleOrder(t, client, 43, OrderStatusCompleted, time.Now().UTC())

	// This models a provider accepting a payment and sending its webhook before
	// the synchronous CreatePayment request returns a timeout to this process.
	for _, order := range []*dbent.PaymentOrder{pending, paid, completed} {
		svc.failPendingOrderAfterGatewayCreateError(ctx, order, "provider create timeout")
	}

	pendingReloaded, err := client.PaymentOrder.Get(ctx, pending.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusFailed, pendingReloaded.Status)
	require.NotNil(t, pendingReloaded.FailedAt)

	paidReloaded, err := client.PaymentOrder.Get(ctx, paid.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, paidReloaded.Status)
	require.Nil(t, paidReloaded.FailedAt)

	completedReloaded, err := client.PaymentOrder.Get(ctx, completed.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, completedReloaded.Status)
	require.Nil(t, completedReloaded.FailedAt)

	failedAudits, err := client.PaymentAuditLog.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, failedAudits, "only the pending order may record a gateway-create failure")
	require.Empty(t, stub.releaseIDs, "a stale create error must not touch coupon handling for paid or completed orders")
}

func TestLateCancelledAndExpiredCouponOrdersWithinGraceConsumeCoupon(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	stub := &paymentCouponLifecycleStub{}
	svc := &PaymentService{entClient: client, couponService: stub}

	withinGrace := time.Now().UTC().Add(-time.Duration(paymentGraceMinutes-1) * time.Minute)
	cancelled := createCouponLifecycleOrder(t, client, 41, OrderStatusCancelled, withinGrace)
	expired := createCouponLifecycleOrder(t, client, 42, OrderStatusExpired, withinGrace)

	released, err := svc.ReleaseCouponLocksAfterLatePaymentGrace(ctx)
	require.NoError(t, err)
	require.Zero(t, released, "the original coupon lock must survive the late-payment grace period")
	require.Empty(t, stub.releaseIDs)

	for _, order := range []*dbent.PaymentOrder{cancelled, expired} {
		updated, markErr := svc.markOrderPaidAndConsumeCoupon(ctx, order, "late-"+order.Status, order.PayAmount, true)
		require.NoError(t, markErr)
		require.True(t, updated)

		reloaded, getErr := client.PaymentOrder.Get(ctx, order.ID)
		require.NoError(t, getErr)
		require.Equal(t, OrderStatusPaid, reloaded.Status)
		require.NotNil(t, reloaded.PaidAt)
	}
	require.ElementsMatch(t, []int64{41, 42}, stub.consumeIDs)
}

func TestLateFailedCouponOrderAfterGraceDoesNotFulfillOrConsumeCoupon(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	stub := &paymentCouponLifecycleStub{}
	svc := &PaymentService{entClient: client, couponService: stub}

	old := time.Now().UTC().Add(-(paymentGraceMinutes + 1) * time.Minute)
	order := createCouponLifecycleOrder(t, client, 41, OrderStatusFailed, old)

	err := svc.toPaid(ctx, order, "late-trade-after-grace", 82, payment.TypeAlipay)
	require.NoError(t, err)
	require.Empty(t, stub.consumeIDs)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusFailed, reloaded.Status)
	require.Nil(t, reloaded.PaidAt)
}

func TestLateCancelledAndExpiredCouponOrdersAfterGraceDoNotConsumeCoupon(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	stub := &paymentCouponLifecycleStub{}
	svc := &PaymentService{entClient: client, couponService: stub}

	outsideGrace := time.Now().UTC().Add(-(paymentGraceMinutes + 1) * time.Minute)
	for _, status := range []string{OrderStatusCancelled, OrderStatusExpired} {
		order := createCouponLifecycleOrder(t, client, int64(len(stub.consumeIDs)+41), status, outsideGrace)
		updated, err := svc.markOrderPaidAndConsumeCoupon(ctx, order, "too-late-"+status, order.PayAmount, true)
		require.NoError(t, err)
		require.False(t, updated)

		reloaded, getErr := client.PaymentOrder.Get(ctx, order.ID)
		require.NoError(t, getErr)
		require.Equal(t, status, reloaded.Status)
		require.Nil(t, reloaded.PaidAt)
	}
	require.Empty(t, stub.consumeIDs)
}

func TestReleaseCouponLocksSkipsFailedOrderThatWasAlreadyPaid(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	stub := &paymentCouponLifecycleStub{}
	svc := &PaymentService{entClient: client, couponService: stub}

	old := time.Now().UTC().Add(-(paymentGraceMinutes + 1) * time.Minute)
	order := createCouponLifecycleOrder(t, client, 41, OrderStatusFailed, old)
	_, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetPaidAt(old).
		SetUpdatedAt(old).
		Save(ctx)
	require.NoError(t, err)

	released, err := svc.ReleaseCouponLocksAfterLatePaymentGrace(ctx)
	require.NoError(t, err)
	require.Zero(t, released)
	require.Empty(t, stub.releaseIDs)
}

func TestCompleteZeroCouponOrderConsumesCouponAndFulfillsBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	user := createCouponLifecycleUser(t, client)
	order := createCouponLifecycleOrder(t, client, 41, OrderStatusPending, time.Now().UTC())
	order, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetGatewayBaseAmount(0).
		SetDiscountAmount(order.ListAmount).
		SetFeeAmount(0).
		SetQualifyingRechargeAmount(0).
		SetPayAmount(0).
		Save(ctx)
	require.NoError(t, err)

	userRepo := &mockUserRepo{
		getByIDUser: &User{ID: user.ID, Email: user.Email, Username: user.Username},
	}
	userRepo.updateBalanceFn = func(_ context.Context, userID int64, amount float64) error {
		require.Equal(t, user.ID, userID)
		userRepo.getByIDUser.Balance += amount
		return nil
	}
	redeemRepo := &paymentOrderLifecycleRedeemRepo{codesByCode: map[string]*RedeemCode{
		order.RechargeCode: {
			ID:     1,
			Code:   order.RechargeCode,
			Type:   RedeemTypeBalance,
			Value:  order.Amount,
			Status: StatusUnused,
		},
	}}
	stub := &paymentCouponLifecycleStub{}
	svc := &PaymentService{
		entClient:     client,
		couponService: stub,
		redeemService: NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, nil, nil),
	}

	require.NoError(t, svc.completeZeroCouponOrder(ctx, order))

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.Equal(t, 0.0, reloaded.PayAmount)
	require.Equal(t, []int64{41}, stub.consumeIDs)
	require.Len(t, redeemRepo.useCalls, 1)
	require.Equal(t, order.Amount, userRepo.getByIDUser.Balance)
}

func TestCouponOrderRefundTracksCashPaidAndQualifyingRechargeSeparately(t *testing.T) {
	order := &dbent.PaymentOrder{
		OrderType:                payment.OrderTypeBalance,
		Amount:                   100,
		ListAmount:               100,
		GatewayBaseAmount:        80,
		DiscountAmount:           20,
		PayAmount:                82,
		QualifyingRechargeAmount: 80,
		PaymentCurrency:          payment.DefaultPaymentCurrency,
	}

	require.Equal(t, 82.0, calculateGatewayRefundAmount(order.Amount, order.PayAmount, 100, order.PaymentCurrency))
	require.Equal(t, 41.0, calculateGatewayRefundAmount(order.Amount, order.PayAmount, 50, order.PaymentCurrency))
	require.Zero(t, calculateGatewayRefundAmount(order.Amount, 0, 100, order.PaymentCurrency))

	base, fromSettlement := paymentOrderRechargeBaseCredited(order)
	require.True(t, fromSettlement)
	require.Equal(t, 80.0, base)
	require.Equal(t, -40.0, paymentOrderRefundTotalRechargedDelta(order, 50))
}
