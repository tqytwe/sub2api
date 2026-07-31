package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	ReferralCampaignStatusDraft     = "draft"
	ReferralCampaignStatusReview    = "review"
	ReferralCampaignStatusApproved  = "approved"
	ReferralCampaignStatusScheduled = "scheduled"
	ReferralCampaignStatusRunning   = "running"
	ReferralCampaignStatusPaused    = "paused"
	ReferralCampaignStatusSettling  = "settling"
	ReferralCampaignStatusClosed    = "closed"
	ReferralCampaignStatusCancelled = "cancelled"

	ReferralRiskPending  = "pending"
	ReferralRiskApproved = "approved"
	ReferralRiskRejected = "rejected"

	ReferralQualificationPending   = "pending"
	ReferralQualificationQualified = "qualified"
	ReferralQualificationRevoked   = "revoked"

	ReferralRewardStatusClaimable     = "claimable"
	ReferralRewardStatusClaimedFrozen = "claimed_frozen"
	ReferralRewardStatusAvailable     = "available"
	ReferralRewardStatusExpired       = "expired"
	ReferralRewardStatusRevoked       = "revoked"
	ReferralRewardStatusDebtReview    = "debt_review"
	ReferralRewardStatusResolved      = "resolved"
)

var (
	ErrReferralCampaignNotFound         = errors.New("referral campaign not found")
	ErrReferralCampaignNotOpen          = errors.New("referral campaign is not open")
	ErrReferralCampaignVersionConflict  = errors.New("referral campaign version conflict")
	ErrReferralCampaignAlreadyEnrolled  = errors.New("already enrolled in referral campaign")
	ErrReferralCampaignInvalidState     = errors.New("invalid referral campaign state transition")
	ErrReferralCampaignImmutable        = errors.New("only draft referral campaigns can be edited")
	ErrReferralCampaignTokenInvalid     = errors.New("invalid referral campaign token")
	ErrReferralCampaignTokenExpired     = errors.New("referral campaign token expired")
	ErrReferralRewardNotClaimable       = errors.New("referral reward is not claimable")
	ErrReferralCampaignBudgetExceeded   = errors.New("referral campaign budget exceeded")
	ErrReferralCampaignCapacityReached  = errors.New("referral campaign enrollment capacity reached")
	ErrReferralCampaignReviewerConflict = infraerrors.Conflict(
		"REFERRAL_CAMPAIGN_REVIEWER_CONFLICT",
		"campaign creator and prior reviewers cannot review another role",
	)
)

type ReferralCampaign struct {
	ID               int64              `json:"id"`
	Key              string             `json:"key"`
	Name             string             `json:"name"`
	Status           string             `json:"status"`
	Version          int64              `json:"version"`
	RegistrationFrom time.Time          `json:"registration_from"`
	RegistrationTo   time.Time          `json:"registration_to"`
	StartsAt         time.Time          `json:"starts_at"`
	EndsAt           time.Time          `json:"ends_at"`
	QualificationTo  time.Time          `json:"qualification_to"`
	ClaimDeadline    time.Time          `json:"claim_deadline"`
	RiskHoldHours    int                `json:"risk_hold_hours"`
	PayThreshold     float64            `json:"pay_threshold"`
	UsageThreshold   float64            `json:"usage_threshold"`
	MaxEnrollments   int                `json:"max_enrollments"`
	BudgetTotal      float64            `json:"budget_total"`
	BudgetReserved   float64            `json:"budget_reserved"`
	BudgetPaid       float64            `json:"budget_paid"`
	RewardMode       string             `json:"reward_mode"`
	RankRewards      map[string]float64 `json:"rank_rewards,omitempty"`
	CreatedBy        int64              `json:"created_by"`
	ApprovedBy       *int64             `json:"approved_by,omitempty"`
	SigningSecret    []byte             `json:"-"`
}

type ReferralCampaignTier struct {
	Tier            int     `json:"tier"`
	RequiredInvites int     `json:"required_invites"`
	RewardAmount    float64 `json:"reward_amount"`
	Currency        string  `json:"currency"`
}

type ReferralCampaignEnrollment struct {
	CampaignID int64     `json:"campaign_id"`
	UserID     int64     `json:"user_id"`
	EnrolledAt time.Time `json:"enrolled_at"`
}

type ReferralAttribution struct {
	ID           int64     `json:"id"`
	CampaignID   int64     `json:"campaign_id"`
	InviterID    int64     `json:"inviter_id"`
	InviteeID    int64     `json:"invitee_id"`
	Nonce        string    `json:"-"`
	RegisteredAt time.Time `json:"registered_at"`
	Status       string    `json:"status"`
}

type ReferralQualificationInput struct {
	NetPaid    float64
	ActualCost float64
	RiskStatus string
}

