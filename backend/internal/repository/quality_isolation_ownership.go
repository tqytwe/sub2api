package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type qualityIsolationAccountView struct {
	Platform    string
	Type        string
	Credentials json.RawMessage
	Extra       json.RawMessage
	Status      string
	Expires     sql.NullTime
	UpdatedAt   time.Time
	Schedulable bool
}

func decodeQualityIsolationState(raw []byte, plan *service.ScheduledTestPlan) (*qualityRuntimeState, bool) {
	var state qualityRuntimeState
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&state) != nil || !qualityIsolationScopeMatches(plan, &state) {
		return &state, false
	}
	return &state, true
}

// The parent acquired account -> plan -> state locks. Both preview and apply
// rebuild this full live view under those locks; the public response contains
// only bounded enums, model IDs and an opaque digest.
func (r *scheduledTestPlanRepository) qualityIsolationOwnership(ctx context.Context, tx *sql.Tx, id, accountID int64, version time.Time, config, rawState []byte, preview *service.QualityOwnershipPreview, req *service.QualityOwnershipRequest) (*service.QualityOwnershipPreview, error) {
	plan := &service.ScheduledTestPlan{ID: id, AccountID: accountID, PelicanConfig: &service.PelicanTestConfig{}}
	if err := json.Unmarshal(config, plan.PelicanConfig); err != nil {
		return nil, err
	}
	var running sql.NullTime
	var planRow []byte
	if err := tx.QueryRowContext(ctx, `SELECT model_id,running_until,to_jsonb(p) FROM scheduled_test_plans p WHERE id=$1`, id).Scan(&plan.ModelID, &running, &planRow); err != nil {
		return nil, err
	}
	var snapshot qualityIsolationAccountView
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(credentials,'{}'::jsonb),COALESCE(extra,'{}'::jsonb),status,expires_at,updated_at,schedulable,platform,type FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, accountID).Scan(&snapshot.Credentials, &snapshot.Extra, &snapshot.Status, &snapshot.Expires, &snapshot.UpdatedAt, &snapshot.Schedulable, &snapshot.Platform, &snapshot.Type); err != nil {
		return nil, err
	}
	// Include existence/deletion of removed groups too. Their deletion without an
	// account version bump must also invalidate an outstanding confirmation.
	var catalogState qualityRuntimeState
	_ = json.Unmarshal(rawState, &catalogState)
	catalogIDs := append([]int64(nil), catalogState.IsolationGroupIDs...)
	if plan.PelicanConfig.Quality != nil {
		catalogIDs = append(catalogIDs, plan.PelicanConfig.Quality.RemoveGroupIDs...)
	}
	if catalogState.Groups != nil {
		for _, group := range catalogState.Groups.Removed {
			catalogIDs = append(catalogIDs, group.GroupID)
		}
	}
	groupCatalog, err := readQualityIsolationGroupCatalog(ctx, tx, catalogIDs)
	if err != nil {
		return nil, err
	}
	// Group deletion locks catalog -> memberships. Never hold a membership row
	// while waiting for that catalog lock, even for a read-only preview.
	if err := lockQualityIsolationGroups(ctx, tx, accountID); err != nil {
		return nil, err
	}
	groups, err := readQualityIsolationGroups(ctx, tx, accountID)
	if err != nil {
		return nil, err
	}
	preview.Action = service.QualityActionIsolateModels
	preview.Fence = qualityOwnershipFence(id, accountID, version.UTC().Format(time.RFC3339Nano), config, rawState, planRow, snapshot, groups, groupCatalog)
	account := &service.Account{Platform: snapshot.Platform, Type: snapshot.Type}
	if err := json.Unmarshal(snapshot.Credentials, &account.Credentials); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(snapshot.Extra, &account.Extra); err != nil {
		return nil, err
	}
	var extra map[string]json.RawMessage
	if err := json.Unmarshal(snapshot.Extra, &extra); err != nil {
		return nil, err
	}
	live := map[string]json.RawMessage{}
	liveValid := true
	if qualityIsolationEntryPresent(extra[service.QualityModelIsolationKey]) {
		liveValid = json.Unmarshal(extra[service.QualityModelIsolationKey], &live) == nil
	}
	if live == nil {
		live = map[string]json.RawMessage{}
	}
	state, valid := decodeQualityIsolationState(rawState, plan)
	valid = valid && liveValid
	if valid {
		targets := map[string]bool{}
		for _, model := range service.QualityProbeModels(plan) {
			targets[account.GetMappedModel(model)] = true
		}
		if state.Models != nil {
			for key := range state.Models.ModelApplied {
				valid = valid && targets[key]
			}
			for key := range state.Models.ModelRateLimits {
				valid = valid && targets[key]
			}
		}
	}
	preview.ModelRemovals = qualityIsolationModelRemovals(plan, account, state, valid, live, running, snapshot)
	preview.RestoreReady, preview.ReconcileReady, preview.RestoreBlockedReason = qualityIsolationLifecycleReadiness(plan, state, valid, live, groups, groupCatalog, running, snapshot)
	if req == nil {
		return preview, tx.Commit()
	}
	if req.Fence != preview.Fence {
		return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_PREVIEW_CHANGED", "ownership changed; preview again before confirming")
	}
	if !req.Confirm {
		return nil, infraerrors.BadRequest("QUALITY_OWNERSHIP_CONFIRM_REQUIRED", "explicit confirmation is required")
	}
	if req.RemoveModel {
		return removeQualityIsolationModel(ctx, tx, plan, state, version, preview, req, live)
	}
	if req.Delete {
		if preview.StatePresent || !liveValid || hasQualityIsolationOwner(live, id) {
			return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_DELETE_BLOCKED", "restore or reconcile owned state before deleting this rule")
		}
		if running.Valid && running.Time.After(time.Now()) {
			return nil, infraerrors.Conflict("QUALITY_RULE_RUNNING", "rule is running; preview again when idle")
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE id=$1`, id); err != nil {
			return nil, err
		}
	} else if req.Restore || req.Reconcile {
		if req.Restore && !preview.RestoreReady || req.Reconcile && !preview.ReconcileReady {
			return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_RESTORE_BLOCKED", "owned state cannot be safely restored or reconciled; review the current preview")
		}
		// Pause and fence the rule before releasing ownership. Reconcile must not
		// allow a previously claimed worker to put the restriction back.
		if _, err = tx.ExecContext(ctx, `UPDATE scheduled_test_plans SET enabled=false,updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1`, id); err != nil {
			return nil, err
		}
		if req.Restore {
			if state.Models != nil {
				for key := range state.Models.ModelApplied {
					delete(live, key)
				}
			}
			if err = writeQualityIsolationLive(ctx, tx, accountID, live); err != nil {
				return nil, err
			}
			if state.Groups != nil {
				conflict, restoreErr := restoreQualityIsolationGroups(ctx, tx, accountID, state.Groups.Removed)
				if restoreErr != nil {
					return nil, restoreErr
				}
				if conflict {
					return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_RESTORE_BLOCKED", "group membership changed; ownership was preserved")
				}
				if err = emitQualityIsolationGroupChange(ctx, tx, accountID, groups, state.Groups.Removed); err != nil {
					return nil, err
				}
			} else {
				if err = enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
					return nil, err
				}
			}
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM account_quality_states WHERE plan_id=$1`, id); err != nil {
			return nil, err
		}
		preview.StatePresent = false
		preview.RestoreReady = false
		preview.ReconcileReady = false
	} else {
		return nil, fmt.Errorf("unsupported quality ownership operation")
	}
	preview.Fence = ""
	return preview, tx.Commit()
}

