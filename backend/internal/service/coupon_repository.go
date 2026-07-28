package service

import (
	"context"
	"time"
)

// CouponRepository owns the persistence and row-locking boundaries for the
// coupon domain. CouponService keeps policy and validation above this layer.
type CouponRepository interface {
	CreateCouponTemplate(ctx context.Context, template CouponTemplate) (*CouponTemplate, error)
	GetCouponTemplate(ctx context.Context, id int64) (*CouponTemplate, error)
	UpdateCouponTemplate(ctx context.Context, template CouponTemplate) (*CouponTemplate, error)
	DeleteCouponTemplate(ctx context.Context, id int64) error
	ListCouponTemplates(ctx context.Context, filter CouponTemplateListFilter) ([]CouponTemplate, int64, error)

	IssueCoupon(ctx context.Context, request CouponIssueInput, issuedAt time.Time) (*UserCoupon, error)
	IssueCouponBatch(ctx context.Context, request CouponBatchIssueInput, issuedAt time.Time) (*CouponIssueBatch, error)
	GetUserCoupon(ctx context.Context, id int64) (*UserCoupon, error)
	ListUserCoupons(ctx context.Context, filter UserCouponListFilter) ([]UserCoupon, int64, error)
	ExpireAvailableUserCoupons(ctx context.Context, userID int64, at time.Time) error
	VoidUserCoupon(ctx context.Context, couponID int64, reason string, actorID int64, at time.Time) (*UserCoupon, error)
	ListCouponIssueBatches(ctx context.Context, filter CouponIssueBatchListFilter) ([]CouponIssueBatch, int64, error)

	SaveCouponRewardPool(ctx context.Context, pool CouponRewardPoolVersion) (*CouponRewardPoolVersion, error)
	GetCouponRewardPool(ctx context.Context, id int64) (*CouponRewardPoolVersion, error)
	ListCouponRewardPools(ctx context.Context, activity CouponRewardActivity) ([]CouponRewardPoolVersion, error)
	DeleteCouponRewardPool(ctx context.Context, id int64) error
	PublishCouponRewardPool(ctx context.Context, id int64, actorID int64, at time.Time) (*CouponRewardPoolVersion, error)
	GetPublishedCouponRewardPool(ctx context.Context, activity CouponRewardActivity) (*CouponRewardPoolVersion, error)
	HasIssuableCouponRewardEntry(ctx context.Context, activity CouponRewardActivity, at time.Time) (bool, error)

	LockUserCouponForOrder(ctx context.Context, request CouponLockRequest) (*CouponLockResult, error)
	ReleaseUserCouponOrderLock(ctx context.Context, couponID, orderID int64, at time.Time) (*UserCoupon, error)
	ConsumeUserCouponOrderLock(ctx context.Context, couponID, orderID int64, at time.Time) (*UserCoupon, error)

	DrawAndIssueCouponRewardInTx(ctx context.Context, request CouponRewardDrawRequest) (*CouponRewardIssueResult, error)
	FindCouponRewardIssueByIdempotency(ctx context.Context, idempotencyKey string) (*CouponRewardIssueResult, error)
}

type CouponLockRequest struct {
	UserCouponID int64
	UserID       int64
	OrderID      int64
	OrderContext CouponOrderContext
	LockedAt     time.Time
}

type CouponLockResult struct {
	Coupon UserCoupon
	Quote  CouponQuote
}

// CouponRewardIssuer lets play flows depend on the coupon branch only. The
// caller selects the outer balance/coupon branch and must never convert an
// issuer configuration error into a balance reward.
type CouponRewardIssuer interface {
	DrawAndIssueInTx(ctx context.Context, request CouponRewardDrawRequest) (*CouponRewardIssueResult, error)
}

// CouponRewardReplayReader restores an already-issued coupon for an
// idempotent play response. It is separate from issuance so a replay cannot
// accidentally create a new user coupon.
type CouponRewardReplayReader interface {
	GetCouponRewardIssueByIdempotency(ctx context.Context, userID int64, idempotencyKey string) (*CouponRewardIssueResult, error)
}

// CouponRewardPoolReader is intentionally separate from CouponRewardIssuer so
// the play service can fail closed before an activity consumes balance or an
// answer submission. Existing lightweight play implementations that only
// issue coupons remain compatible with the legacy test surface.
type CouponRewardPoolReader interface {
	GetPublishedRewardPool(ctx context.Context, activity CouponRewardActivity) (*CouponRewardPoolVersion, error)
}

// CouponRewardPoolReadinessReader adds a live issuance check to the lightweight
// published-pool lookup. A published configuration alone is not enough to let
// a play activity start: at least one entry must be issuable right now.
type CouponRewardPoolReadinessReader interface {
	CouponRewardPoolReady(ctx context.Context, activity CouponRewardActivity) (bool, error)
}

type RedeemCodeRewardIssuer interface {
	ClaimRedeemCodeRewardInTx(ctx context.Context, request RedeemCodeRewardClaimRequest) (*RedeemCode, error)
}

type CouponRewardDrawRequest struct {
	UserID         int64
	Activity       CouponRewardActivity
	IdempotencyKey string
	SourceRef      string
	IssuedAt       time.Time
}

type CouponRewardIssueResult struct {
	PoolVersionID int64      `json:"pool_version_id"`
	PoolVersion   string     `json:"pool_version"`
	PoolEntryID   int64      `json:"pool_entry_id"`
	TemplateID    int64      `json:"template_id"`
	UserCouponID  int64      `json:"user_coupon_id"`
	Coupon        UserCoupon `json:"coupon"`
	ValidFrom     time.Time  `json:"valid_from"`
	ExpiresAt     time.Time  `json:"expires_at"`
	FallbackUsed  bool       `json:"fallback_used"`
}
