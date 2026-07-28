package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func couponRewardTypeAt(activity CouponRewardActivity, draw int64) (PlayRewardType, error) {
	if draw < 0 || draw >= couponWeightBasisPoints {
		return PlayRewardTypeNone, fmt.Errorf("coupon reward draw out of range: %d", draw)
	}

	var couponWeight int64
	switch activity {
	case CouponRewardActivityBlindbox:
		couponWeight = 6000
	case CouponRewardActivityQuiz:
		couponWeight = 8000
	default:
		return PlayRewardTypeNone, fmt.Errorf("unsupported coupon reward activity: %s", activity)
	}
	if draw < couponWeight {
		return PlayRewardTypeCoupon, nil
	}
	return PlayRewardTypeBalance, nil
}

func (s *PlayService) drawCouponRewardType(activity CouponRewardActivity) (PlayRewardType, error) {
	if s == nil || s.rewardDrawSource == nil {
		return PlayRewardTypeNone, fmt.Errorf("coupon reward draw source is not configured")
	}
	draw, err := s.rewardDrawSource(couponWeightBasisPoints)
	if err != nil {
		return PlayRewardTypeNone, fmt.Errorf("coupon reward draw source: %w", err)
	}
	return couponRewardTypeAt(activity, draw)
}

// couponRewardPoolReady checks the production coupon service before a reward
// draw. The optional interface keeps older PlayService test doubles working;
// Wire always supplies CouponService in the running application.
func (s *PlayService) couponRewardPoolReady(ctx context.Context, activity CouponRewardActivity) (bool, error) {
	if s == nil {
		return false, ErrCouponRewardPoolUnavailable
	}
	if readinessReader, ok := s.couponRewardIssuer.(CouponRewardPoolReadinessReader); ok && readinessReader != nil {
		ready, err := readinessReader.CouponRewardPoolReady(ctx, activity)
		if err != nil {
			if errors.Is(err, ErrCouponRewardPoolUnavailable) {
				return false, nil
			}
			return false, err
		}
		return ready, nil
	}
	reader, ok := s.couponRewardIssuer.(CouponRewardPoolReader)
	if !ok || reader == nil {
		return true, nil
	}
	pool, err := reader.GetPublishedRewardPool(ctx, activity)
	if err != nil {
		if errors.Is(err, ErrCouponRewardPoolUnavailable) {
			return false, nil
		}
		return false, err
	}
	return pool != nil, nil
}

func (s *PlayService) couponRewardPrizePreview(ctx context.Context, activity CouponRewardActivity) ([]PlayCouponPrizePreview, error) {
	reader, ok := s.couponRewardIssuer.(CouponRewardPoolReader)
	if !ok || reader == nil {
		return []PlayCouponPrizePreview{}, nil
	}
	pool, err := reader.GetPublishedRewardPool(ctx, activity)
	if err != nil {
		if errors.Is(err, ErrCouponRewardPoolUnavailable) {
			return []PlayCouponPrizePreview{}, nil
		}
		return nil, err
	}
	if pool == nil {
		return []PlayCouponPrizePreview{}, nil
	}
	out := make([]PlayCouponPrizePreview, 0, len(pool.Entries))
	for _, entry := range pool.Entries {
		if !entry.Enabled || entry.TemplateID <= 0 || entry.TemplateID == pool.FallbackTemplateID {
			continue
		}
		out = append(out, PlayCouponPrizePreview{
			TemplateID: entry.TemplateID,
			Name:       entry.TemplateName,
			WeightBP:   entry.WeightBP,
			Tier:       couponPrizePreviewTier(entry.WeightBP),
		})
	}
	return out, nil
}

func couponPrizePreviewTier(weight int) string {
	switch {
	case weight >= 1500:
		return "common"
	case weight >= 500:
		return "standard"
	case weight >= 100:
		return "rare"
	default:
		return "jackpot"
	}
}

func (s *PlayService) requireCouponRewardPool(ctx context.Context, activity CouponRewardActivity) error {
	ready, err := s.couponRewardPoolReady(ctx, activity)
	if err != nil {
		return err
	}
	if !ready {
		return ErrCouponRewardPoolUnavailable
	}
	return nil
}

func (s *PlayService) issueCouponRewardInTx(
	ctx context.Context,
	userID int64,
	activity CouponRewardActivity,
	idempotencyKey string,
	sourceRef string,
	issuedAt time.Time,
) (*CouponRewardIssueResult, error) {
	if s == nil || s.couponRewardIssuer == nil {
		return nil, ErrCouponRewardPoolUnavailable
	}
	result, err := s.couponRewardIssuer.DrawAndIssueInTx(ctx, CouponRewardDrawRequest{
		UserID:         userID,
		Activity:       activity,
		IdempotencyKey: idempotencyKey,
		SourceRef:      sourceRef,
		IssuedAt:       issuedAt,
	})
	if err != nil {
		return nil, err
	}
	if result == nil || result.UserCouponID <= 0 || result.TemplateID <= 0 {
		return nil, ErrCouponRewardPoolUnavailable
	}
	return result, nil
}

func playCouponRewardSummary(result *CouponRewardIssueResult) *PlayCouponRewardSummary {
	if result == nil {
		return nil
	}
	coupon := result.Coupon
	userCouponID := result.UserCouponID
	if coupon.ID > 0 {
		userCouponID = coupon.ID
	}
	templateID := result.TemplateID
	if coupon.TemplateID > 0 {
		templateID = coupon.TemplateID
	}
	validFrom := result.ValidFrom
	if validFrom.IsZero() {
		validFrom = coupon.ValidFrom
	}
	expiresAt := result.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = coupon.ExpiresAt
	}
	terms := coupon.TermsSnapshot
	return &PlayCouponRewardSummary{
		UserCouponID:       userCouponID,
		TemplateID:         templateID,
		Name:               coupon.TemplateName,
		BenefitType:        terms.BenefitType,
		BenefitValue:       terms.BenefitValue,
		MaxDiscountAmount:  cloneFloat64(terms.MaxDiscountAmount),
		Currency:           terms.Currency,
		ApplicableScopes:   append([]CouponScope(nil), terms.ApplicableScopes...),
		MinimumOrderAmount: terms.MinimumOrderAmount,
		ValidFrom:          validFrom,
		ExpiresAt:          expiresAt,
	}
}

func couponPoolVersion(result *CouponRewardIssueResult) string {
	if result == nil {
		return ""
	}
	return result.PoolVersion
}
