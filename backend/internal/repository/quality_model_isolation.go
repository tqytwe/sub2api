package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type qualityIsolationModels struct {
	PersistentModels bool                       `json:"persistent_models"`
	ModelRateLimits  map[string]json.RawMessage `json:"model_rate_limits"`
	ModelApplied     map[string]json.RawMessage `json:"model_applied"`
}

type qualityIsolationGroups struct {
	Action  string                  `json:"action"`
	Removed []qualityIsolationGroup `json:"removed"`
}

type qualityIsolationGroup struct {
	AccountID     int64           `json:"account_id"`
	GroupID       int64           `json:"group_id"`
	Priority      int             `json:"priority"`
	CreatedAt     time.Time       `json:"created_at"`
	AllowedModels json.RawMessage `json:"allowed_models"`
}

func sameQualityIsolationScope[T ~string | ~int64](left, right []T) bool {
	left, right = slices.Clone(left), slices.Clone(right)
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(left, right)
}

func qualityIsolationScopeMatches(plan *service.ScheduledTestPlan, state *qualityRuntimeState) bool {
	if plan == nil || plan.PelicanConfig == nil || plan.PelicanConfig.Quality == nil || plan.PelicanConfig.Quality.Action != service.QualityActionIsolateModels || state.Action != service.QualityActionIsolateModels ||
		!sameQualityIsolationScope(state.IsolationModels, service.QualityProbeModels(plan)) ||
		!sameQualityIsolationScope(state.IsolationGroupIDs, plan.PelicanConfig.Quality.RemoveGroupIDs) ||
		(state.Models != nil && !state.Models.PersistentModels) ||
		(state.Groups != nil && state.Groups.Action != "remove_groups") {
		return false
	}
	seen := map[int64]bool{}
	if state.Groups != nil {
		for _, group := range state.Groups.Removed {
			if group.AccountID != plan.AccountID || group.CreatedAt.IsZero() || seen[group.GroupID] || !slices.Contains(state.IsolationGroupIDs, group.GroupID) {
				return false
			}
			seen[group.GroupID] = true
		}
	}
	return true
}

