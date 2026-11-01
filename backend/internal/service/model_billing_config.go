package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
)

// ModelBillingConfig contains optional customer-side token multipliers selected
// by the requested model and the API key's billing group. It is deliberately
// independent from published prices, account cost multipliers, channel pricing,
// and usage-log persistence. The runtime billing integration is a follow-up
// task that must carry this policy in the same request snapshot as the cost.
type ModelBillingConfig struct {
	Enabled bool               `json:"enabled"`
	Rules   []ModelBillingRule `json:"rules"`
}

// ModelBillingRule applies one multiplier to an exact model name or to a
// trailing-prefix wildcard. A non-empty GroupIDs scope applies only when the
// request's billing API key belongs to one of those groups.
type ModelBillingRule struct {
	Model      string  `json:"model"`
	Multiplier float64 `json:"multiplier"`
	GroupIDs   []int64 `json:"group_ids"`
}

// UnmarshalJSON keeps existing group scopes attached to a model when a legacy
// admin client reorders rules. Explicit [] or null means global scope. When a
// saved config already has scoped rules, a newly introduced rule must provide
// group_ids explicitly so an old client cannot silently widen its scope.
func (c *ModelBillingConfig) UnmarshalJSON(data []byte) error {
	var patch struct {
		Enabled *bool           `json:"enabled"`
		Rules   json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(data, &patch); err != nil {
		return err
	}
	next := *c
	if patch.Enabled != nil {
		next.Enabled = *patch.Enabled
	}
	if patch.Rules != nil {
		var rules []struct {
			Model      string          `json:"model"`
			Multiplier float64         `json:"multiplier"`
			GroupIDs   json.RawMessage `json:"group_ids"`
		}
		if err := json.Unmarshal(patch.Rules, &rules); err != nil {
			return err
		}
		next.Rules = make([]ModelBillingRule, len(rules))
		previous := make(map[string][]int64, len(c.Rules))
		hasScopedRules := false
		for _, rule := range c.Rules {
			previous[modelBillingRuleKey(rule.Model)] = slices.Clone(rule.GroupIDs)
			hasScopedRules = hasScopedRules || len(rule.GroupIDs) > 0
		}
		for i, rule := range rules {
			ids, exists := previous[modelBillingRuleKey(rule.Model)]
			if rule.GroupIDs == nil && !exists && hasScopedRules {
				return errors.New("explicit group_ids required for new models when scoped rules exist")
			}
			if rule.GroupIDs != nil {
				ids = nil
				if err := json.Unmarshal(rule.GroupIDs, &ids); err != nil {
					return err
				}
			}
			next.Rules[i] = ModelBillingRule{Model: rule.Model, Multiplier: rule.Multiplier, GroupIDs: ids}
		}
	}
	*c = NormalizeModelBillingConfig(next)
	return nil
}

func modelBillingRuleKey(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}

// NormalizeModelBillingConfig copies the config and removes duplicate group
// IDs while preserving first-seen order. It never mutates the caller's config,
// which is required for request snapshots and CAS retry handling.
func NormalizeModelBillingConfig(c ModelBillingConfig) ModelBillingConfig {
	c.Rules = slices.Clone(c.Rules)
	for i := range c.Rules {
		if c.Rules[i].GroupIDs == nil {
			continue
		}
		ids := make([]int64, 0, len(c.Rules[i].GroupIDs))
		seen := make(map[int64]struct{}, len(c.Rules[i].GroupIDs))
		for _, id := range c.Rules[i].GroupIDs {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		c.Rules[i].GroupIDs = ids
	}
	return c
}

var modelBillingPattern = regexp.MustCompile(`^[a-zA-Z0-9_./:-]+\*?$`)

// ValidateModelBillingConfig validates the storage/API contract. Group
// existence and active status belong to the admin save service because this
// pure type intentionally has no database dependency.
func ValidateModelBillingConfig(c ModelBillingConfig) error {
	if len(c.Rules) > 100 || (c.Enabled && len(c.Rules) == 0) {
		return errors.New("model billing rules must contain 1-100 entries when enabled")
	}
	seen := make(map[string]struct{}, len(c.Rules))
	for _, rule := range c.Rules {
		model := modelBillingRuleKey(rule.Model)
		if !modelBillingPattern.MatchString(model) || len(model) > 200 {
			return fmt.Errorf("invalid model billing model %q", rule.Model)
		}
		if _, exists := seen[model]; exists {
			return fmt.Errorf("duplicate model billing model %q", rule.Model)
		}
		if math.IsNaN(rule.Multiplier) || math.IsInf(rule.Multiplier, 0) || rule.Multiplier < 1 || rule.Multiplier > 1000 {
			return fmt.Errorf("invalid model billing multiplier for %q", rule.Model)
		}
		for _, id := range rule.GroupIDs {
			if id <= 0 || id > 9007199254740991 {
				return fmt.Errorf("invalid model billing group ID %d", id)
			}
		}
		seen[model] = struct{}{}
	}
	return nil
}

// MultiplierForModel selects one rule. Exact model names win; otherwise the
// longest matching trailing-prefix wildcard wins. Rules never compound. A
// scoped rule does not match a missing or unknown group.
func (c ModelBillingConfig) MultiplierForModel(model string, groupID *int64) float64 {
	if !c.Enabled {
		return 1
	}
	model = modelBillingRuleKey(model)
	multiplier, longest := 1.0, 0
	for _, rule := range c.Rules {
		if len(rule.GroupIDs) > 0 && !modelBillingGroupContains(rule.GroupIDs, groupID) {
			continue
		}
		pattern := modelBillingRuleKey(rule.Model)
		if pattern == model {
			return rule.Multiplier
		}
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if len(prefix) > longest && strings.HasPrefix(model, prefix) {
				multiplier, longest = rule.Multiplier, len(prefix)
			}
		}
	}
	return multiplier
}

func modelBillingGroupContains(groupIDs []int64, groupID *int64) bool {
	if groupID == nil || *groupID <= 0 {
		return false
	}
	for _, id := range groupIDs {
		if id == *groupID {
			return true
		}
	}
	return false
}
