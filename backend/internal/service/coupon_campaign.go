package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// CouponAudience reuses user-management filters. All modes exclude inactive,
// deleted and administrator accounts; explicit IDs are never interpreted as all.
type CouponAudience struct {
	Mode          string           `json:"mode"`
	UserIDs       []int64          `json:"user_ids,omitempty"`
	Search        string           `json:"search,omitempty"`
	GroupName     string           `json:"group_name,omitempty"`
	APIKeyGroupID int64            `json:"api_key_group_id,omitempty"`
	VIPTier       *int             `json:"vip_tier,omitempty"`
	Attributes    map[int64]string `json:"attributes,omitempty"`
}
type CouponClaimPolicy struct {
	TemplateID   int64          `json:"template_id"`
	Code         string         `json:"code"`
	Enabled      bool           `json:"enabled"`
	StartsAt     *time.Time     `json:"starts_at"`
	EndsAt       *time.Time     `json:"ends_at"`
	ClaimLimit   int64          `json:"claim_limit"`
	PerUserLimit int            `json:"per_user_limit"`
	Audience     CouponAudience `json:"audience"`
	ClaimedCount int64          `json:"claimed_count"`
	IssuedCount  int64          `json:"issued_count"`
	UsedCount    int64          `json:"used_count"`
	ManualCount  int64          `json:"manual_count"`
	Version      int            `json:"version"`
}
type CouponRecipient struct {
	UserID int64  `json:"user_id"`
	Reason string `json:"reason"`
}
type CouponCampaignPreview struct {
	Token           string            `json:"token"`
	TemplateID      int64             `json:"template_id"`
	TemplateVersion int               `json:"template_version"`
	PolicyVersion   int               `json:"policy_version"`
	ActorID         int64             `json:"-"`
	Audience        CouponAudience    `json:"audience"`
	Recipients      []int64           `json:"recipients"`
	Skipped         []CouponRecipient `json:"skipped"`
	RequestedCount  int               `json:"requested_count"`
	RemainingStock  *int64            `json:"remaining_stock"`
	ExpiresAt       time.Time         `json:"expires_at"`
	BatchID         *int64            `json:"batch_id"`
}
type CouponCampaignResult struct {
	BatchID     int64             `json:"batch_id"`
	IssuedCount int               `json:"issued_count"`
	Skipped     []CouponRecipient `json:"skipped"`
}
type CouponCurrencySummary struct {
	Currency string  `json:"currency"`
	Paid     float64 `json:"paid"`
	Discount float64 `json:"discount"`
	Refunded float64 `json:"refunded"`
	NetPaid  float64 `json:"net_paid"`
}
type CouponSummary struct {
	Issued    int64                   `json:"issued"`
	Available int64                   `json:"available"`
	Used      int64                   `json:"used"`
	Amounts   []CouponCurrencySummary `json:"amounts"`
}
type CouponEvent struct {
	ID        int64          `json:"id"`
	EventType string         `json:"event_type"`
	OrderID   *int64         `json:"order_id"`
	ActorType string         `json:"actor_type"`
	CreatedAt time.Time      `json:"created_at"`
	Metadata  map[string]any `json:"metadata"`
}
type CouponCampaignRepository interface {
	ListCouponClaimPolicies(context.Context, []int64) ([]CouponClaimPolicy, error)
	GetCouponClaimPolicy(context.Context, int64) (*CouponClaimPolicy, error)
	SaveCouponClaimPolicy(context.Context, int64, CouponClaimPolicy, int64) (*CouponClaimPolicy, error)
	ClaimCouponCode(context.Context, string, int64, time.Time) (*UserCoupon, error)
	PreviewCouponCampaign(context.Context, int64, CouponAudience, int64, time.Time) (*CouponCampaignPreview, error)
	ExecuteCouponCampaign(context.Context, string, int64, time.Time) (*CouponCampaignResult, error)
	CouponSummary(context.Context, UserCouponListFilter) (*CouponSummary, error)
	CouponEvents(context.Context, int64) ([]CouponEvent, error)
}

