// Package balanceforecast evaluates estimates from committed settlement receipts.
package balanceforecast

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Store struct{ DB *sql.DB }

type Candidate struct {
	ID                                              int64
	Balance, Spent, ObservationHours, Hourly, Hours float64
	Covered                                         bool
	Samples                                         int
	First, Last                                     time.Time
}

func (c Candidate) Reliable() bool {
	return c.Covered && c.ObservationHours >= 1 && c.Samples >= 3 && c.Last.Sub(c.First) >= time.Hour && c.Hourly > 0
}

func (s *Store) Candidates(ctx context.Context, now time.Time, after int64, limit int) ([]Candidate, error) {
	if limit < 1 || limit > 200 {
		return nil, errors.New("forecast batch limit must be 1..200")
	}
	rows, err := s.DB.QueryContext(ctx, candidateSQL, now.UTC(), after, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []Candidate
	for rows.Next() {
		var c Candidate
		var first, last sql.NullTime
		if err = rows.Scan(&c.ID, &c.Balance, &c.ObservationHours, &c.Spent, &c.Samples, &first, &last, &c.Covered); err != nil {
			return nil, err
		}
		c.First, c.Last = first.Time, last.Time
		if c.Covered && c.ObservationHours >= 1 && c.Samples >= 3 && c.Last.Sub(c.First) >= time.Hour && c.Spent > 0 {
			c.Hourly = c.Spent / c.ObservationHours
			c.Hours = c.Balance / c.Hourly
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// The page bounds aggregation. Only committed consumption in the Fork balance
// ledger counts. For a capture, the combined balance and frozen delta is the
// final debit after the hold, including any refund of the unused amount.
const candidateSQL = `WITH page AS MATERIALIZED (
 SELECT u.id, u.balance, GREATEST($1::timestamptz - INTERVAL '24 hours', u.created_at, c.started_at) AS since
 FROM users u CROSS JOIN balance_forecast_coverage c
 WHERE u.id > $2 AND u.deleted_at IS NULL AND u.status = 'active' AND u.balance_notify_enabled AND u.balance > 0
 ORDER BY u.id LIMIT $3
), settled AS (
 SELECT p.id AS user_id, COALESCE(NULLIF(b.metadata->>'request_id', ''), b.source_id) AS request_id,
        b.metadata->>'api_key_id' AS api_key_id, -(b.balance_delta + b.frozen_delta) AS amount, b.created_at AS at
 FROM page p JOIN balance_transactions b ON b.user_id = p.id
 WHERE b.source_type IN ('usage_charge', 'image_balance_capture', 'video_balance_capture')
   AND NOT b.is_backfilled AND b.confidence = 'high' AND b.balance_delta + b.frozen_delta < 0
   AND b.created_at >= p.since AND b.created_at < $1
)
 SELECT p.id, p.balance, EXTRACT(EPOCH FROM ($1::timestamptz - p.since)) / 3600,
        COALESCE(SUM(s.amount), 0), COUNT(s.amount), MIN(s.at), MAX(s.at),
        NOT EXISTS (
          SELECT 1 FROM usage_logs u
          WHERE u.user_id = p.id AND u.billing_type = 0 AND u.actual_cost > 0
            AND u.created_at >= p.since AND u.created_at < $1
            AND NOT EXISTS (
              SELECT 1 FROM settled x
              WHERE x.user_id = u.user_id AND x.request_id = u.request_id AND x.api_key_id = u.api_key_id::text
            )
        )
 FROM page p LEFT JOIN settled s ON s.user_id = p.id
 GROUP BY p.id, p.balance, p.since ORDER BY p.id`
