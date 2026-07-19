package service

import "context"

type rechargeTotalRechargedDeltaContextKey struct{}

// ContextWithRechargeTotalRechargedDelta lets payment fulfillment credit the
// user's balance by the final到账 amount while adding only the base recharge to
// VIP lifetime total.
func ContextWithRechargeTotalRechargedDelta(ctx context.Context, delta float64) context.Context {
	return context.WithValue(ctx, rechargeTotalRechargedDeltaContextKey{}, delta)
}

func RechargeTotalRechargedDeltaFromContext(ctx context.Context) (float64, bool) {
	delta, ok := ctx.Value(rechargeTotalRechargedDeltaContextKey{}).(float64)
	return delta, ok
}
