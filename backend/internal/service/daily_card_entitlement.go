package service

import (
	"context"
	"errors"
	"math"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DailyCardQuotaModeOneTime = "one_time"

	DailyCardStatusPending   = "pending"
	DailyCardStatusActive    = "active"
	DailyCardStatusExhausted = "exhausted"
	DailyCardStatusExpired   = "expired"
	DailyCardStatusRevoked   = "revoked"
)

var ErrDailyCardCannotActivate = errors.New("daily card entitlement cannot be activated")
var ErrDailyCardInvalidInput = errors.New("invalid daily card entitlement input")
var ErrDailyCardEntitlementNotFound = errors.New("daily card entitlement not found")
var ErrDailyCardUnavailable = errors.New("daily card quota exhausted or expired")
var ErrDailyCardRequestConflict = errors.New("daily card request reservation conflict")
var ErrDailyCardPaidOrderRequired = errors.New("daily card grants require a tracked payment order")
var ErrDailyCardAdminActionUnavailable = infraerrors.BadRequest("DAILY_CARD_ADMIN_ACTION_UNAVAILABLE", "daily card admin action is unavailable for this entitlement")

type DailyCardEntitlement struct {
	ID               int64      `json:"id"`
	UserID           int64      `json:"user_id"`
	GroupID          int64      `json:"group_id"`
	PlanID           *int64     `json:"plan_id,omitempty"`
	PaymentOrderID   int64      `json:"payment_order_id"`
	QuotaMode        string     `json:"quota_mode"`
	QuotaLimitUSD    float64    `json:"quota_limit_usd"`
	QuotaUsedUSD     float64    `json:"quota_used_usd"`
	QuotaReservedUSD float64    `json:"quota_reserved_usd"`
	DurationHours    int        `json:"duration_hours"`
	Status           string     `json:"status"`
	StartsAt         *time.Time `json:"starts_at,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	ActivatedAt      *time.Time `json:"activated_at,omitempty"`
	ExhaustedAt      *time.Time `json:"exhausted_at,omitempty"`
	EndedAt          *time.Time `json:"ended_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type IssueDailyCardInput struct {
	UserID         int64
	GroupID        int64
	PlanID         int64
	PaymentOrderID int64
	QuotaLimitUSD  float64
	DurationHours  int
	IssuedAt       time.Time
}

type DailyCardRequestHoldInput struct {
	EntitlementID      int64
	UserID             int64
	RequestID          string
	RequestFingerprint string
	ReservedAt         time.Time
}

type DailyCardAdminActionResult struct {
	Card          *DailyCardEntitlement `json:"card"`
	ReleasedHolds int64                 `json:"released_holds"`
}

type DailyCardEntitlementRepository interface {
	IssuePaidCard(ctx context.Context, input IssueDailyCardInput) (*DailyCardEntitlement, bool, error)
	GetActive(ctx context.Context, userID, groupID int64) (*DailyCardEntitlement, error)
	GetByPaymentOrder(ctx context.Context, paymentOrderID int64) (*DailyCardEntitlement, error)
	ReconcileAndGetActive(ctx context.Context, userID, groupID int64, now time.Time) (*DailyCardEntitlement, error)
	ListByUser(ctx context.Context, userID int64) ([]DailyCardEntitlement, error)
	HasRecurringOrderAfter(ctx context.Context, userID, groupID int64, after time.Time) (bool, error)
	IsOneTimeGroup(ctx context.Context, groupID int64) (bool, error)
	ReserveRequest(ctx context.Context, input DailyCardRequestHoldInput) error
	ReleaseRequest(ctx context.Context, entitlementID, userID int64, requestID string, releasedAt time.Time) error
	AdminReleaseReservedHolds(ctx context.Context, entitlementID, userID, groupID int64, releasedAt time.Time) (*DailyCardAdminActionResult, error)
	AdminRestoreQuota(ctx context.Context, entitlementID, userID, groupID int64, restoredAt time.Time) (*DailyCardAdminActionResult, error)
}

