package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const growthGovernanceStateQuery = `
	SELECT a.id,
	       a.decision,
	       a.budget_amount::double precision,
	       a.rollout_percent,
	       a.cohort_start,
	       a.cohort_end,
	       a.cohort_metrics,
	       a.rule_version,
	       a.reason,
	       a.actor_id,
	       a.created_at,
	       COALESCE((
	           SELECT SUM(l.amount)::double precision
	           FROM play_growth_reward_budget_ledger l
	           WHERE l.approval_id = a.id
	       ), 0)::double precision AS budget_spent
	FROM play_growth_governance_approvals a
	-- All mutation paths acquire the same transaction advisory lock before
	-- inserting a decision. Sequence IDs are therefore the linearized order;
	-- created_at is deliberately not used because PostgreSQL NOW() is pinned to
	-- the start of a transaction that might have waited for that lock.
	ORDER BY a.id DESC
	LIMIT 1`

const (
	growthGovernanceAdvisoryLockKey   = "play_growth_governance_mutations_v1"
	growthGovernanceAdvisoryLockQuery = `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`
)

// lockGrowthGovernanceMutation serializes approvals and revocations with a
// reward reservation. Reservations are called inside the enclosing reward
// transaction, so the lock spans activity, budget, and ledger settlement.
func lockGrowthGovernanceMutation(ctx context.Context, exec sqlExecutor) error {
	if _, err := exec.ExecContext(ctx, growthGovernanceAdvisoryLockQuery, growthGovernanceAdvisoryLockKey); err != nil {
		return fmt.Errorf("lock play growth governance: %w", err)
	}
	return nil
}

// withGrowthGovernanceMutationTx runs a governance mutation under a real
// database transaction. Reward paths already supply an ent transaction, which
// keeps the advisory lock through the related activity, qualification proof,
// and reward settlement. The explicit fallback is defensive for direct
// repository callers: a standalone SELECT pg_advisory_xact_lock on *sql.DB
// would release before the following budget read and provide no protection.
func (r *playRepository) withGrowthGovernanceMutationTx(ctx context.Context, fn func(context.Context, sqlExecutor) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		exec := sqlExecutorFromEntClient(tx.Client())
		if exec == nil {
			return fmt.Errorf("play growth governance transaction executor is unavailable")
		}
		return fn(ctx, exec)
	}

	if r != nil && r.client != nil {
		tx, err := r.client.Tx(ctx)
		if err != nil {
			return fmt.Errorf("begin play growth governance transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		txCtx := dbent.NewTxContext(ctx, tx)
		exec := sqlExecutorFromEntClient(tx.Client())
		if exec == nil {
			return fmt.Errorf("play growth governance transaction executor is unavailable")
		}
		if err := fn(txCtx, exec); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit play growth governance transaction: %w", err)
		}
		return nil
	}

	if r == nil {
		return fmt.Errorf("play growth governance repository is unavailable")
	}
	db, ok := r.sql.(*sql.DB)
	if !ok || db == nil {
		return fmt.Errorf("play growth governance requires an active transaction or SQL database")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin play growth governance transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit play growth governance transaction: %w", err)
	}
	return nil
}

