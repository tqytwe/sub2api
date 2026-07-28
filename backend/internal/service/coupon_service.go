package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrCouponRewardPoolUnavailable = errors.New("coupon reward pool is unavailable")
	ErrCouponRepositoryUnavailable = errors.New("coupon repository is unavailable")
)

type CouponService struct {
	repo CouponRepository
	now  func() time.Time
}

func NewCouponService(repo CouponRepository) *CouponService {
	return &CouponService{repo: repo, now: time.Now}
}

func (s *CouponService) CreateTemplate(ctx context.Context, input CouponTemplateInput, actorID int64) (*CouponTemplate, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	template := normalizeCouponTemplate(input.Template())
	if template.Status == "" {
		template.Status = CouponTemplateStatusDraft
	}
	template.Version = 1
	template.CreatedBy = couponActorID(actorID)
	template.UpdatedBy = couponActorID(actorID)
	if err := ValidateCouponTemplate(template); err != nil {
		return nil, couponInvalidInput(err)
	}
	if template.ValidityMode == CouponValidityModeFixed && !template.FixedExpiresAt.After(s.now()) {
		return nil, infraerrors.BadRequest("COUPON_FIXED_EXPIRY_PAST", "coupon fixed expiry must be in the future")
	}
	return s.repo.CreateCouponTemplate(ctx, template)
}

func (s *CouponService) GetTemplate(ctx context.Context, id int64) (*CouponTemplate, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_TEMPLATE_ID", "coupon template id must be positive")
	}
	template, err := s.repo.GetCouponTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, infraerrors.NotFound("COUPON_TEMPLATE_NOT_FOUND", "coupon template not found")
	}
	return template, nil
}

// GetUserCouponForUser is the ownership boundary for user-facing coupon
// reads. A coupon belonging to another account is intentionally
// indistinguishable from a missing coupon.
func (s *CouponService) GetUserCouponForUser(ctx context.Context, couponID, userID int64) (*UserCoupon, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if couponID <= 0 || userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER_COUPON_ID", "coupon and user ids must be positive")
	}
	coupon, err := s.repo.GetUserCoupon(ctx, couponID)
	if err != nil {
		return nil, err
	}
	if coupon == nil || coupon.UserID != userID {
		return nil, infraerrors.NotFound("COUPON_NOT_FOUND", "coupon not found")
	}
	return coupon, nil
}

func (s *CouponService) ListTemplates(ctx context.Context, filter CouponTemplateListFilter) ([]CouponTemplate, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, ErrCouponRepositoryUnavailable
	}
	filter = normalizeCouponTemplateListFilter(filter)
	if filter.Status != "" && !isCouponTemplateStatus(filter.Status) {
		return nil, nil, infraerrors.BadRequest("INVALID_COUPON_TEMPLATE_STATUS", "coupon template status is invalid")
	}
	rows, total, err := s.repo.ListCouponTemplates(ctx, filter)
	if err != nil {
		return nil, nil, err
	}
	return rows, couponPagination(total, filter.Page, filter.PageSize), nil
}

