package service

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"math"
	"strings"
	"time"
)

type AffiliateSettingsChange struct {
	Code      *string
	Rate      *float64
	ClearRate bool
	Reset     bool
	ActorID   int64
}
type AffiliateAtomicSettingsRepository interface {
	UpdateAffiliateSettings(context.Context, int64, AffiliateSettingsChange) error
}

func (s *AffiliateService) AdminUpdateSettings(ctx context.Context, userID int64, change AffiliateSettingsChange) error {
	if userID <= 0 {
		return ErrUserNotFound
	}
	if change.Code != nil {
		code := strings.ToUpper(strings.TrimSpace(*change.Code))
		if !isValidAffiliateCodeFormat(code) {
			return ErrAffiliateCodeInvalid
		}
		change.Code = &code
	}
	if err := validateExclusiveRate(change.Rate); err != nil {
		return err
	}
	if s == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	repo, ok := s.repo.(AffiliateAtomicSettingsRepository)
	if !ok {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "atomic affiliate settings unavailable")
	}
	return repo.UpdateAffiliateSettings(ctx, userID, change)
}

type AffiliateWithdrawalEvidence struct {
	ExternalReference string `json:"external_reference,omitempty"`
	PaidAt            string `json:"paid_at,omitempty"`
	Note              string `json:"note,omitempty"`
	ActorID           int64  `json:"actor_id,omitempty"`
}

func (e AffiliateWithdrawalEvidence) Validate() error {
	paidAt, err := time.Parse(time.RFC3339, e.PaidAt)
	if err != nil || paidAt.After(time.Now().Add(5*time.Minute)) || len(strings.TrimSpace(e.ExternalReference)) == 0 || len(e.ExternalReference) > 200 || len(strings.TrimSpace(e.Note)) == 0 || len(e.Note) > 1000 {
		return infraerrors.BadRequest("AFFILIATE_WITHDRAW_EVIDENCE_INVALID", "external payment reference, past payment time and note are required")
	}
	return nil
}

type AffiliateEvidenceWithdrawalRepository interface {
	WithdrawQuotaWithEvidence(context.Context, int64, float64, string, AffiliateWithdrawalEvidence) (*AffiliateWithdrawResult, error)
}

func (s *AffiliateService) AdminWithdrawWithEvidence(ctx context.Context, userID int64, amount float64, key string, evidence AffiliateWithdrawalEvidence) (*AffiliateWithdrawResult, error) {
	if err := evidence.Validate(); err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	if s == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	repo, ok := s.repo.(AffiliateEvidenceWithdrawalRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate withdrawal evidence unavailable")
	}
	normalized, err := NormalizeIdempotencyKey(key)
	if err != nil {
		return nil, err
	}
	if normalized == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	amount = roundTo(amount, 8)
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, ErrAffiliateWithdrawAmountInvalid
	}
	paidAt, _ := time.Parse(time.RFC3339, evidence.PaidAt)
	evidence.PaidAt = paidAt.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
	evidence.ExternalReference = strings.TrimSpace(evidence.ExternalReference)
	evidence.Note = strings.TrimSpace(evidence.Note)
	return repo.WithdrawQuotaWithEvidence(ctx, userID, amount, affiliateWithdrawOperationID(normalized), evidence)
}

type AffiliateRecordAmounts struct {
	SourceType      string  `json:"source_type"`
	Currency        string  `json:"currency"`
	TotalAmount     float64 `json:"total_amount"`
	AvailableAmount float64 `json:"available_amount"`
	FrozenAmount    float64 `json:"frozen_amount"`
}
type AffiliateRecordSummary struct {
	RecordCount int64                    `json:"record_count"`
	TotalAmount *float64                 `json:"total_amount"`
	Currency    string                   `json:"currency"`
	Amounts     []AffiliateRecordAmounts `json:"amounts"`
}
type AffiliateRecordReport struct {
	Summary AffiliateRecordSummary `json:"summary"`
	Rows    []map[string]any       `json:"rows"`
}
type AffiliateRecordReportingRepository interface {
	AffiliateRecordReport(context.Context, string, AffiliateRecordFilter, bool, int64) (*AffiliateRecordReport, error)
}

func (s *AffiliateService) AdminRecordReport(ctx context.Context, kind string, filter AffiliateRecordFilter, export bool, actorID int64) (*AffiliateRecordReport, error) {
	if kind != "invites" && kind != "rebates" && kind != "transfers" {
		return nil, infraerrors.BadRequest("AFFILIATE_REPORT_KIND", "invalid record kind")
	}
	if s == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	repo, ok := s.repo.(AffiliateRecordReportingRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate reporting unavailable")
	}
	return repo.AffiliateRecordReport(ctx, kind, filter, export, actorID)
}