type ReferralQualification struct {
	ID            int64      `json:"id"`
	CampaignID    int64      `json:"campaign_id"`
	InviterID     int64      `json:"inviter_id"`
	InviteeID     int64      `json:"invitee_id"`
	NetPaid       float64    `json:"net_paid"`
	ActualCost    float64    `json:"actual_cost"`
	SourceOrderID *int64     `json:"source_order_id,omitempty"`
	RiskStatus    string     `json:"risk_status"`
	Status        string     `json:"status"`
	QualifiedAt   *time.Time `json:"qualified_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

type ReferralReward struct {
	ID            int64      `json:"id"`
	CampaignID    int64      `json:"campaign_id"`
	UserID        int64      `json:"user_id"`
	Tier          int        `json:"tier"`
	RewardType    string     `json:"reward_type"`
	Amount        float64    `json:"amount"`
	Currency      string     `json:"currency"`
	Status        string     `json:"status"`
	UnlockAt      *time.Time `json:"unlock_at,omitempty"`
	ClaimDeadline *time.Time `json:"claim_deadline,omitempty"`
	FrozenUntil   *time.Time `json:"frozen_until,omitempty"`
	Version       int64      `json:"version"`
}

type ReferralGrowthRanking struct {
	Rank           int     `json:"rank"`
	EmailMasked    string  `json:"email_masked"`
	QualifiedCount int64   `json:"qualified_count"`
	RewardAmount   float64 `json:"reward_amount"`
	IsMe           bool    `json:"is_me,omitempty"`
}

type ReferralGrowthOverview struct {
	InvitedCount     int64                   `json:"invited_count"`
	QualifiedCount   int64                   `json:"qualified_count"`
	PaidInviteeCount int64                   `json:"paid_invitee_count"`
	RewardUnlocked   float64                 `json:"reward_unlocked"`
	RewardClaimed    float64                 `json:"reward_claimed"`
	Ranking          []ReferralGrowthRanking `json:"ranking"`
}

type ReferralCampaignListFilter struct {
	Page     int
	PageSize int
	Status   string
	Search   string
}

type ReferralCampaignPage struct {
	Items    []ReferralCampaign `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type ReferralCampaignApproval struct {
	Version    int64     `json:"version"`
	ReviewType string    `json:"review_type"`
	Decision   string    `json:"decision"`
	ReviewerID *int64    `json:"reviewer_id,omitempty"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

type ReferralCampaignDetail struct {
	Campaign  ReferralCampaign           `json:"campaign"`
	Tiers     []ReferralCampaignTier     `json:"tiers"`
	Stats     ReferralCampaignStats      `json:"stats"`
	Approvals []ReferralCampaignApproval `json:"approvals"`
}

type ReferralCampaignParticipant struct {
	UserID         int64     `json:"user_id"`
	Email          string    `json:"email"`
	Username       string    `json:"username"`
	EnrolledAt     time.Time `json:"enrolled_at"`
	InvitedCount   int64     `json:"invited_count"`
	QualifiedCount int64     `json:"qualified_count"`
	RewardUnlocked float64   `json:"reward_unlocked"`
	RewardClaimed  float64   `json:"reward_claimed"`
}

type ReferralCampaignParticipantPage struct {
	Items    []ReferralCampaignParticipant `json:"items"`
	Total    int64                         `json:"total"`
	Page     int                           `json:"page"`
	PageSize int                           `json:"page_size"`
}

type ReferralCampaignInviteDetail struct {
	AttributionID int64      `json:"attribution_id"`
	InviterID     int64      `json:"inviter_id"`
	InviterEmail  string     `json:"inviter_email"`
	InviteeID     int64      `json:"invitee_id"`
	InviteeEmail  string     `json:"invitee_email"`
	RegisteredAt  time.Time  `json:"registered_at"`
	Status        string     `json:"status"`
	NetPaid       float64    `json:"net_paid"`
	ActualCost    float64    `json:"actual_cost"`
	RiskStatus    string     `json:"risk_status"`
	Qualification string     `json:"qualification_status"`
	QualifiedAt   *time.Time `json:"qualified_at,omitempty"`
}

type ReferralCampaignInvitePage struct {
	Items    []ReferralCampaignInviteDetail `json:"items"`
	Total    int64                          `json:"total"`
	Page     int                            `json:"page"`
	PageSize int                            `json:"page_size"`
}

type ReferralCampaignRewardPage struct {
	Items    []ReferralCampaignRewardDetail `json:"items"`
	Total    int64                          `json:"total"`
	Page     int                            `json:"page"`
	PageSize int                            `json:"page_size"`
}

type ReferralCampaignRewardDetail struct {
	ReferralReward
	Email    string `json:"email"`
	Username string `json:"username"`
}

type ReferralCampaignProgress struct {
	Campaign       ReferralCampaignPublic      `json:"campaign"`
	Enrollment     *ReferralCampaignEnrollment `json:"enrollment,omitempty"`
	Tiers          []ReferralCampaignTier      `json:"tiers"`
	InvitedCount   int64                       `json:"invited_count"`
	QualifiedCount int64                       `json:"qualified_count"`
	Rewards        []ReferralReward            `json:"rewards"`
	Ranking        *ReferralGrowthRanking      `json:"ranking,omitempty"`
	Leaderboard    []ReferralGrowthRanking     `json:"leaderboard"`
}

type ReferralCampaignPublic struct {
	ID               int64     `json:"id"`
	Key              string    `json:"key"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	Version          int64     `json:"version"`
	RegistrationFrom time.Time `json:"registration_from"`
	RegistrationTo   time.Time `json:"registration_to"`
	StartsAt         time.Time `json:"starts_at"`
	EndsAt           time.Time `json:"ends_at"`
	QualificationTo  time.Time `json:"qualification_to"`
	ClaimDeadline    time.Time `json:"claim_deadline"`
	RiskHoldHours    int       `json:"risk_hold_hours"`
	PayThreshold     float64   `json:"pay_threshold"`
	UsageThreshold   float64   `json:"usage_threshold"`
	RewardMode       string    `json:"reward_mode"`
}

type ReferralClaimInput struct {
	CampaignID      int64
	UserID          int64
	RewardID        int64
	CampaignVersion int64
}

type ReferralCampaignTokenPayload struct {
	CampaignID int64     `json:"campaign_id"`
	InviterID  int64     `json:"inviter_id"`
	Nonce      string    `json:"nonce"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type ReferralCampaignRepository interface {
	CreateReferralCampaign(context.Context, ReferralCampaign, []ReferralCampaignTier) (*ReferralCampaign, error)
	UpdateReferralCampaign(context.Context, ReferralCampaign, []ReferralCampaignTier, int64) (*ReferralCampaign, error)
	ListReferralCampaigns(context.Context, ReferralCampaignListFilter) (*ReferralCampaignPage, error)
	ListRunningReferralCampaignIDs(context.Context, int) ([]int64, error)
	GetReferralCampaign(context.Context, int64) (*ReferralCampaign, error)
	SetReferralCampaignStatus(context.Context, int64, int64, string, int64, string) (*ReferralCampaign, error)
	ReviewReferralCampaign(context.Context, int64, int64, string, string, int64, string) (*ReferralCampaign, error)
	GetReferralCampaignStats(context.Context, int64) (*ReferralCampaignStats, error)
	GetReferralCampaignTiers(context.Context, int64) ([]ReferralCampaignTier, error)
	ListReferralCampaignApprovals(context.Context, int64) ([]ReferralCampaignApproval, error)
	ListReferralCampaignParticipants(context.Context, int64, int, int, string) (*ReferralCampaignParticipantPage, error)
	ListReferralCampaignInvites(context.Context, int64, int, int, string, string) (*ReferralCampaignInvitePage, error)
	ListReferralCampaignRewards(context.Context, int64, int, int, string, string) (*ReferralCampaignRewardPage, error)
	GetReferralCampaignProgress(context.Context, int64, int64) (*ReferralCampaignProgress, error)
	EnrollReferralCampaign(context.Context, int64, int64) (*ReferralCampaignEnrollment, error)
	GetReferralCampaignEnrollment(context.Context, int64, int64) (*ReferralCampaignEnrollment, error)
	GetReferralCampaignSecret(context.Context, int64) ([]byte, error)
	BindReferralAttribution(context.Context, ReferralAttribution) (*ReferralAttribution, error)
	GetReferralAttribution(context.Context, int64, int64) (*ReferralAttribution, error)
	ListReferralAttributionCampaignIDs(context.Context, int64) ([]int64, error)
	ListReferralCampaignInviteeIDs(context.Context, int64, int64, int) ([]int64, error)
	RecomputeReferralQualification(context.Context, int64, int64, time.Time) (*ReferralQualification, error)
	UpdateReferralQualification(context.Context, ReferralQualification) (*ReferralQualification, error)
	ClaimReferralReward(context.Context, ReferralClaimInput) (*ReferralReward, error)
	ResolveReferralRewardDebt(context.Context, int64, int64, int64, string, int64, string) (*ReferralReward, error)
	ReverseReferralRewardsByOrder(context.Context, int64) error
	EnqueueReferralRefundReconcile(context.Context, int64) error
	ProcessReferralReconcileQueue(context.Context, int) (int, error)
	GetReferralGrowthOverview(context.Context, *int64, *int64) (*ReferralGrowthOverview, error)
}

type ReferralCampaignStats struct {
	CampaignID      int64   `json:"campaign_id"`
	Enrolled        int64   `json:"enrolled"`
	Attributed      int64   `json:"attributed"`
	Qualified       int64   `json:"qualified"`
	RiskPending     int64   `json:"risk_pending"`
	RiskRejected    int64   `json:"risk_rejected"`
	RewardsReserved float64 `json:"rewards_reserved"`
	RewardsClaimed  float64 `json:"rewards_claimed"`
	RewardsExpired  float64 `json:"rewards_expired"`
	RewardsRevoked  float64 `json:"rewards_revoked"`
}

type ReferralCampaignService struct{ repo ReferralCampaignRepository }

func NewReferralCampaignService(repo ReferralCampaignRepository) *ReferralCampaignService {
	return &ReferralCampaignService{repo: repo}
}

func (s *ReferralCampaignService) CreateCampaign(ctx context.Context, campaign ReferralCampaign, tiers []ReferralCampaignTier) (*ReferralCampaign, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	campaign.Status = ReferralCampaignStatusDraft
	campaign.Version = 1
	if err := validateReferralCampaignDraft(&campaign, tiers); err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	campaign.SigningSecret = secret
	return s.repo.CreateReferralCampaign(ctx, campaign, tiers)
}

func (s *ReferralCampaignService) UpdateCampaign(ctx context.Context, campaign ReferralCampaign, tiers []ReferralCampaignTier, actorID int64) (*ReferralCampaign, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	if campaign.ID <= 0 || campaign.Version <= 0 {
		return nil, ErrReferralCampaignVersionConflict
	}
	current, err := s.repo.GetReferralCampaign(ctx, campaign.ID)
	if err != nil || current == nil {
		return nil, ErrReferralCampaignNotFound
	}
	if current.Version != campaign.Version {
		return nil, ErrReferralCampaignVersionConflict
	}
	if current.Status != ReferralCampaignStatusDraft {
		return nil, ErrReferralCampaignImmutable
	}
	campaign.Status = ReferralCampaignStatusDraft
	campaign.BudgetReserved = current.BudgetReserved
	campaign.BudgetPaid = current.BudgetPaid
	if err := validateReferralCampaignDraft(&campaign, tiers); err != nil {
		return nil, err
	}
	return s.repo.UpdateReferralCampaign(ctx, campaign, tiers, actorID)
}

func validateReferralCampaignDraft(campaign *ReferralCampaign, tiers []ReferralCampaignTier) error {
	if err := ValidateReferralCampaign(campaign); err != nil {
		return err
	}
	if strings.TrimSpace(campaign.Key) == "" {
		return errors.New("campaign key is required")
	}
	if campaign.RewardMode != "additive" && campaign.RewardMode != "replace" {
		return errors.New("reward mode is invalid")
	}
	if len(tiers) == 0 {
		return errors.New("at least one reward tier is required")
	}
	seenTier := make(map[int]struct{}, len(tiers))
	seenInvites := make(map[int]struct{}, len(tiers))
	maxPerUserPayout := 0.0
	for _, tier := range tiers {
		if tier.Tier <= 0 || tier.RequiredInvites <= 0 || tier.RewardAmount <= 0 {
			return errors.New("reward tier values must be greater than zero")
		}
		if !strings.EqualFold(strings.TrimSpace(tier.Currency), "CNY") {
			return errors.New("reward tier currency must be CNY")
		}
		if _, exists := seenTier[tier.Tier]; exists {
			return errors.New("reward tier number must be unique")
		}
		if _, exists := seenInvites[tier.RequiredInvites]; exists {
			return errors.New("reward tier invite threshold must be unique")
		}
		seenTier[tier.Tier] = struct{}{}
		seenInvites[tier.RequiredInvites] = struct{}{}
		if campaign.RewardMode == "additive" {
			maxPerUserPayout += tier.RewardAmount
		}
	}
	if campaign.RewardMode == "replace" {
		ordered := append([]ReferralCampaignTier(nil), tiers...)
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].RequiredInvites < ordered[j].RequiredInvites })
		previous := 0.0
		for _, tier := range ordered {
			if tier.RewardAmount <= previous {
				return errors.New("replace-mode tier rewards must increase with invite thresholds")
			}
			previous = tier.RewardAmount
		}
		maxPerUserPayout = previous
	}
	maximumLiability := maxPerUserPayout * float64(campaign.MaxEnrollments)
	if campaign.BudgetTotal+1e-9 < maximumLiability {
		return fmt.Errorf("campaign budget %.2f cannot cover maximum liability %.2f for %d enrollments", campaign.BudgetTotal, maximumLiability, campaign.MaxEnrollments)
	}
	if len(campaign.RankRewards) > 0 {
		return errors.New("rank rewards require a separate approved settlement configuration")
	}
	return nil
}

func ValidateReferralCampaign(c *ReferralCampaign) error {
	if c == nil {
		return errors.New("campaign is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("campaign name is required")
	}
	if c.RegistrationFrom.IsZero() || c.RegistrationTo.IsZero() || !c.RegistrationTo.After(c.RegistrationFrom) {
		return errors.New("registration window is invalid")
	}
	if c.StartsAt.IsZero() || c.EndsAt.IsZero() || !c.EndsAt.After(c.StartsAt) {
		return errors.New("campaign window is invalid")
	}
	if c.QualificationTo.IsZero() || c.QualificationTo.Before(c.EndsAt) {
		return errors.New("qualification deadline is invalid")
	}
	if c.ClaimDeadline.IsZero() || !c.ClaimDeadline.After(c.EndsAt) {
		return errors.New("claim deadline is invalid")
	}
	if c.PayThreshold < 0 || c.UsageThreshold < 0 {
		return errors.New("qualification thresholds must be non-negative")
	}
	if c.BudgetTotal <= 0 {
		return errors.New("campaign budget must be greater than zero")
	}
	if c.MaxEnrollments <= 0 {
		return errors.New("max enrollments must be greater than zero")
	}
	if c.RiskHoldHours < 24*7 || c.RiskHoldHours > 24*30 {
		return errors.New("risk hold hours must be between 168 and 720 hours")
	}
	if c.Status == ReferralCampaignStatusRunning && c.Version <= 0 {
		return errors.New("running campaign requires a version")
	}
	return nil
}

func (s *ReferralCampaignService) ListCampaigns(ctx context.Context, filter ReferralCampaignListFilter) (*ReferralCampaignPage, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	return s.repo.ListReferralCampaigns(ctx, filter)
}

func (s *ReferralCampaignService) ListRunningProgress(ctx context.Context, userID int64) ([]ReferralCampaignProgress, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	ids, err := s.repo.ListRunningReferralCampaignIDs(ctx, 20)
	if err != nil {
		return nil, err
	}
	out := make([]ReferralCampaignProgress, 0, len(ids))
	for _, id := range ids {
		progress, err := s.Progress(ctx, id, userID)
		if err != nil {
			return nil, err
		}
		out = append(out, *progress)
	}
	return out, nil
}

func (s *ReferralCampaignService) GetCampaignDetail(ctx context.Context, campaignID int64) (*ReferralCampaignDetail, error) {
	campaign, err := s.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	tiers, err := s.repo.GetReferralCampaignTiers(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	stats, err := s.repo.GetReferralCampaignStats(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	approvals, err := s.repo.ListReferralCampaignApprovals(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	return &ReferralCampaignDetail{Campaign: *campaign, Tiers: tiers, Stats: *stats, Approvals: approvals}, nil
}

func isReferralCampaignStatus(status string) bool {
	switch status {
	case ReferralCampaignStatusDraft, ReferralCampaignStatusReview, ReferralCampaignStatusApproved,
		ReferralCampaignStatusScheduled, ReferralCampaignStatusRunning, ReferralCampaignStatusPaused,
		ReferralCampaignStatusSettling, ReferralCampaignStatusClosed, ReferralCampaignStatusCancelled:
		return true
	default:
		return false
	}
}

func CanTransitionReferralCampaign(from, to string) bool {
	switch from {
	case ReferralCampaignStatusDraft:
		return to == ReferralCampaignStatusReview || to == ReferralCampaignStatusCancelled
	case ReferralCampaignStatusApproved:
		return to == ReferralCampaignStatusScheduled || to == ReferralCampaignStatusCancelled
	case ReferralCampaignStatusScheduled:
		return to == ReferralCampaignStatusRunning || to == ReferralCampaignStatusPaused || to == ReferralCampaignStatusCancelled
	case ReferralCampaignStatusRunning:
		return to == ReferralCampaignStatusPaused || to == ReferralCampaignStatusSettling
	case ReferralCampaignStatusPaused:
		return to == ReferralCampaignStatusRunning || to == ReferralCampaignStatusSettling || to == ReferralCampaignStatusCancelled
	case ReferralCampaignStatusSettling:
		return to == ReferralCampaignStatusClosed
	default:
		return false
	}
}

func ReferralQualificationMeetsThresholds(input ReferralQualificationInput, payThreshold, usageThreshold float64) bool {
	return input.NetPaid+1e-9 >= payThreshold && input.ActualCost+1e-9 >= usageThreshold && input.RiskStatus == ReferralRiskApproved
}

func SignReferralCampaignToken(secret []byte, payload ReferralCampaignTokenPayload) (string, error) {
	if len(secret) < 16 || payload.CampaignID <= 0 || payload.InviterID <= 0 || strings.TrimSpace(payload.Nonce) == "" || payload.ExpiresAt.IsZero() {
		return "", ErrReferralCampaignTokenInvalid
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(encoded))
	return "v1." + encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyReferralCampaignToken(secret []byte, token string, now time.Time) (ReferralCampaignTokenPayload, error) {
	var payload ReferralCampaignTokenPayload
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "v1" {
		return payload, ErrReferralCampaignTokenInvalid
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return payload, ErrReferralCampaignTokenInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(raw, &payload) != nil || payload.CampaignID <= 0 || payload.InviterID <= 0 || payload.Nonce == "" {
		return payload, ErrReferralCampaignTokenInvalid
	}
	if !now.Before(payload.ExpiresAt) {
		return payload, ErrReferralCampaignTokenExpired
	}
	return payload, nil
}

func (s *ReferralCampaignService) Enroll(ctx context.Context, campaignID, userID int64, now time.Time) (*ReferralCampaignEnrollment, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	c, err := s.repo.GetReferralCampaign(ctx, campaignID)
	if err != nil || c == nil {
		return nil, ErrReferralCampaignNotFound
	}
	if c.Status != ReferralCampaignStatusScheduled && c.Status != ReferralCampaignStatusRunning {
		return nil, ErrReferralCampaignNotOpen
	}
	if now.Before(c.RegistrationFrom) || !now.Before(c.RegistrationTo) {
		return nil, ErrReferralCampaignNotOpen
	}
	return s.repo.EnrollReferralCampaign(ctx, campaignID, userID)
}

func (s *ReferralCampaignService) GetCampaign(ctx context.Context, campaignID int64) (*ReferralCampaign, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	return s.repo.GetReferralCampaign(ctx, campaignID)
}

func (s *ReferralCampaignService) SetStatus(ctx context.Context, campaignID, expectedVersion int64, status string, actorID int64, note string) (*ReferralCampaign, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	if !isReferralCampaignStatus(status) || expectedVersion <= 0 {
		return nil, ErrReferralCampaignInvalidState
	}
	campaign, err := s.repo.GetReferralCampaign(ctx, campaignID)
	if err != nil || campaign == nil {
		return nil, ErrReferralCampaignNotFound
	}
	if campaign.Version != expectedVersion {
		return nil, ErrReferralCampaignVersionConflict
	}
	if !CanTransitionReferralCampaign(campaign.Status, status) {
		return nil, ErrReferralCampaignInvalidState
	}
	now := time.Now().UTC()
	if status == ReferralCampaignStatusRunning && (now.Before(campaign.StartsAt) || !now.Before(campaign.EndsAt)) {
		return nil, ErrReferralCampaignInvalidState
	}
	if status == ReferralCampaignStatusSettling && now.Before(campaign.EndsAt) {
		return nil, ErrReferralCampaignInvalidState
	}
	if status == ReferralCampaignStatusClosed && now.Before(campaign.ClaimDeadline) {
		return nil, ErrReferralCampaignInvalidState
	}
	if status == ReferralCampaignStatusCancelled && (campaign.BudgetReserved > 1e-9 || campaign.BudgetPaid > 1e-9) {
		return nil, ErrReferralCampaignInvalidState
	}
	return s.repo.SetReferralCampaignStatus(ctx, campaignID, expectedVersion, status, actorID, note)
}

func (s *ReferralCampaignService) Review(ctx context.Context, campaignID, expectedVersion int64, reviewType, decision string, actorID int64, note string) (*ReferralCampaign, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	switch reviewType {
	case "ops", "finance", "risk", "ux":
	default:
		return nil, errors.New("invalid referral campaign review type")
	}
	if decision != "approved" && decision != "rejected" {
		return nil, errors.New("invalid referral campaign review decision")
	}
	campaign, err := s.repo.GetReferralCampaign(ctx, campaignID)
	if err != nil || campaign == nil {
		return nil, ErrReferralCampaignNotFound
	}
	if campaign.Version != expectedVersion {
		return nil, ErrReferralCampaignVersionConflict
	}
	if campaign.Status != ReferralCampaignStatusReview {
		return nil, ErrReferralCampaignInvalidState
	}
	return s.repo.ReviewReferralCampaign(ctx, campaignID, expectedVersion, reviewType, decision, actorID, note)
}

func (s *ReferralCampaignService) Participants(ctx context.Context, campaignID int64, page, pageSize int, search string) (*ReferralCampaignParticipantPage, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	return s.repo.ListReferralCampaignParticipants(ctx, campaignID, page, pageSize, search)
}

func (s *ReferralCampaignService) Invites(ctx context.Context, campaignID int64, page, pageSize int, search, status string) (*ReferralCampaignInvitePage, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	return s.repo.ListReferralCampaignInvites(ctx, campaignID, page, pageSize, search, status)
}

func (s *ReferralCampaignService) Rewards(ctx context.Context, campaignID int64, page, pageSize int, search, status string) (*ReferralCampaignRewardPage, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	return s.repo.ListReferralCampaignRewards(ctx, campaignID, page, pageSize, search, status)
}

func (s *ReferralCampaignService) Stats(ctx context.Context, campaignID int64) (*ReferralCampaignStats, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	return s.repo.GetReferralCampaignStats(ctx, campaignID)
}

func (s *ReferralCampaignService) CreateInviteToken(ctx context.Context, campaignID, inviterID int64, now time.Time) (string, error) {
	if s == nil || s.repo == nil {
		return "", errors.New("referral campaign service unavailable")
	}
	if _, err := s.repo.GetReferralCampaignEnrollment(ctx, campaignID, inviterID); err != nil {
		return "", err
	}
	c, err := s.repo.GetReferralCampaign(ctx, campaignID)
	if err != nil || c == nil {
		return "", ErrReferralCampaignNotFound
	}
	if (c.Status != ReferralCampaignStatusScheduled && c.Status != ReferralCampaignStatusRunning) || now.Before(c.RegistrationFrom) || !now.Before(c.RegistrationTo) {
		return "", ErrReferralCampaignNotOpen
	}
	secret, err := s.repo.GetReferralCampaignSecret(ctx, campaignID)
	if err != nil {
		return "", err
	}
	nonceBytes := make([]byte, 18)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", err
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	return SignReferralCampaignToken(secret, ReferralCampaignTokenPayload{CampaignID: campaignID, InviterID: inviterID, Nonce: nonce, ExpiresAt: now.Add(30 * 24 * time.Hour)})
}

func (s *ReferralCampaignService) Attribute(ctx context.Context, token string, inviteeID int64, now time.Time) (*ReferralAttribution, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	payload, err := s.resolveInviteToken(ctx, token, now)
	if err != nil {
		return nil, err
	}
	if payload.InviterID == inviteeID {
		return nil, ErrReferralCampaignTokenInvalid
	}
	return s.repo.BindReferralAttribution(ctx, ReferralAttribution{CampaignID: payload.CampaignID, InviterID: payload.InviterID, InviteeID: inviteeID, Nonce: payload.Nonce, RegisteredAt: now, Status: ReferralRiskPending})
}

// ValidateInviteToken verifies campaign state and inviter enrollment before a
// user account is created. Registration handlers use this preflight so an
// invalid or conflicting signed invitation cannot silently create an account
// with only the legacy affiliate attribution attached.
func (s *ReferralCampaignService) ValidateInviteToken(ctx context.Context, token string, now time.Time) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("referral campaign service unavailable")
	}
	payload, err := s.resolveInviteToken(ctx, token, now)
	if err != nil {
		return 0, err
	}
	return payload.InviterID, nil
}

func (s *ReferralCampaignService) resolveInviteToken(ctx context.Context, token string, now time.Time) (ReferralCampaignTokenPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ReferralCampaignTokenPayload{}, ErrReferralCampaignTokenInvalid
	}
	// The campaign id is authenticated by VerifyReferralCampaignToken; loading all
	// campaigns to discover it would create an oracle, so decode only the signed body.
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ReferralCampaignTokenPayload{}, ErrReferralCampaignTokenInvalid
	}
	var hint ReferralCampaignTokenPayload
	if json.Unmarshal(raw, &hint) != nil {
		return ReferralCampaignTokenPayload{}, ErrReferralCampaignTokenInvalid
	}
	secret, err := s.repo.GetReferralCampaignSecret(ctx, hint.CampaignID)
	if err != nil {
		return ReferralCampaignTokenPayload{}, err
	}
	payload, err := VerifyReferralCampaignToken(secret, token, now)
	if err != nil {
		return ReferralCampaignTokenPayload{}, err
	}
	campaign, err := s.repo.GetReferralCampaign(ctx, payload.CampaignID)
	if err != nil || campaign == nil {
		return ReferralCampaignTokenPayload{}, ErrReferralCampaignNotFound
	}
	if (campaign.Status != ReferralCampaignStatusScheduled && campaign.Status != ReferralCampaignStatusRunning) || now.Before(campaign.RegistrationFrom) || !now.Before(campaign.RegistrationTo) {
		return ReferralCampaignTokenPayload{}, ErrReferralCampaignNotOpen
	}
	if _, err := s.repo.GetReferralCampaignEnrollment(ctx, payload.CampaignID, payload.InviterID); err != nil {
		return ReferralCampaignTokenPayload{}, ErrReferralCampaignNotOpen
	}
	return payload, nil
}

func (s *ReferralCampaignService) RefreshQualification(ctx context.Context, qualification ReferralQualification) (*ReferralQualification, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	c, err := s.repo.GetReferralCampaign(ctx, qualification.CampaignID)
	if err != nil || c == nil {
		return nil, ErrReferralCampaignNotFound
	}
	input := ReferralQualificationInput{NetPaid: qualification.NetPaid, ActualCost: qualification.ActualCost, RiskStatus: qualification.RiskStatus}
	if ReferralQualificationMeetsThresholds(input, c.PayThreshold, c.UsageThreshold) {
		qualification.Status = ReferralQualificationQualified
		now := time.Now().UTC()
		qualification.QualifiedAt = &now
	} else if qualification.RiskStatus == ReferralRiskRejected && qualification.Status == ReferralQualificationQualified {
		qualification.Status = ReferralQualificationRevoked
		now := time.Now().UTC()
		qualification.RevokedAt = &now
	} else if qualification.Status == "" || qualification.Status == ReferralQualificationQualified {
		qualification.Status = ReferralQualificationPending
	}
	return s.repo.UpdateReferralQualification(ctx, qualification)
}

func (s *ReferralCampaignService) RecomputeQualification(ctx context.Context, campaignID, inviteeID int64, now time.Time) (*ReferralQualification, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	campaign, err := s.repo.GetReferralCampaign(ctx, campaignID)
	if err != nil || campaign == nil {
		return nil, ErrReferralCampaignNotFound
	}
	if (campaign.Status != ReferralCampaignStatusScheduled && campaign.Status != ReferralCampaignStatusRunning && campaign.Status != ReferralCampaignStatusSettling) || now.After(campaign.QualificationTo) {
		return nil, ErrReferralCampaignNotOpen
	}
	q, err := s.repo.RecomputeReferralQualification(ctx, campaignID, inviteeID, now)
	if err != nil {
		return nil, err
	}
	return s.RefreshQualification(ctx, *q)
}

func (s *ReferralCampaignService) RecomputeForInvitee(ctx context.Context, inviteeID int64, now time.Time) error {
	if s == nil || s.repo == nil {
		return nil
	}
	ids, err := s.repo.ListReferralAttributionCampaignIDs(ctx, inviteeID)
	if err != nil {
		return err
	}
	for _, campaignID := range ids {
		if _, err := s.RecomputeQualification(ctx, campaignID, inviteeID, now); err != nil && !errors.Is(err, ErrReferralCampaignNotOpen) {
			return err
		}
	}
	return nil
}

func (s *ReferralCampaignService) ClaimReward(ctx context.Context, input ReferralClaimInput) (*ReferralReward, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	c, err := s.repo.GetReferralCampaign(ctx, input.CampaignID)
	if err != nil || c == nil {
		return nil, ErrReferralCampaignNotFound
	}
	if c.Version != input.CampaignVersion {
		return nil, ErrReferralCampaignVersionConflict
	}
	reward, err := s.repo.ClaimReferralReward(ctx, input)
	if err != nil {
		return nil, err
	}
	if reward == nil || reward.Status != ReferralRewardStatusClaimedFrozen {
		return nil, ErrReferralRewardNotClaimable
	}
	return reward, nil
}

func (s *ReferralCampaignService) ResolveRewardDebt(ctx context.Context, campaignID, rewardID, expectedVersion int64, decision string, actorID int64, note string) (*ReferralReward, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	if decision != "recovered" && decision != "waived" {
		return nil, errors.New("invalid referral reward debt decision")
	}
	if len([]rune(strings.TrimSpace(note))) < 10 {
		return nil, errors.New("referral reward debt resolution note must contain at least 10 characters")
	}
	return s.repo.ResolveReferralRewardDebt(ctx, campaignID, rewardID, expectedVersion, decision, actorID, note)
}

func (s *ReferralCampaignService) Overview(ctx context.Context, campaignID, currentUserID *int64) (*ReferralGrowthOverview, error) {
	if s == nil || s.repo == nil {
		return &ReferralGrowthOverview{Ranking: []ReferralGrowthRanking{}}, nil
	}
	return s.repo.GetReferralGrowthOverview(ctx, campaignID, currentUserID)
}

func (s *ReferralCampaignService) Progress(ctx context.Context, campaignID, userID int64) (*ReferralCampaignProgress, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("referral campaign service unavailable")
	}
	inviteeIDs, err := s.repo.ListReferralCampaignInviteeIDs(ctx, campaignID, userID, 200)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, inviteeID := range inviteeIDs {
		if _, err := s.RecomputeQualification(ctx, campaignID, inviteeID, now); err != nil && !errors.Is(err, ErrReferralCampaignNotOpen) {
			return nil, err
		}
	}
	return s.repo.GetReferralCampaignProgress(ctx, campaignID, userID)
}
