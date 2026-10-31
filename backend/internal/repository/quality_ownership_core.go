package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type qualityOwnershipAccountSnapshot struct {
	Schedulable bool      `json:"schedulable"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func qualityOwnershipFence(parts ...any) string {
	b, _ := json.Marshal(parts)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// QualityOwnership previews or applies a fenced quality-rule lifecycle action.
// It deliberately only changes this rule's scheduler row and state record;
// account restrictions are owned by the quality runner and are never silently
// cleared by an admin delete.
func (r *scheduledTestPlanRepository) QualityOwnership(ctx context.Context, id int64, req *service.QualityOwnershipRequest) (*service.QualityOwnershipPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var requestedAccountID int64
	err = tx.QueryRowContext(ctx, `SELECT account_id FROM scheduled_test_plans WHERE id=$1`, id).Scan(&requestedAccountID)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(404, "QUALITY_PLAN_NOT_FOUND", "quality rule not found")
	}
	if err != nil {
		return nil, err
	}
	var lockedAccountID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE id=$1 FOR NO KEY UPDATE`, requestedAccountID).Scan(&lockedAccountID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil && lockedAccountID != requestedAccountID {
		return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_PREVIEW_CHANGED", "account assignment changed; preview again before confirming")
	}
	var accountID int64
	var version time.Time
	var config []byte
	err = tx.QueryRowContext(ctx, `SELECT account_id, updated_at, COALESCE(pelican_config,'null'::jsonb) FROM scheduled_test_plans WHERE id=$1 FOR UPDATE`, id).Scan(&accountID, &version, &config)
	if err == sql.ErrNoRows {
		return nil, infraerrors.New(404, "QUALITY_PLAN_NOT_FOUND", "quality rule not found")
	}
	if err != nil {
		return nil, err
	}
	if accountID != requestedAccountID {
		return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_PREVIEW_CHANGED", "account assignment changed; preview again before confirming")
	}
	if string(config) == "null" || len(config) == 0 {
		return nil, infraerrors.BadRequest("QUALITY_OWNERSHIP_UNSUPPORTED", "legacy connectivity plans do not have quality ownership")
	}
	var state []byte
	err = tx.QueryRowContext(ctx, `SELECT state FROM account_quality_states WHERE plan_id=$1 FOR UPDATE`, id).Scan(&state)
	statePresent := err == nil && len(state) > 0 && string(state) != "{}"
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	var owned qualityRuntimeState
	var accountSnapshot any
	preview := &service.QualityOwnershipPreview{PlanID: id, AccountID: accountID, RuleVersion: version.UTC().Format(time.RFC3339Nano), StatePresent: statePresent}
	if statePresent {
		if err := json.Unmarshal(state, &owned); err == nil && owned.Action == "disable_scheduling" {
			var schedulable bool
			var accountUpdatedAt time.Time
			err = tx.QueryRowContext(ctx, `SELECT schedulable, updated_at FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, accountID).
				Scan(&schedulable, &accountUpdatedAt)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if err == nil {
				accountSnapshot = qualityOwnershipAccountSnapshot{schedulable, accountUpdatedAt.UTC()}
				preview.RestoreReady = schedulable == owned.AppliedSchedulable && accountUpdatedAt.Equal(owned.AccountUpdatedAt)
				preview.ReconcileReady = schedulable == owned.PreviousSchedulable
			}
		}
	}
	fence := qualityOwnershipFence(id, accountID, version.UTC().Format(time.RFC3339Nano), config, state, accountSnapshot)
	preview.Fence = fence
	var parsed struct {
		Quality *service.QualityPolicy `json:"quality"`
	}
	if json.Unmarshal(config, &parsed) == nil && parsed.Quality != nil {
		preview.Action = parsed.Quality.Action
	}
	if req == nil {
		return preview, tx.Commit()
	}
	if req.Fence != fence {
		return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_PREVIEW_CHANGED", "ownership changed; preview again before confirming")
	}
	if req.Delete {
		if statePresent {
			return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_DELETE_BLOCKED", "restore or reconcile owned state before deleting this rule")
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE id=$1`, id); err != nil {
			return nil, err
		}
		preview.Fence = ""
		return preview, tx.Commit()
	}
	if req.Reconcile {
		if !statePresent || !preview.ReconcileReady {
			return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_RECONCILE_BLOCKED", "account has not returned to its recorded prior value; the ownership record was preserved")
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM account_quality_states WHERE plan_id=$1`, id); err != nil {
			return nil, err
		}
		preview.Fence = ""
		preview.StatePresent = false
		preview.RestoreReady = false
		preview.ReconcileReady = false
		return preview, tx.Commit()
	}
	if req.Restore {
		if !statePresent || !preview.RestoreReady {
			return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_RESTORE_BLOCKED", "account no longer matches the exact owned value and version; ownership was preserved")
		}
		planUpdate, err := tx.ExecContext(ctx, `UPDATE scheduled_test_plans SET enabled=false, updated_at=clock_timestamp() WHERE id=$1 AND updated_at=$2`, id, version)
		if err != nil {
			return nil, err
		}
		if changed, err := planUpdate.RowsAffected(); err != nil {
			return nil, err
		} else if changed != 1 {
			return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_PREVIEW_CHANGED", "quality rule changed; preview again before restoring")
		}
		action, restoreErr := restoreQualityRuntimeState(ctx, tx, &service.ScheduledTestPlan{ID: id, AccountID: accountID}, owned, owned.AppliedSchedulable, owned.AccountUpdatedAt)
		if restoreErr != nil {
			return nil, restoreErr
		}
		if action != "restored" {
			return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_RESTORE_BLOCKED", "account changed during restore; ownership was preserved")
		}
		preview.Fence = ""
		preview.StatePresent = false
		preview.RestoreReady = false
		preview.ReconcileReady = false
		return preview, tx.Commit()
	}
	return nil, fmt.Errorf("unsupported quality ownership operation")
}
