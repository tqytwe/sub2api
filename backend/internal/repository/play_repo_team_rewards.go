package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

func (r *playRepository) ListTeamRewardContributions(
	ctx context.Context,
	teamID int64,
	start time.Time,
	end time.Time,
) (result []service.TeamContribution, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT m.user_id, COALESCE(SUM(ul.actual_cost), 0)::text
		FROM play_team_members m
		JOIN usage_logs ul
		  ON ul.user_id = m.user_id
		 AND ul.actual_cost > 0
		 AND ul.created_at >= $2
		 AND ul.created_at < $3
		 AND ul.created_at >= m.joined_at
		 AND (m.left_at IS NULL OR ul.created_at < m.left_at)
		WHERE m.team_id = $1
		  AND m.joined_at < $3
		  AND (m.left_at IS NULL OR m.left_at > $2)
		GROUP BY m.user_id
		ORDER BY m.user_id`, teamID, start, end)
	if err != nil {
		return nil, fmt.Errorf("list team reward contributions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	for rows.Next() {
		var contribution service.TeamContribution
		var amount string
		if err := rows.Scan(&contribution.UserID, &amount); err != nil {
			return nil, fmt.Errorf("scan team reward contribution: %w", err)
		}
		contribution.Amount, err = decimal.NewFromString(amount)
		if err != nil {
			return nil, fmt.Errorf("parse team reward contribution: %w", err)
		}
		result = append(result, contribution)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team reward contributions: %w", err)
	}
	return result, nil
}

func (r *playRepository) GetTeamRewardSettlementByTeamPeriod(
	ctx context.Context,
	teamID int64,
	periodStart time.Time,
) (*service.PlayTeamSettlement, error) {
	return getTeamRewardSettlementByTeamPeriod(ctx, r.sqlExec(ctx), teamID, periodStart)
}

func (r *playRepository) CreateTeamRewardSnapshot(
	ctx context.Context,
	settlement service.PlayTeamSettlement,
	allocations []service.PlayTeamRewardAllocation,
) (*service.PlayTeamSettlement, bool, error) {
	if dbent.TxFromContext(ctx) != nil {
		return r.createTeamRewardSnapshot(ctx, r.sqlExec(ctx), settlement, allocations)
	}
	if r.client == nil {
		return nil, false, fmt.Errorf("create team reward snapshot: ent client missing")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin team reward snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	snapshot, created, err := r.createTeamRewardSnapshot(
		txCtx,
		r.sqlExec(txCtx),
		settlement,
		allocations,
	)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("commit team reward snapshot tx: %w", err)
	}
	return snapshot, created, nil
}

func (r *playRepository) createTeamRewardSnapshot(
	ctx context.Context,
	exec sqlExecutor,
	settlement service.PlayTeamSettlement,
	allocations []service.PlayTeamRewardAllocation,
) (*service.PlayTeamSettlement, bool, error) {
	var settlementID int64
	err := scanSingleRow(ctx, exec, `
		INSERT INTO play_team_settlements (
			team_id, period_start, window_start, window_end, team_spend,
			reached_threshold, reward_rate, pool_amount, cap_amount, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending')
		ON CONFLICT (team_id, period_start) DO NOTHING
		RETURNING id`, []any{
		settlement.TeamID,
		settlement.PeriodStart.Format("2006-01-02"),
		settlement.WindowStart,
		settlement.WindowEnd,
		settlement.TeamSpend.StringFixed(8),
		settlement.ReachedThreshold.StringFixed(8),
		settlement.RewardRate.StringFixed(8),
		settlement.PoolAmount.StringFixed(8),
		settlement.CapAmount.StringFixed(8),
	}, &settlementID)
	if errors.Is(err, sql.ErrNoRows) {
		existing, getErr := getTeamRewardSettlementByTeamPeriod(
			ctx,
			exec,
			settlement.TeamID,
			settlement.PeriodStart,
		)
		if getErr != nil {
			return nil, false, getErr
		}
		if existing == nil {
			return nil, false, fmt.Errorf("team reward settlement conflict without existing row")
		}
		return existing, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("insert team reward settlement: %w", err)
	}

	for _, allocation := range allocations {
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO play_team_reward_allocations (
				settlement_id, user_id, contribution, ratio, reward_amount,
				payout_status, idempotency_key
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			settlementID,
			allocation.UserID,
			allocation.Contribution.StringFixed(8),
			allocation.Ratio.StringFixed(8),
			allocation.RewardAmount.StringFixed(8),
			service.PlayTeamRewardAllocationStatusPending,
			allocation.IdempotencyKey,
		); err != nil {
			return nil, false, fmt.Errorf("insert team reward allocation: %w", err)
		}
	}
	created, err := getTeamRewardSettlementByID(ctx, exec, settlementID)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

func (r *playRepository) WithTeamRewardSnapshotLock(
	ctx context.Context,
	teamID int64,
	fn func(context.Context) error,
) error {
	if fn == nil {
		return nil
	}
	if dbent.TxFromContext(ctx) != nil {
		if err := r.lockTeamRewardSnapshot(ctx, teamID); err != nil {
			return err
		}
		return fn(ctx)
	}
	if r.client == nil {
		return fmt.Errorf("team reward snapshot lock: ent client missing")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin team reward snapshot lock tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := r.lockTeamRewardSnapshot(txCtx, teamID); err != nil {
		return err
	}
	if err := fn(txCtx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit team reward snapshot lock tx: %w", err)
	}
	return nil
}

func (r *playRepository) lockTeamRewardSnapshot(ctx context.Context, teamID int64) error {
	var lockedID int64
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT id
		FROM play_teams
		WHERE id = $1
		FOR UPDATE`, []any{teamID}, &lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrPlayTeamNotFound
	}
	if err != nil {
		return fmt.Errorf("lock team reward snapshot: %w", err)
	}
	return nil
}

