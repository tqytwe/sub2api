package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"
)

// Only normal-only templates exclude known exhausted OAuth accounts. Recovery
// selectors, manual runs and upstream-error protection keep their explicit intent.
func QualityTemplateQuotaAvailable(filter QualityRuleAccountFilter, account *Account, now time.Time) bool {
	if len(filter.Statuses) != 1 || filter.Statuses[0] != StatusActive || account == nil || !account.IsOpenAIOAuth() {
		return true
	}
	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		return false
	}
	if !openAICodexSnapshotIdentityTrusted(account) {
		return true
	}
	for _, window := range []string{"5h", "7d"} {
		extra := qualityQuotaWindowObservation(account.Extra, window, now)
		used, known := resolveOpenAIQuotaUtilization(extra, window, now)
		if known && !math.IsNaN(used) && !math.IsInf(used, 0) && used >= 1 {
			return false
		}
	}
	return true
}

// Field timestamps prevent a newer 5h packet from extending old weekly evidence.
func qualityQuotaWindowObservation(extra map[string]any, window string, now time.Time) map[string]any {
	prefix := "codex_" + window + "_"
	observed := func(key string) (time.Time, bool) {
		if raw, exists := extra["codex_usage_field_versions"]; exists {
			fields, ok := raw.(map[string]any)
			if !ok {
				return time.Time{}, false
			}
			stamp, err := strconv.ParseInt(fmt.Sprint(fields[key]), 10, 64)
			if err != nil || stamp <= 0 {
				return time.Time{}, false
			}
			at := time.Unix(0, stamp)
			return at, !at.After(now)
		}
		at, err := parseTime(fmt.Sprint(extra["codex_usage_updated_at"]))
		return at, err == nil && !at.After(now)
	}
	at, ok := observed(prefix + "used_percent")
	if !ok {
		return nil
	}
	out := map[string]any{prefix + "used_percent": extra[prefix+"used_percent"], "codex_usage_updated_at": at.Format(time.RFC3339Nano)}
	if raw, exists := extra[prefix+"reset_at"]; exists {
		reset, err := parseTime(fmt.Sprint(raw))
		if err != nil {
			return nil
		}
		out[prefix+"reset_at"] = reset.Format(time.RFC3339Nano)
	} else if raw, exists := extra[prefix+"reset_after_seconds"]; exists {
		resetObserved, valid := observed(prefix + "reset_after_seconds")
		seconds, err := strconv.ParseInt(fmt.Sprint(raw), 10, 64)
		if !valid || err != nil || seconds < 0 || seconds > int64((7*24*time.Hour)/time.Second) {
			return nil
		}
		out[prefix+"reset_at"] = resetObserved.Add(time.Duration(seconds) * time.Second).Format(time.RFC3339Nano)
	}
	return out
}

type QualityTemplateQuotaRepository interface {
	ForPlan(context.Context, int64) (*QualityRuleTemplate, error)
}

type QualityQuotaSkipRepository interface {
	FinishQuotaSkippedQuality(context.Context, *ScheduledTestPlan, time.Time) error
}

func (s *ScheduledTestRunnerService) qualityTemplateQuotaAvailable(ctx context.Context, plan *ScheduledTestPlan) (bool, error) {
	if plan.ManualQualityRun || plan.TriggerSource != "" || s.scheduledSvc == nil || s.scheduledSvc.templateRepo == nil {
		return true, nil
	}
	repo, ok := s.scheduledSvc.templateRepo.(QualityTemplateQuotaRepository)
	if !ok {
		return false, fmt.Errorf("quality template quota lookup unavailable")
	}
	template, err := repo.ForPlan(ctx, plan.ID)
	if err != nil || template == nil {
		return err == nil, err
	}
	if len(template.AccountFilter.Statuses) != 1 || template.AccountFilter.Statuses[0] != StatusActive {
		return true, nil
	}
	if s.accountTestSvc == nil || s.accountTestSvc.accountRepo == nil {
		return false, fmt.Errorf("quality template account lookup unavailable")
	}
	account, err := s.accountTestSvc.accountRepo.GetByID(ctx, plan.AccountID)
	if err != nil {
		return false, err
	}
	return account != nil && QualityTemplateQuotaAvailable(template.AccountFilter, account, time.Now()), nil
}
