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
	if string(config) == "null" || len(config) == 0 {
		return nil, infraerrors.BadRequest("QUALITY_OWNERSHIP_UNSUPPORTED", "legacy connectivity plans do not have quality ownership")
	}
	var state []byte
	err = tx.QueryRowContext(ctx, `SELECT state FROM account_quality_states WHERE plan_id=$1 FOR UPDATE`, id).Scan(&state)
	statePresent := err == nil && len(state) > 0 && string(state) != "{}"
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	fence := qualityOwnershipFence(id, accountID, version.UTC().Format(time.RFC3339Nano), config, state)
	preview := &service.QualityOwnershipPreview{PlanID: id, AccountID: accountID, RuleVersion: version.UTC().Format(time.RFC3339Nano), Fence: fence, StatePresent: statePresent, RestoreReady: !statePresent}
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
		// Reconciliation is intentionally a fenced, read/verify operation in the
		// core slice. The runner remains the only writer of account restrictions.
		preview.Fence = ""
		return preview, tx.Commit()
	}
	if req.Restore {
		if statePresent {
			return nil, infraerrors.Conflict("QUALITY_OWNERSHIP_RESTORE_BLOCKED", "owned state requires runner reconciliation before restore")
		}
		if _, err = tx.ExecContext(ctx, `UPDATE scheduled_test_plans SET enabled=false, updated_at=clock_timestamp() WHERE id=$1`, id); err != nil {
			return nil, err
		}
		preview.Fence = ""
		preview.RestoreReady = true
		return preview, tx.Commit()
	}
	return nil, fmt.Errorf("unsupported quality ownership operation")
}
