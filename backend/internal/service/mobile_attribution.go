package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	MobileAttributionEventDownload = "download"
	MobileAttributionEventClick    = "click"
	MobileAttributionEventOpen     = "open"
	MobileAttributionEventRegister = "register"
	MobileAttributionEventLogin    = "login"
	MobileAttributionEventActive   = "active"
	MobileAttributionEventShare    = "share"
)

var (
	ErrMobileAttributionInvalid                = infraerrors.BadRequest("MOBILE_ATTRIBUTION_INVALID", "mobile attribution event is invalid")
	ErrMobileAttributionInvalidToken           = infraerrors.BadRequest("MOBILE_ATTRIBUTION_TOKEN_INVALID", "mobile attribution token is invalid or expired")
	ErrMobileAttributionAuthenticationRequired = infraerrors.Unauthorized("MOBILE_ATTRIBUTION_AUTH_REQUIRED", "this attribution event requires authentication")
	ErrMobileAttributionInstallationConflict   = infraerrors.Conflict("MOBILE_ATTRIBUTION_INSTALLATION_CONFLICT", "installation is already bound to another account")
	ErrMobileAttributionInstallationUnbound    = infraerrors.Conflict("MOBILE_ATTRIBUTION_INSTALLATION_UNBOUND", "installation must be bound to the authenticated account")
)

type MobileAttributionEventInput struct {
	InstallationID   string
	EventType        string
	IdempotencyKey   string
	Platform         string
	AppVersion       string
	Locale           string
	AttributionToken string
	OccurredAt       *time.Time
	Metadata         map[string]string
}

type MobileAttributionSignedPayload struct {
	CampaignID   *int64 `json:"campaign_id,omitempty"`
	ReferralCode string `json:"referral_code,omitempty"`
	ExpiresAt    int64  `json:"expires_at"`
}

type MobileAttributionReference struct {
	CampaignID         *int64
	ReferralCampaignID *int64
	ReferrerUserID     *int64
}

type MobileAttributionEventWrite struct {
	InstallationID       string
	UserID               int64
	EventType            string
	IdempotencyKeyDigest []byte
	AttributionDigest    []byte
	CampaignID           *int64
	ReferralCampaignID   *int64
	ReferrerUserID       *int64
	Platform             string
	AppVersion           string
	Locale               string
	OccurredAt           time.Time
	Verified             bool
	Metadata             map[string]string
}

type MobileAttributionBinding struct {
	InstallationID     string
	UserID             int64
	Platform           string
	AppVersion         string
	Locale             string
	CampaignID         *int64
	ReferralCampaignID *int64
	ReferrerUserID     *int64
	AttributionDigest  []byte
	BoundAt            time.Time
	RequireExisting    bool
}

type MobileAttributionRecordResult struct {
	Created bool `json:"created"`
}

type MobileAttributionAdminFilter struct {
	Page       int
	PageSize   int
	EventType  string
	Platform   string
	CampaignID *int64
	UserID     *int64
	From       *time.Time
	To         *time.Time
}

type MobileAttributionInstallationRow struct {
	InstallationID     string    `json:"installation_id"`
	UserID             *int64    `json:"user_id,omitempty"`
	Platform           string    `json:"platform"`
	AppVersion         string    `json:"app_version"`
	CampaignID         *int64    `json:"campaign_id,omitempty"`
	ReferralCampaignID *int64    `json:"referral_campaign_id,omitempty"`
	ReferrerUserID     *int64    `json:"referrer_user_id,omitempty"`
	FirstSeenAt        time.Time `json:"first_seen_at"`
	LastSeenAt         time.Time `json:"last_seen_at"`
	EventCount         int64     `json:"event_count"`
}

type MobileAttributionFunnelStep struct {
	EventType string `json:"event_type"`
	Installs  int64  `json:"installations"`
}

type MobileAttributionRepository interface {
	RecordEvent(context.Context, MobileAttributionEventWrite) (bool, error)
	BindInstallation(context.Context, MobileAttributionBinding) error
	ResolveReference(context.Context, *int64, string) (MobileAttributionReference, error)
	ResolveReferralToken(context.Context, string, time.Time) (MobileAttributionReference, error)
	ListInstallations(context.Context, MobileAttributionAdminFilter) ([]MobileAttributionInstallationRow, int64, error)
	Funnel(context.Context, MobileAttributionAdminFilter) ([]MobileAttributionFunnelStep, error)
}

