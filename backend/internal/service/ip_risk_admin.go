package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"sort"
	"strings"
	"time"
)

type RiskCaseStatus string

const (
	RiskCaseStatusOpen       RiskCaseStatus = "open"
	RiskCaseStatusObserving  RiskCaseStatus = "observing"
	RiskCaseStatusProcessing RiskCaseStatus = "processing"
	RiskCaseStatusResolved   RiskCaseStatus = "resolved"
	RiskCaseStatusIgnored    RiskCaseStatus = "ignored"
)

type RiskActionType string

const (
	RiskActionObserve                  RiskActionType = "observe"
	RiskActionMarkSharedNetwork        RiskActionType = "mark_shared_network"
	RiskActionAllowlist                RiskActionType = "allowlist"
	RiskActionTemporaryRegistrationBan RiskActionType = "temporary_registration_block"
	RiskActionPermanentRegistrationBan RiskActionType = "permanent_registration_block"
	RiskActionDisableAPIKeys           RiskActionType = "disable_api_keys"
	RiskActionDisableUsers             RiskActionType = "disable_users"
	RiskActionResolve                  RiskActionType = "resolve"
	RiskActionIgnore                   RiskActionType = "ignore"
	RiskActionRollback                 RiskActionType = "rollback"
)

type IPRiskOverview struct {
	OpenCases        int64      `json:"open_cases"`
	CriticalCases    int64      `json:"critical_cases"`
	BlockedPolicies  int64      `json:"blocked_policies"`
	ReviewUsers      int64      `json:"review_users"`
	LastDetectedAt   *time.Time `json:"last_detected_at,omitempty"`
	LatestScanStatus string     `json:"latest_scan_status,omitempty"`
}

type IPRiskCaseFilter struct {
	Page       int
	PageSize   int
	Level      string
	Status     string
	Signal     string
	Search     string
	RangeStart *time.Time
	RangeEnd   *time.Time
}

type IPRiskCaseSummary struct {
	ID                 int64          `json:"id"`
	PrimaryIP          string         `json:"primary_ip"`
	PrimaryNetwork     string         `json:"primary_network"`
	Score              int            `json:"score"`
	Level              RiskLevel      `json:"level"`
	Status             string         `json:"status"`
	EvidenceConfidence string         `json:"evidence_confidence"`
	Signals            []IPRiskSignal `json:"signals"`
	RelatedUserCount   int            `json:"related_user_count"`
	SelectedUserCount  int            `json:"selected_user_count"`
	AutoBlockEligible  bool           `json:"auto_block_eligible"`
	FirstDetectedAt    time.Time      `json:"first_detected_at"`
	LastDetectedAt     time.Time      `json:"last_detected_at"`
	Version            int64          `json:"version"`
}

type IPRiskRelatedUser struct {
	UserID                 int64              `json:"user_id"`
	Email                  string             `json:"email"`
	Username               string             `json:"username"`
	Role                   string             `json:"role"`
	Status                 string             `json:"status"`
	SignupSource           string             `json:"signup_source"`
	RelationType           IPRiskUserRelation `json:"relation_type"`
	EvidenceConfidence     EvidenceConfidence `json:"evidence_confidence"`
	RecommendedSelected    bool               `json:"recommended_selected"`
	FirstSeenAt            time.Time          `json:"first_seen_at"`
	LastSeenAt             time.Time          `json:"last_seen_at"`
	CreatedAt              time.Time          `json:"created_at"`
	TotalRecharged         float64            `json:"total_recharged"`
	Balance                float64            `json:"balance"`
	PrimaryIPRegistrations int                `json:"primary_ip_registrations"`
	SharedUAAccountCount   int                `json:"shared_ua_account_count"`
	GiftGranted            float64            `json:"gift_granted"`
	GiftConsumed           float64            `json:"gift_consumed"`
	GiftRemaining          float64            `json:"gift_remaining"`
	APIKeyCount            int                `json:"api_key_count"`
	ActiveAPIKeyCount      int                `json:"active_api_key_count"`
	APIKeys                []IPRiskRelatedKey `json:"api_keys"`
	Evidence               map[string]any     `json:"evidence"`
}

