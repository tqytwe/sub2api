package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

func lockQualityIsolationGroups(ctx context.Context, tx *sql.Tx, accountID int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT group_id FROM account_groups WHERE account_id=$1 ORDER BY group_id FOR UPDATE`, accountID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
	}
	return rows.Err()
}

func readQualityIsolationGroups(ctx context.Context, tx *sql.Tx, accountID int64) ([]byte, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(g) ORDER BY g.group_id),'[]'::jsonb) FROM account_groups g WHERE account_id=$1`, accountID).Scan(&raw)
	return raw, err
}

func removeQualityIsolationGroups(ctx context.Context, tx *sql.Tx, accountID int64, targets []int64, before []byte) ([]qualityIsolationGroup, error) {
	var groups, removed []qualityIsolationGroup
	if err := json.Unmarshal(before, &groups); err != nil {
		return nil, err
	}
	selected := map[int64]bool{}
	for _, id := range targets {
		selected[id] = true
	}
	for _, group := range groups {
		if selected[group.GroupID] {
			if _, err := tx.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id=$1 AND group_id=$2`, accountID, group.GroupID); err != nil {
				return nil, err
			}
			removed = append(removed, group)
		}
	}
	if len(removed) > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE accounts SET updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1`, accountID); err != nil {
			return nil, err
		}
	}
	return removed, nil
}

func restoreQualityIsolationGroups(ctx context.Context, tx *sql.Tx, accountID int64, groups []qualityIsolationGroup) (bool, error) {
	// Validate every row first. A deleted group or manual replacement must keep
	// the entire model/group transition and its ownership snapshot intact.
	for _, group := range groups {
		var exists, replaced bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM groups WHERE id=$1 AND deleted_at IS NULL),EXISTS(SELECT 1 FROM account_groups WHERE account_id=$2 AND group_id=$1)`, group.GroupID, accountID).Scan(&exists, &replaced); err != nil {
			return false, err
		}
		if !exists || replaced {
			return true, nil
		}
	}
	for _, group := range groups {
		var allowed any
		if qualityIsolationEntryPresent(group.AllowedModels) {
			allowed = string(group.AllowedModels)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id,priority,created_at,allowed_models) VALUES($1,$2,$3,$4,$5)`, accountID, group.GroupID, group.Priority, group.CreatedAt, allowed); err != nil {
			return false, err
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE accounts SET updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1`, accountID)
	return false, err
}

func emitQualityIsolationGroupChange(ctx context.Context, tx *sql.Tx, accountID int64, before []byte, removed []qualityIsolationGroup) error {
	var groups []qualityIsolationGroup
	if err := json.Unmarshal(before, &groups); err != nil {
		return err
	}
	ids := make([]int64, 0, len(groups)+len(removed))
	for _, group := range append(groups, removed...) {
		ids = append(ids, group.GroupID)
	}
	payload, err := json.Marshal(map[string]any{"group_ids": ids})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO scheduler_outbox(event_type,account_id,payload) VALUES('account_groups_changed',$1,$2)`, accountID, string(payload))
	return err
}

// Lock referenced catalog rows as well as memberships: a concurrent soft delete
// cannot invalidate restoration between its preview and transaction commit.
func readQualityIsolationGroupCatalog(ctx context.Context, tx *sql.Tx, ids []int64) ([]byte, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,deleted_at FROM groups WHERE id=ANY($1::bigint[]) ORDER BY id FOR SHARE`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	type catalogRow struct {
		ID        int64      `json:"id"`
		DeletedAt *time.Time `json:"deleted_at"`
	}
	catalog := []catalogRow{}
	for rows.Next() {
		var id int64
		var deleted sql.NullTime
		if err := rows.Scan(&id, &deleted); err != nil {
			return nil, err
		}
		row := catalogRow{ID: id}
		if deleted.Valid {
			stamp := deleted.Time
			row.DeletedAt = &stamp
		}
		catalog = append(catalog, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(catalog)
}