func (r *playRepository) GetTeamRewardSettlement(
	ctx context.Context,
	settlementID int64,
) (*service.PlayTeamSettlement, error) {
	return getTeamRewardSettlementByID(ctx, r.sqlExec(ctx), settlementID)
}

func (r *playRepository) ListUnpaidTeamRewardAllocations(
	ctx context.Context,
	settlementID int64,
) (result []service.PlayTeamRewardAllocation, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT a.id, a.settlement_id, a.user_id,
		       COALESCE(u.username, '') AS username,
		       COALESCE(u.email, '') AS email,
		       COALESCE(NULLIF(TRIM(ua.url), ''), '') AS avatar_url,
		       a.contribution::text, a.ratio::text,
		       a.reward_amount::text, a.payout_status, a.idempotency_key, a.paid_at, a.last_error
		FROM play_team_reward_allocations a
		JOIN users u ON u.id = a.user_id
		LEFT JOIN user_avatars ua ON ua.user_id = a.user_id
		WHERE a.settlement_id = $1
		  AND a.payout_status IN ('pending', 'failed')
		  AND a.reward_amount > 0
		ORDER BY a.user_id`, settlementID)
	if err != nil {
		return nil, fmt.Errorf("list unpaid team reward allocations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		allocation, scanErr := scanTeamRewardAllocation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *allocation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unpaid team reward allocations: %w", err)
	}
	return result, nil
}

func (r *playRepository) MarkTeamRewardSettlementProcessing(ctx context.Context, settlementID int64) error {
	res, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE play_team_settlements
		SET status = 'processing',
		    processing_started_at = COALESCE(processing_started_at, NOW()),
		    last_error = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND status <> 'completed'`, settlementID)
	if err != nil {
		return fmt.Errorf("mark team reward settlement processing: %w", err)
	}
	return requireRowsAffected(res, "mark team reward settlement processing")
}

func (r *playRepository) ClaimTeamRewardAllocation(ctx context.Context, allocationID int64) (bool, error) {
	var claimedID int64
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		UPDATE play_team_reward_allocations
		SET payout_status = 'processing', last_error = NULL, updated_at = NOW()
		WHERE id = $1
		  AND payout_status IN ('pending', 'failed')
		RETURNING id`, []any{allocationID}, &claimedID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim team reward allocation: %w", err)
	}
	return true, nil
}

func (r *playRepository) MarkTeamRewardAllocationPaid(ctx context.Context, allocationID int64) error {
	res, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE play_team_reward_allocations
		SET payout_status = 'paid', paid_at = NOW(), last_error = NULL, updated_at = NOW()
		WHERE id = $1
		  AND payout_status = 'processing'`, allocationID)
	if err != nil {
		return fmt.Errorf("mark team reward allocation paid: %w", err)
	}
	return requireRowsAffected(res, "mark team reward allocation paid")
}

