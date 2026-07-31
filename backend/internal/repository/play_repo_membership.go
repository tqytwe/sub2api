package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

func (r *playRepository) GetMembershipPaidTotal(ctx context.Context, userID int64) (float64, error) {
	var total string
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT COALESCE(SUM(net_amount), 0)::text
		FROM play_membership_order_contributions WHERE user_id = $1`, []any{userID}, &total)
	if err != nil {
		return 0, fmt.Errorf("get membership paid total: %w", err)
	}
	value, err := decimal.NewFromString(total)
	if err != nil {
		return 0, fmt.Errorf("parse membership paid total: %w", err)
	}
	return value.InexactFloat64(), nil
}

func (r *playRepository) SyncMembershipOrderContribution(ctx context.Context, orderID, userID int64, orderType string, paidAmount, refundAmount float64, paidAt *time.Time, status string) error {
	if paidAmount < 0 {
		paidAmount = 0
	}
	if refundAmount < 0 {
		refundAmount = 0
	}
	if refundAmount > paidAmount {
		refundAmount = paidAmount
	}
	_, err := r.sqlExec(ctx).ExecContext(ctx, `
		INSERT INTO play_membership_order_contributions
		(order_id, user_id, order_type, paid_amount, refund_amount, net_amount, paid_at, status, processed_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,GREATEST($4-$5,0),$6,$7,NOW(),NOW())
		ON CONFLICT (order_id) DO UPDATE SET
			user_id=EXCLUDED.user_id, order_type=EXCLUDED.order_type,
			paid_amount=EXCLUDED.paid_amount, refund_amount=EXCLUDED.refund_amount,
			net_amount=GREATEST(EXCLUDED.paid_amount-EXCLUDED.refund_amount,0),
			paid_at=EXCLUDED.paid_at, status=EXCLUDED.status, updated_at=NOW()`,
		orderID, userID, orderType, paidAmount, refundAmount, paidAt, status)
	if err != nil {
		return fmt.Errorf("sync membership contribution: %w", err)
	}
	return nil
}

func (r *playRepository) ListTeamLeaderboardBase(ctx context.Context, start, end time.Time, limit int) ([]service.PlayTeamLeaderboardBase, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT t.id, t.name, COUNT(DISTINCT m.user_id)::int,
		       COALESCE(SUM(ul.actual_cost),0)::text
		FROM play_teams t
		JOIN play_team_members m ON m.team_id=t.id
		LEFT JOIN usage_logs ul ON ul.user_id=m.user_id
		  AND ul.actual_cost > 0 AND ul.created_at >= $1 AND ul.created_at < $2
		  AND ul.created_at >= m.joined_at
		  AND (m.left_at IS NULL OR ul.created_at < m.left_at)
		WHERE t.archived_at IS NULL AND m.joined_at < $2
		  AND (m.left_at IS NULL OR m.left_at > $1)
		GROUP BY t.id, t.name
		ORDER BY COALESCE(SUM(ul.actual_cost),0) DESC, t.id ASC
		LIMIT $3`, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("list team leaderboard: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []service.PlayTeamLeaderboardBase
	for rows.Next() {
		var row service.PlayTeamLeaderboardBase
		var spend string
		if err := rows.Scan(&row.TeamID, &row.TeamName, &row.MemberCount, &spend); err != nil {
			return nil, err
		}
		row.Spend, err = decimal.NewFromString(spend)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *playRepository) GetTeamLeaderboardRank(ctx context.Context, teamID int64, start, end time.Time) (int, int, decimal.Decimal, error) {
	var rank, total int
	var previousSpendRaw string
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
			WITH board AS (
				SELECT t.id, COALESCE(SUM(ul.actual_cost),0) AS spend,
				 ROW_NUMBER() OVER (ORDER BY COALESCE(SUM(ul.actual_cost),0) DESC, t.id ASC)::int AS rank
			FROM play_teams t JOIN play_team_members m ON m.team_id=t.id
			LEFT JOIN usage_logs ul ON ul.user_id=m.user_id AND ul.actual_cost>0
			 AND ul.created_at >= $2 AND ul.created_at < $3 AND ul.created_at >= m.joined_at
			 AND (m.left_at IS NULL OR ul.created_at < m.left_at)
			WHERE t.archived_at IS NULL AND m.joined_at < $3 AND (m.left_at IS NULL OR m.left_at > $2)
			GROUP BY t.id
		), totals AS (SELECT COUNT(*)::int AS total FROM board)
			SELECT board.rank, totals.total,
			 COALESCE((SELECT previous.spend FROM board previous WHERE previous.rank=board.rank-1),board.spend)::text
			FROM board CROSS JOIN totals WHERE board.id=$1`, []any{teamID, start, end}, &rank, &total, &previousSpendRaw)
	if err != nil {
		return 0, 0, decimal.Zero, fmt.Errorf("get team leaderboard rank: %w", err)
	}
	previousSpend, err := decimal.NewFromString(previousSpendRaw)
	if err != nil {
		return 0, 0, decimal.Zero, fmt.Errorf("parse previous team spend: %w", err)
	}
	return rank, total, previousSpend, nil
}

func (r *playRepository) MembershipAdminOverview(ctx context.Context, memberThreshold float64) (int, decimal.Decimal, error) {
	var total int
	var netPaid string
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT COUNT(*) FILTER (WHERE total_paid >= $1)::int,
		       COALESCE(SUM(total_paid), 0)::text
		FROM (SELECT user_id, COALESCE(SUM(net_amount), 0) AS total_paid
		      FROM play_membership_order_contributions GROUP BY user_id) totals`, []any{memberThreshold}, &total, &netPaid)
	if err != nil {
		return 0, decimal.Zero, fmt.Errorf("get membership admin overview: %w", err)
	}
	amount, err := decimal.NewFromString(netPaid)
	if err != nil {
		return 0, decimal.Zero, fmt.Errorf("parse membership admin overview: %w", err)
	}
	return total, amount, nil
}

func (r *playRepository) ListMembershipAdminRows(ctx context.Context, query string, memberOnly *bool, memberThreshold float64, page, pageSize int) ([]service.PlayMembershipAdminRow, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	query = strings.TrimSpace(query)
	exec := r.sqlExec(ctx)
	args := make([]any, 0, 4)
	where := []string{"1=1"}
	if memberOnly != nil {
		args = append(args, memberThreshold)
		if *memberOnly {
			where = append(where, "COALESCE(totals.total_paid,0) >= $1")
		} else {
			where = append(where, "COALESCE(totals.total_paid,0) < $1")
		}
	}
	if query != "" {
		args = append(args, "%"+query+"%")
		placeholder := "$" + fmt.Sprint(len(args))
		where = append(where, "(u.email ILIKE "+placeholder+" OR u.username ILIKE "+placeholder+")")
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	baseJoin := ` FROM users u LEFT JOIN (
		SELECT user_id, COALESCE(SUM(net_amount),0) AS total_paid
		FROM play_membership_order_contributions GROUP BY user_id
	) totals ON totals.user_id=u.id WHERE ` + whereSQL
	countSQL := `SELECT COUNT(*)` + baseJoin
	countArgs := append([]any(nil), args...)
	if err := scanSingleRow(ctx, exec, countSQL, countArgs, &total); err != nil {
		return nil, 0, fmt.Errorf("count membership admin rows: %w", err)
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := exec.QueryContext(ctx, `
			SELECT u.id, COALESCE(u.email,''), COALESCE(u.username,''), COALESCE(totals.total_paid,0)::text,
			       u.created_at,
		       (SELECT MIN(paid_at) FROM play_membership_order_contributions c WHERE c.user_id=u.id AND c.net_amount>0),
		       (SELECT MAX(paid_at) FROM play_membership_order_contributions c WHERE c.user_id=u.id AND c.net_amount>0)
			`+baseJoin+` ORDER BY COALESCE(totals.total_paid,0) DESC, u.id ASC LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list membership admin rows: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PlayMembershipAdminRow, 0)
	for rows.Next() {
		var item service.PlayMembershipAdminRow
		var paid string
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &paid, &item.RegisteredAt, &item.FirstPaidAt, &item.LastPaidAt); err != nil {
			return nil, 0, err
		}
		item.NetPaid, err = decimal.NewFromString(paid)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (r *playRepository) ListMembershipPaidTotals(ctx context.Context) (map[int64]decimal.Decimal, error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `SELECT u.id, COALESCE(SUM(c.net_amount),0)::text FROM users u LEFT JOIN play_membership_order_contributions c ON c.user_id=u.id GROUP BY u.id`)
	if err != nil {
		return nil, fmt.Errorf("list membership paid totals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int64]decimal.Decimal)
	for rows.Next() {
		var userID int64
		var raw string
		if err := rows.Scan(&userID, &raw); err != nil {
			return nil, err
		}
		value, err := decimal.NewFromString(raw)
		if err != nil {
			return nil, err
		}
		out[userID] = value
	}
	return out, rows.Err()
}

func (r *playRepository) GetMembershipAdminRow(ctx context.Context, userID int64) (*service.PlayMembershipAdminRow, error) {
	var row service.PlayMembershipAdminRow
	var paid string
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT u.id, COALESCE(u.email,''), COALESCE(u.username,''), COALESCE(SUM(c.net_amount),0)::text,
		       u.created_at, MIN(c.paid_at) FILTER (WHERE c.net_amount>0), MAX(c.paid_at) FILTER (WHERE c.net_amount>0)
		FROM users u LEFT JOIN play_membership_order_contributions c ON c.user_id=u.id
		WHERE u.id=$1 GROUP BY u.id`, []any{userID}, &row.UserID, &row.Email, &row.Username, &paid, &row.RegisteredAt, &row.FirstPaidAt, &row.LastPaidAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get membership admin row: %w", err)
	}
	row.NetPaid, err = decimal.NewFromString(paid)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *playRepository) ListMembershipContributions(ctx context.Context, userID int64, limit int) ([]service.PlayMembershipContribution, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `SELECT order_id,order_type,paid_amount::text,refund_amount::text,net_amount::text,paid_at,status,updated_at FROM play_membership_order_contributions WHERE user_id=$1 ORDER BY COALESCE(paid_at,updated_at) DESC,order_id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PlayMembershipContribution, 0)
	for rows.Next() {
		var item service.PlayMembershipContribution
		var paid, refunded, net string
		if err := rows.Scan(&item.OrderID, &item.OrderType, &paid, &refunded, &net, &item.PaidAt, &item.Status, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.PaidAmount, err = decimal.NewFromString(paid)
		if err != nil {
			return nil, err
		}
		item.RefundAmount, err = decimal.NewFromString(refunded)
		if err != nil {
			return nil, err
		}
		item.NetAmount, err = decimal.NewFromString(net)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *playRepository) ListMembershipTierHistory(ctx context.Context, userID int64, limit int) ([]service.PlayMembershipTierChange, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	query := `SELECT user_id,order_id,from_tier,to_tier,net_paid_before::text,net_paid_after::text,reason,created_at FROM play_membership_tier_history`
	args := []any{limit}
	if userID > 0 {
		query += ` WHERE user_id=$1`
		args = []any{userID, limit}
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT $` + fmt.Sprint(len(args))
	rows, err := r.sqlExec(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PlayMembershipTierChange, 0)
	for rows.Next() {
		var item service.PlayMembershipTierChange
		var before, after string
		if err := rows.Scan(&item.UserID, &item.OrderID, &item.FromTier, &item.ToTier, &before, &after, &item.Reason, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.NetPaidBefore, err = decimal.NewFromString(before)
		if err != nil {
			return nil, err
		}
		item.NetPaidAfter, err = decimal.NewFromString(after)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *playRepository) RecordMembershipTierChange(ctx context.Context, change service.PlayMembershipTierChange) error {
	_, err := r.sqlExec(ctx).ExecContext(ctx, `INSERT INTO play_membership_tier_history (user_id,order_id,from_tier,to_tier,net_paid_before,net_paid_after,reason) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (order_id,from_tier,to_tier) WHERE order_id IS NOT NULL DO NOTHING`, change.UserID, change.OrderID, change.FromTier, change.ToTier, change.NetPaidBefore, change.NetPaidAfter, change.Reason)
	return err
}

func (r *playRepository) CountRecentMembershipTierChanges(ctx context.Context, since time.Time) (int, int, error) {
	var upgrades, downgrades int
	err := scanSingleRow(ctx, r.sqlExec(ctx), `SELECT COUNT(*) FILTER (WHERE to_tier>from_tier)::int,COUNT(*) FILTER (WHERE to_tier<from_tier)::int FROM play_membership_tier_history WHERE created_at >= $1`, []any{since}, &upgrades, &downgrades)
	return upgrades, downgrades, err
}

func (r *playRepository) GetVIPConfigVersion(ctx context.Context) (int64, error) {
	var version int64
	if err := scanSingleRow(ctx, r.sqlExec(ctx), `SELECT COALESCE(MAX(version),0) FROM play_vip_config_versions`, nil, &version); err != nil {
		return 0, err
	}
	return version, nil
}

func (r *playRepository) PublishVIPConfig(ctx context.Context, expectedVersion, actorID int64, reason, tiersJSON string, affected, upgraded, downgraded int, changes []service.PlayMembershipTierChange) (int64, error) {
	if dbent.TxFromContext(ctx) != nil {
		return r.publishVIPConfig(ctx, r.sqlExec(ctx), expectedVersion, actorID, reason, tiersJSON, affected, upgraded, downgraded, changes)
	}
	if r.client == nil {
		return 0, fmt.Errorf("publish vip config: ent client missing")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	version, err := r.publishVIPConfig(txCtx, r.sqlExec(txCtx), expectedVersion, actorID, reason, tiersJSON, affected, upgraded, downgraded, changes)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return version, nil
}

func (r *playRepository) publishVIPConfig(ctx context.Context, exec sqlExecutor, expectedVersion, actorID int64, reason, tiersJSON string, affected, upgraded, downgraded int, changes []service.PlayMembershipTierChange) (int64, error) {
	var lockAcquired bool
	if err := scanSingleRow(ctx, exec, `SELECT pg_advisory_xact_lock(hashtext('play_vip_config_versions')) IS NULL`, nil, &lockAcquired); err != nil {
		return 0, err
	}
	var current int64
	if err := scanSingleRow(ctx, exec, `SELECT COALESCE(MAX(version),0) FROM play_vip_config_versions`, nil, &current); err != nil {
		return 0, err
	}
	if current != expectedVersion {
		return 0, service.ErrPlayVIPConfigConflict
	}
	if _, err := exec.ExecContext(ctx, `INSERT INTO settings (key,value,updated_at) VALUES ('play_vip_tiers',$1,NOW()) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`, tiersJSON); err != nil {
		return 0, err
	}
	var version int64
	if err := scanSingleRow(ctx, exec, `INSERT INTO play_vip_config_versions (tiers_json,actor_admin_id,reason,affected_users,upgraded_users,downgraded_users) VALUES ($1::jsonb,NULLIF($2,0),$3,$4,$5,$6) RETURNING version`, []any{tiersJSON, actorID, reason, affected, upgraded, downgraded}, &version); err != nil {
		return 0, err
	}
	for _, change := range changes {
		if _, err := exec.ExecContext(ctx, `INSERT INTO play_membership_tier_history (user_id,from_tier,to_tier,net_paid_before,net_paid_after,reason) VALUES ($1,$2,$3,$4,$5,'tier_config')`, change.UserID, change.FromTier, change.ToTier, change.NetPaidBefore, change.NetPaidAfter); err != nil {
			return 0, err
		}
	}
	return version, nil
}

func (r *playRepository) AppAnalytics(ctx context.Context, from, to time.Time, version, channel string) (scans, downloadRedirects, firstLaunches, registeredInstalls, activeUsers, dau, wau, mau int64, funnel []map[string]any, versions []map[string]any, err error) {
	exec := r.sqlExec(ctx)
	where := []string{"e.occurred_at >= $1", "e.occurred_at < $2"}
	args := []any{from, to}
	if version != "" {
		args = append(args, version)
		where = append(where, "e.app_version = $"+fmt.Sprint(len(args)))
	}
	if channel != "" {
		args = append(args, channel)
		where = append(where, "e.metadata->>'channel' = $"+fmt.Sprint(len(args)))
	}
	whereSQL := strings.Join(where, " AND ")
	rows, err := exec.QueryContext(ctx, `SELECT CASE WHEN event_type='share' THEN COALESCE(metadata->>'event_name','share') ELSE event_type END,
		COUNT(DISTINCT CASE WHEN e.verified AND e.user_id IS NOT NULL THEN 'user:'||e.user_id::text ELSE 'installation:'||e.installation_id::text END)
		FROM mobile_attribution_events e WHERE `+whereSQL+` AND ((e.event_type IN ('click','download','open') AND NOT e.verified) OR (e.verified AND e.user_id IS NOT NULL)) GROUP BY 1`, args...)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, nil, nil, fmt.Errorf("app analytics funnel: %w", err)
	}
	counts := map[string]int64{}
	for rows.Next() {
		var event string
		var count int64
		if err := rows.Scan(&event, &count); err != nil {
			_ = rows.Close()
			return 0, 0, 0, 0, 0, 0, 0, 0, nil, nil, err
		}
		counts[event] = count
	}
	if completed := counts["share_completed"]; completed > 0 {
		counts["share"] = completed
	}
	if err := rows.Close(); err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, nil, nil, err
	}
	scans = counts["click"]
	downloadRedirects = counts["download"]
	firstLaunches = counts["open"]
	registeredInstalls = counts["register"]
	activeUsers = counts["active"]
	activityWhere := []string{"e.event_type IN ('active','login')", "e.verified", "e.user_id IS NOT NULL"}
	activityArgs := make([]any, 0, 4)
	if version != "" {
		activityArgs = append(activityArgs, version)
		activityWhere = append(activityWhere, "e.app_version = $"+fmt.Sprint(len(activityArgs)))
	}
	if channel != "" {
		activityArgs = append(activityArgs, channel)
		activityWhere = append(activityWhere, "e.metadata->>'channel' = $"+fmt.Sprint(len(activityArgs)))
	}
	for days, target := range map[int]*int64{1: &dau, 7: &wau, 30: &mau} {
		windowArgs := append([]any(nil), activityArgs...)
		windowArgs = append(windowArgs, to.Add(-time.Duration(days)*24*time.Hour), to)
		windowWhere := append([]string(nil), activityWhere...)
		windowWhere = append(windowWhere, "e.occurred_at >= $"+fmt.Sprint(len(windowArgs)-1), "e.occurred_at < $"+fmt.Sprint(len(windowArgs)))
		if err := scanSingleRow(ctx, exec, `SELECT COUNT(DISTINCT user_id) FROM mobile_attribution_events e WHERE `+strings.Join(windowWhere, " AND "), windowArgs, target); err != nil {
			return 0, 0, 0, 0, 0, 0, 0, 0, nil, nil, err
		}
	}
	previous := int64(0)
	for _, event := range []string{"click", "download", "open", "register", "login", "share"} {
		verification := "verified"
		if event == "click" || event == "download" || event == "open" {
			verification = "unverified"
		}
		item := map[string]any{"event": event, "count": counts[event], "verification": verification}
		if previous > 0 {
			item["conversion_rate"] = float64(counts[event]) * 100 / float64(previous)
		}
		funnel = append(funnel, item)
		previous = counts[event]
	}
	versionRows, err := exec.QueryContext(ctx, `SELECT app_version, platform, COUNT(DISTINCT user_id) FROM mobile_attribution_events e WHERE `+whereSQL+` AND e.verified AND e.user_id IS NOT NULL GROUP BY app_version,platform ORDER BY COUNT(DISTINCT user_id) DESC`, args...)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, nil, nil, err
	}
	defer func() { _ = versionRows.Close() }()
	for versionRows.Next() {
		var v, p string
		var count int64
		if err := versionRows.Scan(&v, &p, &count); err != nil {
			return 0, 0, 0, 0, 0, 0, 0, 0, nil, nil, err
		}
		versions = append(versions, map[string]any{"version": v, "platform": p, "active": count})
	}
	return scans, downloadRedirects, firstLaunches, registeredInstalls, activeUsers, dau, wau, mau, funnel, versions, versionRows.Err()
}
