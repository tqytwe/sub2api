package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const growthEligibilitySignalsQuery = `
		SELECT
			EXISTS(
				SELECT 1 FROM auth_identities ai
				WHERE ai.user_id = u.id
				  AND (
					  ai.verified_at IS NOT NULL
					  OR (
						  ai.provider_type IN ('github', 'google', 'oidc')
						  AND LOWER(COALESCE(ai.metadata->>'email_verified', 'false')) IN ('true', '1', 'yes')
					  )
				  )
			) AS email_verified,
			u.created_at,
			EXISTS(
			SELECT 1 FROM usage_logs ul
			WHERE ul.user_id = u.id
			  AND ul.created_at >= $2
			  AND ul.created_at < $7
			  AND (ul.actual_cost > 0 OR ul.billed_cost > 0)
			) AS has_recent_usage,
			COALESCE((
				SELECT SUM(GREATEST(
					CASE WHEN po.list_amount > 0 THEN po.qualifying_recharge_amount ELSE po.amount END
					* (1 - LEAST(GREATEST(COALESCE(po.refund_amount / NULLIF(po.amount, 0), 0), 0), 1)),
					0
				))::double precision
				FROM payment_orders po
				WHERE po.user_id = u.id
				  AND po.order_type = $4
				  AND po.status = $5
				  AND UPPER(COALESCE(po.payment_currency, 'CNY')) = 'CNY'
				  AND po.completed_at IS NOT NULL
				  AND po.completed_at >= $3
				  AND po.completed_at < $7
			), 0)::double precision AS net_balance_recharge_30d,
			EXISTS(
				SELECT 1 FROM user_subscriptions us
				WHERE us.user_id = u.id
				  AND us.status = $6
				  AND us.starts_at <= $7
				  AND us.expires_at > $7
				  AND us.deleted_at IS NULL
			) AS has_active_subscription
		FROM users u
		WHERE u.id = $1
		  AND u.deleted_at IS NULL`

func (r *playRepository) GetGrowthEligibilitySignals(
	ctx context.Context,
	userID int64,
	usageSince, rechargeSince, now time.Time,
) (service.PlayGrowthEligibilitySignals, error) {
	exec := r.sqlExec(ctx)
	var signals service.PlayGrowthEligibilitySignals
	err := scanSingleRow(ctx, exec, growthEligibilitySignalsQuery,
		[]any{
			userID,
			usageSince,
			rechargeSince,
			payment.OrderTypeBalance,
			payment.OrderStatusCompleted,
			service.SubscriptionStatusActive,
			now,
		},
		&signals.EmailVerified,
		&signals.CreatedAt,
		&signals.HasRecentUsage,
		&signals.NetBalanceRecharge30d,
		&signals.HasActiveSubscription,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return service.PlayGrowthEligibilitySignals{}, service.ErrUserNotFound
		}
		return service.PlayGrowthEligibilitySignals{}, fmt.Errorf("get growth eligibility signals: %w", err)
	}
	return signals, nil
}

func (r *playRepository) CreateGrowthEligibilitySnapshot(ctx context.Context, snapshot service.PlayGrowthEligibilitySnapshot) (int64, error) {
	snapshot.ActionID = strings.TrimSpace(snapshot.ActionID)
	if snapshot.ActionID == "" {
		return 0, fmt.Errorf("create growth eligibility snapshot: action id is required")
	}
	exec := r.sqlExec(ctx)
	var id int64
	err := scanSingleRow(ctx, exec, `
		INSERT INTO play_growth_eligibility_snapshots (
			user_id, source, action_id, activity_date, tier, reward_mode, primary_reason,
			email_verified, account_age_days, has_recent_usage,
			net_balance_recharge_30d, has_active_subscription, rule_version
		)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`,
		[]any{
			snapshot.UserID,
			snapshot.Source,
			snapshot.ActionID,
			snapshot.ActivityDate.Format("2006-01-02"),
			snapshot.Eligibility.Tier,
			snapshot.Eligibility.RewardMode,
			snapshot.Eligibility.PrimaryReason,
			snapshot.Eligibility.EmailVerified,
			snapshot.Eligibility.AccountAgeDays,
			snapshot.Eligibility.HasRecentUsage,
			snapshot.Eligibility.NetBalanceRecharge30d,
			snapshot.Eligibility.HasActiveSubscription,
			service.PlayGrowthQualificationRuleVersion(),
		},
		&id,
	)
	if err != nil {
		return 0, fmt.Errorf("create growth eligibility snapshot: %w", err)
	}
	return id, nil
}

