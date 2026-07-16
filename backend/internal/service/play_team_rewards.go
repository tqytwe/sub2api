package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	teamRewardAmountScale       int32 = 8
	teamRewardMaxIntegerDigits        = 12
	teamRewardTiersJSONMaxBytes       = 64 * 1024
)

type TeamRewardTier struct {
	Threshold decimal.Decimal `json:"threshold"`
	Rate      decimal.Decimal `json:"rate"`
}

type TeamRewardConfig struct {
	Enabled bool
	Cap     decimal.Decimal
	Tiers   []TeamRewardTier
}

type TeamContribution struct {
	UserID int64
	Amount decimal.Decimal
}

type teamRewardConfigDiagnostic struct {
	SettingKey string
	Reason     string
}

func defaultTeamRewardConfig() TeamRewardConfig {
	return TeamRewardConfig{
		Enabled: true,
		Cap:     decimal.NewFromInt(250),
		Tiers: []TeamRewardTier{
			{Threshold: decimal.NewFromInt(20), Rate: decimal.RequireFromString("0.02")},
			{Threshold: decimal.NewFromInt(100), Rate: decimal.RequireFromString("0.03")},
			{Threshold: decimal.NewFromInt(500), Rate: decimal.RequireFromString("0.04")},
			{Threshold: decimal.NewFromInt(2000), Rate: decimal.RequireFromString("0.05")},
		},
	}
}

func validateTeamRewardConfig(cfg TeamRewardConfig) error {
	if err := validateTeamRewardDecimal20Scale8(cfg.Cap); err != nil {
		return fmt.Errorf("team reward cap: %w", err)
	}
	if !cfg.Cap.IsPositive() {
		return fmt.Errorf("team reward cap must be positive")
	}
	if len(cfg.Tiers) == 0 {
		return fmt.Errorf("team reward tiers are required")
	}

	for i, tier := range cfg.Tiers {
		if err := validateTeamRewardDecimal20Scale8(tier.Threshold); err != nil {
			return fmt.Errorf("team reward tier %d threshold: %w", i, err)
		}
		if err := validateTeamRewardDecimal20Scale8(tier.Rate); err != nil {
			return fmt.Errorf("team reward tier %d rate: %w", i, err)
		}
		if !tier.Threshold.IsPositive() {
			return fmt.Errorf("team reward tier %d threshold must be positive", i)
		}
		if !tier.Rate.IsPositive() || tier.Rate.GreaterThan(decimal.NewFromInt(1)) {
			return fmt.Errorf("team reward tier %d rate must be within (0, 1]", i)
		}
		if i == 0 {
			continue
		}
		previous := cfg.Tiers[i-1]
		if !tier.Threshold.GreaterThan(previous.Threshold) {
			return fmt.Errorf("team reward tier thresholds must be strictly increasing")
		}
		if !tier.Rate.GreaterThan(previous.Rate) {
			return fmt.Errorf("team reward tier rates must be strictly increasing")
		}
	}
	return nil
}

func validateTeamRewardDecimal20Scale8(value decimal.Decimal) error {
	coefficient := value.Coefficient()
	if coefficient.Sign() == 0 {
		return nil
	}
	coefficient.Abs(coefficient)

	digitsWithTrailingZeros := coefficient.Text(10)
	digits := strings.TrimRight(digitsWithTrailingZeros, "0")
	exponent := int64(value.Exponent()) +
		int64(len(digitsWithTrailingZeros)-len(digits))

	if exponent < -int64(teamRewardAmountScale) {
		return fmt.Errorf("must have at most %d fractional digits", teamRewardAmountScale)
	}

	integerDigits := int64(len(digits)) + exponent
	if integerDigits > teamRewardMaxIntegerDigits {
		return fmt.Errorf("must have at most %d integer digits", teamRewardMaxIntegerDigits)
	}
	return nil
}

func parseTeamRewardConfig(
	enabledRaw string,
	tiersRaw string,
	capRaw string,
) (TeamRewardConfig, *teamRewardConfigDiagnostic) {
	defaults := defaultTeamRewardConfig()
	enabled, enabledDiagnostic := parseTeamRewardEnabled(enabledRaw)
	if enabledDiagnostic != nil {
		return defaults, enabledDiagnostic
	}

	tiers := append([]TeamRewardTier(nil), defaults.Tiers...)
	if len(tiersRaw) > teamRewardTiersJSONMaxBytes {
		defaults.Enabled = enabled
		return defaults, &teamRewardConfigDiagnostic{
			SettingKey: SettingKeyPlayTeamSharedRewardTiers,
			Reason:     "tiers_too_large",
		}
	}
	if strings.TrimSpace(tiersRaw) != "" {
		if err := json.Unmarshal([]byte(tiersRaw), &tiers); err != nil {
			defaults.Enabled = enabled
			return defaults, &teamRewardConfigDiagnostic{
				SettingKey: SettingKeyPlayTeamSharedRewardTiers,
				Reason:     "malformed_tiers",
			}
		}
	}

	cap := defaults.Cap
	if strings.TrimSpace(capRaw) != "" {
		parsedCap, err := decimal.NewFromString(strings.TrimSpace(capRaw))
		if err != nil ||
			validateTeamRewardDecimal20Scale8(parsedCap) != nil ||
			!parsedCap.IsPositive() {
			defaults.Enabled = enabled
			return defaults, &teamRewardConfigDiagnostic{
				SettingKey: SettingKeyPlayTeamSharedRewardCap,
				Reason:     "invalid_cap",
			}
		}
		cap = parsedCap
	}

	cfg := TeamRewardConfig{
		Enabled: enabled,
		Cap:     cap,
		Tiers:   tiers,
	}
	if err := validateTeamRewardConfig(cfg); err != nil {
		defaults.Enabled = enabled
		return defaults, &teamRewardConfigDiagnostic{
			SettingKey: SettingKeyPlayTeamSharedRewardTiers,
			Reason:     "invalid_tiers",
		}
	}
	return cfg, nil
}