func (s *CouponService) UpdateTemplate(ctx context.Context, id int64, input CouponTemplateInput, actorID int64) (*CouponTemplate, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_TEMPLATE_ID", "coupon template id must be positive")
	}
	existing, err := s.GetTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	candidate := normalizeCouponTemplate(input.Template())
	candidate.ID = existing.ID
	candidate.Version = existing.Version
	candidate.IssuedCount = existing.IssuedCount
	candidate.CreatedBy = existing.CreatedBy
	candidate.CreatedAt = existing.CreatedAt
	candidate.UpdatedBy = couponActorID(actorID)
	if candidate.TotalIssueLimit != nil && *candidate.TotalIssueLimit < existing.IssuedCount {
		return nil, infraerrors.BadRequest("COUPON_ISSUE_LIMIT_BELOW_ISSUED", "coupon issue limit cannot be lower than already issued coupons")
	}
	if err := ValidateCouponTemplate(candidate); err != nil {
		return nil, couponInvalidInput(err)
	}
	// A published pool's fallback keeps the outer 60/40 or 80/20 split
	// settleable after every ordinary entry has reached its user-level cap.
	// Do not allow an administrator to make that template bounded after the
	// pool is live; replacing it requires a new pool version instead.
	pools, err := s.repo.ListCouponRewardPools(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if pool.Status != CouponRewardPoolStatusPublished || pool.FallbackTemplateID != id {
			continue
		}
		if err := ValidateCouponRewardFallbackTemplate(&candidate, s.now()); err != nil {
			return nil, infraerrors.Conflict(
				"COUPON_POOL_FALLBACK_TEMPLATE_LOCKED",
				fmt.Sprintf("published coupon pool fallback templates must remain active and unbounded (%v); create a new template and pool version", err),
			)
		}
		break
	}
	if existing.IssuedCount > 0 && !CouponTemplateFinancialTermsEqual(*existing, candidate) {
		return nil, infraerrors.Conflict("COUPON_TEMPLATE_TERMS_LOCKED", "issued coupon terms cannot be changed; create a new template version")
	}
	if existing.IssuedCount == 0 && !reflect.DeepEqual(couponTemplateTermsForComparison(*existing), couponTemplateTermsForComparison(candidate)) {
		candidate.Version = existing.Version + 1
	}
	if candidate.ValidityMode == CouponValidityModeFixed && !candidate.FixedExpiresAt.After(s.now()) {
		return nil, infraerrors.BadRequest("COUPON_FIXED_EXPIRY_PAST", "coupon fixed expiry must be in the future")
	}
	return s.repo.UpdateCouponTemplate(ctx, candidate)
}

func (s *CouponService) DeleteTemplate(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return ErrCouponRepositoryUnavailable
	}
	template, err := s.GetTemplate(ctx, id)
	if err != nil {
		return err
	}
	if template.IssuedCount > 0 {
		return infraerrors.Conflict("COUPON_TEMPLATE_ISSUED", "issued coupon templates cannot be deleted; archive them instead")
	}
	return s.repo.DeleteCouponTemplate(ctx, id)
}

func (s *CouponService) IssueCoupon(ctx context.Context, request CouponIssueInput) (*UserCoupon, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if err := validateCouponIssueInput(request); err != nil {
		return nil, err
	}
	return s.repo.IssueCoupon(ctx, request, s.now())
}

func (s *CouponService) IssueCouponBatch(ctx context.Context, input CouponBatchIssueInput) (*CouponIssueBatch, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	input.Source = normalizedCouponIssueSource(input.Source, CouponIssueSourceAdminBatch)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.TemplateID <= 0 || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 200 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_BATCH", "coupon batch template and idempotency key are required")
	}
	if !isCouponIssueSource(input.Source) {
		return nil, infraerrors.BadRequest("INVALID_COUPON_SOURCE", "coupon issue source is invalid")
	}
	input.UserIDs = normalizeCouponUserIDs(input.UserIDs)
	if len(input.UserIDs) == 0 || len(input.UserIDs) > 10_000 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_BATCH_USERS", "coupon batch must include between 1 and 10000 users")
	}
	return s.repo.IssueCouponBatch(ctx, input, s.now())
}

func (s *CouponService) ListUserCoupons(ctx context.Context, filter UserCouponListFilter) ([]UserCoupon, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, ErrCouponRepositoryUnavailable
	}
	filter = normalizeUserCouponListFilter(filter)
	if filter.Status != "" && !isUserCouponStatus(filter.Status) {
		return nil, nil, infraerrors.BadRequest("INVALID_USER_COUPON_STATUS", "user coupon status is invalid")
	}
	if filter.Source != "" && !isCouponIssueSource(filter.Source) {
		return nil, nil, infraerrors.BadRequest("INVALID_COUPON_SOURCE", "coupon issue source is invalid")
	}
	if shouldPersistCouponExpiry(filter.Status) {
		if err := s.repo.ExpireAvailableUserCoupons(ctx, filter.UserID, s.now()); err != nil {
			return nil, nil, err
		}
	}
	rows, total, err := s.repo.ListUserCoupons(ctx, filter)
	if err != nil {
		return nil, nil, err
	}
	return rows, couponPagination(total, filter.Page, filter.PageSize), nil
}