func qualityIsolationEntryPresent(raw json.RawMessage) bool {
	return len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func qualityIsolationEntriesEqual(left, right json.RawMessage) bool {
	if !qualityIsolationEntryPresent(left) || !qualityIsolationEntryPresent(right) {
		return !qualityIsolationEntryPresent(left) && !qualityIsolationEntryPresent(right)
	}
	var a, b any
	if json.Unmarshal(left, &a) != nil || json.Unmarshal(right, &b) != nil {
		return false
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

// An explicit release may only clear the exact, exclusive marker emitted here.
// Extra fields might encode another restriction and are never interpreted away.
func exclusiveQualityIsolationMarker(raw json.RawMessage, owner int64) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 2 {
		return false
	}
	var reason, stamp string
	if json.Unmarshal(fields["reason"], &reason) != nil || reason != fmt.Sprintf("quality_rule:%d", owner) ||
		json.Unmarshal(fields["isolated_at"], &stamp) != nil {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, stamp)
	return err == nil
}

// Completed wrong answers can isolate immediately. Recovery requires every
// configured sample to pass; missing/unknown probes never release a restriction.
func qualityIsolationOutcomes(plan *service.ScheduledTestPlan, results []*service.ScheduledTestResult) map[string]string {
	out := map[string]string{}
	for _, model := range service.QualityProbeModels(plan) {
		count, passed, failed := 0, 0, false
		for _, result := range results {
			if result == nil || result.PelicanConfig == nil || result.PelicanConfig.ModelID != model {
				continue
			}
			count++
			verdict, _ := result.QualityJudgment["verdict"].(string)
			failed = failed || verdict == "incorrect" && result.Status == "failed" && result.ErrorMessage == "answer_mismatch"
			if verdict == "correct" && result.Status == "success" {
				passed++
			}
		}
		switch {
		case failed:
			out[model] = "failed"
		case count == service.QualityProbeParallelCount(plan.PelicanConfig) && passed == count:
			out[model] = "passed"
		default:
			out[model] = "inconclusive"
		}
	}
	return out
}

// The caller already holds account -> plan -> state locks. Keep the same lock
// order as the existing runner and ownership API, and commit results/outbox with
// the complete transition. A conflict returns before any persistent mutation.
func applyQualityIsolationRound(ctx context.Context, tx *sql.Tx, plan *service.ScheduledTestPlan, state *qualityRuntimeState, rawState []byte, results []*service.ScheduledTestResult, status string) (string, error) {
	policy := plan.PelicanConfig.Quality
	models := service.QualityProbeModels(plan)
	if len(models) == 0 || len(policy.RemoveGroupIDs) == 0 || state.Action != "" && !qualityIsolationScopeMatches(plan, state) {
		return "action_conflict", nil
	}
	if len(rawState) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(rawState))
		decoder.DisallowUnknownFields()
		if decoder.Decode(state) != nil {
			return "action_conflict", nil
		}
	}
	if state.Action == "" && (state.Models != nil || state.Groups != nil || len(state.IsolationModels) > 0 || len(state.IsolationGroupIDs) > 0) {
		return "action_conflict", nil
	}
	var credentials, extra []byte
	var expires sql.NullTime
	account := &service.Account{}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(credentials,'{}'::jsonb),COALESCE(extra,'{}'::jsonb),expires_at,platform,type FROM accounts WHERE id=$1`, plan.AccountID).Scan(&credentials, &extra, &expires, &account.Platform, &account.Type); err != nil {
		return "", err
	}
	if status != service.StatusActive || expires.Valid && !expires.Time.After(time.Now()) {
		return "account_unavailable", nil
	}
	if err := json.Unmarshal(credentials, &account.Credentials); err != nil {
		return "", err
	}
	if err := json.Unmarshal(extra, &account.Extra); err != nil {
		return "", err
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(extra, &outer); err != nil {
		return "", err
	}
	live := map[string]json.RawMessage{}
	if qualityIsolationEntryPresent(outer[service.QualityModelIsolationKey]) && json.Unmarshal(outer[service.QualityModelIsolationKey], &live) != nil {
		return "action_conflict", nil
	}
	if live == nil {
		live = map[string]json.RawMessage{}
	}
	before, err := json.Marshal(live)
	if err != nil {
		return "", err
	}
	if state.Models == nil {
		state.Models = &qualityIsolationModels{PersistentModels: true}
	}
	if state.Models.ModelApplied == nil {
		state.Models.ModelApplied = map[string]json.RawMessage{}
	}
	if state.Models.ModelRateLimits == nil {
		state.Models.ModelRateLimits = map[string]json.RawMessage{}
	}
	outcomes := qualityIsolationOutcomes(plan, results)
	rank := map[string]int{"passed": 1, "inconclusive": 2, "failed": 3}
	decisions := map[string]string{}
	for _, model := range models {
		key := account.GetMappedModel(model)
		if rank[outcomes[model]] > rank[decisions[key]] {
			decisions[key] = outcomes[model]
		}
	}
	for key := range state.Models.ModelApplied {
		if _, targeted := decisions[key]; !targeted {
			return "action_conflict", nil
		}
	}
	for key := range state.Models.ModelRateLimits {
		if _, targeted := decisions[key]; !targeted {
			return "action_conflict", nil
		}
	}
	allFailed, anyPassed := len(decisions) > 0, false
	keys := make([]string, 0, len(decisions))
	for key, outcome := range decisions {
		keys = append(keys, key)
		allFailed = allFailed && outcome == "failed"
		anyPassed = anyPassed || outcome == "passed"
	}
	sort.Strings(keys)
	changed, restored := false, false
	for _, key := range keys {
		outcome := decisions[key]
		if outcome != "failed" && (outcome != "passed" || !policy.AutoRestore) {
			continue
		}
		expected := state.Models.ModelApplied[key]
		previous, captured := state.Models.ModelRateLimits[key]
		if qualityIsolationEntryPresent(expected) && (!exclusiveQualityIsolationMarker(expected, plan.ID) || !qualityIsolationEntriesEqual(live[key], expected)) {
			return "restore_conflict", nil
		}
		if captured && qualityIsolationEntryPresent(previous) {
			return "action_conflict", nil
		}
		if outcome == "failed" {
			if !qualityIsolationEntryPresent(expected) && qualityIsolationEntryPresent(live[key]) || qualityIsolationEntryPresent(expected) && !captured {
				return "action_conflict", nil
			}
			marker, err := json.Marshal(map[string]string{"reason": fmt.Sprintf("quality_rule:%d", plan.ID), "isolated_at": time.Now().UTC().Format(time.RFC3339Nano)})
			if err != nil {
				return "", err
			}
			state.Models.ModelRateLimits[key] = json.RawMessage("null")
			state.Models.ModelApplied[key], live[key] = marker, marker
			changed = true
		} else if qualityIsolationEntryPresent(expected) {
			if !captured {
				return "restore_conflict", nil
			}
			delete(live, key)
			delete(state.Models.ModelApplied, key)
			delete(state.Models.ModelRateLimits, key)
			restored = true
		}
	}
	groupChanged := false
	var priorGroups []byte
	if allFailed && state.Groups == nil || anyPassed && policy.AutoRestore && state.Groups != nil {
		if _, err := readQualityIsolationGroupCatalog(ctx, tx, policy.RemoveGroupIDs); err != nil {
			return "", err
		}
		if err := lockQualityIsolationGroups(ctx, tx, plan.AccountID); err != nil {
			return "", err
		}
		priorGroups, err = readQualityIsolationGroups(ctx, tx, plan.AccountID)
		if err != nil {
			return "", err
		}
	}
	var removed []qualityIsolationGroup
	action := "inconclusive"
	if changed {
		action = "models_isolated"
	} else if restored {
		action = "models_restored"
	} else if anyPassed {
		action = "passed"
	}
	if allFailed && state.Groups == nil {
		groups, err := removeQualityIsolationGroups(ctx, tx, plan.AccountID, policy.RemoveGroupIDs, priorGroups)
		if err != nil {
			return "", err
		}
		if len(groups) > 0 {
			state.Groups = &qualityIsolationGroups{Action: "remove_groups", Removed: groups}
			removed, groupChanged, action = groups, true, "groups_removed"
		}
	} else if anyPassed && policy.AutoRestore && state.Groups != nil {
		removed = state.Groups.Removed
		conflict, err := restoreQualityIsolationGroups(ctx, tx, plan.AccountID, removed)
		if err != nil {
			return "", err
		}
		if conflict {
			return "restore_conflict", nil
		}
		state.Groups = nil
		groupChanged, action = true, "groups_restored_models_isolated"
	}
	after, err := json.Marshal(live)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(before, after) {
		if len(live) == 0 {
			_, err = tx.ExecContext(ctx, `UPDATE accounts SET extra=extra-'quality_model_isolation',updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1`, plan.AccountID)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE accounts SET extra=jsonb_set(COALESCE(extra,'{}'::jsonb),'{quality_model_isolation}',$2::jsonb),updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1`, plan.AccountID, string(after))
		}
		if err != nil {
			return "", err
		}
	}
	if groupChanged {
		if err = emitQualityIsolationGroupChange(ctx, tx, plan.AccountID, priorGroups, removed); err != nil {
			return "", err
		}
	} else if !bytes.Equal(before, after) {
		accountID := plan.AccountID
		if err = enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			return "", err
		}
	}
	if len(state.Models.ModelApplied) == 0 && len(state.Models.ModelRateLimits) == 0 && state.Groups == nil {
		if state.Action != "" && policy.AutoRestore && anyPassed {
			action = "restored"
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM account_quality_states WHERE plan_id=$1`, plan.ID)
		return action, err
	}
	state.Action = service.QualityActionIsolateModels
	state.IsolationModels = slices.Clone(models)
	state.IsolationGroupIDs = slices.Clone(policy.RemoveGroupIDs)
	encoded, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO account_quality_states(plan_id,state,updated_at) VALUES($1,$2,clock_timestamp()) ON CONFLICT(plan_id) DO UPDATE SET state=EXCLUDED.state,updated_at=EXCLUDED.updated_at`, plan.ID, encoded)
	return action, err
}
