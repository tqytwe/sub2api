package repository

import (
	"context"
	"encoding/json"
	"errors"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// ReplaceAccountsFromGroups replaces membership atomically. Surviving target
// bindings keep their policy and priority. New members inherit the union of
// their selected source policies (an unrestricted source makes that union
// unrestricted). This never uses an empty list to represent deny-all.
func (r *groupRepository) ReplaceAccountsFromGroups(ctx context.Context, groupID int64, sourceIDs, accountIDs []int64) error {
	if accountIDs == nil {
		accountIDs = []int64{}
	}
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	client := r.client
	if tx != nil {
		defer func() { _ = tx.Rollback() }()
		client = tx.Client()
	}
	// Serialize two copies into the same target, including initially empty groups.
	if _, err := client.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('group-account-copy:' || $1::text, 0))`, groupID); err != nil {
		return err
	}
	// Match the account-policy writer's lock order: accounts, then groups, then
	// memberships. Read source policies only after acquiring these account locks.
	rows, err := client.QueryContext(ctx, `SELECT id FROM accounts
  WHERE deleted_at IS NULL AND (id = ANY($1) OR id IN (SELECT account_id FROM account_groups WHERE group_id=$2))
  ORDER BY id FOR NO KEY UPDATE`, pq.Array(accountIDs), groupID)
	if err != nil {
		return err
	}
	for rows.Next() {
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil {
		return rowErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := lockCopyPolicyGroups(ctx, client, append(append([]int64{}, sourceIDs...), groupID)); err != nil {
		return err
	}
	// Remove only departed bindings; never delete/recreate survivors.
	if _, err := client.ExecContext(ctx, `DELETE FROM account_groups WHERE group_id=$1 AND NOT(account_id=ANY($2::bigint[]))`, groupID, pq.Array(accountIDs)); err != nil {
		return err
	}
	rows, err = client.QueryContext(ctx, `WITH source AS (
  SELECT ag.account_id, ag.allowed_models FROM account_groups ag
  JOIN accounts a ON a.id=ag.account_id AND a.deleted_at IS NULL
  WHERE ag.group_id=ANY($1::bigint[]) AND ag.account_id=ANY($2::bigint[])
 ), policies AS (
  SELECT account_id, CASE WHEN bool_or(allowed_models IS NULL OR allowed_models='[]'::jsonb)
   THEN NULL ELSE (SELECT jsonb_agg(model ORDER BY model) FROM (
    SELECT DISTINCT jsonb_array_elements_text(s.allowed_models) AS model FROM source s WHERE s.account_id=source.account_id
   ) models) END AS allowed_models
  FROM source GROUP BY account_id
 ) INSERT INTO account_groups(account_id,group_id,priority,allowed_models,created_at)
  SELECT account_id,$3,50,allowed_models,NOW() FROM policies
  ON CONFLICT(account_id,group_id) DO NOTHING RETURNING allowed_models`, pq.Array(sourceIDs), pq.Array(accountIDs), groupID)
	if err != nil {
		return err
	}
	// Validate the final union, not only each source. Reject oversize policies
	// atomically; truncating or dropping them would change access permissions.
	var validationErr error
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			validationErr = err
			break
		}
		var models []string
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &models); err != nil {
				validationErr = err
				break
			}
		}
		if err := service.ValidateGroupAllowedModels(map[int64][]string{groupID: models}); err != nil {
			validationErr = err
			break
		}
	}
	rowErr = rows.Err()
	closeErr = rows.Close()
	if validationErr != nil {
		return validationErr
	}
	if rowErr != nil {
		return rowErr
	}
	if closeErr != nil {
		return closeErr
	}
	// A failed notification rolls back the membership update as well.
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventGroupChanged, nil, &groupID, nil); err != nil {
		return err
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}

// Copy-and-update needs exclusive group locks from the outset. Ordered locks
// avoid SHARE-to-UPDATE upgrades deadlocking two reciprocal empty-group copies.
func lockCopyPolicyGroups(ctx context.Context, client *dbent.Client, ids []int64) error {
	unique := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		unique[id] = struct{}{}
	}
	rows, err := client.QueryContext(ctx, `SELECT id FROM groups WHERE id=ANY($1) AND deleted_at IS NULL ORDER BY id FOR UPDATE`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != len(unique) {
		return service.ErrGroupNotFound
	}
	return nil
}

func (r *groupRepository) CreateWithCopiedAccounts(ctx context.Context, group *service.Group, sourceIDs, accountIDs []int64) error {
	return r.saveWithCopiedAccounts(ctx, group, sourceIDs, accountIDs, true)
}
func (r *groupRepository) UpdateWithCopiedAccounts(ctx context.Context, group *service.Group, sourceIDs, accountIDs []int64) error {
	return r.saveWithCopiedAccounts(ctx, group, sourceIDs, accountIDs, false)
}
func (r *groupRepository) saveWithCopiedAccounts(ctx context.Context, group *service.Group, sourceIDs, accountIDs []int64, create bool) error {
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	client := r.client
	if tx != nil {
		defer func() { _ = tx.Rollback() }()
		client = tx.Client()
	}
	writer := newGroupRepositoryWithSQL(client, client)
	if create {
		if err := createGroupRecord(ctx, client, group); err != nil {
			return err
		}
	}
	// The copy acquires account locks before group locks; update attributes only
	// after those locks, in the same transaction as the policy/outbox mutations.
	if err := writer.ReplaceAccountsFromGroups(ctx, group.ID, sourceIDs, accountIDs); err != nil {
		return err
	}
	if !create {
		if err := writer.Update(ctx, group); err != nil {
			return err
		}
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}
