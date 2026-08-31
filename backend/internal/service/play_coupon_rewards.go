package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func marketingRewardTypeAt(couponWeightBP, redeemCodeWeightBP int, draw int64) (PlayRewardType, error) {
	if draw < 0 || draw >= couponWeightBasisPoints {
		return PlayRewardTypeNone, fmt.Errorf("coupon reward draw out of range: %d", draw)
	}
	if couponWeightBP < 0 || redeemCodeWeightBP < 0 || couponWeightBP+redeemCodeWeightBP > couponWeightBasisPoints {
		return PlayRewardTypeNone, fmt.Errorf("coupon reward weight out of range: %d", couponWeightBP)
	}
	if draw < int64(couponWeightBP) {
		return PlayRewardTypeCoupon, nil
	}
	if draw < int64(couponWeightBP+redeemCodeWeightBP) {
		return PlayRewardTypeRedeem, nil
	}
	return PlayRewardTypeBalance, nil
}

func couponRewardTypeAt(couponWeightBP int, draw int64) (PlayRewardType, error) {
	return marketingRewardTypeAt(couponWeightBP, 0, draw)
}

func defaultCouponRewardSplit(activity CouponRewardActivity) (int, int, int, error) {
	switch activity {
	case CouponRewardActivityBlindbox:
		return 6000, 0, 4000, nil
	case CouponRewardActivityQuiz:
		return 8000, 0, 2000, nil
	case CouponRewardActivityCheckin:
		return 8000, 2000, 0, nil
	default:
		return 0, 0, 0, fmt.Errorf("unsupported coupon reward activity: %s", activity)
	}
}

func (s *PlayService) couponRewardSplit(ctx context.Context, activity CouponRewardActivity) (int, int, int, error) {
	if s == nil {
		return defaultCouponRewardSplit(activity)
	}
	reader, ok := s.couponRewardIssuer.(CouponRewardPoolReader)
	if !ok || reader == nil {
		return defaultCouponRewardSplit(activity)
	}
	pool, err := reader.GetPublishedRewardPool(ctx, activity)
	if err != nil {
		return 0, 0, 0, err
	}
	if pool == nil {
		return 0, 0, 0, ErrCouponRewardPoolUnavailable
	}
	if pool.CouponWeightBP < 0 || pool.RedeemCodeWeightBP < 0 || pool.BalanceWeightBP < 0 || pool.CouponWeightBP+pool.RedeemCodeWeightBP+pool.BalanceWeightBP != couponWeightBasisPoints {
		return 0, 0, 0, fmt.Errorf("coupon reward split is invalid for %s", activity)
	}
	return pool.CouponWeightBP, pool.RedeemCodeWeightBP, pool.BalanceWeightBP, nil
}

func (s *PlayService) drawCouponRewardType(ctx context.Context, activity CouponRewardActivity) (PlayRewardType, error) {
	if s == nil || s.rewardDrawSource == nil {
		return PlayRewardTypeNone, fmt.Errorf("coupon reward draw source is not configured")
	}
	couponWeightBP, redeemCodeWeightBP, _, err := s.couponRewardSplit(ctx, activity)
	if err != nil {
		return PlayRewardTypeNone, err
	}
	draw, err := s.rewardDrawSource(couponWeightBasisPoints)
	if err != nil {
		return PlayRewardTypeNone, fmt.Errorf("coupon reward draw source: %w", err)
	}
	return marketingRewardTypeAt(couponWeightBP, redeemCodeWeightBP, draw)
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
	eligibility PlayGrowthEligibility,
) (*CouponRewardIssueResult, error) {
	if eligibility.RewardMode != PlayGrowthRewardRedeemable {
		return nil, newPlayGrowthRewardIneligibleError(eligibility)
	}
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

func (s *PlayService) issueRedeemCodeRewardInTx(
	ctx context.Context,
	userID int64,
	activity CouponRewardActivity,
	idempotencyKey string,
	sourceRef string,
	issuedAt time.Time,
	eligibility PlayGrowthEligibility,
) (*RedeemCode, error) {
	if eligibility.RewardMode != PlayGrowthRewardRedeemable {
		return nil, newPlayGrowthRewardIneligibleError(eligibility)
	}
	if s == nil || s.redeemRewardIssuer == nil {
		return nil, ErrCouponRewardPoolUnavailable
	}
	reader, ok := s.couponRewardIssuer.(CouponRewardPoolReader)
	if !ok || reader == nil {
		return nil, ErrCouponRewardPoolUnavailable
	}
	pool, err := reader.GetPublishedRewardPool(ctx, activity)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, ErrCouponRewardPoolUnavailable
	}
	entry, err := drawRedeemRewardEntry(pool.RewardConfig.RedeemEntries, s.rewardDrawSource)
	if err != nil {
		return nil, err
	}
	return s.redeemRewardIssuer.ClaimRedeemCodeRewardInTx(ctx, RedeemCodeRewardClaimRequest{
		UserID:            userID,
		BatchName:         entry.BatchName,
		CodeType:          entry.CodeType,
		IssueSource:       string(activity),
		IssueRef:          sourceRef,
		RewardPoolVersion: pool.Version,
		IssuedAt:          issuedAt,
	})
}