func (r *playRepository) MarkTeamRewardAllocationFailed(
	ctx context.Context,
	allocationID int64,
	message string,
) error {
	res, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE play_team_reward_allocations
		SET payout_status = 'failed', paid_at = NULL, last_error = $2, updated_at = NOW()
		WHERE id = $1
		  AND payout_status = 'processing'`, allocationID, message)
	if err != nil {
		return fmt.Errorf("mark team reward allocation failed: %w", err)
	}
	return requireRowsAffected(res, "mark team reward allocation failed")
}

func (r *playRepository) RefreshTeamRewardSettlementStatus(
	ctx context.Context,
	settlementID int64,
) (*service.PlayTeamSettlement, error) {
	var total, paid, processing, failed int
	if err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT
			COUNT(*) FILTER (WHERE reward_amount > 0)::int,
			COUNT(*) FILTER (WHERE reward_amount > 0 AND payout_status = 'paid')::int,
			COUNT(*) FILTER (WHERE reward_amount > 0 AND payout_status = 'processing')::int,
			COUNT(*) FILTER (WHERE reward_amount > 0 AND payout_status = 'failed')::int
		FROM play_team_reward_allocations
		WHERE settlement_id = $1`, []any{settlementID}, &total, &paid, &processing, &failed); err != nil {
		return nil, fmt.Errorf("count team reward allocation statuses: %w", err)
	}

	status := service.PlayTeamSettlementStatusPending
	switch {
	case total > 0 && paid == total:
		status = service.PlayTeamSettlementStatusCompleted
	case paid > 0:
		status = service.PlayTeamSettlementStatusPartial
	case processing > 0:
		status = service.PlayTeamSettlementStatusProcessing
	case failed > 0:
		status = service.PlayTeamSettlementStatusFailed
	}
	if _, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE play_team_settlements
		SET status = $2,
		    completed_at = CASE WHEN $2 = 'completed' THEN COALESCE(completed_at, NOW()) ELSE NULL END,
		    updated_at = NOW()
		WHERE id = $1`, settlementID, status); err != nil {
		return nil, fmt.Errorf("refresh team reward settlement status: %w", err)
	}
	return r.GetTeamRewardSettlement(ctx, settlementID)
}

func (r *playRepository) ListTeamIDsForRewardMonth(
	ctx context.Context,
	start time.Time,
	end time.Time,
) (result []int64, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT DISTINCT team_id
		FROM play_team_members
		WHERE joined_at < $2
		  AND (left_at IS NULL OR left_at > $1)
		ORDER BY team_id`, start, end)
	if err != nil {
		return nil, fmt.Errorf("list team IDs for reward month: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		var teamID int64
		if err := rows.Scan(&teamID); err != nil {
			return nil, fmt.Errorf("scan team ID for reward month: %w", err)
		}
		result = append(result, teamID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team IDs for reward month: %w", err)
	}
	return result, nil
}

func (r *playRepository) ListTeamRewardSettlementsByTeam(
	ctx context.Context,
	teamID int64,
	limit int,
) ([]service.PlayTeamSettlement, error) {
	return listTeamRewardSettlements(ctx, r.sqlExec(ctx), `
		SELECT id, team_id, period_start, window_start, window_end,
		       team_spend::text, reached_threshold::text, reward_rate::text,
		       pool_amount::text, cap_amount::text, status, last_error,
		       processing_started_at, completed_at
		FROM play_team_settlements
		WHERE team_id = $1
		ORDER BY period_start DESC, id DESC
		LIMIT $2`, teamID, normalizeTeamRewardListLimit(limit))
}

func (r *playRepository) ListTeamRewardSettlements(
	ctx context.Context,
	limit int,
) ([]service.PlayTeamSettlement, error) {
	return listTeamRewardSettlements(ctx, r.sqlExec(ctx), `
		SELECT id, team_id, period_start, window_start, window_end,
		       team_spend::text, reached_threshold::text, reward_rate::text,
		       pool_amount::text, cap_amount::text, status, last_error,
		       processing_started_at, completed_at
		FROM play_team_settlements
		ORDER BY period_start DESC, id DESC
		LIMIT $1`, normalizeTeamRewardListLimit(limit))
}

func (r *playRepository) ListTeamRewardAllocations(
	ctx context.Context,
	settlementID int64,
) (result []service.PlayTeamRewardAllocation, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT a.id, a.settlement_id, a.user_id,
		       COALESCE(u.username, '') AS username,
		       COALESCE(u.email, '') AS email,
		       COALESCE(NULLIF(TRIM(ua.url), ''), '') AS avatar_url,
		       a.contribution::text, a.ratio::text,
		       a.reward_amount::text, a.payout_status, a.idempotency_key, a.paid_at, a.last_error
		FROM play_team_reward_allocations a
		JOIN users u ON u.id = a.user_id
		LEFT JOIN user_avatars ua ON ua.user_id = a.user_id
		WHERE a.settlement_id = $1
		ORDER BY a.user_id`, settlementID)
	if err != nil {
		return nil, fmt.Errorf("list team reward allocations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		allocation, scanErr := scanTeamRewardAllocation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *allocation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team reward allocations: %w", err)
	}
	return result, nil
}

func (r *playRepository) ListUserTeamRewardSettlements(
	ctx context.Context,
	userID int64,
	limit int,
) (result []service.PlayUserTeamSettlementRecord, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT s.id, s.team_id, t.name, s.period_start, s.window_start, s.window_end,
		       s.team_spend::text, s.reached_threshold::text, s.reward_rate::text,
		       s.pool_amount::text, s.cap_amount::text, s.status, s.last_error,
		       s.processing_started_at, s.completed_at,
		       a.id, a.settlement_id, a.user_id, a.contribution::text, a.ratio::text,
		       a.reward_amount::text, a.payout_status, a.paid_at, a.last_error
		FROM play_team_reward_allocations a
		JOIN play_team_settlements s ON s.id = a.settlement_id
		JOIN play_teams t ON t.id = s.team_id
		WHERE a.user_id = $1
		  AND a.reward_amount > 0
		ORDER BY s.period_start DESC, a.id DESC
		LIMIT $2`, userID, normalizeTeamRewardListLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list user team reward settlements: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		record, scanErr := scanUserTeamRewardSettlementRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user team reward settlements: %w", err)
	}
	return result, nil
}