func (s *DailyCardService) ReserveRequest(ctx context.Context, input DailyCardRequestHoldInput) error {
	if s == nil || s.repo == nil || input.EntitlementID <= 0 || input.UserID <= 0 || input.RequestID == "" || input.RequestFingerprint == "" {
		return ErrDailyCardInvalidInput
	}
	if input.ReservedAt.IsZero() {
		input.ReservedAt = time.Now()
	}
	return s.repo.ReserveRequest(ctx, input)
}

func (s *DailyCardService) ReleaseRequest(ctx context.Context, entitlementID, userID int64, requestID string, releasedAt time.Time) error {
	if s == nil || s.repo == nil || entitlementID <= 0 || userID <= 0 || requestID == "" {
		return ErrDailyCardInvalidInput
	}
	if releasedAt.IsZero() {
		releasedAt = time.Now()
	}
	return s.repo.ReleaseRequest(ctx, entitlementID, userID, requestID, releasedAt)
}

func (s *DailyCardService) AdminReleaseReservedHolds(ctx context.Context, entitlementID, userID, groupID int64, releasedAt time.Time) (*DailyCardAdminActionResult, error) {
	if s == nil || s.repo == nil || entitlementID <= 0 || userID <= 0 || groupID <= 0 {
		return nil, ErrDailyCardInvalidInput
	}
	if releasedAt.IsZero() {
		releasedAt = time.Now()
	}
	return s.repo.AdminReleaseReservedHolds(ctx, entitlementID, userID, groupID, releasedAt)
}

func (s *DailyCardService) AdminRestoreQuota(ctx context.Context, entitlementID, userID, groupID int64, restoredAt time.Time) (*DailyCardAdminActionResult, error) {
	if s == nil || s.repo == nil || entitlementID <= 0 || userID <= 0 || groupID <= 0 {
		return nil, ErrDailyCardInvalidInput
	}
	if restoredAt.IsZero() {
		restoredAt = time.Now()
	}
	return s.repo.AdminRestoreQuota(ctx, entitlementID, userID, groupID, restoredAt)
}

func (s *DailyCardService) GetByPaymentOrder(ctx context.Context, paymentOrderID int64) (*DailyCardEntitlement, error) {
	if s == nil || s.repo == nil || paymentOrderID <= 0 {
		return nil, ErrDailyCardInvalidInput
	}
	card, err := s.repo.GetByPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		return nil, err
	}
	if card.Status == DailyCardStatusActive {
		if _, reconcileErr := s.repo.ReconcileAndGetActive(ctx, card.UserID, card.GroupID, time.Now()); reconcileErr != nil && !errors.Is(reconcileErr, ErrDailyCardEntitlementNotFound) {
			return nil, reconcileErr
		}
		return s.repo.GetByPaymentOrder(ctx, paymentOrderID)
	}
	return card, nil
}

func (s *DailyCardService) ListForUser(ctx context.Context, userID int64, now time.Time) ([]DailyCardEntitlement, error) {
	if s == nil || s.repo == nil || userID <= 0 {
		return nil, ErrDailyCardInvalidInput
	}
	cards, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	seenGroups := make(map[int64]struct{}, len(cards))
	for i := range cards {
		groupID := cards[i].GroupID
		if _, seen := seenGroups[groupID]; seen {
			continue
		}
		seenGroups[groupID] = struct{}{}
		if _, reconcileErr := s.repo.ReconcileAndGetActive(ctx, userID, groupID, now); reconcileErr != nil && !errors.Is(reconcileErr, ErrDailyCardEntitlementNotFound) {
			return nil, reconcileErr
		}
	}
	return s.repo.ListByUser(ctx, userID)
}

type DailyCardService struct {
	repo DailyCardEntitlementRepository
}

func NewDailyCardService(repo DailyCardEntitlementRepository) *DailyCardService {
	return &DailyCardService{repo: repo}
}

func (s *DailyCardService) IssuePaidCard(ctx context.Context, input IssueDailyCardInput) (*DailyCardEntitlement, bool, error) {
	if s == nil || s.repo == nil || input.UserID <= 0 || input.GroupID <= 0 || input.PlanID <= 0 ||
		input.PaymentOrderID <= 0 || input.QuotaLimitUSD <= 0 || input.DurationHours <= 0 {
		return nil, false, ErrDailyCardInvalidInput
	}
	if input.IssuedAt.IsZero() {
		input.IssuedAt = time.Now()
	}
	return s.repo.IssuePaidCard(ctx, input)
}