func drawRedeemRewardEntry(entries []RedeemRewardPoolEntry, drawSource func(max int64) (int64, error)) (*RedeemRewardPoolEntry, error) {
	if drawSource == nil {
		return nil, fmt.Errorf("redeem code reward draw source is not configured")
	}
	enabled := enabledRedeemRewardEntries(entries)
	total := 0
	for _, entry := range enabled {
		total += entry.WeightBP
	}
	if total <= 0 {
		return nil, ErrCouponRewardPoolUnavailable
	}
	draw, err := drawSource(int64(total))
	if err != nil {
		return nil, fmt.Errorf("redeem code reward draw source: %w", err)
	}
	cursor := 0
	for i := range enabled {
		cursor += enabled[i].WeightBP
		if draw < int64(cursor) {
			return &enabled[i], nil
		}
	}
	return nil, ErrCouponRewardPoolUnavailable
}

func (s *PlayService) drawBalanceRewardEntry(ctx context.Context, activity CouponRewardActivity) (*BalanceRewardPoolEntry, error) {
	if s == nil || s.rewardDrawSource == nil {
		return nil, fmt.Errorf("balance reward draw source is not configured")
	}
	reader, ok := s.couponRewardIssuer.(CouponRewardPoolReader)
	if !ok || reader == nil {
		return nil, ErrCouponRewardPoolUnavailable
	}
	pool, err := reader.GetPublishedRewardPool(ctx, activity)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, ErrCouponRewardPoolUnavailable
	}
	enabled := enabledBalanceRewardEntries(pool.RewardConfig.BalanceEntries)
	total := 0
	for _, entry := range enabled {
		total += entry.WeightBP
	}
	if total <= 0 {
		return nil, ErrCouponRewardPoolUnavailable
	}
	draw, err := s.rewardDrawSource(int64(total))
	if err != nil {
		return nil, fmt.Errorf("balance reward draw source: %w", err)
	}
	cursor := 0
	for i := range enabled {
		cursor += enabled[i].WeightBP
		if draw < int64(cursor) {
			return &enabled[i], nil
		}
	}
	return nil, ErrCouponRewardPoolUnavailable
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

func playRedeemCodeRewardSummary(code *RedeemCode) *PlayRedeemCodeRewardSummary {
	if code == nil {
		return nil
	}
	return &PlayRedeemCodeRewardSummary{
		ID:                code.ID,
		Code:              code.Code,
		Type:              code.Type,
		Value:             code.Value,
		Status:            code.Status,
		BatchName:         code.BatchName,
		IssuedAt:          code.IssuedAt,
		ExpiresAt:         code.ExpiresAt,
		RewardPoolVersion: code.RewardPoolVersion,
	}
}

func couponPoolVersion(result *CouponRewardIssueResult) string {
	if result == nil {
		return ""
	}
	return result.PoolVersion
}