func ValidateCouponAudience(a CouponAudience) error {
	switch a.Mode {
	case "all":
		if len(a.UserIDs) > 0 || a.Search != "" || a.GroupName != "" || a.APIKeyGroupID != 0 || a.VIPTier != nil || len(a.Attributes) > 0 {
			return fmt.Errorf("all users cannot include filters")
		}
	case "users":
		if len(a.UserIDs) == 0 || len(a.UserIDs) > 10000 {
			return fmt.Errorf("select between 1 and 10000 users")
		}
		for _, id := range a.UserIDs {
			if id <= 0 {
				return fmt.Errorf("user IDs must be positive")
			}
		}
	case "filtered":
		if len(a.UserIDs) > 0 {
			return fmt.Errorf("filtered audience cannot include explicit IDs")
		}
	default:
		return fmt.Errorf("select an audience mode")
	}
	if a.APIKeyGroupID < 0 || (a.VIPTier != nil && *a.VIPTier < 0) || len(a.Attributes) > 30 || len(a.Search) > 200 || len(a.GroupName) > 200 {
		return fmt.Errorf("invalid audience filter")
	}
	for id, v := range a.Attributes {
		if id <= 0 || len(v) > 500 {
			return fmt.Errorf("invalid attribute filter")
		}
	}
	return nil
}
func ValidateCouponClaimPolicy(p CouponClaimPolicy) error {
	if p.ClaimLimit < 0 || p.PerUserLimit < 1 || p.PerUserLimit > 10 {
		return fmt.Errorf("per-user limit must be 1–10 and total limit nonnegative")
	}
	if p.Enabled && (p.ClaimLimit <= 0 || p.EndsAt == nil) {
		return fmt.Errorf("enabled codes need a finite claim limit and end time")
	}
	if p.StartsAt != nil && p.EndsAt != nil && !p.EndsAt.After(*p.StartsAt) {
		return fmt.Errorf("end time must follow start time")
	}
	if p.Audience.Mode == "" {
		p.Audience.Mode = "all"
	}
	return ValidateCouponAudience(p.Audience)
}

// CouponOrderCashRefund shares the gateway rounding rules and counts only
// confirmed refunds. Coupon discounts are never refunded as cash.
func CouponOrderCashRefund(amount, payAmount, refundAmount float64, currency, status string) float64 {
	if status != payment.OrderStatusRefunded && status != payment.OrderStatusPartiallyRefunded {
		return 0
	}
	return calculateGatewayRefundAmount(amount, payAmount, refundAmount, currency)
}

func (s *CouponService) CampaignRepository() (CouponCampaignRepository, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	r, ok := s.repo.(CouponCampaignRepository)
	if !ok {
		return nil, ErrCouponRepositoryUnavailable
	}
	return r, nil
}

func (s *CouponService) CampaignSummary(ctx context.Context, filter UserCouponListFilter) (*CouponSummary, error) {
	if filter.Status != "" && !isUserCouponStatus(filter.Status) {
		return nil, infraerrors.BadRequest("INVALID_USER_COUPON_STATUS", "user coupon status is invalid")
	}
	if filter.Source != "" && !isCouponIssueSource(filter.Source) {
		return nil, infraerrors.BadRequest("INVALID_COUPON_SOURCE", "coupon issue source is invalid")
	}
	r, err := s.CampaignRepository()
	if err != nil {
		return nil, err
	}
	return r.CouponSummary(ctx, filter)
}
func (s *CouponService) ClaimCode(ctx context.Context, code string, userID int64) (*UserCoupon, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if userID <= 0 || len(code) < 5 || len(code) > 64 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_CODE", "invalid coupon code")
	}
	r, err := s.CampaignRepository()
	if err != nil {
		return nil, err
	}
	return r.ClaimCouponCode(ctx, code, userID, s.now())
}