func qualityIsolationModelRemovals(plan *service.ScheduledTestPlan, account *service.Account, state *qualityRuntimeState, valid bool, live map[string]json.RawMessage, running sql.NullTime, snapshot qualityIsolationAccountView) []service.QualityModelRemoval {
	out := []service.QualityModelRemoval{}
	models := service.QualityProbeModels(plan)
	for _, model := range models {
		key := account.GetMappedModel(model)
		entry := service.QualityModelRemoval{Model: model, IsolationModel: key, Ready: true, Effect: "remove_missing_target"}
		block := func(reason string) { entry.Ready = false; entry.BlockedReason = reason }
		var expected, previous json.RawMessage
		if state.Models != nil {
			expected = state.Models.ModelApplied[key]
			previous = state.Models.ModelRateLimits[key]
		}
		switch {
		case !qualityIsolationEntryPresent(live[key]):
			if qualityIsolationEntryPresent(expected) && !exclusiveQualityIsolationMarker(expected, plan.ID) {
				block("unrecognized_snapshot")
			}
		case !exclusiveQualityIsolationMarker(live[key], plan.ID):
			block("not_owned_or_unrecognized")
		case !qualityIsolationEntryPresent(expected):
			entry.Effect = "remove_orphan_marker_only"
		case !exclusiveQualityIsolationMarker(expected, plan.ID):
			block("unrecognized_snapshot")
		case !qualityIsolationEntriesEqual(expected, live[key]):
			block("changed_same_owner")
		default:
			entry.Effect = "remove_matching_marker"
		}
		if qualityIsolationEntryPresent(previous) {
			block("previous_restriction")
		}
		if key != model && qualityIsolationEntryPresent(live[model]) {
			block("ambiguous_mapping")
		}
		for _, peer := range models {
			if peer != model && account.GetMappedModel(peer) == key {
				block("shared_mapping")
			}
		}
		if len(models) <= 1 {
			block("last_model")
		}
		if !valid {
			block("scope_conflict")
		}
		if snapshot.Status != service.StatusActive || snapshot.Expires.Valid && !snapshot.Expires.Time.After(time.Now()) {
			block("account_unavailable")
		}
		if running.Valid && running.Time.After(time.Now()) {
			block("rule_running")
		}
		out = append(out, entry)
	}
	return out
}

