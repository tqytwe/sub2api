package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const channelOperationEligible = `EXISTS (
	SELECT 1
	FROM account_groups ag
	JOIN channel_groups cg ON cg.group_id = ag.group_id
	WHERE ag.account_id = a.id AND cg.channel_id = $1
)`

func (r *channelRepository) ChannelOperationsCatalog(ctx context.Context, id int64, q service.ChannelOperationsQuery) (*service.ChannelOperationsCatalog, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	out := &service.ChannelOperationsCatalog{Upstreams: []service.ChannelOperationUpstream{}, Accounts: []service.ChannelOperationAccount{}}
	if err = tx.QueryRowContext(ctx, `SELECT operations_revision FROM channels WHERE id = $1`, id).Scan(&out.Revision); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrChannelNotFound
	} else if err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM channel_operation_upstreams WHERE channel_id = $1 AND ($2 = '' OR name ILIKE '%' || $2 || '%')`, id, q.UpstreamSearch).Scan(&out.UpstreamTotal); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT u.id, u.name, (SELECT count(*) FROM channel_operation_members m WHERE m.upstream_id = u.id)
		FROM channel_operation_upstreams u
		WHERE u.channel_id = $1 AND ($2 = '' OR u.name ILIKE '%' || $2 || '%')
		ORDER BY u.id LIMIT $3 OFFSET $4`, id, q.UpstreamSearch, q.PageSize, (q.UpstreamPage-1)*q.PageSize)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var upstream service.ChannelOperationUpstream
		if err = rows.Scan(&upstream.ID, &upstream.Name, &upstream.Members); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out.Upstreams = append(out.Upstreams, upstream)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}

	predicate := `
		FROM accounts a
		LEFT JOIN channel_operation_members m ON m.account_id = a.id AND m.channel_id = $1
		WHERE ((a.deleted_at IS NULL AND ` + channelOperationEligible + `) OR m.account_id IS NOT NULL)
		AND ($2 = '' OR a.name ILIKE '%' || $2 || '%' OR a.id::text = $2)
		AND ($3 = 'all' OR ($3 = 'assigned' AND m.account_id IS NOT NULL) OR ($3 = 'unassigned' AND m.account_id IS NULL))
		AND ($4::bigint = 0 OR m.upstream_id = $4)`
	args := []any{id, q.Search, q.Membership, q.UpstreamID}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) `+predicate, args...).Scan(&out.Total); err != nil {
		return nil, err
	}
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)
	rows, err = tx.QueryContext(ctx, `
		SELECT a.id, a.name, a.platform, a.status, a.schedulable, a.concurrency,
		       COALESCE(m.upstream_id, 0),
		       (a.deleted_at IS NULL AND `+channelOperationEligible+`)
		`+predicate+`
		ORDER BY a.id LIMIT $5 OFFSET $6`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var account service.ChannelOperationAccount
		if err = rows.Scan(&account.ID, &account.Name, &account.Platform, &account.Status, &account.Schedulable, &account.Concurrency, &account.UpstreamID, &account.Eligible); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out.Accounts = append(out.Accounts, account)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *channelRepository) MutateChannelOperations(ctx context.Context, id int64, m service.ChannelOperationsMutation) (revision int64, err error) {
	if err = service.ValidateChannelOperationsMutation(&m); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	err = r.runInTx(ctx, func(tx *sql.Tx) error {
		if e := tx.QueryRowContext(ctx, `SELECT operations_revision FROM channels WHERE id = $1 FOR UPDATE`, id).Scan(&revision); errors.Is(e, sql.ErrNoRows) {
			return service.ErrChannelNotFound
		} else if e != nil {
			return e
		}
		if revision != *m.Revision {
			return service.ErrChannelOperationsConflict
		}
		if m.UpstreamID > 0 {
			var found int64
			if e := tx.QueryRowContext(ctx, `SELECT id FROM channel_operation_upstreams WHERE id = $1 AND channel_id = $2`, m.UpstreamID, id).Scan(&found); errors.Is(e, sql.ErrNoRows) {
				return service.ErrChannelOperationsValidation
			} else if e != nil {
				return e
			}
		}

		var execErr error
		switch m.Action {
		case "create_upstream":
			_, execErr = tx.ExecContext(ctx, `INSERT INTO channel_operation_upstreams(channel_id, name) VALUES($1, $2)`, id, m.Name)
		case "update_upstream":
			result, e := tx.ExecContext(ctx, `UPDATE channel_operation_upstreams SET name = $3 WHERE id = $1 AND channel_id = $2`, m.UpstreamID, id, m.Name)
			execErr = e
			if e == nil {
				if count, countErr := result.RowsAffected(); countErr != nil {
					execErr = countErr
				} else if count != 1 {
					execErr = service.ErrChannelOperationsValidation
				}
			}
		case "delete_upstream":
			result, e := tx.ExecContext(ctx, `DELETE FROM channel_operation_upstreams WHERE id = $1 AND channel_id = $2`, m.UpstreamID, id)
			execErr = e
			if e == nil {
				if count, countErr := result.RowsAffected(); countErr != nil {
					execErr = countErr
				} else if count != 1 {
					execErr = service.ErrChannelOperationsValidation
				}
			}
		case "assign", "detach":
			var eligible bool
			// Retired accounts remain visible for cleanup but cannot receive new classification.
			e := tx.QueryRowContext(ctx, `SELECT (a.deleted_at IS NULL AND `+channelOperationEligible+`) FROM accounts a WHERE a.id = $2 FOR UPDATE OF a`, id, m.AccountID).Scan(&eligible)
			if errors.Is(e, sql.ErrNoRows) {
				return service.ErrChannelOperationsValidation
			}
			if e != nil {
				return e
			}
			if m.Action == "assign" && !eligible {
				return service.ErrChannelOperationsValidation
			}
			var previous int64
			e = tx.QueryRowContext(ctx, `SELECT upstream_id FROM channel_operation_members WHERE channel_id = $1 AND account_id = $2`, id, m.AccountID).Scan(&previous)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return e
			}
			if previous != m.ExpectedUpstreamID {
				return service.ErrChannelOperationsConflict
			}
			if m.Action == "assign" {
				_, execErr = tx.ExecContext(ctx, `INSERT INTO channel_operation_members(channel_id, account_id, upstream_id) VALUES($1, $2, $3) ON CONFLICT(channel_id, account_id) DO UPDATE SET upstream_id = excluded.upstream_id`, id, m.AccountID, m.UpstreamID)
			} else {
				_, execErr = tx.ExecContext(ctx, `DELETE FROM channel_operation_members WHERE channel_id = $1 AND account_id = $2`, id, m.AccountID)
			}
		default:
			return service.ErrChannelOperationsValidation
		}
		if execErr != nil {
			if isUniqueViolation(execErr) {
				return service.ErrChannelOperationsConflict
			}
			return execErr
		}
		revision++
		_, execErr = tx.ExecContext(ctx, `UPDATE channels SET operations_revision = $2 WHERE id = $1`, id, revision)
		return execErr
	})
	return revision, err
}
