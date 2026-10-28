package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type ModerationUserState struct {
	UserID         int64      `json:"user_id"`
	Version        int64      `json:"version"`
	Generation     int64      `json:"generation"`
	AutoBanExempt  bool       `json:"auto_ban_exempt"`
	KeywordExempt  bool       `json:"keyword_exempt"`
	APIAuditExempt bool       `json:"api_audit_exempt"`
	EmailExempt    bool       `json:"email_exempt"`
	OwnedDisable   bool       `json:"owned_disable"`
	Status         string     `json:"status"`
	CurrentCount   int        `json:"current_count"`
	Reason         string     `json:"reason"`
	ActorID        *int64     `json:"actor_id"`
	UpdatedAt      *time.Time `json:"updated_at"`
}

type ModerationUserAction struct {
	Version        int64  `json:"version"`
	Reason         string `json:"reason"`
	Confirm        bool   `json:"confirm"`
	ActorID        int64  `json:"-"`
	Action         string `json:"action"`
	Exempt         *bool  `json:"auto_ban_exempt"`
	KeywordExempt  *bool  `json:"keyword_exempt"`
	APIAuditExempt *bool  `json:"api_audit_exempt"`
	EmailExempt    *bool  `json:"email_exempt"`
}

type ModerationAutoBanResult struct {
	Changed  bool
	Disabled bool
}

type moderationUserRepository interface {
	GetModerationUser(context.Context, int64, time.Time, bool) (*ModerationUserState, error)
	ListModerationUsers(context.Context, int, int, string, int64, time.Time, bool) ([]ModerationUserState, int64, error)
	ManageModerationUser(context.Context, int64, ModerationUserAction) error
	ApplyModerationAutoBan(context.Context, int64) (ModerationAutoBanResult, error)
	ApplyModerationHit(context.Context, ModerationHit) (ModerationHitResult, error)
}

type ModerationHit struct {
	UserID       int64
	Version      int64
	RequestID    string
	Action       string
	Since        time.Time
	AutoBan      bool
	Threshold    int
	ExcludeCyber bool
}

type ModerationHitResult struct {
	Accepted     bool
	Count        int
	BanApplied   bool
	OwnedDisable bool
}

func (s *ContentModerationService) moderationUserRepo() (moderationUserRepository, error) {
	if s != nil {
		if repo, ok := s.repo.(moderationUserRepository); ok {
			return repo, nil
		}
	}
	return nil, infraerrors.ServiceUnavailable("MODERATION_USER_STATE_UNAVAILABLE", "Content moderation user state unavailable")
}

func (s *ContentModerationService) GetModerationUser(ctx context.Context, userID int64) (*ModerationUserState, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER_ID", "Invalid user ID")
	}
	repo, err := s.moderationUserRepo()
	if err != nil {
		return nil, err
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	return repo.GetModerationUser(ctx, userID, moderationCountSince(cfg), cfg.CyberPolicyExcludeFromBanCount)
}

func (s *ContentModerationService) ListModerationUsers(ctx context.Context, page, size int, state string, userID int64) ([]ModerationUserState, int64, error) {
	if page < 1 || page > 1000000 || size < 1 || size > 100 || userID < 0 {
		return nil, 0, infraerrors.BadRequest("INVALID_MODERATION_FILTER", "Invalid page, size or user ID")
	}
	if state == "" {
		state = "all"
	}
	if !validModerationUserFilter(state) {
		return nil, 0, infraerrors.BadRequest("INVALID_MODERATION_FILTER", "Invalid moderation state filter")
	}
	repo, err := s.moderationUserRepo()
	if err != nil {
		return nil, 0, err
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, 0, err
	}
	return repo.ListModerationUsers(ctx, page, size, state, userID, moderationCountSince(cfg), cfg.CyberPolicyExcludeFromBanCount)
}

func (s *ContentModerationService) ManageModerationUser(ctx context.Context, userID int64, input ModerationUserAction) (*ModerationUserState, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if userID <= 0 || input.ActorID <= 0 || input.Version < 0 || !input.Confirm || input.Reason == "" || utf8.RuneCountInString(input.Reason) > 500 {
		return nil, infraerrors.BadRequest("INVALID_MODERATION_ACTION", "Confirmation, version and a reason (1-500 characters) are required")
	}
	if input.Action != "unban" && input.Action != "reset" && input.Action != "exemption" {
		return nil, infraerrors.BadRequest("INVALID_MODERATION_ACTION", "Invalid moderation action")
	}
	if (input.Action == "exemption") != (input.Exempt != nil || input.KeywordExempt != nil || input.APIAuditExempt != nil || input.EmailExempt != nil) {
		return nil, infraerrors.BadRequest("INVALID_MODERATION_ACTION", "Exemption requires at least one explicit boolean")
	}
	repo, err := s.moderationUserRepo()
	if err != nil {
		return nil, err
	}
	if err := repo.ManageModerationUser(ctx, userID, input); err != nil {
		return nil, err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	return s.GetModerationUser(ctx, userID)
}

func (s *ContentModerationService) ApplyModerationAutoBan(ctx context.Context, userID int64) (bool, bool, error) {
	repo, err := s.moderationUserRepo()
	if err != nil {
		return false, false, err
	}
	result, err := repo.ApplyModerationAutoBan(ctx, userID)
	if err != nil {
		return false, false, err
	}
	if result.Changed && s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	return result.Changed, result.Disabled, nil
}

func moderationCountSince(cfg *ContentModerationConfigView) time.Time {
	if cfg == nil || cfg.ViolationWindowHours <= 0 {
		return time.Now()
	}
	return time.Now().Add(-time.Duration(cfg.ViolationWindowHours) * time.Hour)
}

func validModerationUserFilter(state string) bool {
	switch state {
	case "all", "exempt", "enforced", "auto_ban_exempt", "keyword_exempt", "api_audit_exempt", "email_exempt":
		return true
	default:
		return false
	}
}

func (s *ContentModerationService) captureModerationUserState(ctx context.Context, userID int64) *ModerationUserState {
	repo, err := s.moderationUserRepo()
	if err != nil || userID <= 0 {
		return nil
	}
	state, err := repo.GetModerationUser(ctx, userID, time.Now(), false)
	if err != nil {
		return nil
	}
	return state
}

func (s *ContentModerationService) CaptureModerationUserVersion(ctx context.Context, userID int64) *int64 {
	state := s.captureModerationUserState(ctx, userID)
	if state == nil {
		return nil
	}
	version := state.Version
	return &version
}