func (s *DailyCardService) ResolveAccess(ctx context.Context, userID, groupID int64, now time.Time) (*DailyCardEntitlement, bool, error) {
	if s == nil || s.repo == nil || userID <= 0 || groupID <= 0 {
		return nil, false, ErrDailyCardInvalidInput
	}
	active, err := s.repo.ReconcileAndGetActive(ctx, userID, groupID, now)
	if err == nil {
		return active, true, nil
	}
	if !errors.Is(err, ErrDailyCardEntitlementNotFound) {
		return nil, false, err
	}
	all, listErr := s.repo.ListByUser(ctx, userID)
	if listErr != nil {
		return nil, false, listErr
	}
	var latestPurchase time.Time
	for i := range all {
		if all[i].GroupID != groupID {
			continue
		}
		if all[i].CreatedAt.After(latestPurchase) {
			latestPurchase = all[i].CreatedAt
		}
	}
	if latestPurchase.IsZero() {
		managed, managedErr := s.repo.IsOneTimeGroup(ctx, groupID)
		if managedErr != nil {
			return nil, false, managedErr
		}
		if managed {
			return nil, true, ErrDailyCardUnavailable
		}
		return nil, false, nil
	}
	hasLaterRecurringOrder, recurringErr := s.repo.HasRecurringOrderAfter(ctx, userID, groupID, latestPurchase)
	if recurringErr != nil {
		return nil, true, recurringErr
	}
	if hasLaterRecurringOrder {
		return nil, false, nil
	}
	return nil, true, ErrDailyCardUnavailable
}

func (s *DailyCardService) IsOneTimeGroup(ctx context.Context, groupID int64) (bool, error) {
	if s == nil || s.repo == nil || groupID <= 0 {
		return false, ErrDailyCardInvalidInput
	}
	return s.repo.IsOneTimeGroup(ctx, groupID)
}

func (e *DailyCardEntitlement) RemainingQuotaUSD() float64 {
	if e == nil {
		return 0
	}
	remaining := e.QuotaLimitUSD - e.QuotaUsedUSD - e.QuotaReservedUSD
	if remaining <= 0 {
		return 0
	}
	return remaining
}

func (e *DailyCardEntitlement) IsTerminal() bool {
	if e == nil {
		return true
	}
	switch e.Status {
	case DailyCardStatusExhausted, DailyCardStatusExpired, DailyCardStatusRevoked:
		return true
	default:
		return false
	}
}

func (e *DailyCardEntitlement) Activate(at time.Time, duration time.Duration) error {
	if e == nil || e.Status != DailyCardStatusPending || duration <= 0 {
		return ErrDailyCardCannotActivate
	}
	expiresAt := at.Add(duration)
	e.Status = DailyCardStatusActive
	e.StartsAt = &at
	e.ActivatedAt = &at
	e.ExpiresAt = &expiresAt
	e.ExhaustedAt = nil
	e.EndedAt = nil
	return nil
}

func (e *DailyCardEntitlement) ReconcileAt(now time.Time) {
	if e == nil || e.Status != DailyCardStatusActive || e.ExpiresAt == nil {
		return
	}
	if !now.Before(*e.ExpiresAt) {
		endedAt := *e.ExpiresAt
		e.Status = DailyCardStatusExpired
		e.EndedAt = &endedAt
	}
}

func (e *DailyCardEntitlement) ApplyCapturedUsage(costUSD float64, settledAt time.Time) {
	if e == nil || e.Status != DailyCardStatusActive || costUSD <= 0 {
		return
	}
	e.QuotaUsedUSD = math.Min(e.QuotaLimitUSD, e.QuotaUsedUSD+costUSD)
	if e.QuotaUsedUSD >= e.QuotaLimitUSD {
		e.Status = DailyCardStatusExhausted
		e.ExhaustedAt = &settledAt
		e.EndedAt = &settledAt
	}
}
