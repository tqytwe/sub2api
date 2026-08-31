package service

import "time"

const (
	// PlayGrowthTierExplorer keeps participation available without creating a
	// recurring cash-equivalent reward path for inactive accounts.
	PlayGrowthTierExplorer = "explorer"
	PlayGrowthTierActive   = "active"

	PlayGrowthRewardEnergy     = "energy"
	PlayGrowthRewardRedeemable = "redeemable"

	PlayGrowthEligibilityReasonEligible         = "eligible"
	PlayGrowthEligibilityReasonEmailUnverified  = "email_unverified"
	PlayGrowthEligibilityReasonAccountTooNew    = "account_too_new"
	PlayGrowthEligibilityReasonNoRecentActivity = "no_recent_activity"
)

const (
	playGrowthMinimumAccountAgeDays = 3
	playGrowthMinimumRechargeCNY    = 10.0
)

const playGrowthQualificationRuleVersion = "v1"

// PlayGrowthQualificationRuleVersion exposes the persisted rule marker to the
// repository without letting callers invent an arbitrary version string.
func PlayGrowthQualificationRuleVersion() string {
	return playGrowthQualificationRuleVersion
}

// PlayGrowthEligibilitySignals contains only server-derived facts. Callers
// must not accept any of these values from a browser request.
type PlayGrowthEligibilitySignals struct {
	EmailVerified         bool
	CreatedAt             time.Time
	HasRecentUsage        bool
	NetBalanceRecharge30d float64
	HasActiveSubscription bool
}

// PlayGrowthEligibility is returned to activity endpoints and rendered by the
// client. It describes the current reward mode, never a permission decision
// supplied by the client.
type PlayGrowthEligibility struct {
	Tier                  string                        `json:"tier"`
	RewardMode            string                        `json:"reward_mode"`
	PrimaryReason         string                        `json:"primary_reason"`
	EmailVerified         bool                          `json:"email_verified"`
	AccountAgeDays        int                           `json:"account_age_days"`
	HasRecentUsage        bool                          `json:"has_recent_usage"`
	NetBalanceRecharge30d float64                       `json:"net_balance_recharge_30d"`
	HasActiveSubscription bool                          `json:"has_active_subscription"`
	Progress              PlayGrowthEligibilityProgress `json:"progress"`
}

// PlayGrowthEligibilityProgress is entirely derived by the server from the
// same qualification signals as the decision. It gives an Explorer account a
// precise, non-punitive next step without making the browser an eligibility
// authority.
type PlayGrowthEligibilityProgress struct {
	EmailVerified            bool    `json:"email_verified"`
	AccountAgeDays           int     `json:"account_age_days"`
	MinimumAccountAgeDays    int     `json:"minimum_account_age_days"`
	AccountAgeRequirementMet bool    `json:"account_age_requirement_met"`
	HasRecentUsage           bool    `json:"has_recent_usage"`
	NetBalanceRecharge30d    float64 `json:"net_balance_recharge_30d"`
	MinimumRechargeCNY       float64 `json:"minimum_recharge_cny"`
	HasActiveSubscription    bool    `json:"has_active_subscription"`
	ActivityRequirementMet   bool    `json:"activity_requirement_met"`
	NextAction               string  `json:"next_action"`
}

// EvaluateGrowthEligibility implements the single qualification rule shared by
// check-in, quiz, draws, and future reward redemptions. Exploration remains
// available when the account is not qualified; only redeemable rewards change.
func EvaluateGrowthEligibility(signals PlayGrowthEligibilitySignals, now time.Time) PlayGrowthEligibility {
	ageDays := 0
	if !signals.CreatedAt.IsZero() && now.After(signals.CreatedAt) {
		ageDays = int(now.Sub(signals.CreatedAt).Hours() / 24)
	}
	result := PlayGrowthEligibility{
		Tier:                  PlayGrowthTierExplorer,
		RewardMode:            PlayGrowthRewardEnergy,
		EmailVerified:         signals.EmailVerified,
		AccountAgeDays:        ageDays,
		HasRecentUsage:        signals.HasRecentUsage,
		NetBalanceRecharge30d: signals.NetBalanceRecharge30d,
		HasActiveSubscription: signals.HasActiveSubscription,
	}
	result.Progress = growthEligibilityProgress(result)

	if !signals.EmailVerified {
		result.PrimaryReason = PlayGrowthEligibilityReasonEmailUnverified
		result.Progress.NextAction = PlayGrowthEligibilityReasonEmailUnverified
		return result
	}
	if ageDays < playGrowthMinimumAccountAgeDays {
		result.PrimaryReason = PlayGrowthEligibilityReasonAccountTooNew
		result.Progress.NextAction = PlayGrowthEligibilityReasonAccountTooNew
		return result
	}
	if !signals.HasRecentUsage && signals.NetBalanceRecharge30d < playGrowthMinimumRechargeCNY && !signals.HasActiveSubscription {
		result.PrimaryReason = PlayGrowthEligibilityReasonNoRecentActivity
		result.Progress.NextAction = PlayGrowthEligibilityReasonNoRecentActivity
		return result
	}

	result.Tier = PlayGrowthTierActive
	result.RewardMode = PlayGrowthRewardRedeemable
	result.PrimaryReason = PlayGrowthEligibilityReasonEligible
	result.Progress.NextAction = PlayGrowthEligibilityReasonEligible
	return result
}

func growthEligibilityProgress(eligibility PlayGrowthEligibility) PlayGrowthEligibilityProgress {
	activityRequirementMet := eligibility.HasRecentUsage ||
		eligibility.NetBalanceRecharge30d >= playGrowthMinimumRechargeCNY ||
		eligibility.HasActiveSubscription
	return PlayGrowthEligibilityProgress{
		EmailVerified:            eligibility.EmailVerified,
		AccountAgeDays:           eligibility.AccountAgeDays,
		MinimumAccountAgeDays:    playGrowthMinimumAccountAgeDays,
		AccountAgeRequirementMet: eligibility.AccountAgeDays >= playGrowthMinimumAccountAgeDays,
		HasRecentUsage:           eligibility.HasRecentUsage,
		NetBalanceRecharge30d:    eligibility.NetBalanceRecharge30d,
		MinimumRechargeCNY:       playGrowthMinimumRechargeCNY,
		HasActiveSubscription:    eligibility.HasActiveSubscription,
		ActivityRequirementMet:   activityRequirementMet,
	}
}