func (r *playRepository) LinkGrowthEligibilitySnapshot(ctx context.Context, source string, userID int64, activityDate time.Time, snapshotID int64) error {
	if userID <= 0 {
		return fmt.Errorf("link growth eligibility snapshot: user id is required")
	}
	if snapshotID <= 0 {
		return fmt.Errorf("link growth eligibility snapshot: snapshot id is required")
	}
	exec := r.sqlExec(ctx)
	var query string
	switch source {
	case service.PlayRewardSourceCheckin:
		query = `
			UPDATE play_checkins AS activity
			SET growth_eligibility_snapshot_id = $3
			FROM play_growth_eligibility_snapshots AS snapshot
			WHERE activity.user_id = $1
			  AND activity.checkin_date = $2
			  AND activity.growth_eligibility_snapshot_id IS NULL
			  AND snapshot.id = $3
			  AND snapshot.user_id = $1
			  AND snapshot.source = $4
			  AND snapshot.activity_date = $2`
	case service.PlayRewardSourceQuiz:
		query = `
			UPDATE play_quiz_attempts AS activity
			SET growth_eligibility_snapshot_id = $3
			FROM play_growth_eligibility_snapshots AS snapshot
			WHERE activity.user_id = $1
			  AND activity.attempt_date = $2
			  AND activity.growth_eligibility_snapshot_id IS NULL
			  AND snapshot.id = $3
			  AND snapshot.user_id = $1
			  AND snapshot.source = $4
			  AND snapshot.activity_date = $2`
	default:
		return fmt.Errorf("unsupported growth eligibility source: %s", source)
	}
	result, err := exec.ExecContext(ctx, query, userID, activityDate.Format("2006-01-02"), snapshotID, source)
	if err != nil {
		return fmt.Errorf("link growth eligibility snapshot: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("link growth eligibility snapshot rows: %w", err)
	}
	if rows != 1 {
		return service.ErrPlayRewardDuplicate
	}
	return nil
}

func (r *playRepository) LinkBlindboxGrowthEligibilitySnapshot(ctx context.Context, userID int64, actionID string, snapshotID int64) error {
	actionID = strings.TrimSpace(actionID)
	if actionID == "" {
		return fmt.Errorf("link blindbox growth eligibility snapshot: action id is required")
	}
	if snapshotID <= 0 {
		return fmt.Errorf("link blindbox growth eligibility snapshot: snapshot id is required")
	}
	exec := r.sqlExec(ctx)
	result, err := exec.ExecContext(ctx, `
		UPDATE play_blindbox_opens AS activity
		SET growth_eligibility_snapshot_id = $3
		FROM play_growth_eligibility_snapshots AS snapshot
		WHERE activity.user_id = $1
		  AND activity.idempotency_key = $2
		  AND activity.growth_eligibility_snapshot_id IS NULL
		  AND snapshot.id = $3
		  AND snapshot.user_id = $1
		  AND snapshot.source = $4
		  AND snapshot.action_id = $2
		  AND snapshot.activity_date = activity.open_date`, userID, actionID, snapshotID, service.PlayRewardSourceBlindbox)
	if err != nil {
		return fmt.Errorf("link blindbox growth eligibility snapshot: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("link blindbox growth eligibility snapshot rows: %w", err)
	}
	if rows != 1 {
		return service.ErrPlayRewardDuplicate
	}
	return nil
}

func (r *playRepository) InsertGrowthEnergyLedger(ctx context.Context, entry service.PlayGrowthEnergyLedgerEntry) error {
	entry.ActionID = strings.TrimSpace(entry.ActionID)
	if entry.ActionID == "" {
		return fmt.Errorf("insert growth energy ledger: action id is required")
	}
	if entry.Amount <= 0 {
		return fmt.Errorf("insert growth energy ledger: amount must be positive")
	}
	if entry.EligibilitySnapshotID <= 0 {
		return fmt.Errorf("insert growth energy ledger: eligibility snapshot id is required")
	}
	exec := r.sqlExec(ctx)
	result, err := exec.ExecContext(ctx, `
		INSERT INTO play_growth_energy_ledger (
			user_id, source, action_id, amount, eligibility_snapshot_id
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (action_id) DO NOTHING`,
		entry.UserID,
		entry.Source,
		entry.ActionID,
		entry.Amount,
		entry.EligibilitySnapshotID,
	)
	if err != nil {
		return fmt.Errorf("insert growth energy ledger: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("insert growth energy ledger rows: %w", err)
	}
	if rows != 1 {
		return service.ErrPlayRewardDuplicate
	}
	return nil
}