func hasQualityIsolationOwner(live map[string]json.RawMessage, id int64) bool {
	for _, raw := range live {
		var marker struct {
			Reason string `json:"reason"`
		}
		if json.Unmarshal(raw, &marker) == nil && marker.Reason == fmt.Sprintf("quality_rule:%d", id) {
			return true
		}
	}
	return false
}

func qualityIsolationLifecycleReadiness(plan *service.ScheduledTestPlan, state *qualityRuntimeState, valid bool, live map[string]json.RawMessage, groups, groupCatalog []byte, running sql.NullTime, snapshot qualityIsolationAccountView) (bool, bool, string) {
	if !valid {
		return false, false, "scope_conflict"
	}
	if snapshot.Status != service.StatusActive || snapshot.Expires.Valid && !snapshot.Expires.Time.After(time.Now()) {
		return false, false, "account_unavailable"
	}
	if running.Valid && running.Time.After(time.Now()) {
		return false, false, "rule_running"
	}
	restore, reconcile := true, true
	if state.Models != nil {
		for key, expected := range state.Models.ModelApplied {
			previous, captured := state.Models.ModelRateLimits[key]
			if !exclusiveQualityIsolationMarker(expected, plan.ID) || !captured || qualityIsolationEntryPresent(previous) {
				return false, false, "unrecognized_snapshot"
			}
			restore = restore && qualityIsolationEntriesEqual(expected, live[key])
			reconcile = reconcile && !qualityIsolationEntryPresent(live[key])
		}
		for key, previous := range state.Models.ModelRateLimits {
			if qualityIsolationEntryPresent(previous) || !qualityIsolationEntryPresent(state.Models.ModelApplied[key]) {
				return false, false, "unrecognized_snapshot"
			}
		}
	}
	// An orphan belonging to this rule must be explicitly released first.
	for key, raw := range live {
		if hasQualityIsolationOwner(map[string]json.RawMessage{key: raw}, plan.ID) && (state.Models == nil || !qualityIsolationEntryPresent(state.Models.ModelApplied[key])) {
			return false, false, "orphan_marker"
		}
	}
	if state.Groups != nil {
		var current []qualityIsolationGroup
		var catalog []struct {
			ID        int64      `json:"id"`
			DeletedAt *time.Time `json:"deleted_at"`
		}
		if json.Unmarshal(groups, &current) != nil || json.Unmarshal(groupCatalog, &catalog) != nil {
			return false, false, "group_conflict"
		}
		seen := map[int64]bool{}
		for _, group := range state.Groups.Removed {
			if group.AccountID != plan.AccountID || seen[group.GroupID] {
				return false, false, "group_conflict"
			}
			seen[group.GroupID] = true
			exists := false
			for _, row := range catalog {
				exists = exists || row.ID == group.GroupID && row.DeletedAt == nil
			}
			if !exists {
				return false, false, "group_conflict"
			}
			var row *qualityIsolationGroup
			for i := range current {
				if current[i].GroupID == group.GroupID {
					row = &current[i]
				}
			}
			restore = restore && row == nil
			reconcile = reconcile && row != nil && row.Priority == group.Priority && row.CreatedAt.Equal(group.CreatedAt) && qualityIsolationEntriesEqual(row.AllowedModels, group.AllowedModels)
		}
	}
	if !restore && !reconcile {
		return false, false, "owned_state_changed"
	}
	return restore, reconcile, ""
}

func writeQualityIsolationLive(ctx context.Context, tx *sql.Tx, accountID int64, live map[string]json.RawMessage) error {
	if len(live) == 0 {
		_, err := tx.ExecContext(ctx, `UPDATE accounts SET extra=extra-'quality_model_isolation',updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1`, accountID)
		return err
	}
	encoded, err := json.Marshal(live)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE accounts SET extra=jsonb_set(COALESCE(extra,'{}'::jsonb),'{quality_model_isolation}',$2::jsonb),updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1`, accountID, string(encoded))
	return err
}