type MobileAttributionService struct {
	repo MobileAttributionRepository
	key  []byte
}

func NewMobileAttributionService(repo MobileAttributionRepository, signingKey []byte) *MobileAttributionService {
	key := append([]byte(nil), signingKey...)
	return &MobileAttributionService{repo: repo, key: key}
}

func (s *MobileAttributionService) RecordEvent(ctx context.Context, userID int64, input MobileAttributionEventInput) (MobileAttributionRecordResult, error) {
	if s == nil || s.repo == nil || len(s.key) == 0 {
		return MobileAttributionRecordResult{}, infraerrors.InternalServer("MOBILE_ATTRIBUTION_UNAVAILABLE", "mobile attribution is unavailable")
	}
	installationID := strings.TrimSpace(input.InstallationID)
	if _, err := uuid.Parse(installationID); err != nil {
		return MobileAttributionRecordResult{}, ErrMobileAttributionInvalid
	}
	eventType := strings.ToLower(strings.TrimSpace(input.EventType))
	if !validMobileAttributionEventType(eventType) || len(strings.TrimSpace(input.IdempotencyKey)) < 8 || len(input.IdempotencyKey) > 200 {
		return MobileAttributionRecordResult{}, ErrMobileAttributionInvalid
	}
	platform := strings.ToLower(strings.TrimSpace(input.Platform))
	if platform != "android" && platform != "ios" && platform != "web" {
		return MobileAttributionRecordResult{}, ErrMobileAttributionInvalid
	}
	if len(input.AppVersion) > 64 || len(input.Locale) > 32 {
		return MobileAttributionRecordResult{}, ErrMobileAttributionInvalid
	}
	if (eventType == MobileAttributionEventRegister || eventType == MobileAttributionEventLogin || eventType == MobileAttributionEventActive) && userID <= 0 {
		return MobileAttributionRecordResult{}, ErrMobileAttributionAuthenticationRequired
	}
	now := time.Now().UTC()
	// Authenticated activity is authoritative server state. Client timestamps are
	// accepted only for anonymous acquisition traffic and are never used for
	// registration, login, or active-user metrics.
	if userID <= 0 && input.OccurredAt != nil {
		if input.OccurredAt.Before(now.Add(-30*24*time.Hour)) || input.OccurredAt.After(now.Add(10*time.Minute)) {
			return MobileAttributionRecordResult{}, ErrMobileAttributionInvalid
		}
		now = input.OccurredAt.UTC()
	}
	ref, attributionDigest, err := s.verifyAttribution(ctx, input.AttributionToken, now)
	if err != nil {
		return MobileAttributionRecordResult{}, err
	}
	keyDigest := s.digest(installationID + "\x00" + strings.TrimSpace(input.IdempotencyKey))
	write := MobileAttributionEventWrite{
		InstallationID: installationID, UserID: userID, EventType: eventType,
		IdempotencyKeyDigest: keyDigest, AttributionDigest: attributionDigest,
		CampaignID: ref.CampaignID, ReferrerUserID: ref.ReferrerUserID,
		ReferralCampaignID: ref.ReferralCampaignID,
		Platform:           platform, AppVersion: strings.TrimSpace(input.AppVersion), Locale: strings.TrimSpace(input.Locale),
		OccurredAt: now,
		Verified:   userID > 0 && eventType != MobileAttributionEventClick && eventType != MobileAttributionEventDownload && eventType != MobileAttributionEventOpen,
		Metadata:   sanitizeMobileAttributionMetadata(input.Metadata),
	}
	if userID > 0 {
		err = s.repo.BindInstallation(ctx, MobileAttributionBinding{
			InstallationID: installationID, UserID: userID, Platform: platform, AppVersion: write.AppVersion, Locale: write.Locale,
			CampaignID: ref.CampaignID, ReferrerUserID: ref.ReferrerUserID, AttributionDigest: attributionDigest, BoundAt: now,
			ReferralCampaignID: ref.ReferralCampaignID,
			RequireExisting:    eventType != MobileAttributionEventRegister && eventType != MobileAttributionEventLogin && eventType != MobileAttributionEventOpen,
		})
		if err != nil {
			if errors.Is(err, ErrMobileAttributionInstallationConflict) || errors.Is(err, ErrMobileAttributionInstallationUnbound) {
				return MobileAttributionRecordResult{}, err
			}
			return MobileAttributionRecordResult{}, err
		}
	}
	created, err := s.repo.RecordEvent(ctx, write)
	return MobileAttributionRecordResult{Created: created}, err
}