type IPRiskRelatedKey struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type IPRiskTimelineEvent struct {
	ID         int64     `json:"id"`
	EventType  string    `json:"event_type"`
	UserID     *int64    `json:"user_id,omitempty"`
	IPAddress  string    `json:"ip_address"`
	Confidence string    `json:"confidence"`
	RequestID  string    `json:"request_id,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type IPRiskCaseDetail struct {
	Case               IPRiskCaseSummary     `json:"case"`
	Evidence           IPRiskEvidence        `json:"evidence"`
	RecommendedActions []string              `json:"recommended_actions"`
	Users              []IPRiskRelatedUser   `json:"users"`
	Timeline           []IPRiskTimelineEvent `json:"timeline"`
	Actions            []IPRiskActionRecord  `json:"actions"`
}

type IPPolicyMode string

const (
	IPPolicyAllowlist         IPPolicyMode = "allowlist"
	IPPolicyObserve           IPPolicyMode = "observe"
	IPPolicySharedNetwork     IPPolicyMode = "shared_network"
	IPPolicyBlockRegistration IPPolicyMode = "block_registration"
)

type IPRiskPolicy struct {
	ID             int64        `json:"id"`
	Mode           IPPolicyMode `json:"mode"`
	IPNetwork      string       `json:"ip_network,omitempty"`
	ExactIP        string       `json:"exact_ip,omitempty"`
	Reason         string       `json:"reason"`
	Enabled        bool         `json:"enabled"`
	ExpiresAt      *time.Time   `json:"expires_at,omitempty"`
	CreatedBy      *int64       `json:"created_by,omitempty"`
	SourceActionID *int64       `json:"source_action_id,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

type IPRiskPolicyInput struct {
	Mode      IPPolicyMode `json:"mode"`
	IPNetwork string       `json:"ip_network"`
	ExactIP   string       `json:"exact_ip"`
	Reason    string       `json:"reason"`
	Enabled   bool         `json:"enabled"`
	ExpiresAt *time.Time   `json:"expires_at"`
	CreatedBy *int64       `json:"-"`
}

type IPRiskManagedConfig struct {
	Registration10mThreshold  int  `json:"registration_10m_threshold"`
	Registration10mScore      int  `json:"registration_10m_score"`
	Registration1hThreshold   int  `json:"registration_1h_threshold"`
	Registration1hScore       int  `json:"registration_1h_score"`
	Registration24hThreshold  int  `json:"registration_24h_threshold"`
	Registration24hScore      int  `json:"registration_24h_score"`
	SharedUA3Threshold        int  `json:"shared_ua_3_threshold"`
	SharedUA3Score            int  `json:"shared_ua_3_score"`
	SharedUA5Threshold        int  `json:"shared_ua_5_threshold"`
	SharedUA5Score            int  `json:"shared_ua_5_score"`
	EmailPatternThreshold     int  `json:"email_pattern_threshold"`
	EmailPatternScore         int  `json:"email_pattern_score"`
	SharedAPIIPThreshold      int  `json:"shared_api_ip_threshold"`
	SharedAPIIPScore          int  `json:"shared_api_ip_score"`
	RapidBehaviorThreshold    int  `json:"rapid_behavior_threshold"`
	RapidBehaviorScore        int  `json:"rapid_behavior_score"`
	SharedSignupCodeThreshold int  `json:"shared_signup_code_threshold"`
	SharedSignupCodeScore     int  `json:"shared_signup_code_score"`
	TrustedAccountScore       int  `json:"trusted_account_score"`
	AutoBlockScore            int  `json:"auto_block_score"`
	AutoBlockMinRegistrations int  `json:"auto_block_min_registrations"`
	AutoBlockDurationMinutes  int  `json:"auto_block_duration_minutes"`
	AutoBlockEnabled          bool `json:"auto_block_enabled"`
	HistoricalBackfillEnabled bool `json:"historical_backfill_enabled"`
	EventRetentionDays        int  `json:"event_retention_days"`
	CaseRetentionDays         int  `json:"case_retention_days"`
}

func DefaultIPRiskManagedConfig() IPRiskManagedConfig {
	score := DefaultIPRiskConfig()
	return IPRiskManagedConfig{
		Registration10mThreshold:  score.Registration10mThreshold,
		Registration10mScore:      score.Registration10mScore,
		Registration1hThreshold:   score.Registration1hThreshold,
		Registration1hScore:       score.Registration1hScore,
		Registration24hThreshold:  score.Registration24hThreshold,
		Registration24hScore:      score.Registration24hScore,
		SharedUA3Threshold:        score.SharedUA3Threshold,
		SharedUA3Score:            score.SharedUA3Score,
		SharedUA5Threshold:        score.SharedUA5Threshold,
		SharedUA5Score:            score.SharedUA5Score,
		EmailPatternThreshold:     score.EmailPatternThreshold,
		EmailPatternScore:         score.EmailPatternScore,
		SharedAPIIPThreshold:      score.SharedAPIIPThreshold,
		SharedAPIIPScore:          score.SharedAPIIPScore,
		RapidBehaviorThreshold:    score.RapidBehaviorThreshold,
		RapidBehaviorScore:        score.RapidBehaviorScore,
		SharedSignupCodeThreshold: score.SharedSignupCodeThreshold,
		SharedSignupCodeScore:     score.SharedSignupCodeScore,
		TrustedAccountScore:       score.TrustedAccountScore,
		AutoBlockScore:            score.AutoBlockScore,
		AutoBlockMinRegistrations: score.AutoBlockMinRegistrations,
		AutoBlockDurationMinutes:  int(score.AutoBlockDuration / time.Minute),
		AutoBlockEnabled:          false,
		HistoricalBackfillEnabled: false,
		EventRetentionDays:        90,
		CaseRetentionDays:         365,
	}
}

type IPRiskActionInput struct {
	ActionType      RiskActionType `json:"action_type"`
	UserIDs         []int64        `json:"user_ids"`
	APIKeyIDs       []int64        `json:"api_key_ids"`
	DurationMinutes int            `json:"duration_minutes"`
	Reason          string         `json:"reason"`
	PreviewToken    string         `json:"preview_token"`
}

type IPRiskActionPreview struct {
	CaseID            int64          `json:"case_id"`
	CaseVersion       int64          `json:"case_version"`
	ActionType        RiskActionType `json:"action_type"`
	UserIDs           []int64        `json:"user_ids"`
	APIKeyIDs         []int64        `json:"api_key_ids"`
	UserCount         int            `json:"user_count"`
	APIKeyCount       int            `json:"api_key_count"`
	AlreadyDisabled   int            `json:"already_disabled"`
	ProtectedUsers    []int64        `json:"protected_users"`
	TrustedUsers      []int64        `json:"trusted_users"`
	InferredUsers     []int64        `json:"inferred_users"`
	DurationMinutes   int            `json:"duration_minutes"`
	RequiresStepUp    bool           `json:"requires_step_up"`
	StateDigest       string         `json:"state_digest"`
	ConfirmationToken string         `json:"confirmation_token"`
	ExpiresAt         time.Time      `json:"expires_at"`
}

type IPRiskActionItem struct {
	ID             int64          `json:"id"`
	TargetType     string         `json:"target_type"`
	TargetID       *int64         `json:"target_id,omitempty"`
	TargetIP       string         `json:"target_ip,omitempty"`
	BeforeState    map[string]any `json:"before_state"`
	AfterState     map[string]any `json:"after_state"`
	Status         string         `json:"status"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	RollbackStatus string         `json:"rollback_status"`
}

type IPRiskActionRecord struct {
	ID                 int64              `json:"id"`
	CaseID             *int64             `json:"case_id,omitempty"`
	ActionType         RiskActionType     `json:"action_type"`
	Status             string             `json:"status"`
	ActorType          string             `json:"actor_type"`
	ActorUserID        *int64             `json:"actor_user_id,omitempty"`
	Reason             string             `json:"reason"`
	RollbackStatus     string             `json:"rollback_status"`
	RollbackOfActionID *int64             `json:"rollback_of_action_id,omitempty"`
	Result             map[string]any     `json:"result"`
	Items              []IPRiskActionItem `json:"items,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	CompletedAt        *time.Time         `json:"completed_at,omitempty"`
}

type IPRiskActionCreate struct {
	CaseID             *int64
	CaseVersion        int64
	ActionType         RiskActionType
	ActorType          string
	ActorUserID        *int64
	Reason             string
	ActionSnapshot     map[string]any
	RollbackOfActionID *int64
	PreviewTokenHash   []byte
	PreviewExpiresAt   *time.Time
}

type IPRiskActionItemCreate struct {
	ActionID       int64
	TargetType     string
	TargetID       *int64
	TargetIP       string
	BeforeState    map[string]any
	AfterState     map[string]any
	Status         string
	ErrorMessage   string
	RollbackStatus string
}

type IPRiskAdminService struct {
	repo        IPRiskRepository
	core        *IPRiskService
	admin       AdminService
	apiKeys     APIKeyRepository
	invalidator APIKeyAuthCacheInvalidator
	hasher      *IPRiskHasher
}

func NewIPRiskAdminService(
	repo IPRiskRepository,
	core *IPRiskService,
	admin AdminService,
	apiKeys APIKeyRepository,
	invalidator APIKeyAuthCacheInvalidator,
	hasher *IPRiskHasher,
) *IPRiskAdminService {
	return &IPRiskAdminService{
		repo: repo, core: core, admin: admin, apiKeys: apiKeys,
		invalidator: invalidator, hasher: hasher,
	}
}

func (s *IPRiskAdminService) Overview(ctx context.Context) (*IPRiskOverview, error) {
	return s.repo.GetIPRiskOverview(ctx)
}

func (s *IPRiskAdminService) ListCases(ctx context.Context, filter IPRiskCaseFilter) ([]IPRiskCaseSummary, int64, error) {
	return s.repo.ListIPRiskCases(ctx, filter)
}

func (s *IPRiskAdminService) GetCase(ctx context.Context, caseID int64) (*IPRiskCaseDetail, error) {
	return s.repo.GetIPRiskCaseDetail(ctx, caseID)
}

func (s *IPRiskAdminService) GetConfig(ctx context.Context) (*IPRiskManagedConfig, error) {
	config, err := s.repo.GetIPRiskManagedConfig(ctx)
	if errors.Is(err, ErrIPRiskConfigNotFound) {
		value := DefaultIPRiskManagedConfig()
		return &value, nil
	}
	return config, err
}

func (s *IPRiskAdminService) UpdateConfig(ctx context.Context, config IPRiskManagedConfig, actorID int64) (*IPRiskManagedConfig, error) {
	if err := validateIPRiskManagedConfig(config); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateIPRiskManagedConfig(ctx, config, actorID); err != nil {
		return nil, err
	}
	if s.core != nil {
		s.core.ApplyManagedConfig(config)
	}
	return &config, nil
}

func validateIPRiskManagedConfig(config IPRiskManagedConfig) error {
	values := []int{
		config.Registration10mThreshold, config.Registration1hThreshold, config.Registration24hThreshold,
		config.SharedUA3Threshold, config.SharedUA5Threshold, config.EmailPatternThreshold,
		config.SharedAPIIPThreshold, config.RapidBehaviorThreshold, config.SharedSignupCodeThreshold,
		config.AutoBlockMinRegistrations, config.AutoBlockDurationMinutes,
		config.EventRetentionDays, config.CaseRetentionDays,
	}
	for _, value := range values {
		if value <= 0 {
			return errors.New("ip risk thresholds and retention values must be positive")
		}
	}
	scores := []int{
		config.Registration10mScore, config.Registration1hScore, config.Registration24hScore,
		config.SharedUA3Score, config.SharedUA5Score, config.EmailPatternScore,
		config.SharedAPIIPScore, config.RapidBehaviorScore, config.SharedSignupCodeScore,
		config.AutoBlockScore,
	}
	for _, score := range scores {
		if score < 0 || score > 100 {
			return errors.New("ip risk scores must be between 0 and 100")
		}
	}
	if config.TrustedAccountScore > 0 || config.TrustedAccountScore < -100 {
		return errors.New("trusted account score must be between -100 and 0")
	}
	return nil
}

func (s *IPRiskAdminService) ListPolicies(ctx context.Context) ([]IPRiskPolicy, error) {
	return s.repo.ListIPRiskPolicies(ctx)
}

func (s *IPRiskAdminService) CreatePolicy(ctx context.Context, input IPRiskPolicyInput) (*IPRiskPolicy, error) {
	if err := validateIPRiskPolicyInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreateIPRiskPolicy(ctx, input, nil)
}

func (s *IPRiskAdminService) UpdatePolicy(ctx context.Context, id int64, input IPRiskPolicyInput) (*IPRiskPolicy, error) {
	if err := validateIPRiskPolicyInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdateIPRiskPolicy(ctx, id, input)
}

func (s *IPRiskAdminService) DeletePolicy(ctx context.Context, id int64) error {
	return s.repo.DeleteIPRiskPolicy(ctx, id)
}

func (s *IPRiskAdminService) StartManualScan(ctx context.Context, start, end time.Time, actorID int64) (*IPRiskScan, error) {
	if s.core == nil {
		return nil, errors.New("ip risk scanner unavailable")
	}
	return s.core.StartScan(ctx, IPRiskScanManual, start, end, &actorID)
}

func (s *IPRiskAdminService) GetScan(ctx context.Context, id int64) (*IPRiskScan, error) {
	return s.repo.GetIPRiskScan(ctx, id)
}

func (s *IPRiskAdminService) ListActions(ctx context.Context, page, pageSize int) ([]IPRiskActionRecord, int64, error) {
	return s.repo.ListIPRiskActions(ctx, page, pageSize)
}

func (s *IPRiskAdminService) PreviewAction(ctx context.Context, caseID int64, input IPRiskActionInput) (*IPRiskActionPreview, error) {
	detail, err := s.repo.GetIPRiskCaseDetail(ctx, caseID)
	if err != nil {
		return nil, err
	}
	if !validIPRiskActionType(input.ActionType) {
		return nil, errors.New("unsupported risk action")
	}
	if input.ActionType == RiskActionTemporaryRegistrationBan {
		switch input.DurationMinutes {
		case 0, 30, 120, 1440, 10080:
		default:
			return nil, errors.New("temporary registration block duration is not supported")
		}
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" || len([]rune(input.Reason)) > 1000 {
		return nil, errors.New("action reason is required and must not exceed 1000 characters")
	}
	userSet := make(map[int64]struct{}, len(input.UserIDs))
	for _, id := range input.UserIDs {
		if id > 0 {
			userSet[id] = struct{}{}
		}
	}
	if len(userSet) > 500 {
		return nil, errors.New("risk action cannot include more than 500 users")
	}
	preview := &IPRiskActionPreview{
		CaseID: caseID, CaseVersion: detail.Case.Version, ActionType: input.ActionType,
		DurationMinutes: input.DurationMinutes,
		RequiresStepUp:  riskActionRequiresStepUp(input.ActionType),
		ExpiresAt:       time.Now().UTC().Add(5 * time.Minute),
	}
	for _, user := range detail.Users {
		if _, selected := userSet[user.UserID]; !selected {
			continue
		}
		if user.Role == RoleAdmin {
			preview.ProtectedUsers = append(preview.ProtectedUsers, user.UserID)
			continue
		}
		preview.UserIDs = append(preview.UserIDs, user.UserID)
		if user.Status == StatusDisabled {
			preview.AlreadyDisabled++
		}
		if user.RelationType == IPRiskUserRelationTrustedExisting {
			preview.TrustedUsers = append(preview.TrustedUsers, user.UserID)
		}
		if user.EvidenceConfidence == EvidenceConfidenceInferred {
			preview.InferredUsers = append(preview.InferredUsers, user.UserID)
		}
		for _, key := range user.APIKeys {
			preview.APIKeyIDs = append(preview.APIKeyIDs, key.ID)
		}
	}
	if len(input.APIKeyIDs) > 0 {
		allowed := make(map[int64]struct{}, len(input.APIKeyIDs))
		for _, id := range input.APIKeyIDs {
			allowed[id] = struct{}{}
		}
		filtered := preview.APIKeyIDs[:0]
		for _, id := range preview.APIKeyIDs {
			if _, ok := allowed[id]; ok {
				filtered = append(filtered, id)
			}
		}
		preview.APIKeyIDs = filtered
	}
	sort.Slice(preview.UserIDs, func(i, j int) bool { return preview.UserIDs[i] < preview.UserIDs[j] })
	sort.Slice(preview.APIKeyIDs, func(i, j int) bool { return preview.APIKeyIDs[i] < preview.APIKeyIDs[j] })
	preview.UserCount = len(preview.UserIDs)
	preview.APIKeyCount = len(preview.APIKeyIDs)
	preview.StateDigest = ipRiskActionStateDigest(detail, preview.ActionType, preview.UserIDs, preview.APIKeyIDs)
	token, err := s.signActionPreview(*preview, input.Reason)
	if err != nil {
		return nil, err
	}
	preview.ConfirmationToken = token
	return preview, nil
}

func riskActionRequiresStepUp(action RiskActionType) bool {
	return action == RiskActionDisableUsers ||
		action == RiskActionPermanentRegistrationBan ||
		action == RiskActionRollback
}

func validIPRiskActionType(action RiskActionType) bool {
	switch action {
	case RiskActionObserve,
		RiskActionMarkSharedNetwork,
		RiskActionAllowlist,
		RiskActionTemporaryRegistrationBan,
		RiskActionPermanentRegistrationBan,
		RiskActionDisableAPIKeys,
		RiskActionDisableUsers,
		RiskActionResolve,
		RiskActionIgnore:
		return true
	default:
		return false
	}
}

func validateIPRiskPolicyInput(input IPRiskPolicyInput) error {
	switch input.Mode {
	case IPPolicyAllowlist, IPPolicyObserve, IPPolicySharedNetwork, IPPolicyBlockRegistration:
	default:
		return errors.New("invalid ip risk policy mode")
	}
	input.IPNetwork = strings.TrimSpace(input.IPNetwork)
	input.ExactIP = strings.TrimSpace(input.ExactIP)
	if input.IPNetwork == "" && input.ExactIP == "" {
		return errors.New("ip risk policy requires an IP or CIDR")
	}
	if input.IPNetwork != "" {
		if _, err := netip.ParsePrefix(input.IPNetwork); err != nil {
			return errors.New("invalid ip risk policy CIDR")
		}
	}
	if input.ExactIP != "" {
		if _, err := netip.ParseAddr(input.ExactIP); err != nil {
			return errors.New("invalid ip risk policy IP")
		}
	}
	if strings.TrimSpace(input.Reason) == "" || len([]rune(input.Reason)) > 500 {
		return errors.New("policy reason is required and must not exceed 500 characters")
	}
	return nil
}

func (s *IPRiskAdminService) signActionPreview(preview IPRiskActionPreview, reason string) (string, error) {
	if s.hasher == nil || len(s.hasher.key) == 0 {
		return "", errors.New("ip risk preview signer unavailable")
	}
	payload := struct {
		Preview IPRiskActionPreview `json:"preview"`
		Reason  string              `json:"reason"`
	}{Preview: preview, Reason: reason}
	payload.Preview.ConfirmationToken = ""
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.hasher.key)
	_, _ = mac.Write([]byte("ip-risk-action-preview\x00"))
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

func (s *IPRiskAdminService) verifyActionPreview(token, reason string) (*IPRiskActionPreview, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 || s.hasher == nil {
		return nil, ErrIPRiskActionPreviewInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrIPRiskActionPreviewInvalid
	}
	signature, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, ErrIPRiskActionPreviewInvalid
	}
	mac := hmac.New(sha256.New, s.hasher.key)
	_, _ = mac.Write([]byte("ip-risk-action-preview\x00"))
	_, _ = mac.Write(raw)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, ErrIPRiskActionPreviewInvalid
	}
	var payload struct {
		Preview IPRiskActionPreview `json:"preview"`
		Reason  string              `json:"reason"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, ErrIPRiskActionPreviewInvalid
	}
	if strings.TrimSpace(payload.Reason) != strings.TrimSpace(reason) {
		return nil, ErrIPRiskActionPreviewStale
	}
	if time.Now().UTC().After(payload.Preview.ExpiresAt) {
		return nil, ErrIPRiskActionPreviewExpired
	}
	return &payload.Preview, nil
}

func (s *IPRiskAdminService) ExecuteAction(ctx context.Context, caseID, actorID int64, input IPRiskActionInput) (*IPRiskActionRecord, error) {
	preview, err := s.verifyActionPreview(input.PreviewToken, input.Reason)
	if err != nil {
		return nil, err
	}
	if preview.CaseID != caseID || preview.ActionType != input.ActionType {
		return nil, ErrIPRiskActionPreviewStale
	}
	detail, err := s.repo.GetIPRiskCaseDetail(ctx, caseID)
	if err != nil {
		return nil, err
	}
	if detail.Case.Version != preview.CaseVersion {
		return nil, ErrIPRiskActionPreviewStale
	}
	if current := ipRiskActionStateDigest(detail, preview.ActionType, preview.UserIDs, preview.APIKeyIDs); current != preview.StateDigest {
		return nil, ErrIPRiskActionPreviewStale
	}
	caseRef := caseID
	actorRef := actorID
	tokenHash := sha256.Sum256([]byte(strings.TrimSpace(input.PreviewToken)))
	previewExpiresAt := preview.ExpiresAt.UTC()
	record, err := s.repo.CreateIPRiskAction(ctx, IPRiskActionCreate{
		CaseID: &caseRef, CaseVersion: preview.CaseVersion, ActionType: input.ActionType,
		ActorType: "admin", ActorUserID: &actorRef, Reason: strings.TrimSpace(input.Reason),
		ActionSnapshot: map[string]any{
			"user_ids": preview.UserIDs, "api_key_ids": preview.APIKeyIDs,
			"duration_minutes": preview.DurationMinutes,
		},
		PreviewTokenHash: tokenHash[:],
		PreviewExpiresAt: &previewExpiresAt,
	})
	if err != nil {
		return nil, err
	}
	completed, failed := 0, 0
	var itemWriteErr error
	reserveItem := func(item IPRiskActionItemCreate) (int64, bool) {
		item.ActionID = record.ID
		itemID, writeErr := s.repo.ReserveIPRiskActionItem(ctx, item)
		if writeErr != nil {
			failed++
			itemWriteErr = errors.Join(itemWriteErr, writeErr)
			return 0, false
		}
		return itemID, true
	}
	finalizeItem := func(itemID int64, targetID *int64, status string, itemErr error) {
		if writeErr := s.repo.FinalizeIPRiskActionItem(
			ctx,
			itemID,
			targetID,
			status,
			errorString(itemErr),
			ipRiskActionItemRollbackStatus(status),
		); writeErr != nil {
			failed++
			itemWriteErr = errors.Join(itemWriteErr, writeErr)
		} else if status == "completed" {
			completed++
		} else {
			failed++
		}
	}

	switch input.ActionType {
	case RiskActionObserve, RiskActionResolve, RiskActionIgnore:
		status := RiskCaseStatusObserving
		if input.ActionType == RiskActionResolve {
			status = RiskCaseStatusResolved
		}
		if input.ActionType == RiskActionIgnore {
			status = RiskCaseStatusIgnored
		}
		id := caseID
		itemID, reserved := reserveItem(IPRiskActionItemCreate{
			TargetType: "case", TargetID: &id, BeforeState: map[string]any{"status": detail.Case.Status},
			AfterState: map[string]any{"status": status},
		})
		if reserved {
			err = s.repo.UpdateIPRiskCaseStatus(ctx, caseID, status)
			itemStatus := "completed"
			if err != nil {
				itemStatus = "failed"
			}
			finalizeItem(itemID, &id, itemStatus, err)
		}
	case RiskActionMarkSharedNetwork, RiskActionAllowlist, RiskActionTemporaryRegistrationBan, RiskActionPermanentRegistrationBan:
		mode := IPPolicySharedNetwork
		expiresAt := (*time.Time)(nil)
		ipNetwork := ""
		exactIP := detail.Case.PrimaryIP
		switch input.ActionType {
		case RiskActionMarkSharedNetwork:
			ipNetwork = detail.Case.PrimaryNetwork
			exactIP = ""
		case RiskActionAllowlist:
			mode = IPPolicyAllowlist
		case RiskActionTemporaryRegistrationBan:
			mode = IPPolicyBlockRegistration
			duration := preview.DurationMinutes
			if duration <= 0 {
				duration = 30
			}
			value := time.Now().UTC().Add(time.Duration(duration) * time.Minute)
			expiresAt = &value
		case RiskActionPermanentRegistrationBan:
			mode = IPPolicyBlockRegistration
		}
		policyState := map[string]any{
			"enabled":    true,
			"mode":       string(mode),
			"ip_network": ipNetwork,
			"exact_ip":   exactIP,
			"reason":     strings.TrimSpace(input.Reason),
		}
		if expiresAt != nil {
			policyState["expires_at"] = expiresAt.UTC().Format(time.RFC3339Nano)
		}
		itemID, reserved := reserveItem(IPRiskActionItemCreate{
			TargetType: "ip_policy", TargetIP: detail.Case.PrimaryIP,
			BeforeState: map[string]any{"enabled": false}, AfterState: policyState,
		})
		if reserved {
			policy, policyErr := s.repo.CreateIPRiskPolicy(ctx, IPRiskPolicyInput{
				Mode: mode, IPNetwork: ipNetwork, ExactIP: exactIP,
				Reason: input.Reason, Enabled: true, ExpiresAt: expiresAt, CreatedBy: &actorRef,
			}, &record.ID)
			itemStatus := "completed"
			if policyErr != nil {
				itemStatus = "failed"
			}
			var targetID *int64
			if policy != nil {
				targetID = &policy.ID
			}
			finalizeItem(itemID, targetID, itemStatus, policyErr)
			err = policyErr
		}
	case RiskActionDisableUsers:
		for _, userID := range preview.UserIDs {
			user, loadErr := s.admin.GetUser(ctx, userID)
			if loadErr == nil && user.Role == RoleAdmin {
				loadErr = errors.New("administrator account is protected")
			}
			before := ""
			if user != nil {
				before = user.Status
			}
			id := userID
			itemID, reserved := reserveItem(IPRiskActionItemCreate{
				TargetType: "user", TargetID: &id, BeforeState: map[string]any{"status": before},
				AfterState: map[string]any{"status": StatusDisabled},
			})
			if !reserved {
				continue
			}
			if loadErr == nil && before != StatusDisabled {
				_, loadErr = s.admin.UpdateUser(ctx, userID, &UpdateUserInput{Status: StatusDisabled, ActorAdminID: actorID})
			}
			status := "completed"
			if loadErr != nil {
				status = "failed"
			}
			finalizeItem(itemID, &id, status, loadErr)
		}
	case RiskActionDisableAPIKeys:
		allowed := make(map[int64]struct{}, len(preview.APIKeyIDs))
		for _, id := range preview.APIKeyIDs {
			allowed[id] = struct{}{}
		}
		for _, user := range detail.Users {
			for _, keyInfo := range user.APIKeys {
				if _, ok := allowed[keyInfo.ID]; !ok {
					continue
				}
				key, keyErr := s.apiKeys.GetByID(ctx, keyInfo.ID)
				before := ""
				if key != nil {
					before = key.Status
				}
				id := keyInfo.ID
				itemID, reserved := reserveItem(IPRiskActionItemCreate{
					TargetType: "api_key", TargetID: &id, BeforeState: map[string]any{"status": before},
					AfterState: map[string]any{"status": StatusDisabled},
				})
				if !reserved {
					continue
				}
				if keyErr == nil && key.Status != StatusDisabled {
					key.Status = StatusDisabled
					keyErr = s.apiKeys.Update(ctx, key)
					if keyErr == nil && s.invalidator != nil {
						s.invalidator.InvalidateAuthCacheByKey(ctx, key.Key)
					}
				}
				status := "completed"
				if keyErr != nil {
					status = "failed"
				}
				finalizeItem(itemID, &id, status, keyErr)
			}
		}
	default:
		err = errors.New("unsupported risk action")
	}
	err = errors.Join(err, itemWriteErr)
	status := "completed"
	if failed > 0 && completed > 0 {
		status = "partial"
	} else if failed > 0 || err != nil {
		status = "failed"
	}
	if updateErr := s.repo.CompleteIPRiskAction(ctx, record.ID, status, map[string]any{
		"completed_items": completed, "failed_items": failed,
	}, status == "completed" || status == "partial"); updateErr != nil && err == nil {
		err = updateErr
	}
	result, getErr := s.repo.GetIPRiskAction(ctx, record.ID)
	if err != nil {
		return result, err
	}
	return result, getErr
}

func (s *IPRiskAdminService) RollbackAction(ctx context.Context, actionID, actorID int64, reason string) (*IPRiskActionRecord, error) {
	action, err := s.repo.GetIPRiskAction(ctx, actionID)
	if err != nil {
		return nil, err
	}
	if action.ActionType == RiskActionRollback || action.RollbackStatus != "eligible" {
		return nil, ErrIPRiskActionNotRollbackEligible
	}
	actorRef := actorID
	rollbackOf := actionID
	record, err := s.repo.CreateIPRiskAction(ctx, IPRiskActionCreate{
		CaseID: action.CaseID, ActionType: RiskActionRollback, ActorType: "admin",
		ActorUserID: &actorRef, Reason: strings.TrimSpace(reason), RollbackOfActionID: &rollbackOf,
		ActionSnapshot: map[string]any{"source_action_id": actionID},
	})
	if err != nil {
		return nil, err
	}
	completed, conflicts := 0, 0
	skipped := 0
	var itemWriteErr error
	for _, item := range action.Items {
		if item.Status != "completed" || item.RollbackStatus != "eligible" {
			skipped++
			if writeErr := s.repo.AddIPRiskActionItem(ctx, IPRiskActionItemCreate{
				ActionID: record.ID, TargetType: item.TargetType, TargetID: item.TargetID, TargetIP: item.TargetIP,
				BeforeState: item.AfterState, AfterState: item.BeforeState, Status: "skipped",
				ErrorMessage:   "source action item did not complete; nothing to roll back",
				RollbackStatus: "not_requested",
			}); writeErr != nil {
				conflicts++
				itemWriteErr = errors.Join(itemWriteErr, writeErr)
			}
			continue
		}
		rollbackItemID, reserveErr := s.repo.ReserveIPRiskActionItem(ctx, IPRiskActionItemCreate{
			ActionID:    record.ID,
			TargetType:  item.TargetType,
			TargetID:    item.TargetID,
			TargetIP:    item.TargetIP,
			BeforeState: item.AfterState,
			AfterState:  item.BeforeState,
		})
		if reserveErr != nil {
			conflicts++
			itemWriteErr = errors.Join(itemWriteErr, reserveErr)
			continue
		}
		status := "completed"
		var itemErr error
		switch item.TargetType {
		case "user":
			if item.TargetID == nil {
				itemErr = errors.New("missing user target")
				break
			}
			user, loadErr := s.admin.GetUser(ctx, *item.TargetID)
			itemErr = loadErr
			expected, _ := item.AfterState["status"].(string)
			before, _ := item.BeforeState["status"].(string)
			if itemErr == nil && user.Status != expected {
				itemErr = errors.New("user state changed after action")
			}
			if itemErr == nil && before != "" {
				_, itemErr = s.admin.UpdateUser(ctx, *item.TargetID, &UpdateUserInput{Status: before, ActorAdminID: actorID})
			}
		case "api_key":
			if item.TargetID == nil {
				itemErr = errors.New("missing api key target")
				break
			}
			key, loadErr := s.apiKeys.GetByID(ctx, *item.TargetID)
			itemErr = loadErr
			expected, _ := item.AfterState["status"].(string)
			before, _ := item.BeforeState["status"].(string)
			if itemErr == nil && key.Status != expected {
				itemErr = errors.New("api key state changed after action")
			}
			if itemErr == nil && before != "" {
				key.Status = before
				itemErr = s.apiKeys.Update(ctx, key)
				if itemErr == nil && s.invalidator != nil {
					s.invalidator.InvalidateAuthCacheByKey(ctx, key.Key)
				}
			}
		case "ip_policy":
			if item.TargetID == nil {
				itemErr = errors.New("missing ip policy target")
				break
			}
			policies, loadErr := s.repo.ListIPRiskPolicies(ctx)
			itemErr = loadErr
			var current *IPRiskPolicy
			if itemErr == nil {
				for index := range policies {
					if policies[index].ID == *item.TargetID {
						current = &policies[index]
						break
					}
				}
				if current == nil {
					itemErr = errors.New("ip policy state changed after action")
				}
			}
			if itemErr == nil {
				if current.SourceActionID == nil ||
					*current.SourceActionID != action.ID ||
					!ipRiskPolicyMatchesActionState(*current, item.AfterState) {
					itemErr = errors.New("ip policy state changed after action")
				}
			}
			if itemErr == nil {
				itemErr = s.repo.DeleteIPRiskPolicy(ctx, *item.TargetID)
			}
		case "case":
			if item.TargetID == nil {
				itemErr = errors.New("missing case target")
				break
			}
			current, loadErr := s.repo.GetIPRiskCaseDetail(ctx, *item.TargetID)
			itemErr = loadErr
			expected, _ := item.AfterState["status"].(string)
			if itemErr == nil && current.Case.Status != expected {
				itemErr = errors.New("case state changed after action")
			}
			if itemErr == nil {
				before, _ := item.BeforeState["status"].(string)
				itemErr = s.repo.UpdateIPRiskCaseStatus(ctx, *item.TargetID, RiskCaseStatus(before))
			}
		}
		if itemErr != nil {
			status = "failed"
			conflicts++
		}
		if writeErr := s.repo.FinalizeIPRiskActionItem(
			ctx,
			rollbackItemID,
			item.TargetID,
			status,
			errorString(itemErr),
			status,
		); writeErr != nil {
			conflicts++
			itemWriteErr = errors.Join(itemWriteErr, writeErr)
		} else if itemErr == nil {
			completed++
		}
	}
	actionStatus := "completed"
	if conflicts > 0 && completed > 0 {
		actionStatus = "partial"
	} else if conflicts > 0 {
		actionStatus = "failed"
	}
	if err := s.repo.CompleteIPRiskAction(ctx, record.ID, actionStatus, map[string]any{
		"completed_items": completed, "conflict_items": conflicts, "skipped_items": skipped,
	}, false); err != nil {
		return nil, err
	}
	if err := s.repo.MarkIPRiskActionRolledBack(ctx, actionID, actionStatus); err != nil {
		itemWriteErr = errors.Join(itemWriteErr, err)
	}
	result, getErr := s.repo.GetIPRiskAction(ctx, record.ID)
	if itemWriteErr != nil {
		return result, errors.Join(itemWriteErr, getErr)
	}
	return result, getErr
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func ipRiskActionItemRollbackStatus(status string) string {
	if status == "completed" {
		return "eligible"
	}
	return "not_requested"
}

func ipRiskActionStateDigest(
	detail *IPRiskCaseDetail,
	action RiskActionType,
	userIDs,
	apiKeyIDs []int64,
) string {
	if detail == nil {
		return ""
	}
	type userState struct {
		ID     int64  `json:"id"`
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	type keyState struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	snapshot := struct {
		CaseID      int64          `json:"case_id"`
		CaseVersion int64          `json:"case_version"`
		CaseStatus  string         `json:"case_status"`
		PrimaryIP   string         `json:"primary_ip"`
		Action      RiskActionType `json:"action"`
		Users       []userState    `json:"users,omitempty"`
		APIKeys     []keyState     `json:"api_keys,omitempty"`
	}{
		CaseID:      detail.Case.ID,
		CaseVersion: detail.Case.Version,
		CaseStatus:  detail.Case.Status,
		PrimaryIP:   detail.Case.PrimaryIP,
		Action:      action,
	}
	selectedUsers := make(map[int64]struct{}, len(userIDs))
	for _, id := range userIDs {
		selectedUsers[id] = struct{}{}
	}
	selectedKeys := make(map[int64]struct{}, len(apiKeyIDs))
	for _, id := range apiKeyIDs {
		selectedKeys[id] = struct{}{}
	}
	for _, user := range detail.Users {
		if _, ok := selectedUsers[user.UserID]; !ok {
			continue
		}
		if action == RiskActionDisableUsers || action == RiskActionDisableAPIKeys {
			snapshot.Users = append(snapshot.Users, userState{
				ID: user.UserID, Role: user.Role, Status: user.Status,
			})
		}
		if action != RiskActionDisableAPIKeys {
			continue
		}
		for _, key := range user.APIKeys {
			if _, ok := selectedKeys[key.ID]; ok {
				snapshot.APIKeys = append(snapshot.APIKeys, keyState{ID: key.ID, Status: key.Status})
			}
		}
	}
	sort.Slice(snapshot.Users, func(i, j int) bool { return snapshot.Users[i].ID < snapshot.Users[j].ID })
	sort.Slice(snapshot.APIKeys, func(i, j int) bool { return snapshot.APIKeys[i].ID < snapshot.APIKeys[j].ID })
	raw, _ := json.Marshal(snapshot)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func ipRiskPolicyMatchesActionState(policy IPRiskPolicy, expected map[string]any) bool {
	if enabled, ok := expected["enabled"].(bool); ok && policy.Enabled != enabled {
		return false
	}
	if mode, ok := expected["mode"].(string); ok && string(policy.Mode) != mode {
		return false
	}
	if network, ok := expected["ip_network"].(string); ok && policy.IPNetwork != network {
		return false
	}
	if exactIP, ok := expected["exact_ip"].(string); ok && policy.ExactIP != exactIP {
		return false
	}
	if reason, ok := expected["reason"].(string); ok && policy.Reason != reason {
		return false
	}
	if rawExpires, recorded := expected["expires_at"]; recorded {
		expires, ok := rawExpires.(string)
		if !ok || policy.ExpiresAt == nil {
			return false
		}
		recordedAt, err := time.Parse(time.RFC3339Nano, expires)
		if err != nil || !policy.ExpiresAt.UTC().Equal(recordedAt.UTC()) {
			return false
		}
	}
	return true
}

var ErrIPRiskConfigNotFound = errors.New("ip risk config not found")

var (
	ErrIPRiskActionPreviewInvalid      = errors.New("invalid risk action preview token")
	ErrIPRiskActionPreviewStale        = errors.New("risk action preview is stale")
	ErrIPRiskActionPreviewExpired      = errors.New("risk action preview expired")
	ErrIPRiskActionNotRollbackEligible = errors.New("risk action is not eligible for rollback")
)