func (s *CouponService) VoidUserCoupon(ctx context.Context, couponID int64, reason string, actorID int64) (*UserCoupon, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if couponID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER_COUPON_ID", "user coupon id must be positive")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_VOID_REASON", "coupon void reason is required and must be at most 500 characters")
	}
	return s.repo.VoidUserCoupon(ctx, couponID, reason, actorID, s.now())
}

func (s *CouponService) ListCouponIssueBatches(ctx context.Context, filter CouponIssueBatchListFilter) ([]CouponIssueBatch, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, ErrCouponRepositoryUnavailable
	}
	filter.Page, filter.PageSize = normalizeCouponPage(filter.Page, filter.PageSize)
	rows, total, err := s.repo.ListCouponIssueBatches(ctx, filter)
	if err != nil {
		return nil, nil, err
	}
	return rows, couponPagination(total, filter.Page, filter.PageSize), nil
}

func (s *CouponService) SaveRewardPool(ctx context.Context, pool CouponRewardPoolVersion, actorID int64) (*CouponRewardPoolVersion, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	pool = normalizeCouponRewardPool(pool)
	if pool.Status == "" {
		pool.Status = CouponRewardPoolStatusDraft
	}
	if pool.Status != CouponRewardPoolStatusDraft {
		return nil, infraerrors.BadRequest("COUPON_POOL_MUTATION_REQUIRES_DRAFT", "coupon pools must be saved as drafts before publishing")
	}
	if err := ValidateCouponRewardPool(pool); err != nil {
		return nil, couponInvalidInput(err)
	}
	pool.CreatedBy = couponActorID(actorID)
	pool.UpdatedBy = couponActorID(actorID)
	return s.repo.SaveCouponRewardPool(ctx, pool)
}

func (s *CouponService) GetRewardPool(ctx context.Context, id int64) (*CouponRewardPoolVersion, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_POOL_ID", "coupon pool id must be positive")
	}
	pool, err := s.repo.GetCouponRewardPool(ctx, id)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, infraerrors.NotFound("COUPON_POOL_NOT_FOUND", "coupon pool not found")
	}
	return pool, nil
}

func (s *CouponService) ListRewardPools(ctx context.Context, activity CouponRewardActivity) ([]CouponRewardPoolVersion, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if activity != "" && !isCouponRewardActivity(activity) {
		return nil, infraerrors.BadRequest("INVALID_COUPON_POOL_ACTIVITY", "coupon pool activity is invalid")
	}
	return s.repo.ListCouponRewardPools(ctx, activity)
}

// DeleteRewardPool removes an unpublished configuration mistake. Published and
// retired versions remain immutable because coupon draws retain their pool
// version as an audit record.
func (s *CouponService) DeleteRewardPool(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return ErrCouponRepositoryUnavailable
	}
	pool, err := s.GetRewardPool(ctx, id)
	if err != nil {
		return err
	}
	if pool.Status != CouponRewardPoolStatusDraft {
		return infraerrors.Conflict("COUPON_POOL_DELETE_REJECTED", "only draft coupon pools can be deleted")
	}
	return s.repo.DeleteCouponRewardPool(ctx, id)
}

func (s *CouponService) PublishRewardPool(ctx context.Context, id int64, actorID int64) (*CouponRewardPoolVersion, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	pool, err := s.GetRewardPool(ctx, id)
	if err != nil {
		return nil, err
	}
	if pool.Status != CouponRewardPoolStatusDraft {
		return nil, infraerrors.Conflict("COUPON_POOL_NOT_DRAFT", "only draft coupon pools can be published")
	}
	if err := ValidateCouponRewardPool(*pool); err != nil {
		return nil, couponInvalidInput(err)
	}
	var fallbackTemplate *CouponTemplate
	for _, templateID := range couponPoolTemplateIDs(*pool) {
		template, getErr := s.GetTemplate(ctx, templateID)
		if getErr != nil {
			return nil, getErr
		}
		if template == nil || template.Status != CouponTemplateStatusActive {
			return nil, infraerrors.Conflict("COUPON_POOL_TEMPLATE_INACTIVE", "published coupon pool templates must be active")
		}
		if template.ID == pool.FallbackTemplateID {
			fallbackTemplate = template
		}
	}
	if err := ValidateCouponRewardFallbackTemplate(fallbackTemplate, s.now()); err != nil {
		return nil, couponInvalidInput(err)
	}
	return s.repo.PublishCouponRewardPool(ctx, id, actorID, s.now())
}