func listTeamRewardSettlements(
	ctx context.Context,
	exec sqlExecutor,
	query string,
	args ...any,
) (result []service.PlayTeamSettlement, err error) {
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list team reward settlements: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		settlement, scanErr := scanTeamRewardSettlement(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *settlement)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team reward settlements: %w", err)
	}
	return result, nil
}

func normalizeTeamRewardListLimit(limit int) int {
	if limit <= 0 || limit > 200 {
		return 50
	}
	return limit
}

func getTeamRewardSettlementByTeamPeriod(
	ctx context.Context,
	exec sqlExecutor,
	teamID int64,
	periodStart time.Time,
) (*service.PlayTeamSettlement, error) {
	return scanTeamRewardSettlementQuery(ctx, exec, `
		SELECT id, team_id, period_start, window_start, window_end,
		       team_spend::text, reached_threshold::text, reward_rate::text,
		       pool_amount::text, cap_amount::text, status, last_error,
		       processing_started_at, completed_at
		FROM play_team_settlements
		WHERE team_id = $1 AND period_start = $2`,
		teamID,
		periodStart.Format("2006-01-02"),
	)
}

func getTeamRewardSettlementByID(
	ctx context.Context,
	exec sqlExecutor,
	settlementID int64,
) (*service.PlayTeamSettlement, error) {
	return scanTeamRewardSettlementQuery(ctx, exec, `
		SELECT id, team_id, period_start, window_start, window_end,
		       team_spend::text, reached_threshold::text, reward_rate::text,
		       pool_amount::text, cap_amount::text, status, last_error,
		       processing_started_at, completed_at
		FROM play_team_settlements
		WHERE id = $1`, settlementID)
}

func scanTeamRewardSettlementQuery(
	ctx context.Context,
	exec sqlExecutor,
	query string,
	args ...any,
) (*service.PlayTeamSettlement, error) {
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	settlement, err := scanTeamRewardSettlement(rows)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get team reward settlement: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return settlement, nil
}

func scanTeamRewardSettlement(scan rowScanner) (*service.PlayTeamSettlement, error) {
	var settlement service.PlayTeamSettlement
	var teamSpend, threshold, rate, pool, cap string
	var lastError sql.NullString
	var processingAt, completedAt sql.NullTime
	if err := scan.Scan(
		&settlement.ID,
		&settlement.TeamID,
		&settlement.PeriodStart,
		&settlement.WindowStart,
		&settlement.WindowEnd,
		&teamSpend,
		&threshold,
		&rate,
		&pool,
		&cap,
		&settlement.Status,
		&lastError,
		&processingAt,
		&completedAt,
	); err != nil {
		return nil, fmt.Errorf("scan team reward settlement: %w", err)
	}
	values := []*decimal.Decimal{
		&settlement.TeamSpend,
		&settlement.ReachedThreshold,
		&settlement.RewardRate,
		&settlement.PoolAmount,
		&settlement.CapAmount,
	}
	raw := []string{teamSpend, threshold, rate, pool, cap}
	for i := range values {
		parsed, parseErr := decimal.NewFromString(raw[i])
		if parseErr != nil {
			return nil, fmt.Errorf("parse team reward settlement decimal: %w", parseErr)
		}
		*values[i] = parsed
	}
	settlement.LastError = lastError.String
	if processingAt.Valid {
		settlement.ProcessingStartedAt = &processingAt.Time
	}
	if completedAt.Valid {
		settlement.CompletedAt = &completedAt.Time
	}
	return &settlement, nil
}

