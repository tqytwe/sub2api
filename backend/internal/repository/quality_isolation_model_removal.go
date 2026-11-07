package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Contract one target only. JSON paths preserve sibling state, counters, group
// snapshots and all historical results; any failed write rolls back the lot.
func removeQualityIsolationModel(ctx context.Context, tx *sql.Tx, plan *service.ScheduledTestPlan, state *qualityRuntimeState, version time.Time, view *service.QualityOwnershipPreview, req *service.QualityOwnershipRequest, live map[string]json.RawMessage) (*service.QualityOwnershipPreview, error) {
	var selected *service.QualityModelRemoval
	for i := range view.ModelRemovals {
		if view.ModelRemovals[i].Model == req.Model {
			selected = &view.ModelRemovals[i]
			break
		}
	}
	if selected == nil || !selected.Ready || state.Action != service.QualityActionIsolateModels {
		return nil, infraerrors.Conflict("QUALITY_MODEL_REMOVAL_BLOCKED", "model removal is blocked; review the current preview")
	}
	if req.ReasonCode != "orphan_marker_recovery" && req.ReasonCode != "retire_model" {
		return nil, infraerrors.BadRequest("QUALITY_MODEL_REMOVAL_REASON_REQUIRED", "select an explicit removal reason")
	}
	models := slices.DeleteFunc(slices.Clone(service.QualityProbeModels(plan)), func(model string) bool { return model == req.Model })
	if len(models) == 0 {
		return nil, infraerrors.Conflict("QUALITY_MODEL_REMOVAL_BLOCKED", "at least one tested model must remain")
	}
	encoded, err := json.Marshal(models)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE account_quality_states SET state=jsonb_set(state #- ARRAY['models','model_applied',$2] #- ARRAY['models','model_rate_limits',$2],'{isolation_models}',$3::jsonb),updated_at=clock_timestamp() WHERE plan_id=$1`, plan.ID, selected.IsolationModel, string(encoded)); err != nil {
		return nil, err
	}
	if qualityIsolationEntryPresent(live[selected.IsolationModel]) {
		delete(live, selected.IsolationModel)
		if err = writeQualityIsolationLive(ctx, tx, plan.AccountID, live); err != nil {
			return nil, err
		}
		if err = enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &plan.AccountID, nil, nil); err != nil {
			return nil, err
		}
	}
	primary := plan.ModelID
	if primary == req.Model {
		primary = models[0]
	}
	configExpression := "jsonb_set(pelican_config,'{model_ids}',$3::jsonb)"
	args := []any{plan.ID, primary, string(encoded), version}
	if plan.PelicanConfig.ModelID == req.Model {
		encodedPrimary, err := json.Marshal(primary)
		if err != nil {
			return nil, err
		}
		configExpression = "jsonb_set(" + configExpression + ",'{model_id}',$5::jsonb)"
		args = append(args, string(encodedPrimary))
	}
	result, err := tx.ExecContext(ctx, `UPDATE scheduled_test_plans SET model_id=$2,pelican_config=`+configExpression+`,updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1 AND updated_at=$4 AND (running_until IS NULL OR running_until<=clock_timestamp())`, args...)
	if err != nil {
		return nil, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if changed != 1 {
		return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_PREVIEW_CHANGED", "rule version or lease changed; preview again")
	}
	view.Fence = ""
	return view, tx.Commit()
}