// ValidateCouponRewardFallbackTemplate protects the fixed outer game split.
// Its matching entry is intentionally unbounded, so the template must be
// unbounded as well; otherwise a user who exhausted every normal entry could
// hit a coupon branch that cannot settle.
// Repository transactions call this again after locking the fallback template
// because a service-layer preflight alone cannot cover concurrent publishing.
func ValidateCouponRewardFallbackTemplate(template *CouponTemplate, at time.Time) error {
	if template == nil {
		return fmt.Errorf("coupon reward pool fallback template is missing")
	}
	if template.TotalIssueLimit != nil {
		return fmt.Errorf("coupon reward pool fallback template cannot set a total issue limit")
	}
	if template.ValidityMode == CouponValidityModeFixed {
		return fmt.Errorf("coupon reward pool fallback template cannot use fixed expiry")
	}
	if !couponRewardTemplateIssuable(template, at) {
		return fmt.Errorf("coupon reward pool fallback template must be active and issuable")
	}
	return nil
}

func (s *CouponService) GetPublishedRewardPool(ctx context.Context, activity CouponRewardActivity) (*CouponRewardPoolVersion, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if !isCouponRewardActivity(activity) {
		return nil, infraerrors.BadRequest("INVALID_COUPON_POOL_ACTIVITY", "coupon pool activity is invalid")
	}
	pool, err := s.repo.GetPublishedCouponRewardPool(ctx, activity)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, ErrCouponRewardPoolUnavailable
	}
	ready, err := s.publishedCouponRewardPoolReady(ctx, *pool, s.now())
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, ErrCouponRewardPoolUnavailable
	}
	return pool, nil
}

// CouponRewardPoolReady is the runtime gate used before blind-box and quiz
// flows consume any state. The repository check is repeated by the draw
// transaction under locks, so this preflight never substitutes for settlement.
func (s *CouponService) CouponRewardPoolReady(ctx context.Context, activity CouponRewardActivity) (bool, error) {
	if s == nil || s.repo == nil {
		return false, ErrCouponRepositoryUnavailable
	}
	if !isCouponRewardActivity(activity) {
		return false, infraerrors.BadRequest("INVALID_COUPON_POOL_ACTIVITY", "coupon pool activity is invalid")
	}
	ready, err := s.repo.HasIssuableCouponRewardEntry(ctx, activity, s.now())
	if err != nil {
		return false, err
	}
	return ready, nil
}

func shouldPersistCouponExpiry(status UserCouponStatus) bool {
	return status == "" || status == UserCouponStatusAvailable || status == UserCouponStatusExpired
}

func (s *CouponService) publishedCouponRewardPoolReady(ctx context.Context, pool CouponRewardPoolVersion, at time.Time) (bool, error) {
	for _, entry := range pool.Entries {
		if !couponRewardEntryWindowHasCapacity(entry, at) {
			continue
		}
		template, err := s.repo.GetCouponTemplate(ctx, entry.TemplateID)
		if err != nil {
			return false, err
		}
		if couponRewardTemplateIssuable(template, at) {
			return true, nil
		}
	}
	return false, nil
}

func couponRewardEntryWindowHasCapacity(entry CouponRewardPoolEntry, at time.Time) bool {
	if !entry.Enabled || entry.TemplateID <= 0 {
		return false
	}
	if entry.StartsAt != nil && entry.StartsAt.After(at) {
		return false
	}
	if entry.EndsAt != nil && !entry.EndsAt.After(at) {
		return false
	}
	return entry.StockCap == nil || entry.IssuedCount < *entry.StockCap
}