func scanTeamRewardAllocation(scan rowScanner) (*service.PlayTeamRewardAllocation, error) {
	var allocation service.PlayTeamRewardAllocation
	var username, email string
	var contribution, ratio, reward string
	var paidAt sql.NullTime
	var lastError sql.NullString
	if err := scan.Scan(
		&allocation.ID,
		&allocation.SettlementID,
		&allocation.UserID,
		&username,
		&email,
		&allocation.AvatarURL,
		&contribution,
		&ratio,
		&reward,
		&allocation.PayoutStatus,
		&allocation.IdempotencyKey,
		&paidAt,
		&lastError,
	); err != nil {
		return nil, fmt.Errorf("scan team reward allocation: %w", err)
	}
	allocation.DisplayName = service.PublicPlayDisplayName(username, email, allocation.UserID)
	allocation.Email = email
	var err error
	if allocation.Contribution, err = decimal.NewFromString(contribution); err != nil {
		return nil, fmt.Errorf("parse team reward allocation contribution: %w", err)
	}
	if allocation.Ratio, err = decimal.NewFromString(ratio); err != nil {
		return nil, fmt.Errorf("parse team reward allocation ratio: %w", err)
	}
	if allocation.RewardAmount, err = decimal.NewFromString(reward); err != nil {
		return nil, fmt.Errorf("parse team reward allocation amount: %w", err)
	}
	if paidAt.Valid {
		allocation.PaidAt = &paidAt.Time
	}
	allocation.LastError = lastError.String
	return &allocation, nil
}

func scanUserTeamRewardSettlementRecord(scan rowScanner) (*service.PlayUserTeamSettlementRecord, error) {
	var record service.PlayUserTeamSettlementRecord
	var (
		teamSpend     string
		threshold     string
		rate          string
		pool          string
		capAmount     string
		settleError   sql.NullString
		processingAt  sql.NullTime
		completedAt   sql.NullTime
		contribution  string
		ratio         string
		reward        string
		paidAt        sql.NullTime
		allocationErr sql.NullString
	)
	if err := scan.Scan(
		&record.Settlement.ID,
		&record.Settlement.TeamID,
		&record.TeamName,
		&record.Settlement.PeriodStart,
		&record.Settlement.WindowStart,
		&record.Settlement.WindowEnd,
		&teamSpend,
		&threshold,
		&rate,
		&pool,
		&capAmount,
		&record.Settlement.Status,
		&settleError,
		&processingAt,
		&completedAt,
		&record.Allocation.ID,
		&record.Allocation.SettlementID,
		&record.Allocation.UserID,
		&contribution,
		&ratio,
		&reward,
		&record.Allocation.PayoutStatus,
		&paidAt,
		&allocationErr,
	); err != nil {
		return nil, fmt.Errorf("scan user team reward settlement: %w", err)
	}
	settlementValues := []*decimal.Decimal{
		&record.Settlement.TeamSpend,
		&record.Settlement.ReachedThreshold,
		&record.Settlement.RewardRate,
		&record.Settlement.PoolAmount,
		&record.Settlement.CapAmount,
	}
	for i, raw := range []string{teamSpend, threshold, rate, pool, capAmount} {
		parsed, parseErr := decimal.NewFromString(raw)
		if parseErr != nil {
			return nil, fmt.Errorf("parse user team reward settlement decimal: %w", parseErr)
		}
		*settlementValues[i] = parsed
	}
	allocationValues := []*decimal.Decimal{
		&record.Allocation.Contribution,
		&record.Allocation.Ratio,
		&record.Allocation.RewardAmount,
	}
	for i, raw := range []string{contribution, ratio, reward} {
		parsed, parseErr := decimal.NewFromString(raw)
		if parseErr != nil {
			return nil, fmt.Errorf("parse user team reward allocation decimal: %w", parseErr)
		}
		*allocationValues[i] = parsed
	}
	record.Settlement.LastError = settleError.String
	if processingAt.Valid {
		record.Settlement.ProcessingStartedAt = &processingAt.Time
	}
	if completedAt.Valid {
		record.Settlement.CompletedAt = &completedAt.Time
	}
	if paidAt.Valid {
		record.Allocation.PaidAt = &paidAt.Time
	}
	record.Allocation.LastError = allocationErr.String
	return &record, nil
}

func requireRowsAffected(result sql.Result, operation string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", operation, err)
	}
	if rows == 0 {
		return fmt.Errorf("%s: row not found or state changed", operation)
	}
	return nil
}
