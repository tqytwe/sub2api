package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) GetLatestSettledDailyArenaPeriod(ctx context.Context) (*service.PlayArenaPeriod, error) {
	exec := r.sqlExec(ctx)
	var p service.PlayArenaPeriod
	var periodType sql.NullString
	var settledAt sql.NullTime
	err := scanSingleRow(ctx, exec, `
		SELECT id, name, start_at, end_at, status, COALESCE(period_type, 'daily') AS period_type, settled_at
		FROM play_arena_periods
		WHERE period_type = 'daily' AND status = 'settled'
		ORDER BY settled_at DESC NULLS LAST, end_at DESC, updated_at DESC, id DESC
		LIMIT 1`, nil, &p.ID, &p.Name, &p.StartAt, &p.EndAt, &p.Status, &periodType, &settledAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest settled daily arena period: %w", err)
	}
	applyPlayArenaPeriodOptionalFields(&p, periodType, settledAt)
	return &p, nil
}

func (r *playRepository) ListArenaDailyRewardLedger(
	ctx context.Context,
	periodID int64,
) (result []service.PlayArenaDailyRewardLedgerRow, err error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT prl.user_id,
		       COALESCE(u.username, '') AS username,
		       COALESCE(u.email, '') AS email,
		       COALESCE(NULLIF(TRIM(ua.url), ''), '') AS avatar_url,
		       prl.amount::float8 AS amount,
		       CASE
		         WHEN COALESCE(prl.detail->>'rank', '') ~ '^[0-9]+$' THEN (prl.detail->>'rank')::int
		         ELSE 0
		       END AS rank,
		       CASE
		         WHEN COALESCE(prl.detail->>'token', '') ~ '^[0-9]+$' THEN (prl.detail->>'token')::bigint
		         WHEN COALESCE(prl.detail->>'token_sum', '') ~ '^[0-9]+$' THEN (prl.detail->>'token_sum')::bigint
		         ELSE 0
		       END AS token_sum,
		       prl.created_at
		FROM play_reward_ledger prl
		JOIN users u ON u.id = prl.user_id
		LEFT JOIN user_avatars ua ON ua.user_id = prl.user_id
		WHERE prl.source = $1
		  AND (
		    CASE
		      WHEN COALESCE(prl.detail->>'period_id', '') ~ '^[0-9]+$' THEN (prl.detail->>'period_id')::bigint
		      ELSE NULL
		    END = $2
		    OR prl.idempotency_key LIKE 'arena_daily_settlement:' || $2::text || ':%'
		  )
		ORDER BY
		  CASE
		    WHEN COALESCE(prl.detail->>'rank', '') ~ '^[0-9]+$' THEN (prl.detail->>'rank')::int
		    ELSE 2147483647
		  END ASC,
		  prl.created_at ASC,
		  prl.user_id ASC`, service.PlayRewardSourceArenaDaily, periodID)
	if err != nil {
		return nil, fmt.Errorf("list daily arena reward ledger: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	out := make([]service.PlayArenaDailyRewardLedgerRow, 0)
	for rows.Next() {
		var row service.PlayArenaDailyRewardLedgerRow
		var username, email string
		if err := rows.Scan(
			&row.UserID,
			&username,
			&email,
			&row.AvatarURL,
			&row.Amount,
			&row.Rank,
			&row.TokenSum,
			&row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan daily arena reward ledger: %w", err)
		}
		row.DisplayName = service.PublicPlayDisplayName(username, email, row.UserID)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily arena reward ledger: %w", err)
	}
	return out, nil
}