func couponRewardTemplateIssuable(template *CouponTemplate, at time.Time) bool {
	if template == nil || template.Status != CouponTemplateStatusActive {
		return false
	}
	if template.TotalIssueLimit != nil && template.IssuedCount >= *template.TotalIssueLimit {
		return false
	}
	_, expiresAt, err := CouponExpiryForIssue(*template, at)
	return err == nil && expiresAt.After(at)
}

func (s *CouponService) QuoteUserCoupon(ctx context.Context, couponID int64, order CouponOrderContext) (*CouponQuote, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if couponID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER_COUPON_ID", "user coupon id must be positive")
	}
	coupon, err := s.GetUserCouponForUser(ctx, couponID, order.UserID)
	if err != nil {
		return nil, err
	}
	if order.At.IsZero() {
		order.At = s.now()
	}
	return QuoteUserCoupon(*coupon, order)
}

func (s *CouponService) LockUserCouponForOrder(ctx context.Context, request CouponLockRequest) (*CouponLockResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if request.UserCouponID <= 0 || request.UserID <= 0 || request.OrderID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_LOCK", "coupon, user, and order ids are required")
	}
	request.OrderContext.UserID = request.UserID
	if request.LockedAt.IsZero() {
		request.LockedAt = s.now()
	}
	if request.OrderContext.At.IsZero() {
		request.OrderContext.At = request.LockedAt
	}
	return s.repo.LockUserCouponForOrder(ctx, request)
}

func (s *CouponService) ReleaseUserCouponOrderLock(ctx context.Context, couponID, orderID int64) (*UserCoupon, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if couponID <= 0 || orderID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_LOCK", "coupon and order ids are required")
	}
	return s.repo.ReleaseUserCouponOrderLock(ctx, couponID, orderID, s.now())
}

func (s *CouponService) ConsumeUserCouponOrderLock(ctx context.Context, couponID, orderID int64) (*UserCoupon, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if couponID <= 0 || orderID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_LOCK", "coupon and order ids are required")
	}
	return s.repo.ConsumeUserCouponOrderLock(ctx, couponID, orderID, s.now())
}

func (s *CouponService) DrawAndIssueInTx(ctx context.Context, request CouponRewardDrawRequest) (*CouponRewardIssueResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRewardPoolUnavailable
	}
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.SourceRef = strings.TrimSpace(request.SourceRef)
	if request.UserID <= 0 || !isCouponRewardActivity(request.Activity) || request.IdempotencyKey == "" || len(request.IdempotencyKey) > 200 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_REWARD_DRAW", "coupon reward draw user, activity, and idempotency key are required")
	}
	if request.IssuedAt.IsZero() {
		request.IssuedAt = s.now()
	}
	result, err := s.repo.DrawAndIssueCouponRewardInTx(ctx, request)
	if err != nil {
		if errors.Is(err, ErrCouponRewardPoolUnavailable) {
			return nil, ErrCouponRewardPoolUnavailable
		}
		return nil, err
	}
	return result, nil
}

// GetCouponRewardIssueByIdempotency returns the exact issued-coupon snapshot
// for a completed play request. Ownership is checked here so callers cannot
// use an idempotency hash to inspect another user's coupon.
func (s *CouponService) GetCouponRewardIssueByIdempotency(ctx context.Context, userID int64, idempotencyKey string) (*CouponRewardIssueResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCouponRepositoryUnavailable
	}
	if userID <= 0 || strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 200 {
		return nil, infraerrors.BadRequest("INVALID_COUPON_REWARD_DRAW", "coupon reward user and idempotency key are required")
	}
	result, err := s.repo.FindCouponRewardIssueByIdempotency(ctx, strings.TrimSpace(idempotencyKey))
	if err != nil {
		return nil, err
	}
	if result == nil || result.Coupon.UserID != userID {
		return nil, nil
	}
	return result, nil
}

func couponInvalidInput(err error) error {
	return infraerrors.BadRequest("INVALID_COUPON_CONFIGURATION", err.Error())
}

func couponActorID(actorID int64) *int64 {
	if actorID <= 0 {
		return nil
	}
	copy := actorID
	return &copy
}