func (s *MobileAttributionService) ListInstallations(ctx context.Context, filter MobileAttributionAdminFilter) ([]MobileAttributionInstallationRow, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.InternalServer("MOBILE_ATTRIBUTION_UNAVAILABLE", "mobile attribution is unavailable")
	}
	filter.Page, filter.PageSize = normalizeMobileAttributionPagination(filter.Page, filter.PageSize)
	return s.repo.ListInstallations(ctx, filter)
}

func (s *MobileAttributionService) Funnel(ctx context.Context, filter MobileAttributionAdminFilter) ([]MobileAttributionFunnelStep, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("MOBILE_ATTRIBUTION_UNAVAILABLE", "mobile attribution is unavailable")
	}
	return s.repo.Funnel(ctx, filter)
}

func normalizeMobileAttributionPagination(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func (s *MobileAttributionService) verifyAttribution(ctx context.Context, token string, now time.Time) (MobileAttributionReference, []byte, error) {
	if strings.TrimSpace(token) == "" {
		return MobileAttributionReference{}, nil, nil
	}
	if strings.HasPrefix(token, "v1.") {
		ref, err := s.repo.ResolveReferralToken(ctx, token, now)
		if err != nil {
			if errors.Is(err, ErrMobileAttributionInvalidToken) {
				return MobileAttributionReference{}, nil, ErrMobileAttributionInvalidToken
			}
			return MobileAttributionReference{}, nil, err
		}
		return ref, s.digest(token), nil
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(parts[0]) > 4096 || len(parts[1]) > 256 {
		return MobileAttributionReference{}, nil, ErrMobileAttributionInvalidToken
	}
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte("sub2api:mobile-attribution:v1:" + parts[0]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return MobileAttributionReference{}, nil, ErrMobileAttributionInvalidToken
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(payloadBytes) > 4096 {
		return MobileAttributionReference{}, nil, ErrMobileAttributionInvalidToken
	}
	var payload MobileAttributionSignedPayload
	if json.Unmarshal(payloadBytes, &payload) != nil || payload.ExpiresAt <= now.Unix() || payload.ExpiresAt > now.Add(365*24*time.Hour).Unix() || (payload.CampaignID == nil && strings.TrimSpace(payload.ReferralCode) == "") {
		return MobileAttributionReference{}, nil, ErrMobileAttributionInvalidToken
	}
	if payload.CampaignID != nil && *payload.CampaignID <= 0 {
		return MobileAttributionReference{}, nil, ErrMobileAttributionInvalidToken
	}
	if len(payload.ReferralCode) > 64 {
		return MobileAttributionReference{}, nil, ErrMobileAttributionInvalidToken
	}
	ref, err := s.repo.ResolveReference(ctx, payload.CampaignID, strings.TrimSpace(payload.ReferralCode))
	if err != nil {
		return MobileAttributionReference{}, nil, err
	}
	digest := s.digest(token)
	return ref, digest, nil
}

func (s *MobileAttributionService) digest(value string) []byte {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte("sub2api:mobile-attribution:digest:v1:" + value))
	return mac.Sum(nil)
}

func validMobileAttributionEventType(value string) bool {
	switch value {
	case MobileAttributionEventDownload, MobileAttributionEventClick, MobileAttributionEventOpen, MobileAttributionEventRegister, MobileAttributionEventLogin, MobileAttributionEventActive, MobileAttributionEventShare:
		return true
	}
	return false
}

func sanitizeMobileAttributionMetadata(input map[string]string) map[string]string {
	clean := map[string]string{}
	for key, value := range input {
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "surface" && key != "channel" && key != "share_target" && key != "poster_theme" && key != "event_name" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) > 64 {
			value = value[:64]
		}
		if key == "event_name" {
			switch value {
			case "first_launch", "registered", "login", "active", "poster_scanned", "share_opened", "share_completed":
			default:
				continue
			}
		}
		clean[key] = value
	}
	return clean
}
