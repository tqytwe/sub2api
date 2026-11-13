package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) ListAdminCheckins(ctx context.Context, filter service.PlayAdminCheckinFilter) (*service.PlayAdminCheckinPage, error) {
	// Total and rows come from one PostgreSQL statement/snapshot, including
	// empty pages. The as-of fence prevents new check-ins moving later pages.
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
        WITH filtered AS MATERIALIZED (
            SELECT c.*,u.email,COALESCE(u.username,'') AS username,COALESCE(s.reward_mode,'unknown') AS reward_mode,
                   COALESCE(s.primary_reason,'unknown') AS eligibility_reason
            FROM play_checkins c JOIN users u ON u.id=c.user_id
            LEFT JOIN play_growth_eligibility_snapshots s ON s.id=c.growth_eligibility_snapshot_id AND s.user_id=c.user_id
            WHERE c.checkin_date >= $1::date AND c.checkin_date <= $2::date
              AND ($3::bigint=0 OR c.user_id=$3)
              AND ($4='' OR COALESCE(s.reward_mode,'unknown')=$4)
              AND c.created_at <= $5::timestamptz
              AND ($8='' OR strpos(lower(u.email),lower($8))>0 OR strpos(lower(COALESCE(u.username,'')),lower($8))>0)
        ), selected AS (
            SELECT * FROM filtered ORDER BY checkin_date DESC,id DESC LIMIT $6 OFFSET $7
        ), details AS (
            SELECT c.id,c.user_id,c.email,c.checkin_date::text,c.streak_count,
                   consecutive.n AS observed_streak,c.reward_amount::text,
                   money.amount::text AS ledger_amount,energy.amount AS growth_energy,
                   c.reward_mode,c.eligibility_reason,c.growth_eligibility_snapshot_id,c.created_at,
                   reconciliation.amount::text,reconciliation.latest_id,reconciliation.streak,
                   c.username,consecutive.start_day::text,consecutive.at_checkin,previous.day::text,
                   CASE WHEN previous.day IS NOT NULL THEN (previous.day+1)::text END AS gap_start,
                   CASE WHEN previous.day IS NOT NULL THEN (consecutive.start_day-1)::text END AS gap_end,
                   consecutive.late_days,consecutive.evidence,
                   money.daily::text,money.milestone::text
            FROM selected c
            CROSS JOIN LATERAL (
                WITH RECURSIVE dates AS (
                    SELECT c.checkin_date AS day,c.created_at
                    UNION ALL
                    SELECT p.checkin_date,p.created_at FROM play_checkins p JOIN dates d ON p.checkin_date=d.day-1
                    WHERE p.user_id=c.user_id AND p.created_at <= $5::timestamptz
                ) SELECT COUNT(*)::int AS n,MIN(day) AS start_day,
                  COALESCE(MIN(c.checkin_date-day) FILTER (WHERE created_at>c.created_at),COUNT(*)::int) AS at_checkin,
                  COUNT(*) FILTER (WHERE created_at>c.created_at)::int AS late_days,
                  jsonb_agg(jsonb_build_object('date',day::text,'created_at',created_at) ORDER BY day) AS evidence
                  FROM dates
            ) consecutive
            CROSS JOIN LATERAL (
                SELECT MAX(p.checkin_date) AS day FROM play_checkins p
                WHERE p.user_id=c.user_id AND p.checkin_date<consecutive.start_day
                  AND p.created_at <= $5::timestamptz
            ) previous
            CROSS JOIN LATERAL (
                SELECT COALESCE(SUM(l.amount),0) AS amount,
                       COALESCE(SUM(l.amount) FILTER (WHERE l.source IN ('checkin','checkin_makeup')),0) AS daily,
                       COALESCE(SUM(l.amount) FILTER (WHERE l.source='checkin_milestone'),0) AS milestone
                FROM play_reward_ledger l
                WHERE l.user_id=c.user_id AND l.source IN ('checkin','checkin_makeup','checkin_milestone')
                  AND l.detail->>'checkin_date'=c.checkin_date::text AND l.created_at <= $5::timestamptz
            ) money
            CROSS JOIN LATERAL (
                SELECT COALESCE(SUM(e.amount),0)::bigint AS amount FROM play_growth_energy_ledger e
                WHERE e.eligibility_snapshot_id=c.growth_eligibility_snapshot_id AND e.user_id=c.user_id
                  AND e.created_at <= $5::timestamptz
            ) energy
            CROSS JOIN LATERAL (
                SELECT COALESCE(SUM(a.milestone_added),0) AS amount,MAX(a.id) AS latest_id,
                       MAX(a.observed_streak) AS streak FROM play_checkin_reconciliations a
                WHERE a.checkin_id=c.id AND a.created_at <= $5::timestamptz
            ) reconciliation
        ) SELECT totals.n,d.* FROM (SELECT COUNT(*)::int AS n FROM filtered) totals
          LEFT JOIN details d ON TRUE ORDER BY d.checkin_date DESC,d.id DESC`,
		filter.StartDate, filter.EndDate, filter.UserID, filter.RewardMode, filter.AsOf, filter.PageSize, (filter.Page-1)*filter.PageSize, filter.Account)
	if err != nil {
		return nil, fmt.Errorf("list admin check-ins: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := &service.PlayAdminCheckinPage{Items: []service.PlayAdminCheckinRecord{}}
	for rows.Next() {
		var id, userID, stored, observed, energy, snapshot, reconciliationID, reconciledStreak sql.NullInt64
		var email, date, reward, ledger, mode, reason, adjustment sql.NullString
		var created sql.NullTime
		var username, startDay, previousDay, gapStart, gapEnd, daily, milestone sql.NullString
		var atCheckin, lateDays sql.NullInt64
		var evidence []byte
		if err := rows.Scan(&result.Total, &id, &userID, &email, &date, &stored, &observed, &reward, &ledger, &energy, &mode, &reason, &snapshot, &created, &adjustment, &reconciliationID, &reconciledStreak, &username, &startDay, &atCheckin, &previousDay, &gapStart, &gapEnd, &lateDays, &evidence, &daily, &milestone); err != nil {
			return nil, err
		}
		if !id.Valid {
			continue
		}
		item := service.PlayAdminCheckinRecord{ID: id.Int64, UserID: userID.Int64, Email: email.String, Date: date.String, StoredStreak: int(stored.Int64), ObservedStreak: int(observed.Int64), RewardAmount: reward.String, LedgerAmount: ledger.String, GrowthEnergy: energy.Int64, RewardMode: mode.String, EligibilityReason: reason.String, CreatedAt: created.Time}
		item.Username, item.StreakStartDate, item.StreakAtCheckin = username.String, startDay.String, int(atCheckin.Int64)
		item.LateRecordedDays = int(lateDays.Int64)
		item.DailyLedgerAmount, item.MilestoneLedgerAmount = daily.String, milestone.String
		if previousDay.Valid {
			item.PreviousCheckinDate = &previousDay.String
			item.GapStartDate, item.GapEndDate = &gapStart.String, &gapEnd.String
		}
		if err := json.Unmarshal(evidence, &item.Continuity); err != nil {
			return nil, fmt.Errorf("decode check-in continuity: %w", err)
		}
		item.AdjustmentAmount = adjustment.String
		if reconciliationID.Valid {
			n := reconciliationID.Int64
			item.ReconciliationID = &n
		}
		if snapshot.Valid {
			n := snapshot.Int64
			item.SnapshotID = &n
		}
		// A missing snapshot or a stale counter warrants review; it is not
		// evidence of an owed cash reward. Amounts remain decimal strings.
		item.EvidenceStatus = "recorded"
		if item.SnapshotID == nil {
			item.EvidenceStatus = "legacy_evidence_missing"
		} else if item.StoredStreak != item.ObservedStreak {
			item.EvidenceStatus = "streak_mismatch"
			if reconciledStreak.Valid && reconciledStreak.Int64 == observed.Int64 {
				item.EvidenceStatus = "reconciled"
			}
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

var _ service.PlayAdminCheckinRepository = (*playRepository)(nil)
