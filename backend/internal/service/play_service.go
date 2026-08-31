package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/shopspring/decimal"
)

type PlayService struct {
	repo               PlayRepository
	userRepo           UserRepository
	channelService     *ChannelService
	settingService     *SettingService
	affiliateService   *AffiliateService
	entClient          *dbent.Client
	balanceLedger      *BalanceLedgerService
	mobilePush         *MobilePushService
	couponRewardIssuer CouponRewardIssuer
	redeemRewardIssuer RedeemCodeRewardIssuer
	rewardDrawSource   func(max int64) (int64, error)
	blindboxDrawSource func(max int64) (int64, error)
	// requireGrowthQualification is enabled by production wiring after the
	// qualification migrations are part of the deployed schema. Keeping the
	// default false preserves focused legacy test doubles without allowing a
	// real production service to silently fall back to redeemable rewards.
	requireGrowthQualification bool
	// requireGrowthGovernance is enabled by production wiring. It keeps the
	// legacy feature toggles from re-opening cash-equivalent rewards before an
	// operations approval, budget, and rollout record exists.
	requireGrowthGovernance bool
	teamAdmissionRisk       PlayTeamAdmissionRiskHook
	vipObserver             playVIPChangeObserver
	now                     func() time.Time
}

type PlayMembershipAdminOverview struct {
	TotalMembers     int                       `json:"total_members"`
	NetPaid          decimal.Decimal           `json:"net_paid_amount"`
	RecentUpgrades   int                       `json:"recent_upgrades"`
	RecentDowngrades int                       `json:"recent_downgrades"`
	TierCounts       []PlayMembershipTierCount `json:"tier_counts"`
}