func validateCouponIssueInput(request CouponIssueInput) error {
	if request.TemplateID <= 0 || request.UserID <= 0 {
		return infraerrors.BadRequest("INVALID_COUPON_ISSUE", "coupon template and user ids are required")
	}
	if !isCouponIssueSource(request.Source) {
		return infraerrors.BadRequest("INVALID_COUPON_SOURCE", "coupon issue source is invalid")
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" || len(request.IdempotencyKey) > 200 {
		return infraerrors.BadRequest("INVALID_COUPON_IDEMPOTENCY", "coupon issue idempotency key is required and must be at most 200 characters")
	}
	return nil
}

func normalizedCouponIssueSource(source, fallback CouponIssueSource) CouponIssueSource {
	source = CouponIssueSource(strings.ToLower(strings.TrimSpace(string(source))))
	if source == "" {
		return fallback
	}
	return source
}

func normalizeCouponUserIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizeCouponTemplateListFilter(filter CouponTemplateListFilter) CouponTemplateListFilter {
	filter.Status = CouponTemplateStatus(strings.ToLower(strings.TrimSpace(string(filter.Status))))
	filter.Search = strings.TrimSpace(filter.Search)
	filter.Page, filter.PageSize = normalizeCouponPage(filter.Page, filter.PageSize)
	return filter
}

func normalizeUserCouponListFilter(filter UserCouponListFilter) UserCouponListFilter {
	filter.Status = UserCouponStatus(strings.ToLower(strings.TrimSpace(string(filter.Status))))
	filter.Source = CouponIssueSource(strings.ToLower(strings.TrimSpace(string(filter.Source))))
	filter.Page, filter.PageSize = normalizeCouponPage(filter.Page, filter.PageSize)
	return filter
}

func normalizeCouponPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func couponPagination(total int64, page, pageSize int) *pagination.PaginationResult {
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return &pagination.PaginationResult{Total: total, Page: page, PageSize: pageSize, Pages: pages}
}

func isUserCouponStatus(status UserCouponStatus) bool {
	switch status {
	case UserCouponStatusAvailable, UserCouponStatusLocked, UserCouponStatusUsed, UserCouponStatusExpired, UserCouponStatusVoided:
		return true
	default:
		return false
	}
}

// CouponTemplateFinancialTermsEqual reports whether two template revisions
// preserve every term that is copied into an issued coupon or determines its
// absolute availability window. Repository writes call this again after their
// row lock, because the service-level preflight may become stale while a
// coupon is being issued.
func CouponTemplateFinancialTermsEqual(a, b CouponTemplate) bool {
	return reflect.DeepEqual(couponTemplateTermsForComparison(a), couponTemplateTermsForComparison(b))
}

type couponTemplateTermsComparison struct {
	Terms          CouponTermsSnapshot
	FixedExpiresAt *time.Time
	ValidFrom      *time.Time
}

// couponTemplateTermsForComparison deliberately includes the two absolute
// validity controls which are not needed in a user-coupon terms snapshot: a
// user coupon persists its resolved valid_from and expires_at separately.
// They are nevertheless financial terms of a template and must be frozen
// once that template has issued coupons.
func couponTemplateTermsForComparison(template CouponTemplate) couponTemplateTermsComparison {
	template = normalizeCouponTemplate(template)
	return couponTemplateTermsComparison{
		Terms:          CouponTermsFromTemplate(template),
		FixedExpiresAt: couponComparableTime(template.FixedExpiresAt),
		ValidFrom:      couponComparableTime(template.ValidFrom),
	}
}

func couponComparableTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func couponPoolTemplateIDs(pool CouponRewardPoolVersion) []int64 {
	seen := make(map[int64]struct{}, len(pool.Entries)+1)
	ids := make([]int64, 0, len(pool.Entries)+1)
	for _, entry := range pool.Entries {
		if _, exists := seen[entry.TemplateID]; !exists {
			seen[entry.TemplateID] = struct{}{}
			ids = append(ids, entry.TemplateID)
		}
	}
	if _, exists := seen[pool.FallbackTemplateID]; !exists {
		ids = append(ids, pool.FallbackTemplateID)
	}
	return ids
}
