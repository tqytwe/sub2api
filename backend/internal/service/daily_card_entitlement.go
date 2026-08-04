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

	DailyCardSourcePaymentOrder = "payment_order"
	DailyCardSourceRedeemCode   = "redeem_code"
	DailyCardSourceBackfill     = "backfill"

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
var ErrDailyCardDuplicateRequest = infraerrors.Conflict("DAILY_CARD_DUPLICATE_REQUEST", "daily card request was already completed")
var ErrDailyCardRequestPendingConfirmation = infraerrors.Conflict("DAILY_CARD_REQUEST_PENDING_CONFIRMATION", "daily card request outcome is pending confirmation")
var ErrDailyCardPaidOrderRequired = errors.New("daily card grants require a tracked payment order")
var ErrDailyCardAdminActionUnavailable = infraerrors.BadRequest("DAILY_CARD_ADMIN_ACTION_UNAVAILABLE", "daily card admin action is unavailable for this entitlement")

type DailyCardEntitlement struct {
	ID               int64      `json:"id"`
	UserID           int64      `json:"user_id"`
	GroupID          int64      `json:"group_id"`
	PlanID           *int64     `json:"plan_id,omitempty"`
	PaymentOrderID   *int64     `json:"payment_order_id,omitempty"`
	SourceType       string     `json:"source_type"`
	SourceID         string     `json:"source_id"`
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
	SourceType     string
	SourceID       string
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

// DailyCardRequestAdmissionInput deliberately separates the client correlation
// key from the private settlement key used by billing.
type DailyCardRequestAdmissionInput struct {
	EntitlementID       int64
	UserID              int64
	ClientRequestID     string
	SettlementRequestID string
	RequestFingerprint  string
	RequestPath         string
	AdmittedAt          time.Time
}

type DailyCardRequestReplay struct {
	EntitlementID       int64      `json:"entitlement_id"`
	ClientRequestID     string     `json:"client_request_id"`
	SettlementRequestID string     `json:"settlement_request_id"`
	State               string     `json:"state"`
	RequestPath         string     `json:"request_path"`
	DispatchedAt        *time.Time `json:"dispatched_at,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	ReconciledAt        *time.Time `json:"reconciled_at,omitempty"`
	ReconciledBy        *int64     `json:"reconciled_by,omitempty"`
	Evidence            string     `json:"reconciliation_evidence,omitempty"`
}

type DailyCardRequestReconciliationInput struct {
	EntitlementID   int64
	ClientRequestID string
	Action          string
	Evidence        string
	ActorID         int64
	ReconciledAt    time.Time
}

type DailyCardAdminActionResult struct {
	Card          *DailyCardEntitlement `json:"card"`
	ReleasedHolds int64                 `json:"released_holds"`
}

type DailyCardEntitlementRepository interface {
	IssuePaidCard(ctx context.Context, input IssueDailyCardInput) (*DailyCardEntitlement, bool, error)
	IssueSourcedCard(ctx context.Context, input IssueDailyCardInput) (*DailyCardEntitlement, bool, error)
	GetActive(ctx context.Context, userID, groupID int64) (*DailyCardEntitlement, error)
	GetByPaymentOrder(ctx context.Context, paymentOrderID int64) (*DailyCardEntitlement, error)
	ReconcileAndGetActive(ctx context.Context, userID, groupID int64, now time.Time) (*DailyCardEntitlement, error)
	ListByUser(ctx context.Context, userID int64) ([]DailyCardEntitlement, error)
	HasRecurringOrderAfter(ctx context.Context, userID, groupID int64, after time.Time) (bool, error)
	IsOneTimeGroup(ctx context.Context, groupID int64) (bool, error)
	AdmitRequest(ctx context.Context, input DailyCardRequestAdmissionInput) error
	MarkRequestRetryable(ctx context.Context, entitlementID int64, settlementRequestID string, updatedAt time.Time) error
	GetRequestReplay(ctx context.Context, entitlementID int64, clientRequestID string) (*DailyCardRequestReplay, error)
	ReconcileRequest(ctx context.Context, input DailyCardRequestReconciliationInput) (*DailyCardRequestReplay, error)
	ReserveRequest(ctx context.Context, input DailyCardRequestHoldInput) error
	ReleaseRequest(ctx context.Context, entitlementID, userID int64, requestID string, releasedAt time.Time) error
	AdminReleaseReservedHolds(ctx context.Context, entitlementID, userID, groupID int64, releasedAt time.Time) (*DailyCardAdminActionResult, error)
	AdminRestoreQuota(ctx context.Context, entitlementID, userID, groupID int64, restoredAt time.Time) (*DailyCardAdminActionResult, error)
	AdminAdjustExpiry(ctx context.Context, entitlementID, userID, groupID int64, newExpiresAt, adjustedAt time.Time) (*DailyCardEntitlement, error)
}

func (s *DailyCardService) GetRequestReplay(ctx context.Context, entitlementID int64, clientRequestID string) (*DailyCardRequestReplay, error) {
	if s == nil || s.repo == nil || entitlementID <= 0 || clientRequestID == "" {
		return nil, ErrDailyCardInvalidInput
	}
	return s.repo.GetRequestReplay(ctx, entitlementID, clientRequestID)
}

func (s *DailyCardService) ReconcileRequest(ctx context.Context, input DailyCardRequestReconciliationInput) (*DailyCardRequestReplay, error) {
	if s == nil || s.repo == nil || input.EntitlementID <= 0 || input.ClientRequestID == "" || input.ActorID <= 0 || input.Evidence == "" {
		return nil, ErrDailyCardInvalidInput
	}
	if input.ReconciledAt.IsZero() {
		input.ReconciledAt = time.Now()
	}
	return s.repo.ReconcileRequest(ctx, input)
}

func (s *DailyCardService) AdmitRequest(ctx context.Context, input DailyCardRequestAdmissionInput) error {
	if s == nil || s.repo == nil || input.EntitlementID <= 0 || input.UserID <= 0 || input.ClientRequestID == "" || input.SettlementRequestID == "" || input.RequestFingerprint == "" {
		return ErrDailyCardInvalidInput
	}
	if input.AdmittedAt.IsZero() {
		input.AdmittedAt = time.Now()
	}
	return s.repo.AdmitRequest(ctx, input)
}

func (s *DailyCardService) MarkRequestRetryable(ctx context.Context, entitlementID int64, settlementRequestID string, updatedAt time.Time) error {
	if s == nil || s.repo == nil || entitlementID <= 0 || settlementRequestID == "" {
		return ErrDailyCardInvalidInput
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	return s.repo.MarkRequestRetryable(ctx, entitlementID, settlementRequestID, updatedAt)
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

func (s *DailyCardService) AdminAdjustExpiry(ctx context.Context, entitlementID, userID, groupID int64, newExpiresAt, adjustedAt time.Time) (*DailyCardEntitlement, error) {
	if s == nil || s.repo == nil || entitlementID <= 0 || userID <= 0 || groupID <= 0 || newExpiresAt.IsZero() {
		return nil, ErrDailyCardInvalidInput
	}
	if adjustedAt.IsZero() {
		adjustedAt = time.Now()
	}
	return s.repo.AdminAdjustExpiry(ctx, entitlementID, userID, groupID, newExpiresAt, adjustedAt)
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
	input.SourceType = DailyCardSourcePaymentOrder
	input.SourceID = ""
	return s.repo.IssuePaidCard(ctx, input)
}

func (s *DailyCardService) IssueRedeemCard(ctx context.Context, input IssueDailyCardInput) (*DailyCardEntitlement, bool, error) {
	if s == nil || s.repo == nil || input.UserID <= 0 || input.GroupID <= 0 || input.PlanID <= 0 ||
		input.SourceID == "" || input.QuotaLimitUSD <= 0 || input.DurationHours <= 0 {
		return nil, false, ErrDailyCardInvalidInput
	}
	if input.IssuedAt.IsZero() {
		input.IssuedAt = time.Now()
	}
	input.PaymentOrderID = 0
	input.SourceType = DailyCardSourceRedeemCode
	return s.repo.IssueSourcedCard(ctx, input)
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
	remaining := e.QuotaLimitUSD - e.QuotaUsedUSD
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