type PlayMembershipTierCount struct {
	Tier  int    `json:"tier"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type PlayMembershipAdminUser struct {
	UserID       int64           `json:"user_id"`
	EmailMasked  string          `json:"email_masked"`
	Username     string          `json:"username,omitempty"`
	Tier         int             `json:"tier"`
	TierLabel    string          `json:"tier_label"`
	IsMember     bool            `json:"is_member"`
	NetPaid      decimal.Decimal `json:"net_paid_amount"`
	RegisteredAt time.Time       `json:"registered_at"`
	FirstPaidAt  *time.Time      `json:"first_paid_at,omitempty"`
	LastPaidAt   *time.Time      `json:"last_paid_at,omitempty"`
}

func (s *PlayService) MembershipAdminOverview(ctx context.Context) (*PlayMembershipAdminOverview, error) {
	repo, ok := s.repo.(PlayMembershipAdminRepository)
	if !ok {
		return &PlayMembershipAdminOverview{}, nil
	}
	threshold := firstMemberThreshold(s.GetRuntime(ctx).VIPTiers)
	total, amount, err := repo.MembershipAdminOverview(ctx, threshold)
	if err != nil {
		return nil, err
	}
	totals, err := repo.ListMembershipPaidTotals(ctx)
	if err != nil {
		return nil, err
	}
	tiers := s.GetRuntime(ctx).VIPTiers
	counts := make(map[int]int, len(tiers))
	for _, paid := range totals {
		counts[resolveVIPStatus(paid.InexactFloat64(), tiers).Tier]++
	}
	tierCounts := make([]PlayMembershipTierCount, 0, len(tiers))
	for _, tier := range tiers {
		tierCounts = append(tierCounts, PlayMembershipTierCount{Tier: tier.Tier, Label: tier.Label, Count: counts[tier.Tier]})
	}
	upgrades, downgrades, err := repo.CountRecentMembershipTierChanges(ctx, s.serverNow().Add(-30*24*time.Hour))
	if err != nil {
		return nil, err
	}
	return &PlayMembershipAdminOverview{TotalMembers: total, NetPaid: amount, RecentUpgrades: upgrades, RecentDowngrades: downgrades, TierCounts: tierCounts}, nil
}

func (s *PlayService) ListMembershipAdminUsers(ctx context.Context, query string, memberOnly *bool, page, pageSize int) ([]PlayMembershipAdminUser, int, error) {
	repo, ok := s.repo.(PlayMembershipAdminRepository)
	if !ok {
		return []PlayMembershipAdminUser{}, 0, nil
	}
	tiers := s.GetRuntime(ctx).VIPTiers
	threshold := firstMemberThreshold(tiers)
	rows, total, err := repo.ListMembershipAdminRows(ctx, query, memberOnly, threshold, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	out := make([]PlayMembershipAdminUser, 0, len(rows))
	for _, row := range rows {
		status := resolveVIPStatus(row.NetPaid.InexactFloat64(), tiers)
		out = append(out, PlayMembershipAdminUser{UserID: row.UserID, EmailMasked: maskMembershipEmail(row.Email), Username: row.Username, Tier: status.Tier, TierLabel: status.Label, IsMember: row.NetPaid.InexactFloat64()+1e-9 >= threshold, NetPaid: row.NetPaid, RegisteredAt: row.RegisteredAt, FirstPaidAt: row.FirstPaidAt, LastPaidAt: row.LastPaidAt})
	}
	return out, total, nil
}

type PlayMembershipAdminDetail struct {
	User          PlayMembershipAdminUser      `json:"user"`
	Contributions []PlayMembershipContribution `json:"contributions"`
	TierHistory   []PlayMembershipTierChange   `json:"tier_history"`
}

func (s *PlayService) GetMembershipAdminUser(ctx context.Context, userID int64) (*PlayMembershipAdminDetail, error) {
	repo, ok := s.repo.(PlayMembershipAdminRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("PLAY_MEMBERSHIP_UNAVAILABLE", "membership service unavailable")
	}
	row, err := repo.GetMembershipAdminRow(ctx, userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, infraerrors.NotFound("PLAY_MEMBERSHIP_USER_NOT_FOUND", "membership user not found")
	}
	tiers := s.GetRuntime(ctx).VIPTiers
	status := resolveVIPStatus(row.NetPaid.InexactFloat64(), tiers)
	threshold := firstMemberThreshold(tiers)
	user := PlayMembershipAdminUser{UserID: row.UserID, EmailMasked: maskMembershipEmail(row.Email), Username: row.Username, Tier: status.Tier, TierLabel: status.Label, IsMember: row.NetPaid.InexactFloat64()+1e-9 >= threshold, NetPaid: row.NetPaid, RegisteredAt: row.RegisteredAt, FirstPaidAt: row.FirstPaidAt, LastPaidAt: row.LastPaidAt}
	contributions, err := repo.ListMembershipContributions(ctx, userID, 50)
	if err != nil {
		return nil, err
	}
	history, err := repo.ListMembershipTierHistory(ctx, userID, 50)
	if err != nil {
		return nil, err
	}
	return &PlayMembershipAdminDetail{User: user, Contributions: contributions, TierHistory: history}, nil
}

var ErrPlayVIPConfigConflict = infraerrors.Conflict("PLAY_VIP_CONFIG_CONFLICT", "VIP configuration changed; preview again")

type PlayVIPConfigImpact struct {
	Version         int64                      `json:"version"`
	Tiers           []PlayVIPTier              `json:"tiers"`
	AffectedUsers   int                        `json:"affected_users"`
	UpgradedUsers   int                        `json:"upgraded_users"`
	DowngradedUsers int                        `json:"downgraded_users"`
	Changes         []PlayMembershipTierChange `json:"-"`
}

func (s *PlayService) PreviewVIPConfig(ctx context.Context, requested []PlayVIPTier) (*PlayVIPConfigImpact, error) {
	tiers, err := validateAdminVIPTiers(requested)
	if err != nil {
		var validationErr *vipConfigValidationError
		if errors.As(err, &validationErr) {
			return nil, infraerrors.BadRequest(validationErr.reason, validationErr.message)
		}
		return nil, infraerrors.BadRequest("PLAY_VIP_CONFIG_INVALID", err.Error())
	}
	repo, ok := s.repo.(PlayMembershipAdminRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("PLAY_MEMBERSHIP_UNAVAILABLE", "membership service unavailable")
	}
	version, err := repo.GetVIPConfigVersion(ctx)
	if err != nil {
		return nil, err
	}
	totals, err := repo.ListMembershipPaidTotals(ctx)
	if err != nil {
		return nil, err
	}
	current := s.GetRuntime(ctx).VIPTiers
	impact := &PlayVIPConfigImpact{Version: version, Tiers: tiers}
	for userID, paid := range totals {
		before := resolveVIPStatus(paid.InexactFloat64(), current).Tier
		after := resolveVIPStatus(paid.InexactFloat64(), tiers).Tier
		if before == after {
			continue
		}
		impact.AffectedUsers++
		impact.Changes = append(impact.Changes, PlayMembershipTierChange{UserID: userID, FromTier: before, ToTier: after, NetPaidBefore: paid, NetPaidAfter: paid, Reason: "tier_config"})
		if after > before {
			impact.UpgradedUsers++
		} else {
			impact.DowngradedUsers++
		}
	}
	return impact, nil
}

func (s *PlayService) PublishVIPConfig(ctx context.Context, requested []PlayVIPTier, expectedVersion, actorID int64, reason string) (*PlayVIPConfigImpact, error) {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) < 10 || len([]rune(reason)) > 500 {
		return nil, infraerrors.BadRequest("PLAY_VIP_CONFIG_REASON_INVALID", "VIP configuration reason must contain 10 to 500 characters")
	}
	impact, err := s.PreviewVIPConfig(ctx, requested)
	if err != nil {
		return nil, err
	}
	if impact.Version != expectedVersion {
		return nil, ErrPlayVIPConfigConflict
	}
	raw, err := json.Marshal(impact.Tiers)
	if err != nil {
		return nil, err
	}
	repo, ok := s.repo.(PlayMembershipAdminRepository)
	if !ok {
		return nil, errors.New("membership administration is unavailable")
	}
	version, err := repo.PublishVIPConfig(ctx, expectedVersion, actorID, reason, string(raw), impact.AffectedUsers, impact.UpgradedUsers, impact.DowngradedUsers, impact.Changes)
	if err != nil {
		return nil, err
	}
	impact.Version = version
	if len(impact.Changes) > 0 && s.vipObserver != nil {
		for _, change := range impact.Changes {
			s.notifyVIPChanged(ctx, change.UserID, resolveVIPStatus(change.NetPaidAfter.InexactFloat64(), impact.Tiers))
		}
	}

	return impact, nil
}

func maskMembershipEmail(value string) string {
	value = strings.TrimSpace(value)
	parts := strings.SplitN(value, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}
	local := parts[0]
	if len(local) == 1 {
		local += "*"
	} else if len(local) == 2 {
		local = local[:1] + "*"
	} else {
		local = local[:1] + "***" + local[len(local)-1:]
	}
	return local + "@" + parts[1]
}

type PlayAppAnalytics struct {
	Scans              int64            `json:"scans"`
	DownloadRedirects  int64            `json:"download_redirects"`
	FirstLaunches      int64            `json:"first_launches"`
	Installs           int64            `json:"installs"`
	RegisteredInstalls int64            `json:"registered_installs"`
	ActiveUsers        int64            `json:"active_users"`
	DAU                int64            `json:"dau"`
	WAU                int64            `json:"wau"`
	MAU                int64            `json:"mau"`
	Funnel             []map[string]any `json:"funnel"`
	Versions           []map[string]any `json:"versions"`
}

func (s *PlayService) AppAnalytics(ctx context.Context, from, to time.Time, version, channel string) (*PlayAppAnalytics, error) {
	repo, ok := s.repo.(PlayAppAnalyticsRepository)
	if !ok {
		return &PlayAppAnalytics{Funnel: []map[string]any{}, Versions: []map[string]any{}}, nil
	}
	scans, downloads, first, registered, active, dau, wau, mau, funnel, versions, err := repo.AppAnalytics(ctx, from, to, version, channel)
	if err != nil {
		return nil, err
	}
	if funnel == nil {
		funnel = []map[string]any{}
	}
	if versions == nil {
		versions = []map[string]any{}
	}
	return &PlayAppAnalytics{Scans: scans, DownloadRedirects: downloads, FirstLaunches: first, Installs: first, RegisteredInstalls: registered, ActiveUsers: active, DAU: dau, WAU: wau, MAU: mau, Funnel: funnel, Versions: versions}, nil
}

func (s *PlayService) SetMobilePushService(push *MobilePushService) {
	if s != nil {
		s.mobilePush = push
	}
}

// SetCouponRewardIssuer connects the optional coupon domain to game reward
// flows. The game service chooses the configured outer reward branch; the
// issuer only selects and grants a coupon within that branch's transaction.
func (s *PlayService) SetCouponRewardIssuer(issuer CouponRewardIssuer) {
	if s != nil {
		s.couponRewardIssuer = issuer
	}
}

func (s *PlayService) SetRedeemCodeRewardIssuer(issuer RedeemCodeRewardIssuer) {
	if s != nil {
		s.redeemRewardIssuer = issuer
	}
}

// SetTeamAdmissionRiskHook installs the optional admission policy used before
// team creation, invite joins, applications, and approvals.
func (s *PlayService) SetTeamAdmissionRiskHook(hook PlayTeamAdmissionRiskHook) {
	if s != nil && hook != nil {
		s.teamAdmissionRisk = hook
	}
}

func NewPlayService(
	repo PlayRepository,
	userRepo UserRepository,
	channelService *ChannelService,
	settingService *SettingService,
	affiliateService *AffiliateService,
	entClient *dbent.Client,
	balanceLedger ...*BalanceLedgerService,
) *PlayService {
	var ledger *BalanceLedgerService
	if len(balanceLedger) > 0 {
		ledger = balanceLedger[0]
	}
	svc := &PlayService{
		repo:               repo,
		userRepo:           userRepo,
		channelService:     channelService,
		settingService:     settingService,
		affiliateService:   affiliateService,
		entClient:          entClient,
		balanceLedger:      ledger,
		rewardDrawSource:   cryptoBlindboxDrawSource,
		blindboxDrawSource: cryptoBlindboxDrawSource,
	}
	if riskRepo, ok := repo.(PlayTeamAdmissionRiskRepository); ok {
		svc.teamAdmissionRisk = defaultPlayTeamAdmissionRisk{repo: riskRepo}
	}
	return svc
}

// RequireGrowthQualification makes missing qualification storage fail closed.
// It is set by the production dependency graph, after the repository and
// forward-only migrations are wired together.
func (s *PlayService) RequireGrowthQualification(required bool) {
	if s != nil {
		s.requireGrowthQualification = required
	}
}

func (s *PlayService) GetRuntime(ctx context.Context) PlayRuntime {
	if s.settingService == nil {
		return PlayRuntime{}
	}
	return s.settingService.GetPlayRuntime(ctx)
}

func (s *PlayService) serverNow() time.Time {
	if s.now != nil {
		return s.now()
	}
	return timezone.Now()
}

func (s *PlayService) serverDate(now time.Time) time.Time {
	return timezone.StartOfDay(now)
}

func (s *PlayService) GetCheckinStatus(ctx context.Context, userID int64) (*PlayCheckinStatus, error) {
	rt := s.GetRuntime(ctx)
	status := &PlayCheckinStatus{
		Enabled:                   rt.CheckinEnabled,
		Eligible:                  true,
		RewardAmount:              rt.CheckinReward,
		GrowthGovernanceAvailable: true,
		CouponPoolReady:           true,
		CouponWeightBP:            8000,
		RedeemCodeWeightBP:        2000,
		BalanceWeightBP:           0,
	}
	now := s.serverNow()
	status.ServerDate = s.serverDate(now).Format("2006-01-02")
	if !rt.CheckinEnabled || userID <= 0 {
		return status, nil
	}
	eligibility, err := s.growthEligibility(ctx, userID, s.serverNow())
	if err != nil {
		return nil, err
	}
	status.GrowthEligibility = eligibility
	status.Eligible = true
	status.GrowthEnergyEnabled = eligibility.RewardMode == PlayGrowthRewardEnergy
	status.RedeemableRewardEligible = eligibility.RewardMode == PlayGrowthRewardRedeemable
	if eligibility.RewardMode == PlayGrowthRewardRedeemable {
		_, governanceAvailable, governanceReason := s.growthGovernanceForStatus(ctx, userID, now)
		status.GrowthGovernanceAvailable = governanceAvailable
		status.GrowthGovernanceReason = governanceReason
		status.RedeemableRewardEligible = governanceAvailable
		if !governanceAvailable {
			status.IneligibleReason = governanceReason
			status.CouponPoolReady = false
		}
	}
	if status.GrowthEnergyEnabled {
		status.IneligibleReason = eligibility.PrimaryReason
	}
	if status.GrowthGovernanceAvailable {
		ready, err := s.couponRewardPoolReady(ctx, CouponRewardActivityCheckin)
		if err != nil {
			return nil, err
		}
		status.CouponPoolReady = ready
		if ready {
			couponWeightBP, redeemCodeWeightBP, balanceWeightBP, err := s.couponRewardSplit(ctx, CouponRewardActivityCheckin)
			if err != nil {
				return nil, err
			}
			status.CouponWeightBP = couponWeightBP
			status.RedeemCodeWeightBP = redeemCodeWeightBP
			status.BalanceWeightBP = balanceWeightBP
		}
	}
	done, err := s.repo.HasCheckin(ctx, userID, s.serverDate(now))
	if err != nil {
		return nil, err
	}
	status.CheckedInToday = done
	if err := s.enrichCheckinStatus(ctx, userID, status, rt); err != nil {
		return nil, err
	}
	return status, nil
}

func (s *PlayService) Checkin(ctx context.Context, userID int64) (*PlayCheckinResult, error) {
	rt := s.GetRuntime(ctx)
	if !rt.CheckinEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	now := s.serverNow()
	growthEligibility, err := s.growthEligibility(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	var growthGovernance *PlayGrowthGovernanceState
	if growthEligibility.RewardMode == PlayGrowthRewardRedeemable {
		growthGovernance, err = s.requireGrowthGovernanceForReward(ctx, userID, now)
		if err != nil {
			return nil, err
		}
		if err := s.requireCouponRewardPool(ctx, CouponRewardActivityCheckin); err != nil {
			return nil, err
		}
	}
	if userID <= 0 {
		return nil, ErrPlayCheckinIneligible
	}

	date := s.serverDate(now)
	dateKey := date.Format("2006-01-02")
	idempotencyKey := fmt.Sprintf("checkin:%d:%s", userID, dateKey)
	if growthEligibility.RewardMode == PlayGrowthRewardEnergy {
		var snapshotID int64
		err := s.withPlayTx(ctx, func(txCtx context.Context) error {
			if err := s.repo.InsertCheckin(txCtx, userID, date, 0, 0); err != nil {
				return err
			}
			var snapshotErr error
			snapshotID, snapshotErr = s.createGrowthSnapshot(txCtx, PlayGrowthEligibilitySnapshot{UserID: userID, Source: PlayRewardSourceCheckin, ActionID: idempotencyKey, ActivityDate: date, Eligibility: growthEligibility})
			if snapshotErr != nil {
				return snapshotErr
			}
			if snapshotID > 0 {
				repo, ok := s.growthQualificationRepository()
				if !ok {
					return fmt.Errorf("growth qualification repository disappeared during check-in")
				}
				if err := repo.LinkGrowthEligibilitySnapshot(txCtx, PlayRewardSourceCheckin, userID, date, snapshotID); err != nil {
					return err
				}
				if err := s.insertGrowthEnergy(txCtx, PlayGrowthEnergyLedgerEntry{UserID: userID, Source: PlayRewardSourceCheckin, ActionID: idempotencyKey, Amount: 1, EligibilitySnapshotID: snapshotID}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, ErrPlayCheckinAlreadyDone) || errors.Is(err, ErrPlayRewardDuplicate) {
				return nil, ErrPlayCheckinAlreadyDone
			}
			return nil, err
		}
		_ = s.MarkQuestCompleted(ctx, userID, PlayQuestKeyCheckin)
		return &PlayCheckinResult{RewardType: PlayRewardTypeNone, ServerDate: dateKey, GrowthEnergy: 1, GrowthEligibility: growthEligibility}, nil
	}

	streak, err := s.computeNextStreak(ctx, userID, date)
	if err != nil {
		return nil, err
	}
	boost, err := s.getRechargeBoostStatus(ctx, userID, rt)
	if err != nil {
		return nil, err
	}
	rewardType, err := s.drawCouponRewardType(ctx, CouponRewardActivityCheckin)
	if err != nil {
		return nil, err
	}
	reward := rt.CheckinReward
	var balanceEntry *BalanceRewardPoolEntry
	if rewardType == PlayRewardTypeBalance {
		balanceEntry, err = s.drawBalanceRewardEntry(ctx, CouponRewardActivityCheckin)
		if err != nil {
			return nil, err
		}
		reward = balanceEntry.Amount
	}
	if boost.Active && boost.CheckinMultiplier > 1 {
		reward *= boost.CheckinMultiplier
	}
	milestoneBonus := s.resolveStreakMilestoneBonus(streak, rt.StreakMilestones)
	totalReward := reward + milestoneBonus
	if rewardType != PlayRewardTypeBalance {
		totalReward = 0
	}

	var couponIssue *CouponRewardIssueResult
	var redeemCode *RedeemCode
	var growthSnapshotID int64
	if err := s.withPlayTx(ctx, func(txCtx context.Context) error {
		// Claim the once-per-day activity before any externally valuable reward
		// is issued. This unique insert is the concurrency boundary: a second
		// request loses before it can draw a coupon or reserve a redeem code.
		if err := s.repo.InsertCheckin(txCtx, userID, date, totalReward, streak); err != nil {
			return err
		}
		var snapshotErr error
		growthSnapshotID, snapshotErr = s.createGrowthSnapshot(txCtx, PlayGrowthEligibilitySnapshot{
			UserID: userID, Source: PlayRewardSourceCheckin, ActionID: idempotencyKey, ActivityDate: date, Eligibility: growthEligibility,
		})
		if snapshotErr != nil {
			return snapshotErr
		}
		if growthSnapshotID > 0 {
			growthRepo, ok := s.growthQualificationRepository()
			if !ok {
				return ErrPlayGrowthQualificationUnavailable
			}
			if err := growthRepo.LinkGrowthEligibilitySnapshot(txCtx, PlayRewardSourceCheckin, userID, date, growthSnapshotID); err != nil {
				return err
			}
		}
		if growthGovernance != nil {
			if err := s.reserveGrowthRewardBudget(txCtx, growthGovernance, userID, PlayRewardSourceCheckin, idempotencyKey, growthRewardBudgetCost(rewardType, totalReward)); err != nil {
				return err
			}
		}
		switch rewardType {
		case PlayRewardTypeCoupon:
			var issueErr error
			couponIssue, issueErr = s.issueCouponRewardInTx(txCtx, userID, CouponRewardActivityCheckin, idempotencyKey, dateKey, now, growthEligibility)
			if issueErr != nil {
				return issueErr
			}
		case PlayRewardTypeRedeem:
			var issueErr error
			redeemCode, issueErr = s.issueRedeemCodeRewardInTx(txCtx, userID, CouponRewardActivityCheckin, idempotencyKey, dateKey, now, growthEligibility)
			if issueErr != nil {
				return issueErr
			}
		}
		if rewardType == PlayRewardTypeBalance {
			detail := map[string]any{
				"checkin_date":    dateKey,
				"streak_count":    streak,
				"milestone_bonus": milestoneBonus,
				"boost_active":    boost.Active,
				"reward_type":     string(rewardType),
				"balance_entry":   balanceEntry,
			}
			if growthSnapshotID > 0 {
				detail["growth_eligibility_snapshot_id"] = growthSnapshotID
				detail["growth_rule_version"] = "v1"
				detail["growth_tier"] = growthEligibility.Tier
			}
			return s.grantBalanceLedgerOnlyInTx(txCtx, userID, totalReward, PlayRewardSourceCheckin, idempotencyKey, detail, growthSnapshotID)
		}
		return nil
	}); err != nil {
		if errors.Is(err, ErrPlayCheckinAlreadyDone) || errors.Is(err, ErrPlayRewardDuplicate) {
			return nil, ErrPlayCheckinAlreadyDone
		}
		return nil, err
	}

	/*
		if err := s.grantBalance(ctx, userID, totalReward, PlayRewardSourceCheckin, idempotencyKey, map[string]any{
			"checkin_date":    dateKey,
			"streak_count":    streak,
			"milestone_bonus": milestoneBonus,
			"boost_active":    boost.Active,
		}, func(txCtx context.Context) error {
			return s.repo.InsertCheckin(txCtx, userID, date, totalReward, streak)
		}); err != nil {
			if errors.Is(err, ErrPlayCheckinAlreadyDone) {
				return nil, ErrPlayCheckinAlreadyDone
			}
			return nil, err
		}
	*/

	_ = s.MarkQuestCompleted(ctx, userID, PlayQuestKeyCheckin)
	return &PlayCheckinResult{
		RewardAmount:      totalReward,
		BalanceAdded:      totalReward,
		RewardType:        rewardType,
		Coupon:            playCouponRewardSummary(couponIssue),
		RedeemCode:        playRedeemCodeRewardSummary(redeemCode),
		CouponPoolVersion: couponPoolVersion(couponIssue),
		ServerDate:        dateKey,
		StreakCount:       streak,
		MilestoneBonus:    milestoneBonus,
		GrowthEligibility: growthEligibility,
	}, nil
}

func (s *PlayService) growthEligibility(ctx context.Context, userID int64, now time.Time) (PlayGrowthEligibility, error) {
	if userID <= 0 {
		return PlayGrowthEligibility{Tier: PlayGrowthTierExplorer, RewardMode: PlayGrowthRewardEnergy, PrimaryReason: "not_logged_in"}, nil
	}
	repo, ok := s.repo.(PlayGrowthQualificationRepository)
	if !ok || repo == nil {
		if s.requireGrowthQualification {
			return PlayGrowthEligibility{}, ErrPlayGrowthQualificationUnavailable
		}
		// Focused legacy test doubles may not implement the new repository port;
		// production wiring explicitly enables the fail-closed path above.
		return PlayGrowthEligibility{Tier: PlayGrowthTierActive, RewardMode: PlayGrowthRewardRedeemable, PrimaryReason: PlayGrowthEligibilityReasonEligible}, nil
	}
	signals, err := repo.GetGrowthEligibilitySignals(ctx, userID, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), now)
	if err != nil {
		return PlayGrowthEligibility{}, err
	}
	return EvaluateGrowthEligibility(signals, now), nil
}

func (s *PlayService) createGrowthSnapshot(ctx context.Context, snapshot PlayGrowthEligibilitySnapshot) (int64, error) {
	snapshot.ActionID = strings.TrimSpace(snapshot.ActionID)
	if snapshot.ActionID == "" {
		return 0, fmt.Errorf("create growth eligibility snapshot: action id is required")
	}
	repo, ok := s.growthQualificationRepository()
	if !ok {
		if s.requireGrowthQualification {
			return 0, ErrPlayGrowthQualificationUnavailable
		}
		return 0, nil
	}
	id, err := repo.CreateGrowthEligibilitySnapshot(ctx, snapshot)
	if err != nil {
		return 0, err
	}
	// A successful snapshot write must always return a persisted positive ID.
	// Treat a zero/negative ID as unavailable so callers cannot issue energy or
	// a redeemable reward without immutable qualification evidence.
	if id <= 0 {
		return 0, ErrPlayGrowthQualificationUnavailable
	}
	return id, nil
}

func (s *PlayService) insertGrowthEnergy(ctx context.Context, entry PlayGrowthEnergyLedgerEntry) error {
	if entry.Amount <= 0 {
		return nil
	}
	entry.ActionID = strings.TrimSpace(entry.ActionID)
	if entry.ActionID == "" {
		return fmt.Errorf("insert growth energy ledger: action id is required")
	}
	repo, ok := s.growthQualificationRepository()
	if !ok {
		if s.requireGrowthQualification {
			return ErrPlayGrowthQualificationUnavailable
		}
		return nil
	}
	return repo.InsertGrowthEnergyLedger(ctx, entry)
}

func (s *PlayService) growthQualificationRepository() (PlayGrowthQualificationRepository, bool) {
	repo, ok := s.repo.(PlayGrowthQualificationRepository)
	return repo, ok && repo != nil
}

func (s *PlayService) grantBalance(
	ctx context.Context,
	userID int64,
	amount float64,
	source string,
	idempotencyKey string,
	detail map[string]any,
	beforeLedger func(txCtx context.Context) error,
) error {
	return s.grantBalanceWithGrowthSnapshot(ctx, userID, amount, source, idempotencyKey, detail, nil, beforeLedger)
}

func (s *PlayService) grantBalanceWithGrowthSnapshot(
	ctx context.Context,
	userID int64,
	amount float64,
	source string,
	idempotencyKey string,
	detail map[string]any,
	growthSnapshotID *int64,
	beforeLedger func(txCtx context.Context) error,
) error {
	if s.entClient == nil {
		return fmt.Errorf("play service: ent client missing")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin play reward tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.grantBalanceInTx(txCtx, userID, amount, source, idempotencyKey, detail, growthSnapshotID, beforeLedger); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit play reward tx: %w", err)
	}
	return nil
}

func growthRuleVersionForSnapshot(snapshotID int64) string {
	if snapshotID <= 0 {
		return ""
	}
	return playGrowthQualificationRuleVersion
}

func (s *PlayService) withPlayTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if s.entClient == nil {
		return fmt.Errorf("play service: ent client missing")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin play tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit play tx: %w", err)
	}
	return nil
}

func (s *PlayService) grantBalanceLedgerOnlyInTx(
	txCtx context.Context,
	userID int64,
	amount float64,
	source string,
	idempotencyKey string,
	detail map[string]any,
	growthSnapshotID int64,
) error {
	entry := PlayRewardLedgerEntry{
		UserID:                      userID,
		Source:                      source,
		Amount:                      amount,
		IdempotencyKey:              idempotencyKey,
		Detail:                      detail,
		GrowthEligibilitySnapshotID: growthSnapshotID,
		GrowthRuleVersion:           growthRuleVersionForSnapshot(growthSnapshotID),
	}
	if err := s.repo.InsertRewardLedger(txCtx, entry); err != nil {
		return err
	}
	if s.balanceLedger != nil {
		if err := s.applyPlayBalanceLedgerDelta(txCtx, userID, amount, source, idempotencyKey, detail); err != nil {
			return err
		}
		return nil
	}
	if amount != 0 {
		return s.repo.UpdatePlayBalance(txCtx, userID, amount)
	}
	return nil
}

func (s *PlayService) grantBalanceInTx(
	txCtx context.Context,
	userID int64,
	amount float64,
	source string,
	idempotencyKey string,
	detail map[string]any,
	growthSnapshotID *int64,
	beforeLedger func(txCtx context.Context) error,
) error {
	if beforeLedger != nil {
		if err := beforeLedger(txCtx); err != nil {
			return err
		}
	}

	snapshotID := int64(0)
	if growthSnapshotID != nil {
		snapshotID = *growthSnapshotID
	}
	entry := PlayRewardLedgerEntry{
		UserID:                      userID,
		Source:                      source,
		Amount:                      amount,
		IdempotencyKey:              idempotencyKey,
		Detail:                      detail,
		GrowthEligibilitySnapshotID: snapshotID,
		GrowthRuleVersion:           growthRuleVersionForSnapshot(snapshotID),
	}
	if err := s.repo.InsertRewardLedger(txCtx, entry); err != nil {
		return err
	}

	if s.balanceLedger != nil {
		if err := s.applyPlayBalanceLedgerDelta(txCtx, userID, amount, source, idempotencyKey, detail); err != nil {
			return fmt.Errorf("update balance: %w", err)
		}
		return nil
	}
	if err := s.repo.UpdatePlayBalance(txCtx, userID, amount); err != nil {
		return fmt.Errorf("update balance: %w", err)
	}
	return nil
}

func (s *PlayService) applyPlayBalanceLedgerDelta(ctx context.Context, userID int64, amount float64, source string, idempotencyKey string, detail map[string]any) error {
	metadata := make(map[string]any, len(detail)+1)
	for k, v := range detail {
		metadata[k] = v
	}
	metadata["play_reward_idempotency_key"] = idempotencyKey
	_, err := s.balanceLedger.ApplyDelta(ctx, BalanceLedgerApplyInput{
		UserID:         userID,
		BalanceDelta:   amount,
		SourceType:     source,
		SourceID:       idempotencyKey,
		IdempotencyKey: idempotencyKey,
		ActorType:      BalanceLedgerActorSystem,
		Description:    playBalanceLedgerDescription(source),
		Metadata:       metadata,
	})
	return err
}

func playBalanceLedgerDescription(source string) string {
	switch source {
	case PlayRewardSourceCheckin:
		return "签到奖励"
	case PlayRewardSourceCheckinMakeup:
		return "补签奖励"
	case PlayRewardSourceQuiz:
		return "答题奖励"
	case PlayRewardSourceBlindbox:
		return "盲盒净变动"
	case PlayRewardSourceArenaSettlement:
		return "竞技场结算"
	case PlayRewardSourceArenaDaily:
		return "日榜竞技场结算"
	case PlayRewardSourceTeamSharedReward:
		return "组队共享奖励"
	default:
		return "玩法奖励"
	}
}

func (s *PlayService) GetArenaCurrent(ctx context.Context, userID int64) (*PlayArenaCurrent, error) {
	rt := s.GetRuntime(ctx)
	out := &PlayArenaCurrent{Enabled: rt.ArenaEnabled}
	if !rt.ArenaEnabled {
		return out, nil
	}
	now := s.serverNow()
	period, err := s.getExistingMonthlyArenaPeriod(ctx, now)
	if err != nil {
		return nil, err
	}
	if period == nil {
		return out, nil
	}
	out.Period = period
	if userID <= 0 {
		return out, nil
	}
	tokenSum, rank, err := s.repo.GetUserArenaScore(ctx, userID, period.StartAt, period.EndAt)
	if err != nil {
		return nil, err
	}
	out.TokenSum = tokenSum
	out.Rank = rank
	rewards, err := s.arenaRewardTiersForPeriod(ctx, period)
	if err != nil {
		return nil, err
	}
	out.EstimatedReward = arenaRewardForRank(rank, rewards)
	out.DisplayTokenSum = tokenSum
	mods, err := s.resolvePlayEffectModifiers(ctx, userID, rt)
	if err != nil {
		return nil, err
	}
	if boost, err := s.getRechargeBoostStatus(ctx, userID, rt); err != nil {
		return nil, err
	} else if boost.Active {
		out.RechargeBoostActive = true
	}
	if mods.ArenaScoreMultiplier > 1 {
		out.ArenaScoreMultiplier = mods.ArenaScoreMultiplier
		out.DisplayTokenSum = applyArenaScoreMultiplier(tokenSum, mods.ArenaScoreMultiplier)
	}
	if mods.CampaignActive {
		out.CampaignActive = true
	}
	if rank > 1 && period != nil {
		gap, err := s.repo.GetArenaTokensToPrevRank(ctx, userID, period.StartAt, period.EndAt, rank, tokenSum)
		if err != nil {
			return nil, err
		}
		out.TokensToPrevRank = gap
	}
	return out, nil
}

func (s *PlayService) ListArenaLeaderboard(ctx context.Context, limit int) ([]PlayArenaScoreRow, *PlayArenaPeriod, error) {
	rt := s.GetRuntime(ctx)
	if !rt.ArenaEnabled {
		return nil, nil, ErrPlayFeatureDisabled
	}
	now := s.serverNow()
	period, err := s.getExistingMonthlyArenaPeriod(ctx, now)
	if err != nil {
		return nil, nil, err
	}
	if period == nil {
		return []PlayArenaScoreRow{}, nil, nil
	}
	rows, err := s.repo.ListArenaLeaderboard(ctx, period.StartAt, period.EndAt, limit)
	if err != nil {
		return nil, nil, err
	}
	return rows, period, nil
}

func (s *PlayService) ListPublicModels(ctx context.Context) ([]AvailableChannel, error) {
	rt := s.GetRuntime(ctx)
	if !rt.PublicModelsEnabled {
		return []AvailableChannel{}, nil
	}
	if s.channelService == nil {
		return []AvailableChannel{}, nil
	}
	channels, err := s.channelService.ListAvailable(ctx)
	if err != nil {
		return nil, err
	}
	active := make([]AvailableChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.Status == StatusActive {
			active = append(active, ch)
		}
	}
	return active, nil
}

func countPlayPublicModels(channels []AvailableChannel) int {
	seen := make(map[string]struct{})
	for _, ch := range channels {
		for _, model := range ch.SupportedModels {
			if model.Name == "" {
				continue
			}
			platform := model.Platform
			if platform == "" {
				platform = "_"
			}
			seen[model.Name+"::"+platform] = struct{}{}
		}
	}
	return len(seen)
}

// PublicMarketingModelCount returns unique model count for landing pages.
// Falls back to all configured channels when no active public channels are available.
func (s *PlayService) PublicMarketingModelCount(ctx context.Context) int {
	rt := s.GetRuntime(ctx)
	if !rt.PublicModelsEnabled {
		return 0
	}
	channels, err := s.ListPublicModels(ctx)
	if err == nil {
		if n := countPlayPublicModels(channels); n > 0 {
			return n
		}
	}
	if s.channelService == nil {
		return 0
	}
	all, err := s.channelService.ListAvailable(ctx)
	if err != nil {
		return 0
	}
	if n := countPlayPublicModels(all); n > 0 {
		return n
	}
	return s.channelService.PricingCatalogModelCount()
}