func (r *playRepository) GetGrowthGovernance(ctx context.Context, now time.Time) (*service.PlayGrowthGovernanceState, error) {
	var (
		state                  service.PlayGrowthGovernanceState
		cohortStart, cohortEnd sql.NullTime
		metricsJSON            []byte
		actorID                sql.NullInt64
		budgetSpent            float64
	)
	err := scanSingleRow(ctx, r.sqlExec(ctx), growthGovernanceStateQuery, nil,
		&state.ID,
		&state.Decision,
		&state.BudgetAmount,
		&state.RolloutPercent,
		&cohortStart,
		&cohortEnd,
		&metricsJSON,
		&state.RuleVersion,
		&state.Reason,
		&actorID,
		&state.CreatedAt,
		&budgetSpent,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get play growth governance: %w", err)
	}
	if cohortStart.Valid {
		state.Cohort.WindowStart = cohortStart.Time
	}
	if cohortEnd.Valid {
		state.Cohort.WindowEnd = cohortEnd.Time
	}
	if len(metricsJSON) > 0 && string(metricsJSON) != "null" {
		if err := json.Unmarshal(metricsJSON, &state.Cohort); err != nil {
			return nil, fmt.Errorf("decode play growth cohort metrics: %w", err)
		}
	}
	// Window columns are authoritative even when an older JSON payload omitted
	// them. This also prevents an operator from changing the effective window
	// merely by editing a serialized payload outside the service.
	if cohortStart.Valid {
		state.Cohort.WindowStart = cohortStart.Time
	}
	if cohortEnd.Valid {
		state.Cohort.WindowEnd = cohortEnd.Time
	}
	state.Cohort.MetricsAvailable = state.Cohort.MetricsAvailable && len(state.Cohort.UnavailableMetrics) == 0
	state.BudgetSpent = budgetSpent
	state.BudgetRemaining = state.BudgetAmount - budgetSpent
	if state.BudgetRemaining < 0 {
		state.BudgetRemaining = 0
	}
	state.Approved = state.Decision == service.PlayGrowthGovernanceDecisionApproved
	if actorID.Valid {
		id := actorID.Int64
		state.ActorID = &id
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	// A future-dated or malformed approval is returned as inactive. The service
	// still exposes the row to admins for diagnosis, but reward endpoints fail
	// closed through AllowsReward.
	if state.Cohort.WindowEnd.After(now) {
		state.Approved = false
	}
	return &state, nil
}

// growthCohortQuery intentionally computes all fixed metrics in one query.
// All windows use [start,end), and post-participation usage uses the first
// activity timestamp for each participant to avoid an N+1 query per account.
const growthCohortQuery = `
WITH participants AS (
    SELECT user_id, MIN(activity_at) AS first_activity_at
    FROM (
        SELECT user_id, created_at AS activity_at
        FROM play_checkins
        WHERE checkin_date >= $1::date AND checkin_date < $2::date
        UNION ALL
        SELECT user_id, created_at AS activity_at
        FROM play_quiz_attempts
        WHERE attempt_date >= $1::date AND attempt_date < $2::date
    ) activities
    GROUP BY user_id
),
usage_7d AS (
    SELECT DISTINCT p.user_id
    FROM participants p
    JOIN usage_logs u ON u.user_id = p.user_id
      AND u.created_at >= p.first_activity_at
      AND u.created_at < p.first_activity_at + INTERVAL '7 days'
      AND (u.actual_cost > 0 OR u.billed_cost > 0)
),
usage_30d AS (
    SELECT DISTINCT p.user_id
    FROM participants p
    JOIN usage_logs u ON u.user_id = p.user_id
      AND u.created_at >= p.first_activity_at
      AND u.created_at < p.first_activity_at + INTERVAL '30 days'
      AND (u.actual_cost > 0 OR u.billed_cost > 0)
),
d7_retained AS (
    SELECT DISTINCT p.user_id
    FROM participants p
    JOIN usage_logs u ON u.user_id = p.user_id
      AND u.created_at >= p.first_activity_at + INTERVAL '7 days'
      AND u.created_at < p.first_activity_at + INTERVAL '30 days'
      AND (u.actual_cost > 0 OR u.billed_cost > 0)
),
first_recharge AS (
    SELECT DISTINCT p.user_id
    FROM participants p
    JOIN payment_orders po ON po.user_id = p.user_id
      AND po.order_type = 'balance'
      AND po.status = 'COMPLETED'
      AND po.completed_at IS NOT NULL
      AND po.completed_at >= p.first_activity_at
      AND po.completed_at >= $1
      AND po.completed_at < $2
      AND UPPER(COALESCE(po.payment_currency, 'CNY')) = 'CNY'
      -- Conversion is based on the first qualifying recharge, not any repeat
      -- order that happens to fall inside the cohort window. The id tie-breaker
      -- makes equal completion timestamps deterministic.
      AND NOT EXISTS (
          SELECT 1
          FROM payment_orders prior
          WHERE prior.user_id = po.user_id
            AND prior.order_type = 'balance'
            AND prior.status = 'COMPLETED'
            AND prior.completed_at IS NOT NULL
            AND UPPER(COALESCE(prior.payment_currency, 'CNY')) = 'CNY'
            AND (prior.completed_at, prior.id) < (po.completed_at, po.id)
      )
),
coupon_totals AS (
    SELECT
      COUNT(*) FILTER (WHERE uc.created_at >= $1 AND uc.created_at < $2)::bigint AS issued,
      COUNT(*) FILTER (WHERE uc.status = 'used' AND uc.used_at >= $1 AND uc.used_at < $2)::bigint AS redeemed
    FROM user_coupons uc
    JOIN participants p ON p.user_id = uc.user_id
    WHERE uc.source IN ('checkin', 'quiz', 'blindbox')
),
reward_cost AS (
    SELECT COALESCE(SUM(GREATEST(l.amount, 0)), 0)::double precision AS amount
    FROM play_reward_ledger l
    JOIN participants p ON p.user_id = l.user_id
    WHERE l.source IN ('checkin', 'checkin_makeup', 'quiz', 'blindbox')
      AND l.created_at >= $1 AND l.created_at < $2
),
counts AS (
    SELECT
      COUNT(*)::bigint AS participants,
      (SELECT COUNT(*) FROM usage_7d)::bigint AS usage_7d,
      (SELECT COUNT(*) FROM usage_30d)::bigint AS usage_30d,
      (SELECT COUNT(*) FROM first_recharge)::bigint AS first_recharge,
      (SELECT COUNT(*) FROM d7_retained)::bigint AS d7_retained
    FROM participants
)
SELECT
  $1::timestamptz AS window_start,
  $2::timestamptz AS window_end,
  c.participants,
  c.usage_7d,
  COALESCE(c.usage_7d::double precision / NULLIF(c.participants, 0), 0),
  c.usage_30d,
  COALESCE(c.usage_30d::double precision / NULLIF(c.participants, 0), 0),
  c.first_recharge,
  COALESCE(c.first_recharge::double precision / NULLIF(c.participants, 0), 0),
  COALESCE(ct.issued, 0),
  COALESCE(ct.redeemed, 0),
  COALESCE(ct.redeemed::double precision / NULLIF(ct.issued, 0), 0),
  COALESCE(rc.amount, 0),
  c.d7_retained,
  COALESCE(c.d7_retained::double precision / NULLIF(c.participants, 0), 0),
  0::bigint AS abnormal_redemption_users,
  NULL::double precision AS abnormal_redemption_ratio,
  0::bigint AS appeal_count,
  0::bigint AS false_positive_appeals,
  NULL::double precision AS appeal_false_positive_ratio
FROM counts c
CROSS JOIN coupon_totals ct
CROSS JOIN reward_cost rc`

func (r *playRepository) GetGrowthCohort(ctx context.Context, start, end time.Time) (service.PlayGrowthCohortMetrics, error) {
	var (
		metrics                    service.PlayGrowthCohortMetrics
		abnormalRatio, appealRatio sql.NullFloat64
	)
	err := scanSingleRow(ctx, r.sqlExec(ctx), growthCohortQuery, []any{start, end},
		&metrics.WindowStart,
		&metrics.WindowEnd,
		&metrics.ParticipationUsers,
		&metrics.RealCall7dUsers,
		&metrics.RealCall7dRatio,
		&metrics.RealCall30dUsers,
		&metrics.RealCall30dRatio,
		&metrics.FirstRechargeUsers,
		&metrics.FirstRechargeRatio,
		&metrics.CouponsIssued,
		&metrics.CouponsRedeemed,
		&metrics.CouponRedemptionRatio,
		&metrics.ActualRewardCost,
		&metrics.D7RetainedUsers,
		&metrics.D7RetentionRatio,
		&metrics.AbnormalRedemptionUsers,
		&abnormalRatio,
		&metrics.AppealCount,
		&metrics.FalsePositiveAppeals,
		&appealRatio,
	)
	if err != nil {
		return service.PlayGrowthCohortMetrics{}, fmt.Errorf("get play growth cohort: %w", err)
	}
	if abnormalRatio.Valid {
		value := abnormalRatio.Float64
		metrics.AbnormalRedemptionRatio = &value
	}
	if appealRatio.Valid {
		value := appealRatio.Float64
		metrics.AppealFalsePositiveRatio = &value
	}
	metrics.MetricsAvailable = false
	metrics.UnavailableMetrics = []string{"abnormal_redemption_ratio", "appeal_false_positive_ratio"}
	return metrics, nil
}

func (r *playRepository) CreateGrowthApproval(ctx context.Context, input service.PlayGrowthGovernanceApprovalInput) (*service.PlayGrowthGovernanceState, error) {
	metricsJSON, err := json.Marshal(input.Cohort)
	if err != nil {
		return nil, fmt.Errorf("marshal play growth cohort metrics: %w", err)
	}
	var (
		id        int64
		actorID   sql.NullInt64
		createdAt time.Time
	)
	err = r.withGrowthGovernanceMutationTx(ctx, func(txCtx context.Context, exec sqlExecutor) error {
		if err := lockGrowthGovernanceMutation(txCtx, exec); err != nil {
			return err
		}
		return scanSingleRow(txCtx, exec, `
			INSERT INTO play_growth_governance_approvals (
				decision, budget_amount, rollout_percent, cohort_start, cohort_end,
				cohort_metrics, rule_version, reason, actor_id
		)
			VALUES ('approved', $1, $2, $3, $4, $5::jsonb, $6, $7, NULLIF($8, 0))
			RETURNING id, actor_id, created_at`, []any{
			input.BudgetAmount,
			input.RolloutPercent,
			input.Cohort.WindowStart,
			input.Cohort.WindowEnd,
			metricsJSON,
			strings.TrimSpace(input.RuleVersion),
			strings.TrimSpace(input.Reason),
			input.ActorID,
		}, &id, &actorID, &createdAt)
	})
	if err != nil {
		return nil, fmt.Errorf("create play growth approval: %w", err)
	}
	var stateActorID *int64
	if actorID.Valid {
		value := actorID.Int64
		stateActorID = &value
	}
	return &service.PlayGrowthGovernanceState{
		ID: id, Decision: service.PlayGrowthGovernanceDecisionApproved, Approved: true,
		BudgetAmount: input.BudgetAmount, BudgetRemaining: input.BudgetAmount,
		RolloutPercent: input.RolloutPercent, Cohort: input.Cohort,
		RuleVersion: strings.TrimSpace(input.RuleVersion), Reason: strings.TrimSpace(input.Reason),
		ActorID: stateActorID, CreatedAt: createdAt.UTC(),
	}, nil
}

func (r *playRepository) RevokeGrowthApproval(ctx context.Context, actorID int64, reason string) (*service.PlayGrowthGovernanceState, error) {
	var (
		id            int64
		returnedActor sql.NullInt64
		createdAt     time.Time
	)
	err := r.withGrowthGovernanceMutationTx(ctx, func(txCtx context.Context, exec sqlExecutor) error {
		if err := lockGrowthGovernanceMutation(txCtx, exec); err != nil {
			return err
		}
		return scanSingleRow(txCtx, exec, `
			INSERT INTO play_growth_governance_approvals (
				decision, budget_amount, rollout_percent, cohort_metrics, rule_version, reason, actor_id
			)
			VALUES ('revoked', 0, 0, '{}'::jsonb, $1, $2, NULLIF($3, 0))
			RETURNING id, actor_id, created_at`, []any{
			service.PlayGrowthQualificationRuleVersion(),
			strings.TrimSpace(reason),
			actorID,
		}, &id, &returnedActor, &createdAt)
	})
	if err != nil {
		return nil, fmt.Errorf("revoke play growth approval: %w", err)
	}
	var actor *int64
	if returnedActor.Valid {
		value := returnedActor.Int64
		actor = &value
	}
	return &service.PlayGrowthGovernanceState{
		ID: id, Decision: service.PlayGrowthGovernanceDecisionRevoked, Approved: false,
		RuleVersion: service.PlayGrowthQualificationRuleVersion(), Reason: strings.TrimSpace(reason),
		ActorID: actor, CreatedAt: createdAt.UTC(),
	}, nil
}

func (r *playRepository) GetGrowthRewardSpend(ctx context.Context, start, end time.Time) (float64, error) {
	var amount float64
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT COALESCE(SUM(amount), 0)::double precision
		FROM play_growth_reward_budget_ledger
		WHERE created_at >= $1 AND created_at < $2`, []any{start, end}, &amount)
	if err != nil {
		return 0, fmt.Errorf("get play growth reward spend: %w", err)
	}
	return amount, nil
}

func (r *playRepository) ReserveGrowthRewardBudget(ctx context.Context, approvalID, userID int64, source, actionID string, amount float64) (bool, error) {
	if approvalID <= 0 || userID <= 0 {
		return false, fmt.Errorf("reserve play growth budget: approval and user are required")
	}
	source = strings.TrimSpace(source)
	actionID = strings.TrimSpace(actionID)
	if source == "" || actionID == "" {
		return false, fmt.Errorf("reserve play growth budget: source and action id are required")
	}
	switch source {
	case service.PlayRewardSourceCheckin, service.PlayRewardSourceQuiz, service.PlayRewardSourceBlindbox:
	default:
		return false, fmt.Errorf("reserve play growth budget: unsupported source %q", source)
	}
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return false, fmt.Errorf("reserve play growth budget: amount must be finite and positive")
	}
	var reserved bool
	err := r.withGrowthGovernanceMutationTx(ctx, func(txCtx context.Context, exec sqlExecutor) error {
		// The lock must be its own statement. PostgreSQL READ COMMITTED takes a
		// statement snapshot before a CTE can wait on a lock; querying only after
		// this call guarantees a fresh view of both the latest decision and spent
		// budget once this transaction owns the serialization boundary.
		if err := lockGrowthGovernanceMutation(txCtx, exec); err != nil {
			return err
		}

		// A retry may reuse a reservation only when it is the exact same immutable
		// activity. A source/action collision must not inherit somebody else's
		// budget reservation or silently accept a different reward amount.
		var existing struct {
			UserID int64
			Source string
			Amount float64
		}
		err := scanSingleRow(txCtx, exec, `
			SELECT user_id, source, amount::double precision
			FROM play_growth_reward_budget_ledger
			WHERE source = $1 AND action_id = $2`, []any{source, actionID},
			&existing.UserID, &existing.Source, &existing.Amount)
		if err == nil {
			if existing.UserID == userID && existing.Source == source && math.Abs(existing.Amount-amount) <= 1e-9 {
				reserved = true
				return nil
			}
			return service.ErrPlayGrowthGovernanceInvalid.WithCause(fmt.Errorf("budget action id conflicts with an existing reservation"))
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("read existing play growth budget reservation: %w", err)
		}

		return scanSingleRow(txCtx, exec, `
			WITH current_approval AS (
				SELECT a.id, a.decision, a.budget_amount
				FROM play_growth_governance_approvals a
				ORDER BY a.id DESC
				LIMIT 1
			), inserted AS (
				INSERT INTO play_growth_reward_budget_ledger (approval_id, user_id, source, action_id, amount)
				SELECT a.id, $2, $3, $4, $5
				FROM current_approval a
				WHERE a.id = $1
				  AND a.decision = 'approved'
				  AND (
					SELECT COALESCE(SUM(l.amount), 0)
					FROM play_growth_reward_budget_ledger l
					WHERE l.approval_id = a.id
				  ) + $5 <= a.budget_amount
				ON CONFLICT (source, action_id) DO NOTHING
				RETURNING TRUE AS ok
			)
			SELECT COALESCE((SELECT ok FROM inserted LIMIT 1), FALSE)`, []any{
			approvalID, userID, source, actionID, amount,
		}, &reserved)
	})
	if err != nil {
		return false, fmt.Errorf("reserve play growth budget: %w", err)
	}
	return reserved, nil
}