func parseTeamRewardEnabled(raw string) (bool, *teamRewardConfigDiagnostic) {
	switch strings.TrimSpace(raw) {
	case "", "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return true, &teamRewardConfigDiagnostic{
			SettingKey: SettingKeyPlayTeamSharedRewardEnabled,
			Reason:     "invalid_enabled",
		}
	}
}

func parseTeamRewardStartMonth(raw string) (string, *teamRewardConfigDiagnostic) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if len(value) != len("YYYY-MM") || value[4] != '-' {
		return "", &teamRewardConfigDiagnostic{
			SettingKey: SettingKeyPlayTeamSharedRewardStartMonth,
			Reason:     "invalid_format",
		}
	}
	for _, index := range [...]int{0, 1, 2, 3, 5, 6} {
		if value[index] < '0' || value[index] > '9' {
			return "", &teamRewardConfigDiagnostic{
				SettingKey: SettingKeyPlayTeamSharedRewardStartMonth,
				Reason:     "invalid_format",
			}
		}
	}
	if value[:4] == "0000" {
		return "", &teamRewardConfigDiagnostic{
			SettingKey: SettingKeyPlayTeamSharedRewardStartMonth,
			Reason:     "invalid_year",
		}
	}
	month := int(value[5]-'0')*10 + int(value[6]-'0')
	if month < 1 || month > 12 {
		return "", &teamRewardConfigDiagnostic{
			SettingKey: SettingKeyPlayTeamSharedRewardStartMonth,
			Reason:     "invalid_month",
		}
	}
	return value, nil
}

func resolveTeamRewardPool(teamSpend decimal.Decimal, cfg TeamRewardConfig) decimal.Decimal {
	if !cfg.Enabled || !teamSpend.IsPositive() || validateTeamRewardConfig(cfg) != nil {
		return decimal.Zero
	}

	rate := decimal.Zero
	for _, tier := range cfg.Tiers {
		if teamSpend.LessThan(tier.Threshold) {
			break
		}
		rate = tier.Rate
	}
	if rate.IsZero() {
		return decimal.Zero
	}

	pool := teamSpend.Mul(rate).Round(teamRewardAmountScale)
	cap := cfg.Cap.Round(teamRewardAmountScale)
	if pool.GreaterThan(cap) {
		return cap
	}
	return pool
}

func allocateTeamReward(
	pool decimal.Decimal,
	contributions []TeamContribution,
) (map[int64]decimal.Decimal, error) {
	for _, contribution := range contributions {
		if contribution.UserID <= 0 {
			return map[int64]decimal.Decimal{}, fmt.Errorf(
				"team reward contribution user ID must be positive: %d",
				contribution.UserID,
			)
		}
	}

	pool = pool.Round(teamRewardAmountScale)
	if !pool.IsPositive() {
		return map[int64]decimal.Decimal{}, nil
	}

	amountByUser := make(map[int64]decimal.Decimal, len(contributions))
	for _, contribution := range contributions {
		if !contribution.Amount.IsPositive() {
			continue
		}
		amountByUser[contribution.UserID] = amountByUser[contribution.UserID].Add(contribution.Amount)
	}
	if len(amountByUser) == 0 {
		return map[int64]decimal.Decimal{}, nil
	}

	positive := make([]TeamContribution, 0, len(amountByUser))
	totalContribution := decimal.Zero
	for userID, amount := range amountByUser {
		positive = append(positive, TeamContribution{UserID: userID, Amount: amount})
		totalContribution = totalContribution.Add(amount)
	}
	sort.Slice(positive, func(i, j int) bool {
		if positive[i].Amount.Equal(positive[j].Amount) {
			return positive[i].UserID < positive[j].UserID
		}
		return positive[i].Amount.GreaterThan(positive[j].Amount)
	})

	allocations := make(map[int64]decimal.Decimal, len(positive))
	allocated := decimal.Zero
	for _, contribution := range positive {
		share, _ := pool.
			Mul(contribution.Amount).
			QuoRem(totalContribution, teamRewardAmountScale)
		allocations[contribution.UserID] = share
		allocated = allocated.Add(share)
	}

	remainder := pool.Sub(allocated)
	remainderUserID := positive[0].UserID
	allocations[remainderUserID] = allocations[remainderUserID].Add(remainder)
	return allocations, nil
}
