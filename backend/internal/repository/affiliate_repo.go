package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const (
	affiliateCodeLength      = 12
	affiliateCodeMaxAttempts = 12
)

var affiliateCodeCharset = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

const affiliateUserOverviewSQL = `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ua.aff_code,
       COALESCE(ua.aff_rebate_rate_percent, 0)::double precision,
       (ua.aff_rebate_rate_percent IS NOT NULL) AS has_custom_rate,
       ua.aff_count,
       COALESCE(rebated.rebated_invitee_count, 0),
       (ua.aff_quota + COALESCE(matured.matured_frozen_quota, 0))::double precision,
       ua.aff_history_quota::double precision
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id
LEFT JOIN (
    SELECT user_id, COUNT(DISTINCT source_user_id)::integer AS rebated_invitee_count
    FROM user_affiliate_ledger
    WHERE action = 'accrue' AND source_user_id IS NOT NULL
    GROUP BY user_id
) rebated ON rebated.user_id = ua.user_id
LEFT JOIN (
    SELECT user_id, COALESCE(SUM(amount), 0)::double precision AS matured_frozen_quota
    FROM user_affiliate_ledger
    WHERE action = 'accrue' AND frozen_until IS NOT NULL AND frozen_until <= NOW()
    GROUP BY user_id
) matured ON matured.user_id = ua.user_id
WHERE ua.user_id = $1
LIMIT 1`

const affiliateDefaultTeamJoinSQL = `
WITH inviter_team AS (
	SELECT m.team_id
	FROM play_team_members m
	JOIN play_teams t ON t.id = m.team_id
	WHERE m.user_id = $1
	  AND m.left_at IS NULL
	  AND t.archived_at IS NULL
	ORDER BY m.joined_at ASC, m.id ASC
	LIMIT 1
),
inserted_member AS (
	INSERT INTO play_team_members (team_id, user_id)
	SELECT team_id, $2
	FROM inviter_team
	ON CONFLICT DO NOTHING
	RETURNING team_id
)
INSERT INTO play_team_events (
	team_id,
	actor_user_id,
	subject_user_id,
	event_type,
	detail
)
SELECT
	team_id,
	$2,
	$2,
	$3,
	jsonb_build_object('source', 'affiliate_invite', 'inviter_user_id', $1)
FROM inserted_member`

type affiliateQueryExecer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type affiliateRepository struct {
	client *dbent.Client
}

var _ service.ReferralCampaignRepository = (*affiliateRepository)(nil)

func NewAffiliateRepository(client *dbent.Client, _ *sql.DB) service.AffiliateRepository {
	return &affiliateRepository{client: client}
}

func (r *affiliateRepository) EnsureUserAffiliate(ctx context.Context, userID int64) (*service.AffiliateSummary, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	client := clientFromContext(ctx, r.client)
	return ensureUserAffiliateWithClient(ctx, client, userID)
}

func (r *affiliateRepository) GetAffiliateByCode(ctx context.Context, code string) (*service.AffiliateSummary, error) {
	client := clientFromContext(ctx, r.client)
	return queryAffiliateByCode(ctx, client, code)
}

func (r *affiliateRepository) BindInviter(ctx context.Context, userID, inviterID int64) (bool, error) {
	var bound bool
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, inviterID); err != nil {
			return err
		}

		res, err := txClient.ExecContext(txCtx,
			"UPDATE user_affiliates SET inviter_id = $1, updated_at = NOW() WHERE user_id = $2 AND inviter_id IS NULL",
			inviterID, userID,
		)
		if err != nil {
			return fmt.Errorf("bind inviter: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			bound = false
			return nil
		}

		if _, err = txClient.ExecContext(txCtx,
			"UPDATE user_affiliates SET aff_count = aff_count + 1, updated_at = NOW() WHERE user_id = $1",
			inviterID,
		); err != nil {
			return fmt.Errorf("increment inviter aff_count: %w", err)
		}
		bound = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return bound, nil
}

func (r *affiliateRepository) JoinInviterActiveTeam(ctx context.Context, inviterID, inviteeUserID int64) (bool, error) {
	if inviterID <= 0 || inviteeUserID <= 0 || inviterID == inviteeUserID {
		return false, nil
	}

	joined := false
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		res, err := txClient.ExecContext(
			txCtx,
			affiliateDefaultTeamJoinSQL,
			inviterID,
			inviteeUserID,
			service.PlayTeamEventMemberJoined,
		)
		if err != nil {
			return fmt.Errorf("join inviter active team: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("join inviter active team rows affected: %w", err)
		}
		joined = affected > 0
		return nil
	})
	if err != nil {
		return false, err
	}
	return joined, nil
}

func (r *affiliateRepository) AccrueQuota(ctx context.Context, inviterID, inviteeUserID int64, amount float64, freezeHours int, sourceOrderID *int64) (bool, error) {
	if amount <= 0 {
		return false, nil
	}

	var applied bool
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		// freezeHours > 0: add to frozen quota; == 0: add to available quota directly
		var updateSQL string
		if freezeHours > 0 {
			updateSQL = "UPDATE user_affiliates SET aff_frozen_quota = aff_frozen_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2"
		} else {
			updateSQL = "UPDATE user_affiliates SET aff_quota = aff_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2"
		}
		res, err := txClient.ExecContext(txCtx, updateSQL, amount, inviterID)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			applied = false
			return nil
		}

		if freezeHours > 0 {
			if _, err = txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, source_order_id, frozen_until, created_at, updated_at)
VALUES ($1, 'accrue', $2, $3, $4, NOW() + make_interval(hours => $5), NOW(), NOW())`,
				inviterID, amount, inviteeUserID, nullableInt64Arg(sourceOrderID), freezeHours); err != nil {
				return fmt.Errorf("insert affiliate accrue ledger: %w", err)
			}
		} else {
			if _, err = txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, source_order_id, created_at, updated_at)
VALUES ($1, 'accrue', $2, $3, $4, NOW(), NOW())`, inviterID, amount, inviteeUserID, nullableInt64Arg(sourceOrderID)); err != nil {
				return fmt.Errorf("insert affiliate accrue ledger: %w", err)
			}
		}

		applied = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (r *affiliateRepository) GetAccruedRebateFromInvitee(ctx context.Context, inviterID, inviteeUserID int64) (float64, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx,
		`SELECT COALESCE(SUM(amount), 0)::double precision FROM user_affiliate_ledger WHERE user_id = $1 AND source_user_id = $2 AND action = 'accrue'`,
		inviterID, inviteeUserID)
	if err != nil {
		return 0, fmt.Errorf("query accrued rebate from invitee: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var total float64
	if rows.Next() {
		if err := rows.Scan(&total); err != nil {
			return 0, err
		}
	}
	return total, rows.Close()
}

func (r *affiliateRepository) ThawFrozenQuota(ctx context.Context, userID int64) (float64, error) {
	var thawed float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var err error
		thawed, err = thawFrozenQuotaTx(txCtx, txClient, userID)
		return err
	})
	return thawed, err
}

// thawFrozenQuotaTx moves matured frozen quota to available quota within an existing tx.
func thawFrozenQuotaTx(txCtx context.Context, txClient *dbent.Client, userID int64) (float64, error) {
	rows, err := txClient.QueryContext(txCtx, `
WITH matured AS (
    UPDATE user_affiliate_ledger
    SET frozen_until = NULL, updated_at = NOW()
    WHERE user_id = $1
      AND frozen_until IS NOT NULL
      AND frozen_until <= NOW()
    RETURNING amount, referral_reward_id
), referral_available AS (
    UPDATE referral_campaign_rewards r
    SET status = 'available', updated_at = NOW(), version = version + 1
    FROM matured m
    WHERE r.id = m.referral_reward_id
      AND r.status = 'claimed_frozen'
    RETURNING r.id
)
SELECT COALESCE(SUM(amount), 0) FROM matured`, userID)
	if err != nil {
		return 0, fmt.Errorf("thaw frozen quota: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var thawed float64
	if rows.Next() {
		if err := rows.Scan(&thawed); err != nil {
			return 0, err
		}
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if thawed <= 0 {
		return 0, nil
	}

	_, err = txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_quota = aff_quota + $1,
    aff_frozen_quota = GREATEST(aff_frozen_quota - $1, 0),
    updated_at = NOW()
WHERE user_id = $2`, thawed, userID)
	if err != nil {
		return 0, fmt.Errorf("move thawed quota: %w", err)
	}
	return thawed, nil
}

func (r *affiliateRepository) TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error) {
	return r.transferQuotaToBalance(ctx, userID, nil)
}

func (r *affiliateRepository) TransferQuotaToBalanceWithLedger(ctx context.Context, userID int64, ledger service.BalanceLedgerApplier) (float64, float64, error) {
	return r.transferQuotaToBalance(ctx, userID, ledger)
}

func (r *affiliateRepository) transferQuotaToBalance(ctx context.Context, userID int64, ledger service.BalanceLedgerApplier) (float64, float64, error) {
	var transferred float64
	var newBalance float64

	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}

		// Thaw any matured frozen quota before transfer.
		if _, err := thawFrozenQuotaTx(txCtx, txClient, userID); err != nil {
			return fmt.Errorf("thaw before transfer: %w", err)
		}
		var hasDebtReview bool
		if err := scanAffiliateRow(txCtx, txClient, `SELECT EXISTS (SELECT 1 FROM referral_campaign_rewards WHERE user_id=$1 AND status='debt_review')`, []any{userID}, &hasDebtReview); err != nil {
			return fmt.Errorf("check referral reward debt: %w", err)
		}
		if hasDebtReview {
			return service.ErrAffiliateDebtReview
		}

		rows, err := txClient.QueryContext(txCtx, `
WITH claimed AS (
	SELECT aff_quota::double precision AS amount
	FROM user_affiliates
	WHERE user_id = $1
	  AND aff_quota > 0
	FOR UPDATE
),
cleared AS (
	UPDATE user_affiliates ua
	SET aff_quota = 0,
	    updated_at = NOW()
	FROM claimed c
	WHERE ua.user_id = $1
	RETURNING c.amount
)
SELECT amount
FROM cleared`, userID)
		if err != nil {
			return fmt.Errorf("claim affiliate quota: %w", err)
		}

		if !rows.Next() {
			_ = rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			return service.ErrAffiliateQuotaEmpty
		}
		if err := rows.Scan(&transferred); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if transferred <= 0 {
			return service.ErrAffiliateQuotaEmpty
		}

		snapshot, err := queryAffiliateTransferSnapshot(txCtx, txClient, userID)
		if err != nil {
			return err
		}

		if ledger == nil {
			affected, err := txClient.User.Update().
				Where(user.IDEQ(userID)).
				AddBalance(transferred).
				AddTotalRecharged(transferred).
				Save(txCtx)
			if err != nil {
				return fmt.Errorf("credit user balance by affiliate quota: %w", err)
			}
			if affected == 0 {
				return service.ErrUserNotFound
			}

			newBalance, err = queryUserBalance(txCtx, txClient, userID)
			if err != nil {
				return err
			}

			snapshot.BalanceAfter = newBalance
			if _, err = insertAffiliateTransferLedger(txCtx, txClient, userID, transferred, &snapshot.BalanceAfter, snapshot); err != nil {
				return err
			}
			return nil
		}

		transferLedgerID, err := insertAffiliateTransferLedger(txCtx, txClient, userID, transferred, nil, snapshot)
		if err != nil {
			return err
		}
		transaction, err := ledger.ApplyDelta(txCtx, service.BalanceLedgerApplyInput{
			UserID:         userID,
			BalanceDelta:   transferred,
			SourceType:     "affiliate_balance",
			SourceID:       fmt.Sprintf("%d", transferLedgerID),
			IdempotencyKey: fmt.Sprintf("affiliate_transfer:%d", transferLedgerID),
			ActorType:      service.BalanceLedgerActorUser,
			ActorUserID:    &userID,
			Description:    "返利余额转入",
			Metadata: map[string]any{
				"affiliate_ledger_id":         transferLedgerID,
				"aff_quota_after":             snapshot.AvailableQuotaAfter,
				"aff_frozen_quota_after":      snapshot.FrozenQuotaAfter,
				"aff_history_quota_after":     snapshot.HistoryQuotaAfter,
				"transferred_affiliate_quota": transferred,
			},
		})
		if err != nil {
			return fmt.Errorf("apply affiliate balance ledger delta: %w", err)
		}
		if transaction.BalanceAfter == nil {
			return fmt.Errorf("affiliate balance ledger transaction missing balance_after")
		}
		newBalance = *transaction.BalanceAfter
		if err := adjustAffiliateTransferTotalRecharged(txCtx, txClient, userID, transferred); err != nil {
			return err
		}
		if err := updateAffiliateTransferBalanceAfter(txCtx, txClient, transferLedgerID, newBalance); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	return transferred, newBalance, nil
}

func (r *affiliateRepository) ListInvitees(ctx context.Context, inviterID int64, limit int) ([]service.AffiliateInvitee, error) {
	if limit <= 0 {
		limit = 100
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ua.created_at,
       COALESCE(SUM(ual.amount), 0)::double precision AS total_rebate
FROM user_affiliates ua
LEFT JOIN users u ON u.id = ua.user_id
LEFT JOIN user_affiliate_ledger ual
       ON ual.user_id = $1
      AND ual.source_user_id = ua.user_id
      AND ual.action = 'accrue'
WHERE ua.inviter_id = $1
GROUP BY ua.user_id, u.email, u.username, ua.created_at
ORDER BY ua.created_at DESC
LIMIT $2`, inviterID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	invitees := make([]service.AffiliateInvitee, 0)
	for rows.Next() {
		var item service.AffiliateInvitee
		var createdAt time.Time
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &createdAt, &item.TotalRebate); err != nil {
			return nil, err
		}
		item.CreatedAt = &createdAt
		invitees = append(invitees, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return invitees, nil
}

func (r *affiliateRepository) ListAffiliateInviteRecords(ctx context.Context, filter service.AffiliateRecordFilter) ([]service.AffiliateInviteRecord, int64, error) {
	client := clientFromContext(ctx, r.client)
	where, args := buildAffiliateRecordWhere(filter, "ua.created_at", []string{
		"inviter.email", "inviter.username", "invitee.email", "invitee.username",
		"ua.inviter_id::text", "ua.user_id::text", "inviter_aff.aff_code",
	})

	total, err := queryAffiliateRecordCount(ctx, client, `
SELECT COUNT(*)
FROM user_affiliates ua
JOIN users invitee ON invitee.id = ua.user_id
JOIN users inviter ON inviter.id = ua.inviter_id
JOIN user_affiliates inviter_aff ON inviter_aff.user_id = ua.inviter_id
`+where, args...)
	if err != nil {
		return nil, 0, err
	}

	orderBy := buildAffiliateRecordOrderBy(filter, map[string]string{
		"inviter":      "inviter.email",
		"invitee":      "invitee.email",
		"aff_code":     "inviter_aff.aff_code",
		"total_rebate": "total_rebate",
		"created_at":   "ua.created_at",
	}, "ua.created_at")
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := client.QueryContext(ctx, `
SELECT ua.inviter_id,
       COALESCE(inviter.email, ''),
       COALESCE(inviter.username, ''),
       ua.user_id,
       COALESCE(invitee.email, ''),
       COALESCE(invitee.username, ''),
       COALESCE(inviter_aff.aff_code, ''),
       COALESCE(SUM(ual.amount), 0)::double precision AS total_rebate,
       ua.created_at
FROM user_affiliates ua
JOIN users invitee ON invitee.id = ua.user_id
JOIN users inviter ON inviter.id = ua.inviter_id
JOIN user_affiliates inviter_aff ON inviter_aff.user_id = ua.inviter_id
LEFT JOIN user_affiliate_ledger ual
       ON ual.user_id = ua.inviter_id
      AND ual.source_user_id = ua.user_id
      AND ual.action = 'accrue'
`+where+`
GROUP BY ua.inviter_id, inviter.email, inviter.username, ua.user_id, invitee.email, invitee.username, inviter_aff.aff_code, ua.created_at
`+orderBy+`
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateInviteRecord, 0)
	for rows.Next() {
		var item service.AffiliateInviteRecord
		if err := rows.Scan(
			&item.InviterID,
			&item.InviterEmail,
			&item.InviterUsername,
			&item.InviteeID,
			&item.InviteeEmail,
			&item.InviteeUsername,
			&item.AffCode,
			&item.TotalRebate,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) ListAffiliateRebateRecords(ctx context.Context, filter service.AffiliateRecordFilter) ([]service.AffiliateRebateRecord, int64, error) {
	client := clientFromContext(ctx, r.client)
	where, args := buildAffiliateRecordWhere(filter, "ual.created_at", []string{
		"inviter.email", "inviter.username", "invitee.email", "invitee.username",
		"po.id::text", "po.out_trade_no", "po.payment_type", "po.status",
	})
	baseJoin := `
FROM user_affiliate_ledger ual
JOIN payment_orders po ON po.id = ual.source_order_id
JOIN users invitee ON invitee.id = ual.source_user_id
JOIN users inviter ON inviter.id = ual.user_id
WHERE ual.action = 'accrue'
  AND ual.source_order_id IS NOT NULL`
	if where != "" {
		where = strings.Replace(where, "WHERE ", " AND ", 1)
	}

	total, err := queryAffiliateRecordCount(ctx, client, "SELECT COUNT(*) "+baseJoin+where, args...)
	if err != nil {
		return nil, 0, err
	}

	orderBy := buildAffiliateRecordOrderBy(filter, map[string]string{
		"order":         "po.id",
		"inviter":       "inviter.email",
		"invitee":       "invitee.email",
		"order_amount":  "po.amount",
		"pay_amount":    "po.pay_amount",
		"rebate_amount": "ual.amount",
		"payment_type":  "po.payment_type",
		"order_status":  "po.status",
		"created_at":    "ual.created_at",
	}, "ual.created_at")
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := client.QueryContext(ctx, `
SELECT po.id,
       po.out_trade_no,
       ual.user_id,
       COALESCE(inviter.email, ''),
       COALESCE(inviter.username, ''),
       ual.source_user_id,
       COALESCE(invitee.email, ''),
       COALESCE(invitee.username, ''),
       po.amount::double precision,
       po.pay_amount::double precision,
       ual.amount::double precision,
       po.payment_type,
       po.status,
       ual.created_at
`+baseJoin+where+`
`+orderBy+`
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateRebateRecord, 0)
	for rows.Next() {
		var item service.AffiliateRebateRecord
		if err := rows.Scan(
			&item.OrderID,
			&item.OutTradeNo,
			&item.InviterID,
			&item.InviterEmail,
			&item.InviterUsername,
			&item.InviteeID,
			&item.InviteeEmail,
			&item.InviteeUsername,
			&item.OrderAmount,
			&item.PayAmount,
			&item.RebateAmount,
			&item.PaymentType,
			&item.OrderStatus,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) ListAffiliateTransferRecords(ctx context.Context, filter service.AffiliateRecordFilter) ([]service.AffiliateTransferRecord, int64, error) {
	client := clientFromContext(ctx, r.client)
	where, args := buildAffiliateRecordWhere(filter, "ual.created_at", []string{
		"u.email", "u.username", "u.id::text",
	})
	baseJoin := `
FROM user_affiliate_ledger ual
JOIN users u ON u.id = ual.user_id
WHERE ual.action = 'transfer'`
	if where != "" {
		where = strings.Replace(where, "WHERE ", " AND ", 1)
	}

	total, err := queryAffiliateRecordCount(ctx, client, "SELECT COUNT(*) "+baseJoin+where, args...)
	if err != nil {
		return nil, 0, err
	}

	orderBy := buildAffiliateRecordOrderBy(filter, map[string]string{
		"user":                  "u.email",
		"amount":                "ual.amount",
		"balance_after":         "ual.balance_after",
		"available_quota_after": "ual.aff_quota_after",
		"frozen_quota_after":    "ual.aff_frozen_quota_after",
		"history_quota_after":   "ual.aff_history_quota_after",
		"created_at":            "ual.created_at",
	}, "ual.created_at")
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := client.QueryContext(ctx, `
SELECT ual.id,
       ual.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ual.amount::double precision,
       ual.balance_after::double precision,
       ual.aff_quota_after::double precision,
       ual.aff_frozen_quota_after::double precision,
       ual.aff_history_quota_after::double precision,
       ual.created_at
`+baseJoin+where+`
`+orderBy+`
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateTransferRecord, 0)
	for rows.Next() {
		var item service.AffiliateTransferRecord
		var balanceAfter sql.NullFloat64
		var availableQuotaAfter sql.NullFloat64
		var frozenQuotaAfter sql.NullFloat64
		var historyQuotaAfter sql.NullFloat64
		if err := rows.Scan(
			&item.LedgerID,
			&item.UserID,
			&item.UserEmail,
			&item.Username,
			&item.Amount,
			&balanceAfter,
			&availableQuotaAfter,
			&frozenQuotaAfter,
			&historyQuotaAfter,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		item.BalanceAfter = nullableFloat64Ptr(balanceAfter)
		item.AvailableQuotaAfter = nullableFloat64Ptr(availableQuotaAfter)
		item.FrozenQuotaAfter = nullableFloat64Ptr(frozenQuotaAfter)
		item.HistoryQuotaAfter = nullableFloat64Ptr(historyQuotaAfter)
		item.SnapshotAvailable = balanceAfter.Valid &&
			availableQuotaAfter.Valid &&
			frozenQuotaAfter.Valid &&
			historyQuotaAfter.Valid
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) GetAffiliateUserOverview(ctx context.Context, userID int64) (*service.AffiliateUserOverview, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, affiliateUserOverviewSQL, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrUserNotFound
	}

	var overview service.AffiliateUserOverview
	var customRate float64
	var hasCustomRate bool
	if err := rows.Scan(
		&overview.UserID,
		&overview.Email,
		&overview.Username,
		&overview.AffCode,
		&customRate,
		&hasCustomRate,
		&overview.InvitedCount,
		&overview.RebatedInviteeCount,
		&overview.AvailableQuota,
		&overview.HistoryQuota,
	); err != nil {
		return nil, err
	}
	if hasCustomRate {
		overview.RebateRatePercent = customRate
		overview.RebateRateCustom = true
	}
	return &overview, rows.Err()
}

func buildAffiliateRecordWhere(filter service.AffiliateRecordFilter, timeColumn string, searchColumns []string) (string, []any) {
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if filter.StartAt != nil {
		args = append(args, *filter.StartAt)
		clauses = append(clauses, fmt.Sprintf("%s >= $%d", timeColumn, len(args)))
	}
	if filter.EndAt != nil {
		args = append(args, *filter.EndAt)
		clauses = append(clauses, fmt.Sprintf("%s <= $%d", timeColumn, len(args)))
	}
	search := strings.TrimSpace(filter.Search)
	if search != "" && len(searchColumns) > 0 {
		args = append(args, "%"+strings.ToLower(search)+"%")
		parts := make([]string, 0, len(searchColumns))
		for _, col := range searchColumns {
			parts = append(parts, fmt.Sprintf("LOWER(%s) LIKE $%d", col, len(args)))
		}
		clauses = append(clauses, "("+strings.Join(parts, " OR ")+")")
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func buildAffiliateRecordOrderBy(filter service.AffiliateRecordFilter, sortColumns map[string]string, fallbackColumn string) string {
	column := sortColumns[filter.SortBy]
	if column == "" {
		column = fallbackColumn
	}
	direction := "DESC"
	if !filter.SortDesc {
		direction = "ASC"
	}
	return "ORDER BY " + column + " " + direction + " NULLS LAST"
}

func queryAffiliateRecordCount(ctx context.Context, client affiliateQueryExecer, query string, args ...any) (int64, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var total int64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, rows.Err()
}

func scanAffiliateRow(ctx context.Context, client affiliateQueryExecer, query string, args []any, dest ...any) error {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	return rows.Err()
}

func (r *affiliateRepository) withTx(ctx context.Context, fn func(txCtx context.Context, txClient *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin affiliate transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit affiliate transaction: %w", err)
	}
	return nil
}

func ensureUserAffiliateWithClient(ctx context.Context, client affiliateQueryExecer, userID int64) (*service.AffiliateSummary, error) {
	summary, err := queryAffiliateByUserID(ctx, client, userID)
	if err == nil {
		return summary, nil
	}
	if !errors.Is(err, service.ErrAffiliateProfileNotFound) {
		return nil, err
	}

	for i := 0; i < affiliateCodeMaxAttempts; i++ {
		code, codeErr := generateAffiliateCode()
		if codeErr != nil {
			return nil, codeErr
		}
		_, insertErr := client.ExecContext(ctx, `
INSERT INTO user_affiliates (user_id, aff_code, created_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (user_id) DO NOTHING`, userID, code)
		if insertErr == nil {
			break
		}
		if isAffiliateUniqueViolation(insertErr) {
			continue
		}
		return nil, insertErr
	}

	return queryAffiliateByUserID(ctx, client, userID)
}

func queryAffiliateByUserID(ctx context.Context, client affiliateQueryExecer, userID int64) (*service.AffiliateSummary, error) {
	rows, err := client.QueryContext(ctx, `
SELECT user_id,
       aff_code,
       aff_code_custom,
       aff_rebate_rate_percent,
       inviter_id,
       aff_count,
       aff_quota::double precision,
       aff_frozen_quota::double precision,
       aff_history_quota::double precision,
       created_at,
       updated_at
FROM user_affiliates
WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAffiliateProfileNotFound
	}

	var out service.AffiliateSummary
	var inviterID sql.NullInt64
	var rebateRate sql.NullFloat64
	if err := rows.Scan(
		&out.UserID,
		&out.AffCode,
		&out.AffCodeCustom,
		&rebateRate,
		&inviterID,
		&out.AffCount,
		&out.AffQuota,
		&out.AffFrozenQuota,
		&out.AffHistoryQuota,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if inviterID.Valid {
		out.InviterID = &inviterID.Int64
	}
	if rebateRate.Valid {
		v := rebateRate.Float64
		out.AffRebateRatePercent = &v
	}
	return &out, nil
}

func queryAffiliateByCode(ctx context.Context, client affiliateQueryExecer, code string) (*service.AffiliateSummary, error) {
	rows, err := client.QueryContext(ctx, `
SELECT user_id,
       aff_code,
       aff_code_custom,
       aff_rebate_rate_percent,
       inviter_id,
       aff_count,
       aff_quota::double precision,
       aff_frozen_quota::double precision,
       aff_history_quota::double precision,
       created_at,
       updated_at
FROM user_affiliates
WHERE aff_code = $1
LIMIT 1`, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAffiliateProfileNotFound
	}

	var out service.AffiliateSummary
	var inviterID sql.NullInt64
	var rebateRate sql.NullFloat64
	if err := rows.Scan(
		&out.UserID,
		&out.AffCode,
		&out.AffCodeCustom,
		&rebateRate,
		&inviterID,
		&out.AffCount,
		&out.AffQuota,
		&out.AffFrozenQuota,
		&out.AffHistoryQuota,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if inviterID.Valid {
		out.InviterID = &inviterID.Int64
	}
	if rebateRate.Valid {
		v := rebateRate.Float64
		out.AffRebateRatePercent = &v
	}
	return &out, nil
}

func queryUserBalance(ctx context.Context, client affiliateQueryExecer, userID int64) (float64, error) {
	rows, err := client.QueryContext(ctx,
		"SELECT balance::double precision FROM users WHERE id = $1 LIMIT 1",
		userID,
	)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, service.ErrUserNotFound
	}
	var balance float64
	if err := rows.Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

type affiliateTransferSnapshot struct {
	BalanceAfter        float64
	AvailableQuotaAfter float64
	FrozenQuotaAfter    float64
	HistoryQuotaAfter   float64
}

func queryAffiliateTransferSnapshot(ctx context.Context, client affiliateQueryExecer, userID int64) (*affiliateTransferSnapshot, error) {
	rows, err := client.QueryContext(ctx, `
SELECT u.balance::double precision,
       ua.aff_quota::double precision,
       ua.aff_frozen_quota::double precision,
       ua.aff_history_quota::double precision
FROM users u
JOIN user_affiliates ua ON ua.user_id = u.id
WHERE u.id = $1
LIMIT 1`, userID)
	if err != nil {
		return nil, fmt.Errorf("query affiliate transfer snapshot: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrUserNotFound
	}

	var snapshot affiliateTransferSnapshot
	if err := rows.Scan(
		&snapshot.BalanceAfter,
		&snapshot.AvailableQuotaAfter,
		&snapshot.FrozenQuotaAfter,
		&snapshot.HistoryQuotaAfter,
	); err != nil {
		return nil, err
	}
	return &snapshot, rows.Err()
}

func insertAffiliateTransferLedger(ctx context.Context, client affiliateQueryExecer, userID int64, amount float64, balanceAfter *float64, snapshot *affiliateTransferSnapshot) (int64, error) {
	var balanceAfterArg any
	if balanceAfter != nil {
		balanceAfterArg = *balanceAfter
	}
	rows, err := client.QueryContext(ctx, `
INSERT INTO user_affiliate_ledger (
    user_id,
    action,
    amount,
    source_user_id,
    balance_after,
    aff_quota_after,
    aff_frozen_quota_after,
    aff_history_quota_after,
    created_at,
    updated_at
)
VALUES ($1, 'transfer', $2, NULL, $3, $4, $5, $6, NOW(), NOW())
RETURNING id`,
		userID,
		amount,
		balanceAfterArg,
		snapshot.AvailableQuotaAfter,
		snapshot.FrozenQuotaAfter,
		snapshot.HistoryQuotaAfter,
	)
	if err != nil {
		return 0, fmt.Errorf("insert affiliate transfer ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("insert affiliate transfer ledger returned no id")
	}
	var id int64
	if err := rows.Scan(&id); err != nil {
		return 0, err
	}
	return id, rows.Err()
}

func updateAffiliateTransferBalanceAfter(ctx context.Context, client affiliateQueryExecer, ledgerID int64, balanceAfter float64) error {
	result, err := client.ExecContext(ctx, `
UPDATE user_affiliate_ledger
SET balance_after = $1,
    updated_at = NOW()
WHERE id = $2 AND action = 'transfer'`, balanceAfter, ledgerID)
	if err != nil {
		return fmt.Errorf("update affiliate transfer balance snapshot: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("affiliate transfer ledger not found: %d", ledgerID)
	}
	return nil
}

// The referral campaign repository intentionally lives beside the existing
// affiliate repository so registration binding and campaign attribution can be
// committed in one transaction. The service uses a separate optional
// interface, keeping old AffiliateRepository test doubles source-compatible.
func (r *affiliateRepository) CreateReferralCampaign(ctx context.Context, campaign service.ReferralCampaign, tiers []service.ReferralCampaignTier) (*service.ReferralCampaign, error) {
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		rankRewards, err := json.Marshal(campaign.RankRewards)
		if err != nil {
			return fmt.Errorf("encode referral rank rewards: %w", err)
		}
		if err := scanAffiliateRow(txCtx, txClient, `
INSERT INTO referral_campaigns (
 campaign_key,name,status,version,registration_from,registration_to,starts_at,ends_at,
 qualification_to,claim_deadline,risk_hold_hours,pay_threshold,usage_threshold,max_enrollments,
 budget_total,reward_mode,rank_rewards_json,signing_secret,created_by,public_rules_md,invitee_notice_md,legacy_rebate_policy,rules_version
) VALUES ($1,$2,'draft',1,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,1)
RETURNING id`, []any{strings.TrimSpace(campaign.Key), strings.TrimSpace(campaign.Name), campaign.RegistrationFrom,
			campaign.RegistrationTo, campaign.StartsAt, campaign.EndsAt, campaign.QualificationTo,
			campaign.ClaimDeadline, campaign.RiskHoldHours, campaign.PayThreshold, campaign.UsageThreshold,
			campaign.MaxEnrollments, campaign.BudgetTotal, campaign.RewardMode, rankRewards,
			campaign.SigningSecret, campaign.CreatedBy, strings.TrimSpace(campaign.PublicRulesMD), strings.TrimSpace(campaign.InviteeNoticeMD), campaign.LegacyRebatePolicy}, &campaign.ID); err != nil {
			return fmt.Errorf("create referral campaign: %w", err)
		}
		for _, tier := range tiers {
			if _, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_tiers (campaign_id,tier_no,required_invites,reward_amount,currency) VALUES ($1,$2,$3,$4,$5)`, campaign.ID, tier.Tier, tier.RequiredInvites, tier.RewardAmount, strings.ToUpper(strings.TrimSpace(tier.Currency))); err != nil {
				return fmt.Errorf("create referral campaign tier: %w", err)
			}
		}
		snapshot, err := referralCampaignSnapshot(campaign, tiers)
		if err != nil {
			return err
		}
		if _, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_rule_versions (campaign_id,rules_version,change_kind,snapshot,changed_by) VALUES ($1,1,'created',$2,$3)`, campaign.ID, snapshot, campaign.CreatedBy); err != nil {
			return err
		}
		_, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) VALUES ($1,1,$2,'created',jsonb_build_object('campaign_key',$3::text))`, campaign.ID, campaign.CreatedBy, campaign.Key)
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.GetReferralCampaign(ctx, campaign.ID)
}

func (r *affiliateRepository) UpdateReferralCampaignContent(ctx context.Context, campaign service.ReferralCampaign, tiers []service.ReferralCampaignTier, actorID int64) (*service.ReferralCampaign, error) {
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var pendingCount int
		if err := scanAffiliateRow(txCtx, txClient, `SELECT COUNT(*) FROM referral_campaign_financial_versions WHERE campaign_id=$1 AND status='review'`, []any{campaign.ID}, &pendingCount); err != nil {
			return err
		}
		if pendingCount > 0 {
			return infraerrors.Conflict("REFERRAL_CAMPAIGN_FINANCIAL_VERSION_PENDING", "a financial rules version is already awaiting review")
		}
		var rulesVersion int64
		if err := scanAffiliateRow(txCtx, txClient, `
UPDATE referral_campaigns SET campaign_key=$3,name=$4,public_rules_md=$5,invitee_notice_md=$6,
 rules_version=rules_version+1,rules_updated_at=NOW(),version=version+1,updated_at=NOW()
WHERE id=$1 AND version=$2 AND status NOT IN ('settling','closed','cancelled')
RETURNING rules_version`, []any{campaign.ID, campaign.Version, strings.TrimSpace(campaign.Key), strings.TrimSpace(campaign.Name), strings.TrimSpace(campaign.PublicRulesMD), strings.TrimSpace(campaign.InviteeNoticeMD)}, &rulesVersion); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralCampaignVersionConflict
			}
			return err
		}
		campaign.RulesVersion = rulesVersion
		snapshot, err := referralCampaignSnapshot(campaign, tiers)
		if err != nil {
			return err
		}
		if _, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_rule_versions (campaign_id,rules_version,change_kind,snapshot,changed_by) VALUES ($1,$2,'content',$3,$4)`, campaign.ID, rulesVersion, snapshot, actorID); err != nil {
			return err
		}
		_, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) SELECT id,version,$2,'content_rules_updated',jsonb_build_object('rules_version',$3::bigint) FROM referral_campaigns WHERE id=$1`, campaign.ID, actorID, rulesVersion)
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.GetReferralCampaign(ctx, campaign.ID)
}

func (r *affiliateRepository) UpdateReferralCampaign(ctx context.Context, campaign service.ReferralCampaign, tiers []service.ReferralCampaignTier, actorID int64) (*service.ReferralCampaign, error) {
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		rankRewards, err := json.Marshal(campaign.RankRewards)
		if err != nil {
			return err
		}
		var currentStatus string
		var currentRulesVersion int64
		if err := scanAffiliateRow(txCtx, txClient, `SELECT status,rules_version FROM referral_campaigns WHERE id=$1 AND version=$2 AND status NOT IN ('settling','closed','cancelled') FOR UPDATE`, []any{campaign.ID, campaign.Version}, &currentStatus, &currentRulesVersion); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralCampaignVersionConflict
			}
			return fmt.Errorf("lock referral campaign for update: %w", err)
		}

		if currentStatus != service.ReferralCampaignStatusDraft {
			var activeCount int
			if err := scanAffiliateRow(txCtx, txClient, `SELECT COUNT(*) FROM referral_campaign_financial_versions WHERE campaign_id=$1 AND status='review'`, []any{campaign.ID}, &activeCount); err != nil {
				return err
			}
			if activeCount > 0 {
				return infraerrors.Conflict("REFERRAL_CAMPAIGN_FINANCIAL_VERSION_PENDING", "a financial rules version is already awaiting review")
			}
			var nextRulesVersion int64
			if err := scanAffiliateRow(txCtx, txClient, `
SELECT GREATEST(
  $2::bigint + 1,
  COALESCE((SELECT MAX(rules_version) + 1 FROM referral_campaign_rule_versions WHERE campaign_id=$1), 1),
  COALESCE((SELECT MAX(rules_version) + 1 FROM referral_campaign_financial_versions WHERE campaign_id=$1), 1)
)`, []any{campaign.ID, currentRulesVersion}, &nextRulesVersion); err != nil {
				return err
			}
			campaign.RulesVersion = nextRulesVersion
			snapshot, err := referralCampaignSnapshot(campaign, tiers)
			if err != nil {
				return err
			}
			if _, err = txClient.ExecContext(txCtx, `
INSERT INTO referral_campaign_financial_versions (campaign_id,rules_version,base_rules_version,snapshot,status,created_by)
VALUES ($1,$2,$3,$4,'review',$5)`, campaign.ID, nextRulesVersion, currentRulesVersion, snapshot, actorID); err != nil {
				return fmt.Errorf("stage referral campaign financial version: %w", err)
			}
			_, err = txClient.ExecContext(txCtx, `
INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail)
VALUES ($1,$2,$3,'financial_rules_submitted',jsonb_build_object('rules_version',$4::bigint,'base_rules_version',$5::bigint))`, campaign.ID, campaign.Version, actorID, nextRulesVersion, currentRulesVersion)
			return err
		}

		var rulesVersion int64
		if err := scanAffiliateRow(txCtx, txClient, `
UPDATE referral_campaigns SET campaign_key=$3,name=$4,registration_from=$5,registration_to=$6,
 starts_at=$7,ends_at=$8,qualification_to=$9,claim_deadline=$10,risk_hold_hours=$11,
 pay_threshold=$12,usage_threshold=$13,max_enrollments=$14,budget_total=$15,reward_mode=$16,
 rank_rewards_json=$17,public_rules_md=$18,invitee_notice_md=$19,legacy_rebate_policy=$20,
 rules_version=rules_version+1,rules_updated_at=NOW(),version=version+1,updated_at=NOW()
WHERE id=$1 AND version=$2 AND status='draft'
  AND $15 >= budget_reserved + budget_paid
RETURNING rules_version`, []any{campaign.ID, campaign.Version,
			strings.TrimSpace(campaign.Key), strings.TrimSpace(campaign.Name), campaign.RegistrationFrom,
			campaign.RegistrationTo, campaign.StartsAt, campaign.EndsAt, campaign.QualificationTo,
			campaign.ClaimDeadline, campaign.RiskHoldHours, campaign.PayThreshold, campaign.UsageThreshold,
			campaign.MaxEnrollments, campaign.BudgetTotal, campaign.RewardMode, rankRewards,
			strings.TrimSpace(campaign.PublicRulesMD), strings.TrimSpace(campaign.InviteeNoticeMD), campaign.LegacyRebatePolicy}, &rulesVersion); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralCampaignVersionConflict
			}
			return fmt.Errorf("update referral campaign: %w", err)
		}
		if _, err := txClient.ExecContext(txCtx, `DELETE FROM referral_campaign_tiers WHERE campaign_id=$1`, campaign.ID); err != nil {
			return err
		}
		for _, tier := range tiers {
			if _, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_tiers (campaign_id,tier_no,required_invites,reward_amount,currency) VALUES ($1,$2,$3,$4,$5)`, campaign.ID, tier.Tier, tier.RequiredInvites, tier.RewardAmount, strings.ToUpper(strings.TrimSpace(tier.Currency))); err != nil {
				return err
			}
		}
		campaign.RulesVersion = rulesVersion
		snapshot, err := referralCampaignSnapshot(campaign, tiers)
		if err != nil {
			return err
		}
		if _, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_rule_versions (campaign_id,rules_version,change_kind,snapshot,changed_by) VALUES ($1,$2,'financial',$3,$4)`, campaign.ID, rulesVersion, snapshot, actorID); err != nil {
			return err
		}
		_, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) SELECT id,version,$2,'rules_updated',jsonb_build_object('rules_version',$3::bigint) FROM referral_campaigns WHERE id=$1`, campaign.ID, actorID, rulesVersion)
		return err
	})
	if err != nil {
		return nil, err
	}
	return r.GetReferralCampaign(ctx, campaign.ID)
}

func referralCampaignSnapshot(campaign service.ReferralCampaign, tiers []service.ReferralCampaignTier) ([]byte, error) {
	return json.Marshal(referralCampaignSnapshotPayload{Campaign: campaign, Tiers: tiers})
}

type referralCampaignSnapshotPayload struct {
	Campaign service.ReferralCampaign       `json:"campaign"`
	Tiers    []service.ReferralCampaignTier `json:"tiers"`
}

func normalizeReferralPage(page, pageSize int) (int, int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize, (page - 1) * pageSize
}

func (r *affiliateRepository) ListReferralCampaigns(ctx context.Context, filter service.ReferralCampaignListFilter) (*service.ReferralCampaignPage, error) {
	page, pageSize, offset := normalizeReferralPage(filter.Page, filter.PageSize)
	client := clientFromContext(ctx, r.client)
	search := "%" + strings.TrimSpace(filter.Search) + "%"
	status := strings.TrimSpace(filter.Status)
	where := `WHERE ($1='' OR status=$1) AND ($2='%%' OR campaign_key ILIKE $2 OR name ILIKE $2)`
	total, err := scanInt64(ctx, client, `SELECT COUNT(*) FROM referral_campaigns `+where, status, search)
	if err != nil {
		return nil, fmt.Errorf("count referral campaigns: %w", err)
	}
	rows, err := client.QueryContext(ctx, `
SELECT id,campaign_key,name,status,version,registration_from,registration_to,starts_at,ends_at,
 qualification_to,claim_deadline,risk_hold_hours,pay_threshold::double precision,usage_threshold::double precision,
 max_enrollments,budget_total::double precision,budget_reserved::double precision,budget_paid::double precision,
 reward_mode,public_rules_md,invitee_notice_md,legacy_rebate_policy,rules_version,rules_updated_at,created_by,approved_by
FROM referral_campaigns `+where+` ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, status, search, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list referral campaigns: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ReferralCampaign, 0)
	for rows.Next() {
		var c service.ReferralCampaign
		var createdBy, approvedBy sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Key, &c.Name, &c.Status, &c.Version, &c.RegistrationFrom, &c.RegistrationTo,
			&c.StartsAt, &c.EndsAt, &c.QualificationTo, &c.ClaimDeadline, &c.RiskHoldHours, &c.PayThreshold,
			&c.UsageThreshold, &c.MaxEnrollments, &c.BudgetTotal, &c.BudgetReserved, &c.BudgetPaid, &c.RewardMode,
			&c.PublicRulesMD, &c.InviteeNoticeMD, &c.LegacyRebatePolicy, &c.RulesVersion, &c.RulesUpdatedAt, &createdBy, &approvedBy); err != nil {
			return nil, err
		}
		if createdBy.Valid {
			c.CreatedBy = createdBy.Int64
		}
		if approvedBy.Valid {
			v := approvedBy.Int64
			c.ApprovedBy = &v
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &service.ReferralCampaignPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *affiliateRepository) ListRunningReferralCampaignIDs(ctx context.Context, limit int) ([]int64, error) {
	if limit < 1 || limit > 20 {
		limit = 20
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT id FROM referral_campaigns
WHERE (status IN ('scheduled','running') AND registration_from<=NOW() AND registration_to>NOW())
   OR (status='settling' AND claim_deadline>NOW())
ORDER BY CASE WHEN status='settling' THEN 0 ELSE 1 END, starts_at DESC,id DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *affiliateRepository) GetReferralCampaign(ctx context.Context, id int64) (*service.ReferralCampaign, error) {
	client := clientFromContext(ctx, r.client)
	var c service.ReferralCampaign
	var createdBy, approvedBy sql.NullInt64
	var rankRewards []byte
	err := scanAffiliateRow(ctx, client, `
SELECT id, campaign_key, name, status, version, registration_from,
       registration_to, starts_at, ends_at, qualification_to, claim_deadline,
       risk_hold_hours, pay_threshold::double precision, usage_threshold::double precision,
       max_enrollments, budget_total::double precision, budget_reserved::double precision,
       budget_paid::double precision, reward_mode, public_rules_md, invitee_notice_md, legacy_rebate_policy,
       rules_version, rules_updated_at, rank_rewards_json, created_by, approved_by
FROM referral_campaigns WHERE id = $1`, []any{id}, &c.ID, &c.Key, &c.Name, &c.Status, &c.Version, &c.RegistrationFrom,
		&c.RegistrationTo, &c.StartsAt, &c.EndsAt, &c.QualificationTo, &c.ClaimDeadline,
		&c.RiskHoldHours, &c.PayThreshold, &c.UsageThreshold, &c.MaxEnrollments,
		&c.BudgetTotal, &c.BudgetReserved, &c.BudgetPaid, &c.RewardMode, &c.PublicRulesMD, &c.InviteeNoticeMD,
		&c.LegacyRebatePolicy, &c.RulesVersion, &c.RulesUpdatedAt, &rankRewards, &createdBy, &approvedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrReferralCampaignNotFound
		}
		return nil, fmt.Errorf("get referral campaign: %w", err)
	}
	if createdBy.Valid {
		c.CreatedBy = createdBy.Int64
	}
	if approvedBy.Valid {
		v := approvedBy.Int64
		c.ApprovedBy = &v
	}
	if len(rankRewards) > 0 {
		_ = json.Unmarshal(rankRewards, &c.RankRewards)
	}
	return &c, nil
}

func (r *affiliateRepository) ListReferralCampaignRuleVersions(ctx context.Context, campaignID int64) ([]service.ReferralCampaignRuleVersion, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `SELECT rules_version,change_kind,snapshot,changed_by,created_at FROM referral_campaign_rule_versions WHERE campaign_id=$1 ORDER BY rules_version DESC`, campaignID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.ReferralCampaignRuleVersion, 0)
	for rows.Next() {
		var item service.ReferralCampaignRuleVersion
		var changedBy sql.NullInt64
		if err := rows.Scan(&item.RulesVersion, &item.ChangeKind, &item.Snapshot, &changedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		if changedBy.Valid {
			value := changedBy.Int64
			item.ChangedBy = &value
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *affiliateRepository) GetReferralCampaignRuleVersion(ctx context.Context, campaignID, rulesVersion int64) (*service.ReferralCampaignRuleVersion, error) {
	client := clientFromContext(ctx, r.client)
	var item service.ReferralCampaignRuleVersion
	var changedBy sql.NullInt64
	if err := scanAffiliateRow(ctx, client, `SELECT rules_version,change_kind,snapshot,changed_by,created_at FROM referral_campaign_rule_versions WHERE campaign_id=$1 AND rules_version=$2`, []any{campaignID, rulesVersion}, &item.RulesVersion, &item.ChangeKind, &item.Snapshot, &changedBy, &item.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrReferralCampaignNotFound
		}
		return nil, err
	}
	if changedBy.Valid {
		value := changedBy.Int64
		item.ChangedBy = &value
	}
	return &item, nil
}

func (r *affiliateRepository) GetPendingReferralCampaignFinancialVersion(ctx context.Context, campaignID int64) (*service.ReferralCampaignFinancialVersion, error) {
	client := clientFromContext(ctx, r.client)
	var item service.ReferralCampaignFinancialVersion
	var createdBy sql.NullInt64
	err := scanAffiliateRow(ctx, client, `
SELECT rules_version,base_rules_version,snapshot,status,created_by,created_at
FROM referral_campaign_financial_versions
WHERE campaign_id=$1 AND status='review'`, []any{campaignID},
		&item.RulesVersion, &item.BaseRulesVersion, &item.Snapshot, &item.Status, &createdBy, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pending referral campaign financial version: %w", err)
	}
	if createdBy.Valid {
		value := createdBy.Int64
		item.CreatedBy = &value
	}
	return &item, nil
}

func (r *affiliateRepository) MarkReferralCampaignViewed(ctx context.Context, campaignID, userID, rulesVersion int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ExecContext(ctx, `INSERT INTO referral_campaign_user_views (campaign_id,user_id,rules_version,seen_at) VALUES ($1,$2,$3,NOW()) ON CONFLICT (campaign_id,user_id) DO UPDATE SET rules_version=EXCLUDED.rules_version,seen_at=NOW()`, campaignID, userID, rulesVersion)
	return err
}

func (r *affiliateRepository) ShouldSuppressLegacyReferralRebate(ctx context.Context, inviteeID int64) (bool, error) {
	client := clientFromContext(ctx, r.client)
	var found bool
	err := scanAffiliateRow(ctx, client, `SELECT EXISTS (SELECT 1 FROM referral_campaign_attributions WHERE invitee_id=$1 AND legacy_rebate_policy='exclude' AND status IN ('pending','approved','rejected','revoked'))`, []any{inviteeID}, &found)
	return found, err
}

func (r *affiliateRepository) AdvanceReferralCampaigns(ctx context.Context, now time.Time) (int, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
UPDATE referral_campaigns
SET status=CASE
    WHEN status='scheduled' AND starts_at <= $1 THEN 'running'
    WHEN status IN ('running','paused') AND ends_at <= $1 THEN 'settling'
    WHEN status='settling' AND claim_deadline <= $1 THEN 'closed'
    ELSE status END,
    version=version+1,updated_at=NOW()
WHERE (status='scheduled' AND starts_at <= $1)
   OR (status IN ('running','paused') AND ends_at <= $1)
   OR (status='settling' AND claim_deadline <= $1)
RETURNING id,version,status`, now)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		var id, version int64
		var status string
		if err := rows.Scan(&id, &version, &status); err != nil {
			return count, err
		}
		if _, err := client.ExecContext(ctx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) VALUES ($1,$2,NULL,'auto_status_changed',jsonb_build_object('status',$3::text))`, id, version, status); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func (r *affiliateRepository) SetReferralCampaignStatus(ctx context.Context, id, expectedVersion int64, status string, actorID int64, note string) (*service.ReferralCampaign, error) {
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var version int64
		if err := scanAffiliateRow(txCtx, txClient, `SELECT version FROM referral_campaigns WHERE id=$1 FOR UPDATE`, []any{id}, &version); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralCampaignVersionConflict
			}
			return err
		}
		if version != expectedVersion {
			return service.ErrReferralCampaignVersionConflict
		}

		type releasedReward struct {
			id     int64
			amount float64
		}
		released := make([]releasedReward, 0)
		totalReleased := 0.0
		if status == service.ReferralCampaignStatusClosed {
			rows, err := txClient.QueryContext(txCtx, `SELECT id,amount::double precision FROM referral_campaign_rewards WHERE campaign_id=$1 AND status='claimable' AND claim_deadline<=NOW() FOR UPDATE`, id)
			if err != nil {
				return err
			}
			for rows.Next() {
				var reward releasedReward
				if err := rows.Scan(&reward.id, &reward.amount); err != nil {
					_ = rows.Close()
					return err
				}
				released = append(released, reward)
				totalReleased += reward.amount
			}
			if err := rows.Close(); err != nil {
				return err
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if _, err := txClient.ExecContext(txCtx, `UPDATE referral_campaign_rewards SET status='expired',updated_at=NOW(),version=version+1 WHERE campaign_id=$1 AND status='claimable' AND claim_deadline<=NOW()`, id); err != nil {
				return err
			}
		}

		var nextVersion int64
		if err := scanAffiliateRow(txCtx, txClient, `UPDATE referral_campaigns SET status=$1,version=version+1,budget_reserved=CASE WHEN $1='closed' THEN GREATEST(budget_reserved-$2,0) ELSE budget_reserved END,updated_at=NOW() WHERE id=$3 AND version=$4 RETURNING version`, []any{status, totalReleased, id, expectedVersion}, &nextVersion); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralCampaignVersionConflict
			}
			return err
		}
		for _, reward := range released {
			if _, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key) VALUES ($1,$2,'release',$3,$4) ON CONFLICT (idempotency_key) DO NOTHING`, id, reward.id, reward.amount, fmt.Sprintf("referral:reward:%d:closed-expire", reward.id)); err != nil {
				return err
			}
		}
		_, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) VALUES ($1,$2,$3,'status_changed',jsonb_build_object('status',$4::text,'note',$5::text))`, id, nextVersion, actorID, status, strings.TrimSpace(note))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("set referral campaign status: %w", err)
	}
	return r.GetReferralCampaign(ctx, id)
}

func (r *affiliateRepository) GetReferralCampaignEarlyClosePreview(ctx context.Context, campaignID int64) (*service.ReferralCampaignEarlyClosePreview, error) {
	client := clientFromContext(ctx, r.client)
	preview := &service.ReferralCampaignEarlyClosePreview{CampaignID: campaignID}
	err := scanAffiliateRow(ctx, client, `
SELECT c.version,c.status,c.claim_deadline,
       COUNT(*) FILTER (WHERE r.status='claimable'),
       COALESCE(SUM(r.amount) FILTER (WHERE r.status='claimable'),0)::double precision,
       COUNT(*) FILTER (WHERE r.status IN ('claimed_frozen','available','debt_review','resolved')),
       COALESCE(SUM(r.amount) FILTER (WHERE r.status IN ('claimed_frozen','available','debt_review','resolved')),0)::double precision
FROM referral_campaigns c
LEFT JOIN referral_campaign_rewards r ON r.campaign_id=c.id
WHERE c.id=$1
GROUP BY c.id,c.version,c.status,c.claim_deadline`, []any{campaignID},
		&preview.CampaignVersion, &preview.Status, &preview.ClaimDeadline,
		&preview.ClaimableRewardCount, &preview.ClaimableRewardAmount,
		&preview.PreservedRewardCount, &preview.PreservedRewardAmount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrReferralCampaignNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get referral campaign early-close preview: %w", err)
	}
	return preview, nil
}

func (r *affiliateRepository) EarlyCloseReferralCampaign(ctx context.Context, id, expectedVersion, actorID int64, reason string) (*service.ReferralCampaign, error) {
	reason = strings.TrimSpace(reason)
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var version int64
		var status string
		var claimDeadline time.Time
		if err := scanAffiliateRow(txCtx, txClient, `SELECT version,status,claim_deadline FROM referral_campaigns WHERE id=$1 FOR UPDATE`, []any{id}, &version, &status, &claimDeadline); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralCampaignNotFound
			}
			return err
		}
		if version != expectedVersion {
			return service.ErrReferralCampaignVersionConflict
		}
		if status != service.ReferralCampaignStatusSettling || !time.Now().UTC().Before(claimDeadline) {
			return service.ErrReferralCampaignInvalidState
		}

		type expiredReward struct {
			id     int64
			amount float64
		}
		expired := make([]expiredReward, 0)
		totalExpired := 0.0
		rows, err := txClient.QueryContext(txCtx, `SELECT id,amount::double precision FROM referral_campaign_rewards WHERE campaign_id=$1 AND status='claimable' FOR UPDATE`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var reward expiredReward
			if err := rows.Scan(&reward.id, &reward.amount); err != nil {
				_ = rows.Close()
				return err
			}
			expired = append(expired, reward)
			totalExpired += reward.amount
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if _, err := txClient.ExecContext(txCtx, `UPDATE referral_campaign_rewards SET status='expired',updated_at=NOW(),version=version+1 WHERE campaign_id=$1 AND status='claimable'`, id); err != nil {
			return err
		}

		var nextVersion int64
		if err := scanAffiliateRow(txCtx, txClient, `UPDATE referral_campaigns SET status='closed',version=version+1,budget_reserved=GREATEST(budget_reserved-$1,0),updated_at=NOW() WHERE id=$2 AND version=$3 AND status='settling' AND claim_deadline>NOW() RETURNING version`, []any{totalExpired, id, expectedVersion}, &nextVersion); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralCampaignVersionConflict
			}
			return err
		}
		for _, reward := range expired {
			if _, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key,metadata) VALUES ($1,$2,'release',$3,$4,jsonb_build_object('source','early_close','reason',$5::text)) ON CONFLICT (idempotency_key) DO NOTHING`, id, reward.id, reward.amount, fmt.Sprintf("referral:reward:%d:early-close-expire", reward.id), reason); err != nil {
				return err
			}
		}
		_, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) VALUES ($1,$2,$3,'early_closed',jsonb_build_object('reason',$4::text,'expired_reward_count',$5,'expired_reward_amount',$6))`, id, nextVersion, actorID, reason, len(expired), totalExpired)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("early close referral campaign: %w", err)
	}
	return r.GetReferralCampaign(ctx, id)
}

func (r *affiliateRepository) ReviewReferralCampaign(ctx context.Context, id, expectedVersion int64, reviewType, decision string, actorID int64, note string) (*service.ReferralCampaign, error) {
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var financialSnapshot []byte
		var financialBaseVersion int64
		financialErr := scanAffiliateRow(txCtx, txClient, `
SELECT snapshot,base_rules_version FROM referral_campaign_financial_versions
WHERE campaign_id=$1 AND rules_version=$2 AND status='review' FOR UPDATE`, []any{id, expectedVersion}, &financialSnapshot, &financialBaseVersion)
		if financialErr == nil {
			if _, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_approvals (campaign_id,version,review_type,decision,reviewer_id,note) VALUES ($1,$2,$3,$4,$5,$6)`, id, expectedVersion, reviewType, decision, actorID, strings.TrimSpace(note)); err != nil {
				return err
			}
			if decision == "rejected" {
				if _, err := txClient.ExecContext(txCtx, `UPDATE referral_campaign_financial_versions SET status='rejected',resolved_at=NOW() WHERE campaign_id=$1 AND rules_version=$2 AND status='review'`, id, expectedVersion); err != nil {
					return err
				}
				_, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) SELECT id,version,$3,'financial_rules_rejected',jsonb_build_object('rules_version',$2::bigint,'review_type',$4::text,'note',$5::text) FROM referral_campaigns WHERE id=$1`, id, expectedVersion, actorID, reviewType, strings.TrimSpace(note))
				return err
			}
			var approvals int
			if err := scanAffiliateRow(txCtx, txClient, `SELECT COUNT(DISTINCT review_type) FROM referral_campaign_approvals WHERE campaign_id=$1 AND version=$2 AND decision='approved'`, []any{id, expectedVersion}, &approvals); err != nil {
				return err
			}
			if approvals < 4 {
				return nil
			}
			var candidate referralCampaignSnapshotPayload
			if err := json.Unmarshal(financialSnapshot, &candidate); err != nil {
				return fmt.Errorf("decode financial campaign version: %w", err)
			}
			var currentRulesVersion int64
			if err := scanAffiliateRow(txCtx, txClient, `SELECT rules_version FROM referral_campaigns WHERE id=$1 FOR UPDATE`, []any{id}, &currentRulesVersion); err != nil {
				return err
			}
			if currentRulesVersion != financialBaseVersion {
				return service.ErrReferralCampaignVersionConflict
			}
			rankRewards, err := json.Marshal(candidate.Campaign.RankRewards)
			if err != nil {
				return err
			}
			if _, err = txClient.ExecContext(txCtx, `
UPDATE referral_campaigns SET campaign_key=$2,name=$3,registration_from=$4,registration_to=$5,
 starts_at=$6,ends_at=$7,qualification_to=$8,claim_deadline=$9,risk_hold_hours=$10,
 pay_threshold=$11,usage_threshold=$12,max_enrollments=$13,budget_total=$14,reward_mode=$15,
 rank_rewards_json=$16,public_rules_md=$17,invitee_notice_md=$18,legacy_rebate_policy=$19,
 rules_version=$20,rules_updated_at=NOW(),version=version+1,updated_at=NOW()
WHERE id=$1 AND $14 >= budget_reserved + budget_paid`, id,
				strings.TrimSpace(candidate.Campaign.Key), strings.TrimSpace(candidate.Campaign.Name), candidate.Campaign.RegistrationFrom,
				candidate.Campaign.RegistrationTo, candidate.Campaign.StartsAt, candidate.Campaign.EndsAt, candidate.Campaign.QualificationTo,
				candidate.Campaign.ClaimDeadline, candidate.Campaign.RiskHoldHours, candidate.Campaign.PayThreshold, candidate.Campaign.UsageThreshold,
				candidate.Campaign.MaxEnrollments, candidate.Campaign.BudgetTotal, candidate.Campaign.RewardMode, rankRewards,
				strings.TrimSpace(candidate.Campaign.PublicRulesMD), strings.TrimSpace(candidate.Campaign.InviteeNoticeMD), candidate.Campaign.LegacyRebatePolicy,
				expectedVersion); err != nil {
				return fmt.Errorf("apply financial campaign version: %w", err)
			}
			if _, err := txClient.ExecContext(txCtx, `DELETE FROM referral_campaign_tiers WHERE campaign_id=$1`, id); err != nil {
				return err
			}
			for _, tier := range candidate.Tiers {
				if _, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_tiers (campaign_id,tier_no,required_invites,reward_amount,currency) VALUES ($1,$2,$3,$4,$5)`, id, tier.Tier, tier.RequiredInvites, tier.RewardAmount, strings.ToUpper(strings.TrimSpace(tier.Currency))); err != nil {
					return err
				}
			}
			candidate.Campaign.ID = id
			candidate.Campaign.RulesVersion = expectedVersion
			appliedSnapshot, err := referralCampaignSnapshot(candidate.Campaign, candidate.Tiers)
			if err != nil {
				return err
			}
			if _, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_rule_versions (campaign_id,rules_version,change_kind,snapshot,changed_by) VALUES ($1,$2,'financial',$3,$4)`, id, expectedVersion, appliedSnapshot, actorID); err != nil {
				return err
			}
			if _, err = txClient.ExecContext(txCtx, `UPDATE referral_campaign_financial_versions SET status='approved',resolved_at=NOW() WHERE campaign_id=$1 AND rules_version=$2 AND status='review'`, id, expectedVersion); err != nil {
				return err
			}
			_, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) SELECT id,version,$3,'financial_rules_approved',jsonb_build_object('rules_version',$2::bigint) FROM referral_campaigns WHERE id=$1`, id, expectedVersion, actorID)
			return err
		}
		if !errors.Is(financialErr, sql.ErrNoRows) {
			return financialErr
		}

		var version int64
		if err := scanAffiliateRow(txCtx, txClient, `SELECT version FROM referral_campaigns WHERE id=$1 AND version=$2 AND status='review' FOR UPDATE`, []any{id, expectedVersion}, &version); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralCampaignVersionConflict
			}
			return err
		}
		if _, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_approvals (campaign_id,version,review_type,decision,reviewer_id,note) VALUES ($1,$2,$3,$4,$5,$6)`, id, version, reviewType, decision, actorID, strings.TrimSpace(note)); err != nil {
			return err
		}
		if decision == "rejected" {
			if _, err := txClient.ExecContext(txCtx, `UPDATE referral_campaigns SET status='draft',version=version+1,approved_by=NULL,updated_at=NOW() WHERE id=$1 AND version=$2`, id, version); err != nil {
				return err
			}
			_, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) VALUES ($1,$2+1,$3,'review_rejected',jsonb_build_object('review_type',$4::text,'note',$5::text))`, id, version, actorID, reviewType, strings.TrimSpace(note))
			return err
		}
		var approvals int
		if err := scanAffiliateRow(txCtx, txClient, `SELECT COUNT(DISTINCT review_type) FROM referral_campaign_approvals WHERE campaign_id=$1 AND version=$2 AND decision='approved'`, []any{id, version}, &approvals); err != nil {
			return err
		}
		if approvals >= 4 {
			if _, err := txClient.ExecContext(txCtx, `UPDATE referral_campaigns SET status='approved',approved_by=$3,version=version+1,updated_at=NOW() WHERE id=$1 AND version=$2`, id, version, actorID); err != nil {
				return err
			}
			_, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) VALUES ($1,$2+1,$3,'approved','{}'::jsonb)`, id, version, actorID)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetReferralCampaign(ctx, id)
}

func (r *affiliateRepository) GetReferralCampaignStats(ctx context.Context, campaignID int64) (*service.ReferralCampaignStats, error) {
	client := clientFromContext(ctx, r.client)
	stats := &service.ReferralCampaignStats{CampaignID: campaignID}
	err := scanAffiliateRow(ctx, client, `
SELECT
 (SELECT COUNT(*) FROM referral_campaign_enrollments WHERE campaign_id=$1),
 (SELECT COUNT(*) FROM referral_campaign_attributions WHERE campaign_id=$1),
 (SELECT COUNT(*) FROM referral_campaign_qualifications WHERE campaign_id=$1 AND status='qualified'),
 (SELECT COUNT(*) FROM referral_campaign_qualifications WHERE campaign_id=$1 AND risk_status='pending'),
 (SELECT COUNT(*) FROM referral_campaign_qualifications WHERE campaign_id=$1 AND risk_status='rejected'),
 COALESCE((SELECT budget_reserved FROM referral_campaigns WHERE id=$1),0)::double precision,
 COALESCE((SELECT budget_paid FROM referral_campaigns WHERE id=$1),0)::double precision,
 COALESCE((SELECT SUM(amount) FROM referral_campaign_rewards WHERE campaign_id=$1 AND status='expired'),0)::double precision,
 COALESCE((SELECT SUM(amount) FROM referral_campaign_rewards WHERE campaign_id=$1 AND status IN ('revoked','debt_review')),0)::double precision`, []any{campaignID}, &stats.Enrolled, &stats.Attributed, &stats.Qualified, &stats.RiskPending, &stats.RiskRejected, &stats.RewardsReserved, &stats.RewardsClaimed, &stats.RewardsExpired, &stats.RewardsRevoked)
	if err != nil {
		return nil, fmt.Errorf("get referral campaign stats: %w", err)
	}
	return stats, nil
}

func (r *affiliateRepository) GetReferralCampaignTiers(ctx context.Context, campaignID int64) ([]service.ReferralCampaignTier, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `SELECT tier_no, required_invites, reward_amount::double precision, currency FROM referral_campaign_tiers WHERE campaign_id = $1 ORDER BY required_invites, tier_no`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list referral campaign tiers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.ReferralCampaignTier, 0)
	for rows.Next() {
		var t service.ReferralCampaignTier
		if err := rows.Scan(&t.Tier, &t.RequiredInvites, &t.RewardAmount, &t.Currency); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *affiliateRepository) ListReferralCampaignApprovals(ctx context.Context, campaignID int64) ([]service.ReferralCampaignApproval, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `SELECT version,review_type,decision,reviewer_id,note,created_at FROM referral_campaign_approvals WHERE campaign_id=$1 ORDER BY version DESC,created_at ASC,id ASC`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list referral campaign approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.ReferralCampaignApproval, 0)
	for rows.Next() {
		var item service.ReferralCampaignApproval
		var reviewer sql.NullInt64
		if err := rows.Scan(&item.Version, &item.ReviewType, &item.Decision, &reviewer, &item.Note, &item.CreatedAt); err != nil {
			return nil, err
		}
		if reviewer.Valid {
			v := reviewer.Int64
			item.ReviewerID = &v
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *affiliateRepository) ListReferralCampaignParticipants(ctx context.Context, campaignID int64, page, pageSize int, search string) (*service.ReferralCampaignParticipantPage, error) {
	page, pageSize, offset := normalizeReferralPage(page, pageSize)
	client := clientFromContext(ctx, r.client)
	like := "%" + strings.TrimSpace(search) + "%"
	total, err := scanInt64(ctx, client, `SELECT COUNT(*) FROM referral_campaign_enrollments e JOIN users u ON u.id=e.user_id WHERE e.campaign_id=$1 AND ($2='%%' OR u.email ILIKE $2 OR u.username ILIKE $2)`, campaignID, like)
	if err != nil {
		return nil, err
	}
	rows, err := client.QueryContext(ctx, `
SELECT e.user_id,COALESCE(u.email,''),COALESCE(u.username,''),e.enrolled_at,
 (SELECT COUNT(*) FROM referral_campaign_attributions a WHERE a.campaign_id=e.campaign_id AND a.inviter_id=e.user_id),
 (SELECT COUNT(*) FROM referral_campaign_qualifications q WHERE q.campaign_id=e.campaign_id AND q.inviter_id=e.user_id AND q.status='qualified'),
 COALESCE((SELECT SUM(r.amount) FROM referral_campaign_rewards r WHERE r.campaign_id=e.campaign_id AND r.user_id=e.user_id AND r.status IN ('claimable','claimed_frozen','available','resolved')),0)::double precision,
 COALESCE((SELECT SUM(r.amount) FROM referral_campaign_rewards r WHERE r.campaign_id=e.campaign_id AND r.user_id=e.user_id AND r.status IN ('claimed_frozen','available','resolved')),0)::double precision
FROM referral_campaign_enrollments e JOIN users u ON u.id=e.user_id
WHERE e.campaign_id=$1 AND ($2='%%' OR u.email ILIKE $2 OR u.username ILIKE $2)
ORDER BY e.enrolled_at DESC,e.id DESC LIMIT $3 OFFSET $4`, campaignID, like, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ReferralCampaignParticipant, 0)
	for rows.Next() {
		var item service.ReferralCampaignParticipant
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.EnrolledAt, &item.InvitedCount, &item.QualifiedCount, &item.RewardUnlocked, &item.RewardClaimed); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &service.ReferralCampaignParticipantPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *affiliateRepository) ListReferralCampaignInvites(ctx context.Context, campaignID int64, page, pageSize int, search, status string) (*service.ReferralCampaignInvitePage, error) {
	page, pageSize, offset := normalizeReferralPage(page, pageSize)
	client := clientFromContext(ctx, r.client)
	like := "%" + strings.TrimSpace(search) + "%"
	status = strings.TrimSpace(status)
	from := ` FROM referral_campaign_attributions a JOIN users inviter ON inviter.id=a.inviter_id JOIN users invitee ON invitee.id=a.invitee_id LEFT JOIN referral_campaign_qualifications q ON q.campaign_id=a.campaign_id AND q.invitee_id=a.invitee_id WHERE a.campaign_id=$1 AND ($2='%%' OR inviter.email ILIKE $2 OR inviter.username ILIKE $2 OR invitee.email ILIKE $2 OR invitee.username ILIKE $2) AND ($3='' OR COALESCE(q.status,'pending')=$3)`
	total, err := scanInt64(ctx, client, `SELECT COUNT(*)`+from, campaignID, like, status)
	if err != nil {
		return nil, err
	}
	rows, err := client.QueryContext(ctx, `SELECT a.id,a.inviter_id,COALESCE(inviter.email,''),a.invitee_id,COALESCE(invitee.email,''),a.registered_at,a.status,COALESCE(q.net_paid,0)::double precision,COALESCE(q.actual_cost,0)::double precision,COALESCE(q.risk_status,'pending'),COALESCE(q.status,'pending'),q.qualified_at`+from+` ORDER BY a.registered_at DESC,a.id DESC LIMIT $4 OFFSET $5`, campaignID, like, status, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ReferralCampaignInviteDetail, 0)
	for rows.Next() {
		var item service.ReferralCampaignInviteDetail
		var qualified sql.NullTime
		if err := rows.Scan(&item.AttributionID, &item.InviterID, &item.InviterEmail, &item.InviteeID, &item.InviteeEmail, &item.RegisteredAt, &item.Status, &item.NetPaid, &item.ActualCost, &item.RiskStatus, &item.Qualification, &qualified); err != nil {
			return nil, err
		}
		if qualified.Valid {
			v := qualified.Time
			item.QualifiedAt = &v
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &service.ReferralCampaignInvitePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *affiliateRepository) ListReferralCampaignRewards(ctx context.Context, campaignID int64, page, pageSize int, search, status string) (*service.ReferralCampaignRewardPage, error) {
	page, pageSize, offset := normalizeReferralPage(page, pageSize)
	client := clientFromContext(ctx, r.client)
	like := "%" + strings.TrimSpace(search) + "%"
	status = strings.TrimSpace(status)
	from := ` FROM referral_campaign_rewards r JOIN users u ON u.id=r.user_id WHERE r.campaign_id=$1 AND ($2='%%' OR u.email ILIKE $2 OR u.username ILIKE $2) AND ($3='' OR r.status=$3)`
	total, err := scanInt64(ctx, client, `SELECT COUNT(*)`+from, campaignID, like, status)
	if err != nil {
		return nil, err
	}
	rows, err := client.QueryContext(ctx, `SELECT r.id,r.campaign_id,r.user_id,r.tier_no,r.reward_type,r.amount::double precision,r.currency,r.status,r.unlock_at,r.claim_deadline,r.frozen_until,r.version,COALESCE(u.email,''),COALESCE(u.username,'')`+from+` ORDER BY r.created_at DESC,r.id DESC LIMIT $4 OFFSET $5`, campaignID, like, status, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ReferralCampaignRewardDetail, 0)
	for rows.Next() {
		var item service.ReferralCampaignRewardDetail
		var unlock, deadline, frozen sql.NullTime
		if err := rows.Scan(&item.ID, &item.CampaignID, &item.UserID, &item.Tier, &item.RewardType, &item.Amount, &item.Currency, &item.Status, &unlock, &deadline, &frozen, &item.Version, &item.Email, &item.Username); err != nil {
			return nil, err
		}
		if unlock.Valid {
			v := unlock.Time
			item.UnlockAt = &v
		}
		if deadline.Valid {
			v := deadline.Time
			item.ClaimDeadline = &v
		}
		if frozen.Valid {
			v := frozen.Time
			item.FrozenUntil = &v
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &service.ReferralCampaignRewardPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *affiliateRepository) GetReferralCampaignSecret(ctx context.Context, id int64) ([]byte, error) {
	client := clientFromContext(ctx, r.client)
	var secret []byte
	if err := scanAffiliateRow(ctx, client, `SELECT signing_secret FROM referral_campaigns WHERE id = $1`, []any{id}, &secret); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrReferralCampaignNotFound
		}
		return nil, fmt.Errorf("get referral campaign signing secret: %w", err)
	}
	if len(secret) < 16 {
		return nil, service.ErrReferralCampaignTokenInvalid
	}
	return secret, nil
}

func (r *affiliateRepository) EnrollReferralCampaign(ctx context.Context, campaignID, userID int64) (*service.ReferralCampaignEnrollment, error) {
	var e service.ReferralCampaignEnrollment
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if err := scanAffiliateRow(txCtx, txClient, `SELECT campaign_id,user_id,enrolled_at FROM referral_campaign_enrollments WHERE campaign_id=$1 AND user_id=$2`, []any{campaignID, userID}, &e.CampaignID, &e.UserID, &e.EnrolledAt); err == nil {
			return nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var maxEnrollments, enrolled int
		if err := scanAffiliateRow(txCtx, txClient, `SELECT max_enrollments,(SELECT COUNT(*) FROM referral_campaign_enrollments WHERE campaign_id=$1) FROM referral_campaigns WHERE id=$1 FOR UPDATE`, []any{campaignID}, &maxEnrollments, &enrolled); err != nil {
			return err
		}
		if enrolled >= maxEnrollments {
			return service.ErrReferralCampaignCapacityReached
		}
		return scanAffiliateRow(txCtx, txClient, `INSERT INTO referral_campaign_enrollments (campaign_id,user_id) VALUES ($1,$2) ON CONFLICT (campaign_id,user_id) DO UPDATE SET user_id=EXCLUDED.user_id RETURNING campaign_id,user_id,enrolled_at`, []any{campaignID, userID}, &e.CampaignID, &e.UserID, &e.EnrolledAt)
	})
	if err != nil {
		return nil, fmt.Errorf("enroll referral campaign: %w", err)
	}
	return &e, nil
}

func (r *affiliateRepository) GetReferralCampaignEnrollment(ctx context.Context, campaignID, userID int64) (*service.ReferralCampaignEnrollment, error) {
	client := clientFromContext(ctx, r.client)
	var e service.ReferralCampaignEnrollment
	if err := scanAffiliateRow(ctx, client, `SELECT campaign_id, user_id, enrolled_at FROM referral_campaign_enrollments WHERE campaign_id = $1 AND user_id = $2`, []any{campaignID, userID}, &e.CampaignID, &e.UserID, &e.EnrolledAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrReferralCampaignNotOpen
		}
		return nil, err
	}
	return &e, nil
}

func (r *affiliateRepository) BindReferralAttribution(ctx context.Context, attribution service.ReferralAttribution) (*service.ReferralAttribution, error) {
	var out service.ReferralAttribution
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var existing service.ReferralAttribution
		if err := scanAffiliateRow(txCtx, txClient, `SELECT id,campaign_id,inviter_id,invitee_id,token_nonce,registered_at,status,rules_version,legacy_rebate_policy,qualification_to_snapshot,pay_threshold_snapshot::double precision,usage_threshold_snapshot::double precision,risk_hold_hours_snapshot FROM referral_campaign_attributions WHERE campaign_id=$1 AND invitee_id=$2`, []any{attribution.CampaignID, attribution.InviteeID}, &existing.ID, &existing.CampaignID, &existing.InviterID, &existing.InviteeID, &existing.Nonce, &existing.RegisteredAt, &existing.Status, &existing.RulesVersion, &existing.LegacyRebatePolicy, &existing.QualificationToSnapshot, &existing.PayThresholdSnapshot, &existing.UsageThresholdSnapshot, &existing.RiskHoldHoursSnapshot); err == nil {
			if existing.InviterID != attribution.InviterID {
				return service.ErrAffiliateAlreadyBound
			}
			out = existing
			return nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var userCreatedAt time.Time
		if err := scanAffiliateRow(txCtx, txClient, `
SELECT u.created_at FROM users u JOIN referral_campaigns c ON c.id=$1
JOIN referral_campaign_enrollments e ON e.campaign_id=c.id AND e.user_id=$2
WHERE u.id=$3 AND u.created_at>=c.registration_from AND u.created_at<c.registration_to
  AND u.created_at >= $4::timestamptz - INTERVAL '24 hours'
	  AND c.status IN ('scheduled','running')`, []any{attribution.CampaignID, attribution.InviterID, attribution.InviteeID, attribution.RegisteredAt}, &userCreatedAt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralCampaignNotOpen
			}
			return err
		}
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, attribution.InviteeID); err != nil {
			return err
		}
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, attribution.InviterID); err != nil {
			return err
		}
		res, err := txClient.ExecContext(txCtx, `UPDATE user_affiliates SET inviter_id = $1, updated_at = NOW() WHERE user_id = $2 AND inviter_id IS NULL`, attribution.InviterID, attribution.InviteeID)
		if err != nil {
			return fmt.Errorf("bind campaign inviter: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			if _, err = txClient.ExecContext(txCtx, `UPDATE user_affiliates SET aff_count=aff_count+1,updated_at=NOW() WHERE user_id=$1`, attribution.InviterID); err != nil {
				return err
			}
		} else {
			var boundInviter sql.NullInt64
			if err := scanAffiliateRow(txCtx, txClient, `SELECT inviter_id FROM user_affiliates WHERE user_id=$1`, []any{attribution.InviteeID}, &boundInviter); err != nil {
				return err
			}
			if !boundInviter.Valid || boundInviter.Int64 != attribution.InviterID {
				return service.ErrAffiliateAlreadyBound
			}
		}
		err = scanAffiliateRow(txCtx, txClient, `
INSERT INTO referral_campaign_attributions (campaign_id, inviter_id, invitee_id, token_nonce, registered_at, status, rules_version, legacy_rebate_policy, qualification_to_snapshot, pay_threshold_snapshot, usage_threshold_snapshot, risk_hold_hours_snapshot)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (campaign_id, invitee_id) DO UPDATE SET inviter_id = referral_campaign_attributions.inviter_id
RETURNING id, campaign_id, inviter_id, invitee_id, token_nonce, registered_at, status, rules_version, legacy_rebate_policy, qualification_to_snapshot, pay_threshold_snapshot::double precision, usage_threshold_snapshot::double precision, risk_hold_hours_snapshot`, []any{attribution.CampaignID, attribution.InviterID, attribution.InviteeID, attribution.Nonce, attribution.RegisteredAt, attribution.Status, attribution.RulesVersion, attribution.LegacyRebatePolicy, attribution.QualificationToSnapshot, attribution.PayThresholdSnapshot, attribution.UsageThresholdSnapshot, attribution.RiskHoldHoursSnapshot}, &out.ID, &out.CampaignID, &out.InviterID, &out.InviteeID, &out.Nonce, &out.RegisteredAt, &out.Status, &out.RulesVersion, &out.LegacyRebatePolicy, &out.QualificationToSnapshot, &out.PayThresholdSnapshot, &out.UsageThresholdSnapshot, &out.RiskHoldHoursSnapshot)
		if err != nil {
			return fmt.Errorf("record campaign attribution: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *affiliateRepository) GetReferralAttribution(ctx context.Context, campaignID, inviteeID int64) (*service.ReferralAttribution, error) {
	client := clientFromContext(ctx, r.client)
	var a service.ReferralAttribution
	if err := scanAffiliateRow(ctx, client, `SELECT id, campaign_id, inviter_id, invitee_id, token_nonce, registered_at, status, rules_version, legacy_rebate_policy, qualification_to_snapshot, pay_threshold_snapshot::double precision, usage_threshold_snapshot::double precision, risk_hold_hours_snapshot FROM referral_campaign_attributions WHERE campaign_id = $1 AND invitee_id = $2`, []any{campaignID, inviteeID}, &a.ID, &a.CampaignID, &a.InviterID, &a.InviteeID, &a.Nonce, &a.RegisteredAt, &a.Status, &a.RulesVersion, &a.LegacyRebatePolicy, &a.QualificationToSnapshot, &a.PayThresholdSnapshot, &a.UsageThresholdSnapshot, &a.RiskHoldHoursSnapshot); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrReferralCampaignNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *affiliateRepository) ListReferralAttributionCampaignIDs(ctx context.Context, inviteeID int64) ([]int64, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `SELECT a.campaign_id FROM referral_campaign_attributions a JOIN referral_campaigns c ON c.id=a.campaign_id WHERE a.invitee_id=$1 AND a.status IN ('pending','approved') AND c.status IN ('scheduled','running','settling') ORDER BY a.campaign_id`, inviteeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *affiliateRepository) ListReferralCampaignInviteeIDs(ctx context.Context, campaignID, inviterID int64, limit int) ([]int64, error) {
	if limit < 1 || limit > 1000 {
		limit = 200
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT a.invitee_id FROM referral_campaign_attributions a
LEFT JOIN referral_campaign_qualifications q ON q.campaign_id=a.campaign_id AND q.invitee_id=a.invitee_id
WHERE a.campaign_id=$1 AND a.inviter_id=$2 AND a.status IN ('pending','approved')
ORDER BY CASE WHEN
COALESCE((SELECT SUM(m.net_amount) FROM play_membership_verified_contributions m WHERE m.user_id=a.invitee_id AND m.paid_at>=a.registered_at AND m.paid_at<=a.qualification_to_snapshot),0)>=a.pay_threshold_snapshot
 AND COALESCE((SELECT SUM(u.actual_cost) FROM usage_logs u WHERE u.user_id=a.invitee_id AND u.created_at>=a.registered_at AND u.created_at<=a.qualification_to_snapshot),0)>=a.usage_threshold_snapshot
 THEN 0 ELSE 1 END,
 CASE WHEN COALESCE(q.status,'pending')='pending' THEN 0 ELSE 1 END,a.id
LIMIT $3`, campaignID, inviterID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *affiliateRepository) RecomputeReferralQualification(ctx context.Context, campaignID, inviteeID int64, now time.Time) (*service.ReferralQualification, error) {
	client := clientFromContext(ctx, r.client)
	var q service.ReferralQualification
	var registeredAt time.Time
	var sourceOrder sql.NullInt64
	err := scanAffiliateRow(ctx, client, `
SELECT a.campaign_id, a.inviter_id, a.invitee_id, a.registered_at,
       COALESCE((SELECT SUM(m.net_amount) FROM play_membership_verified_contributions m
                 WHERE m.user_id=a.invitee_id AND m.qualification_state='verified' AND m.paid_at >= a.registered_at AND m.paid_at <= a.qualification_to_snapshot),0)::double precision,
       COALESCE((SELECT SUM(u.actual_cost) FROM usage_logs u
                 WHERE u.user_id=a.invitee_id AND u.created_at >= a.registered_at AND u.created_at <= a.qualification_to_snapshot),0)::double precision,
       (SELECT m.order_id FROM play_membership_verified_contributions m
        WHERE m.user_id=a.invitee_id AND m.qualification_state='verified' AND m.net_amount > 0 AND m.paid_at >= a.registered_at AND m.paid_at <= a.qualification_to_snapshot
        ORDER BY m.paid_at DESC, m.order_id DESC LIMIT 1),
       CASE WHEN EXISTS (
          SELECT 1 FROM ip_risk_case_users cu JOIN ip_risk_cases rc ON rc.id=cu.case_id
          WHERE cu.user_id=a.invitee_id AND rc.status IN ('open','observing','processing') AND rc.level IN ('high','severe','critical')
       ) THEN 'rejected'
       WHEN NOW() < a.registered_at + make_interval(hours => a.risk_hold_hours_snapshot) THEN 'pending'
       ELSE 'approved' END,
       COALESCE(q.status, 'pending')
FROM referral_campaign_attributions a
JOIN referral_campaigns c ON c.id=a.campaign_id
LEFT JOIN referral_campaign_qualifications q ON q.campaign_id=a.campaign_id AND q.invitee_id=a.invitee_id
WHERE a.campaign_id=$1 AND a.invitee_id=$2`, []any{campaignID, inviteeID},
		&q.CampaignID, &q.InviterID, &q.InviteeID, &registeredAt, &q.NetPaid, &q.ActualCost,
		&sourceOrder, &q.RiskStatus, &q.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrReferralCampaignNotFound
		}
		return nil, fmt.Errorf("recompute referral qualification: %w", err)
	}
	if sourceOrder.Valid {
		v := sourceOrder.Int64
		q.SourceOrderID = &v
	}
	_ = registeredAt
	_ = now
	return &q, nil
}

func (r *affiliateRepository) UpdateReferralQualification(ctx context.Context, q service.ReferralQualification) (*service.ReferralQualification, error) {
	var out service.ReferralQualification
	var qualifiedAt, revokedAt sql.NullTime
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var rewardMode, campaignStatus string
		if err := scanAffiliateRow(txCtx, txClient, `SELECT reward_mode,status FROM referral_campaigns WHERE id=$1 FOR UPDATE`, []any{q.CampaignID}, &rewardMode, &campaignStatus); err != nil {
			return err
		}
		if err := scanAffiliateRow(txCtx, sqlExecutorFromEntClient(txClient), `
INSERT INTO referral_campaign_qualifications (campaign_id, inviter_id, invitee_id, net_paid, actual_cost, source_order_id, risk_status, status, qualified_at, revoked_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (campaign_id, invitee_id) DO UPDATE SET
 inviter_id = EXCLUDED.inviter_id, net_paid = EXCLUDED.net_paid, actual_cost = EXCLUDED.actual_cost,
 source_order_id = COALESCE(EXCLUDED.source_order_id, referral_campaign_qualifications.source_order_id),
 risk_status = EXCLUDED.risk_status, status = EXCLUDED.status,
 qualified_at = COALESCE(referral_campaign_qualifications.qualified_at, EXCLUDED.qualified_at),
 revoked_at = CASE WHEN EXCLUDED.status='revoked' THEN COALESCE(referral_campaign_qualifications.revoked_at,EXCLUDED.revoked_at,NOW()) ELSE NULL END,
 updated_at = NOW()
		RETURNING id, campaign_id, inviter_id, invitee_id, net_paid::double precision, actual_cost::double precision, source_order_id, risk_status, status, qualified_at, revoked_at`, []any{q.CampaignID, q.InviterID, q.InviteeID, q.NetPaid, q.ActualCost, q.SourceOrderID, q.RiskStatus, q.Status, q.QualifiedAt, q.RevokedAt}, &out.ID, &out.CampaignID, &out.InviterID, &out.InviteeID, &out.NetPaid, &out.ActualCost, &out.SourceOrderID, &out.RiskStatus, &out.Status, &qualifiedAt, &revokedAt); err != nil {
			return err
		}
		if qualifiedAt.Valid {
			t := qualifiedAt.Time
			out.QualifiedAt = &t
		}
		attributionStatus := "pending"
		switch out.RiskStatus {
		case service.ReferralRiskApproved:
			attributionStatus = "approved"
		case service.ReferralRiskRejected:
			attributionStatus = "rejected"
		}
		if _, err := txClient.ExecContext(txCtx, `UPDATE referral_campaign_attributions SET status=$1 WHERE campaign_id=$2 AND invitee_id=$3 AND status<>'revoked'`, attributionStatus, out.CampaignID, out.InviteeID); err != nil {
			return err
		}
		if revokedAt.Valid {
			t := revokedAt.Time
			out.RevokedAt = &t
		}
		if out.Status != service.ReferralQualificationQualified {
			var qualifiedCount int64
			if err := scanAffiliateRow(txCtx, txClient, `SELECT COUNT(*) FROM referral_campaign_qualifications WHERE campaign_id=$1 AND inviter_id=$2 AND status='qualified'`, []any{out.CampaignID, out.InviterID}, &qualifiedCount); err != nil {
				return err
			}
			return revokeReferralRewardsAboveCountTx(txCtx, txClient, out.CampaignID, out.InviterID, qualifiedCount)
		}
		// A claim window accepts claims on rewards already earned, but it never
		// creates more eligibility. Existing reward reversals above remain valid
		// so refunds and risk actions retain their normal financial controls.
		if campaignStatus != service.ReferralCampaignStatusScheduled && campaignStatus != service.ReferralCampaignStatusRunning {
			return nil
		}
		// Serialize reward generation for one inviter. Different invitees can
		// qualify concurrently, but they must observe one ordered tier state.
		var lockedInviterID int64
		if err := scanAffiliateRow(txCtx, txClient, `SELECT user_id FROM referral_campaign_enrollments WHERE campaign_id=$1 AND user_id=$2 FOR UPDATE`, []any{out.CampaignID, out.InviterID}, &lockedInviterID); err != nil {
			return err
		}
		// One transaction owns both the qualification transition and reward
		// reservation, so concurrent payment callbacks cannot overspend.
		tiers, err := queryReferralTiers(txCtx, txClient, out.CampaignID)
		if err != nil {
			return err
		}
		previousTierReward := 0.0
		for _, tier := range tiers {
			rewardAmount := tier.RewardAmount
			if rewardMode == "replace" {
				rewardAmount = tier.RewardAmount - previousTierReward
				previousTierReward = tier.RewardAmount
			}
			var qualifiedCount int64
			if err := scanAffiliateRow(txCtx, txClient, `SELECT COUNT(*) FROM referral_campaign_qualifications WHERE campaign_id=$1 AND inviter_id=$2 AND status='qualified'`, []any{out.CampaignID, out.InviterID}, &qualifiedCount); err != nil {
				return err
			}
			if qualifiedCount < int64(tier.RequiredInvites) {
				continue
			}
			var latestStatus string
			var latestGeneration int
			latestErr := scanAffiliateRow(txCtx, txClient, `SELECT status,generation FROM referral_campaign_rewards WHERE campaign_id=$1 AND user_id=$2 AND reward_type='tier' AND tier_no=$3 ORDER BY generation DESC LIMIT 1 FOR UPDATE`, []any{out.CampaignID, out.InviterID, tier.Tier}, &latestStatus, &latestGeneration)
			generation, shouldCreate, generationErr := nextReferralRewardGeneration(latestStatus, latestGeneration, latestErr)
			if generationErr != nil {
				return generationErr
			}
			if !shouldCreate {
				continue
			}
			var rewardID int64
			idempotency := fmt.Sprintf("referral:%d:%d:tier:%d:generation:%d", out.CampaignID, out.InviterID, tier.Tier, generation)
			err = scanAffiliateRow(txCtx, sqlExecutorFromEntClient(txClient), `
INSERT INTO referral_campaign_rewards (campaign_id,user_id,tier_no,reward_type,amount,currency,status,qualification_id,idempotency_key,claim_deadline,generation)
SELECT $1,$2,$3,'tier',$4,$5,'claimable',$6,$7,claim_deadline,$9
FROM referral_campaigns
WHERE id=$1 AND budget_reserved + budget_paid + $4 <= budget_total
  AND (SELECT COUNT(*) FROM referral_campaign_qualifications WHERE campaign_id=$1 AND inviter_id=$2 AND status='qualified') >= $8
				ON CONFLICT (campaign_id,user_id,reward_type,tier_no,generation) DO NOTHING RETURNING id`, []any{out.CampaignID, out.InviterID, tier.Tier, rewardAmount, tier.Currency, out.ID, idempotency, tier.RequiredInvites, generation}, &rewardID)
			if err != nil {
				return translateReferralRewardReservationError(err)
			}
			if _, err = txClient.ExecContext(txCtx, `UPDATE referral_campaigns SET budget_reserved=budget_reserved+$1, updated_at=NOW() WHERE id=$2`, rewardAmount, out.CampaignID); err != nil {
				return err
			}
			_, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key) VALUES ($1,$2,'reserve',$3,$4) ON CONFLICT DO NOTHING`, out.CampaignID, rewardID, rewardAmount, idempotency+":reserve")
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func nextReferralRewardGeneration(latestStatus string, latestGeneration int, latestErr error) (int, bool, error) {
	switch {
	case latestErr == nil && latestStatus == service.ReferralRewardStatusRevoked:
		return latestGeneration + 1, true, nil
	case latestErr == nil:
		return latestGeneration, false, nil
	case errors.Is(latestErr, sql.ErrNoRows):
		return 1, true, nil
	default:
		return 0, false, latestErr
	}
}

func translateReferralRewardReservationError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrReferralCampaignBudgetExceeded
	}
	return err
}

func queryReferralTiers(ctx context.Context, client affiliateQueryExecer, campaignID int64) ([]service.ReferralCampaignTier, error) {
	rows, err := client.QueryContext(ctx, `SELECT tier_no, required_invites, reward_amount::double precision, currency FROM referral_campaign_tiers WHERE campaign_id=$1 ORDER BY required_invites,tier_no`, campaignID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.ReferralCampaignTier
	for rows.Next() {
		var t service.ReferralCampaignTier
		if err := rows.Scan(&t.Tier, &t.RequiredInvites, &t.RewardAmount, &t.Currency); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func revokeReferralRewardsAboveCountTx(ctx context.Context, client *dbent.Client, campaignID, inviterID, qualifiedCount int64) error {
	rows, err := client.QueryContext(ctx, `
SELECT r.id,r.amount::double precision,r.status,r.user_id,
 CASE WHEN l.frozen_until IS NOT NULL THEN 'frozen' ELSE 'available' END
FROM referral_campaign_rewards r JOIN referral_campaign_tiers t ON t.campaign_id=r.campaign_id AND t.tier_no=r.tier_no
LEFT JOIN user_affiliate_ledger l ON l.referral_reward_id=r.id AND l.action='accrue'
WHERE r.campaign_id=$1 AND r.user_id=$2 AND t.required_invites>$3 AND r.status IN ('claimable','claimed_frozen','available')
ORDER BY r.id FOR UPDATE OF r`, campaignID, inviterID, qualifiedCount)
	if err != nil {
		return err
	}
	type affectedReward struct {
		id     int64
		amount float64
		status string
		userID int64
		bucket string
	}
	rewards := make([]affectedReward, 0)
	for rows.Next() {
		var reward affectedReward
		if err := rows.Scan(&reward.id, &reward.amount, &reward.status, &reward.userID, &reward.bucket); err != nil {
			_ = rows.Close()
			return err
		}
		rewards = append(rewards, reward)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, reward := range rewards {
		if reward.status == service.ReferralRewardStatusClaimable {
			res, err := client.ExecContext(ctx, `UPDATE referral_campaign_rewards SET status='revoked',revoked_at=NOW(),updated_at=NOW(),version=version+1 WHERE id=$1 AND status='claimable'`, reward.id)
			if err != nil {
				return err
			}
			changed, _ := res.RowsAffected()
			if changed == 0 {
				continue
			}
			if _, err := client.ExecContext(ctx, `UPDATE referral_campaigns SET budget_reserved=GREATEST(budget_reserved-$1,0),updated_at=NOW() WHERE id=$2`, reward.amount, campaignID); err != nil {
				return err
			}
			if _, err := client.ExecContext(ctx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key) VALUES ($1,$2,'release',$3,$4) ON CONFLICT DO NOTHING`, campaignID, reward.id, reward.amount, fmt.Sprintf("referral:reward:%d:qualification-release", reward.id)); err != nil {
				return err
			}
			continue
		}
		column := "aff_quota"
		if reward.bucket == "frozen" {
			column = "aff_frozen_quota"
		}
		res, err := client.ExecContext(ctx, `UPDATE user_affiliates SET `+column+`=`+column+`-$1,updated_at=NOW() WHERE user_id=$2 AND `+column+` >= $1`, reward.amount, reward.userID)
		if err != nil {
			return err
		}
		recovered, _ := res.RowsAffected()
		if recovered == 0 {
			if _, err := client.ExecContext(ctx, `UPDATE referral_campaign_rewards SET status='debt_review',revoked_at=NOW(),updated_at=NOW(),version=version+1 WHERE id=$1 AND status IN ('claimed_frozen','available')`, reward.id); err != nil {
				return err
			}
			continue
		}
		if _, err := client.ExecContext(ctx, `UPDATE referral_campaign_rewards SET status='revoked',revoked_at=NOW(),updated_at=NOW(),version=version+1 WHERE id=$1 AND status IN ('claimed_frozen','available')`, reward.id); err != nil {
			return err
		}
		if _, err := client.ExecContext(ctx, `UPDATE user_affiliate_ledger SET frozen_until=NULL,updated_at=NOW() WHERE referral_reward_id=$1 AND action='accrue'`, reward.id); err != nil {
			return err
		}
		if _, err := client.ExecContext(ctx, `INSERT INTO user_affiliate_ledger (user_id,action,amount,referral_reward_id,created_at,updated_at) VALUES ($1,'revoke',$2,$3,NOW(),NOW())`, reward.userID, -reward.amount, reward.id); err != nil {
			return err
		}
		if _, err := client.ExecContext(ctx, `UPDATE referral_campaigns SET budget_paid=GREATEST(budget_paid-$1,0),updated_at=NOW() WHERE id=$2`, reward.amount, campaignID); err != nil {
			return err
		}
		if _, err := client.ExecContext(ctx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key) VALUES ($1,$2,'reverse',$3,$4) ON CONFLICT DO NOTHING`, campaignID, reward.id, reward.amount, fmt.Sprintf("referral:reward:%d:qualification-reverse", reward.id)); err != nil {
			return err
		}
	}
	return nil
}

func (r *affiliateRepository) ClaimReferralReward(ctx context.Context, input service.ReferralClaimInput) (*service.ReferralReward, error) {
	var out service.ReferralReward
	var claimDeadline, frozenUntil, unlockAt sql.NullTime
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		now := time.Now().UTC()
		// All campaign financial transitions lock the campaign before its rewards.
		// This keeps claim, automatic close, and early close from taking inverse
		// locks while a claim window is being concluded.
		var lockedCampaignID int64
		if err := scanAffiliateRow(txCtx, txClient, `SELECT id FROM referral_campaigns WHERE id=$1 FOR UPDATE`, []any{input.CampaignID}, &lockedCampaignID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralRewardNotClaimable
			}
			return err
		}
		err := scanAffiliateRow(txCtx, sqlExecutorFromEntClient(txClient), `
UPDATE referral_campaign_rewards r SET status='claimed_frozen', claimed_at=NOW(), frozen_until=NOW()+make_interval(hours => c.risk_hold_hours), updated_at=NOW(), version=version+1
FROM referral_campaigns c
WHERE r.id=$1 AND r.campaign_id=$2 AND r.user_id=$3 AND c.id=r.campaign_id AND c.version=$4 AND c.status IN ('running','settling','closed')
  AND r.status='claimable' AND r.claim_deadline > $5
	  AND NOT EXISTS (SELECT 1 FROM referral_campaign_rewards debt WHERE debt.user_id=r.user_id AND debt.status='debt_review')
	  AND NOT EXISTS (
	    SELECT 1 FROM ip_risk_case_users cu JOIN ip_risk_cases rc ON rc.id=cu.case_id
	    WHERE cu.user_id=r.user_id AND rc.status IN ('open','observing','processing') AND rc.level IN ('high','severe','critical')
	  )
		RETURNING r.id,r.campaign_id,r.user_id,r.tier_no,r.reward_type,r.amount::double precision,r.currency,r.status,r.unlock_at,r.claim_deadline,r.frozen_until,r.version`, []any{input.RewardID, input.CampaignID, input.UserID, input.CampaignVersion, now}, &out.ID, &out.CampaignID, &out.UserID, &out.Tier, &out.RewardType, &out.Amount, &out.Currency, &out.Status, &unlockAt, &claimDeadline, &frozenUntil, &out.Version)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			err = scanAffiliateRow(txCtx, sqlExecutorFromEntClient(txClient), `SELECT id,campaign_id,user_id,tier_no,reward_type,amount::double precision,currency,status,unlock_at,claim_deadline,frozen_until,version FROM referral_campaign_rewards WHERE id=$1 AND campaign_id=$2 AND user_id=$3`, []any{input.RewardID, input.CampaignID, input.UserID}, &out.ID, &out.CampaignID, &out.UserID, &out.Tier, &out.RewardType, &out.Amount, &out.Currency, &out.Status, &unlockAt, &claimDeadline, &frozenUntil, &out.Version)
			if err != nil {
				return service.ErrReferralRewardNotClaimable
			}
			if out.Status == service.ReferralRewardStatusClaimable && claimDeadline.Valid && !now.Before(claimDeadline.Time) {
				if _, err := txClient.ExecContext(txCtx, `UPDATE referral_campaign_rewards SET status='expired',updated_at=NOW(),version=version+1 WHERE id=$1 AND status='claimable'`, out.ID); err != nil {
					return err
				}
				if _, err := txClient.ExecContext(txCtx, `UPDATE referral_campaigns SET budget_reserved=GREATEST(budget_reserved-$1,0),updated_at=NOW() WHERE id=$2`, out.Amount, out.CampaignID); err != nil {
					return err
				}
				if _, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key) VALUES ($1,$2,'release',$3,$4) ON CONFLICT DO NOTHING`, out.CampaignID, out.ID, out.Amount, fmt.Sprintf("referral:reward:%d:expired-release", out.ID)); err != nil {
					return err
				}
				out.Status = service.ReferralRewardStatusExpired
				return nil
			}
			if out.Status != service.ReferralRewardStatusClaimedFrozen && out.Status != service.ReferralRewardStatusAvailable {
				return service.ErrReferralRewardNotClaimable
			}
			if unlockAt.Valid {
				v := unlockAt.Time
				out.UnlockAt = &v
			}
			if claimDeadline.Valid {
				v := claimDeadline.Time
				out.ClaimDeadline = &v
			}
			if frozenUntil.Valid {
				v := frozenUntil.Time
				out.FrozenUntil = &v
			}
			return nil
		}
		if unlockAt.Valid {
			t := unlockAt.Time
			out.UnlockAt = &t
		}
		if claimDeadline.Valid {
			t := claimDeadline.Time
			out.ClaimDeadline = &t
		}
		if frozenUntil.Valid {
			t := frozenUntil.Time
			out.FrozenUntil = &t
		}
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, out.UserID); err != nil {
			return err
		}
		res, err := txClient.ExecContext(txCtx, `UPDATE referral_campaigns SET budget_reserved=budget_reserved-$1,budget_paid=budget_paid+$1,updated_at=NOW() WHERE id=$2 AND budget_reserved >= $1`, out.Amount, out.CampaignID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return service.ErrReferralCampaignBudgetExceeded
		}
		if _, err = txClient.ExecContext(txCtx, `UPDATE user_affiliates SET aff_frozen_quota=aff_frozen_quota+$1,aff_history_quota=aff_history_quota+$1,updated_at=NOW() WHERE user_id=$2`, out.Amount, out.UserID); err != nil {
			return err
		}
		if _, err = txClient.ExecContext(txCtx, `INSERT INTO user_affiliate_ledger (user_id,action,amount,referral_reward_id,frozen_until,created_at,updated_at) VALUES ($1,'accrue',$2,$3,$4,NOW(),NOW())`, out.UserID, out.Amount, out.ID, out.FrozenUntil); err != nil {
			return err
		}
		if _, err = txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key) VALUES ($1,$2,'pay',$3,$4) ON CONFLICT (idempotency_key) DO NOTHING`, out.CampaignID, out.ID, out.Amount, fmt.Sprintf("referral:reward:%d:pay", out.ID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *affiliateRepository) ResolveReferralRewardDebt(ctx context.Context, campaignID, rewardID, expectedVersion int64, decision string, actorID int64, note string) (*service.ReferralReward, error) {
	var out service.ReferralReward
	var unlock, deadline, frozen sql.NullTime
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		resolvedStatus := "resolved"
		if decision == "recovered" {
			resolvedStatus = "revoked"
		}
		if err := scanAffiliateRow(txCtx, txClient, `UPDATE referral_campaign_rewards SET status=$4,updated_at=NOW(),version=version+1 WHERE id=$1 AND campaign_id=$2 AND version=$3 AND status='debt_review' RETURNING id,campaign_id,user_id,tier_no,reward_type,amount::double precision,currency,status,unlock_at,claim_deadline,frozen_until,version`, []any{rewardID, campaignID, expectedVersion, resolvedStatus}, &out.ID, &out.CampaignID, &out.UserID, &out.Tier, &out.RewardType, &out.Amount, &out.Currency, &out.Status, &unlock, &deadline, &frozen, &out.Version); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrReferralCampaignVersionConflict
			}
			return err
		}
		if decision == "recovered" {
			if _, err := txClient.ExecContext(txCtx, `UPDATE referral_campaigns SET budget_paid=GREATEST(budget_paid-$1,0),updated_at=NOW() WHERE id=$2`, out.Amount, campaignID); err != nil {
				return err
			}
			if _, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key,metadata) VALUES ($1,$2,'reverse',$3,$4,jsonb_build_object('source','manual_debt_resolution')) ON CONFLICT DO NOTHING`, campaignID, rewardID, out.Amount, fmt.Sprintf("referral:reward:%d:debt-recovered", rewardID)); err != nil {
				return err
			}
		}
		_, err := txClient.ExecContext(txCtx, `INSERT INTO referral_campaign_audit_logs (campaign_id,campaign_version,actor_id,action,detail) SELECT campaign_id,$2,$3,'reward_debt_resolved',jsonb_build_object('reward_id',id,'decision',$4::text,'note',$5::text) FROM referral_campaign_rewards WHERE id=$1`, rewardID, out.Version, actorID, decision, strings.TrimSpace(note))
		return err
	})
	if err != nil {
		return nil, err
	}
	if unlock.Valid {
		v := unlock.Time
		out.UnlockAt = &v
	}
	if deadline.Valid {
		v := deadline.Time
		out.ClaimDeadline = &v
	}
	if frozen.Valid {
		v := frozen.Time
		out.FrozenUntil = &v
	}
	return &out, nil
}

func (r *affiliateRepository) ReverseReferralRewardsByOrder(ctx context.Context, orderID int64) error {
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		rows, err := txClient.QueryContext(txCtx, `
SELECT DISTINCT a.campaign_id,a.inviter_id,a.invitee_id
FROM play_membership_verified_contributions m
JOIN referral_campaign_attributions a ON a.invitee_id=m.user_id
WHERE m.order_id=$1 AND m.qualification_state='verified'`, orderID)
		if err != nil {
			return err
		}
		type affectedInvite struct{ campaignID, inviterID, inviteeID int64 }
		affected := make([]affectedInvite, 0)
		for rows.Next() {
			var item affectedInvite
			if err := rows.Scan(&item.campaignID, &item.inviterID, &item.inviteeID); err != nil {
				_ = rows.Close()
				return err
			}
			affected = append(affected, item)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, item := range affected {
			var payThreshold, usageThreshold, netPaid, actualCost float64
			var riskStatus string
			if err := scanAffiliateRow(txCtx, txClient, `
SELECT c.pay_threshold::double precision,c.usage_threshold::double precision,
 COALESCE((SELECT SUM(m.net_amount) FROM play_membership_verified_contributions m WHERE m.user_id=$2 AND m.paid_at>=a.registered_at AND m.paid_at<=c.qualification_to),0)::double precision,
 COALESCE((SELECT SUM(u.actual_cost) FROM usage_logs u WHERE u.user_id=$2 AND u.created_at>=a.registered_at AND u.created_at<=c.qualification_to),0)::double precision,
 COALESCE(q.risk_status,'pending')
FROM referral_campaigns c JOIN referral_campaign_attributions a ON a.campaign_id=c.id AND a.invitee_id=$2
LEFT JOIN referral_campaign_qualifications q ON q.campaign_id=c.id AND q.invitee_id=$2
WHERE c.id=$1 FOR UPDATE`, []any{item.campaignID, item.inviteeID}, &payThreshold, &usageThreshold, &netPaid, &actualCost, &riskStatus); err != nil {
				return err
			}
			meets := netPaid+1e-9 >= payThreshold && actualCost+1e-9 >= usageThreshold && riskStatus == service.ReferralRiskApproved
			newStatus := service.ReferralQualificationRevoked
			if meets {
				newStatus = service.ReferralQualificationQualified
			}
			if _, err := txClient.ExecContext(txCtx, `UPDATE referral_campaign_qualifications SET net_paid=$1,actual_cost=$2,status=$3,revoked_at=CASE WHEN $3='revoked' THEN COALESCE(revoked_at,NOW()) ELSE NULL END,revocation_reason=CASE WHEN $3='revoked' THEN 'payment order refunded below qualification threshold' ELSE '' END,updated_at=NOW() WHERE campaign_id=$4 AND invitee_id=$5`, netPaid, actualCost, newStatus, item.campaignID, item.inviteeID); err != nil {
				return err
			}
			if meets {
				continue
			}
			var qualifiedCount int64
			if err := scanAffiliateRow(txCtx, txClient, `SELECT COUNT(*) FROM referral_campaign_qualifications WHERE campaign_id=$1 AND inviter_id=$2 AND status='qualified'`, []any{item.campaignID, item.inviterID}, &qualifiedCount); err != nil {
				return err
			}
			if err := revokeReferralRewardsAboveCountTx(txCtx, txClient, item.campaignID, item.inviterID, qualifiedCount); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *affiliateRepository) EnqueueReferralRefundReconcile(ctx context.Context, orderID int64) error {
	if orderID <= 0 {
		return nil
	}
	client := clientFromContext(ctx, r.client)
	_, err := client.ExecContext(ctx, `
INSERT INTO referral_campaign_reconcile_queue (campaign_id,invitee_id,source_order_id,action,status,available_at)
SELECT DISTINCT a.campaign_id,a.invitee_id,$1,'refund_revoke','pending',NOW()
FROM play_membership_verified_contributions m
JOIN referral_campaign_attributions a ON a.invitee_id=m.user_id
WHERE m.order_id=$1 AND m.qualification_state='verified'
ON CONFLICT (campaign_id,invitee_id,source_order_id,action) DO UPDATE SET
 status='pending',available_at=NOW(),processed_at=NULL,last_error=''`, orderID)
	return err
}

func (r *affiliateRepository) ProcessReferralReconcileQueue(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	processed := 0
	for processed < limit {
		var queueID, orderID int64
		err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
			err := scanAffiliateRow(txCtx, txClient, `
SELECT id,source_order_id FROM referral_campaign_reconcile_queue
WHERE action='refund_revoke' AND status IN ('pending','failed','processing') AND available_at<=NOW() AND source_order_id IS NOT NULL
ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1`, nil, &queueID, &orderID)
			if err != nil {
				return err
			}
			_, err = txClient.ExecContext(txCtx, `UPDATE referral_campaign_reconcile_queue SET status='processing',attempts=attempts+1,last_error='',available_at=NOW()+INTERVAL '5 minutes' WHERE source_order_id=$1 AND action='refund_revoke' AND status IN ('pending','failed','processing') AND available_at<=NOW()`, orderID)
			return err
		})
		if errors.Is(err, sql.ErrNoRows) {
			return processed, nil
		}
		if err != nil {
			return processed, err
		}
		if err = r.ReverseReferralRewardsByOrder(ctx, orderID); err != nil {
			message := err.Error()
			if len(message) > 1000 {
				message = message[:1000]
			}
			_, updateErr := r.client.ExecContext(ctx, `
UPDATE referral_campaign_reconcile_queue SET status='failed',last_error=$2,
 available_at=NOW()+LEAST(make_interval(mins => GREATEST(attempts,1)*5),INTERVAL '24 hours')
WHERE source_order_id=$1 AND action='refund_revoke' AND status='processing'`, orderID, message)
			if updateErr != nil {
				return processed, errors.Join(err, updateErr)
			}
			return processed, err
		}
		if _, err = r.client.ExecContext(ctx, `UPDATE referral_campaign_reconcile_queue SET status='done',processed_at=NOW(),last_error='' WHERE source_order_id=$1 AND action='refund_revoke' AND status='processing'`, orderID); err != nil {
			return processed, err
		}
		processed++
		_ = queueID
	}
	return processed, nil
}

func (r *affiliateRepository) GetReferralGrowthOverview(ctx context.Context, campaignID, currentUserID *int64) (*service.ReferralGrowthOverview, error) {
	client := clientFromContext(ctx, r.client)
	filter := "($1::bigint IS NULL OR campaign_id = $1)"
	args := []any{nullableInt64Arg(campaignID)}
	var out service.ReferralGrowthOverview
	if err := scanAffiliateRow(ctx, client, `SELECT COUNT(*) FROM referral_campaign_attributions WHERE `+filter, args, &out.InvitedCount); err != nil {
		return nil, err
	}
	if err := scanAffiliateRow(ctx, client, `SELECT COUNT(*) FROM referral_campaign_qualifications WHERE `+filter+` AND status='qualified'`, args, &out.QualifiedCount); err != nil {
		return nil, err
	}
	if err := scanAffiliateRow(ctx, client, `SELECT COUNT(*) FROM referral_campaign_qualifications WHERE `+filter+` AND net_paid > 0`, args, &out.PaidInviteeCount); err != nil {
		return nil, err
	}
	if err := scanAffiliateRow(ctx, client, `SELECT COALESCE(SUM(amount),0)::double precision FROM referral_campaign_rewards WHERE `+filter+` AND status IN ('claimable','claimed_frozen','available')`, args, &out.RewardUnlocked); err != nil {
		return nil, err
	}
	if err := scanAffiliateRow(ctx, client, `SELECT COALESCE(SUM(amount),0)::double precision FROM referral_campaign_rewards WHERE `+filter+` AND status IN ('claimed_frozen','available','resolved')`, args, &out.RewardClaimed); err != nil {
		return nil, err
	}
	rows, err := client.QueryContext(ctx, `
WITH qualified AS (
 SELECT q.inviter_id,COUNT(*)::bigint AS qualified_count,COALESCE(SUM(q.net_paid),0)::double precision AS paid
 FROM referral_campaign_qualifications q
 WHERE ($1::bigint IS NULL OR q.campaign_id=$1) AND q.status='qualified'
 GROUP BY q.inviter_id
), rewards AS (
 SELECT r.user_id,COALESCE(SUM(r.amount) FILTER (WHERE r.status IN ('claimable','claimed_frozen','available','resolved')),0)::double precision AS reward
 FROM referral_campaign_rewards r WHERE ($1::bigint IS NULL OR r.campaign_id=$1) GROUP BY r.user_id
), ranked AS (
 SELECT q.inviter_id,COALESCE(u.email,'') AS email,q.qualified_count,q.paid,COALESCE(r.reward,0)::double precision AS reward,
 ROW_NUMBER() OVER (ORDER BY q.qualified_count DESC,q.paid DESC,q.inviter_id ASC)::bigint AS rank
 FROM qualified q JOIN users u ON u.id=q.inviter_id LEFT JOIN rewards r ON r.user_id=q.inviter_id
)
SELECT inviter_id,email,qualified_count,paid,reward,rank FROM ranked
WHERE rank<=50 OR inviter_id=$2 ORDER BY rank`, nullableInt64Arg(campaignID), nullableInt64Arg(currentUserID))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var userID int64
		var email string
		var qualified int64
		var paid, reward float64
		var rank int64
		if err := rows.Scan(&userID, &email, &qualified, &paid, &reward, &rank); err != nil {
			return nil, err
		}
		out.Ranking = append(out.Ranking, service.ReferralGrowthRanking{Rank: int(rank), EmailMasked: maskReferralEmail(email), QualifiedCount: qualified, RewardAmount: reward, IsMe: currentUserID != nil && userID == *currentUserID})
	}
	if out.Ranking == nil {
		out.Ranking = []service.ReferralGrowthRanking{}
	}
	return &out, rows.Err()
}

func (r *affiliateRepository) GetReferralCampaignProgress(ctx context.Context, campaignID, userID int64) (*service.ReferralCampaignProgress, error) {
	campaign, err := r.GetReferralCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	tiers, err := r.GetReferralCampaignTiers(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	progress := &service.ReferralCampaignProgress{Campaign: service.ReferralCampaignPublic{ID: campaign.ID, Key: campaign.Key, Name: campaign.Name, Status: campaign.Status, Version: campaign.Version, RegistrationFrom: campaign.RegistrationFrom, RegistrationTo: campaign.RegistrationTo, StartsAt: campaign.StartsAt, EndsAt: campaign.EndsAt, QualificationTo: campaign.QualificationTo, ClaimDeadline: campaign.ClaimDeadline, RiskHoldHours: campaign.RiskHoldHours, PayThreshold: campaign.PayThreshold, UsageThreshold: campaign.UsageThreshold, MaxEnrollments: campaign.MaxEnrollments, RewardMode: campaign.RewardMode, PublicRulesMD: campaign.PublicRulesMD, InviteeNoticeMD: campaign.InviteeNoticeMD, LegacyRebatePolicy: campaign.LegacyRebatePolicy, RulesVersion: campaign.RulesVersion, RulesUpdatedAt: campaign.RulesUpdatedAt}, Tiers: tiers, Rewards: []service.ReferralReward{}}
	if enrollment, err := r.GetReferralCampaignEnrollment(ctx, campaignID, userID); err == nil {
		progress.Enrollment = enrollment
	} else if !errors.Is(err, service.ErrReferralCampaignNotOpen) {
		return nil, err
	}
	client := clientFromContext(ctx, r.client)
	if err := scanAffiliateRow(ctx, client, `SELECT (SELECT COUNT(*) FROM referral_campaign_attributions WHERE campaign_id=$1 AND inviter_id=$2),(SELECT COUNT(*) FROM referral_campaign_qualifications WHERE campaign_id=$1 AND inviter_id=$2 AND status='qualified')`, []any{campaignID, userID}, &progress.InvitedCount, &progress.QualifiedCount); err != nil {
		return nil, err
	}
	rows, err := client.QueryContext(ctx, `SELECT id,campaign_id,user_id,tier_no,reward_type,amount::double precision,currency,status,unlock_at,claim_deadline,frozen_until,version FROM referral_campaign_rewards WHERE campaign_id=$1 AND user_id=$2 ORDER BY tier_no,id`, campaignID, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var reward service.ReferralReward
		var unlock, deadline, frozen sql.NullTime
		if err := rows.Scan(&reward.ID, &reward.CampaignID, &reward.UserID, &reward.Tier, &reward.RewardType, &reward.Amount, &reward.Currency, &reward.Status, &unlock, &deadline, &frozen, &reward.Version); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if unlock.Valid {
			v := unlock.Time
			reward.UnlockAt = &v
		}
		if deadline.Valid {
			v := deadline.Time
			reward.ClaimDeadline = &v
		}
		if frozen.Valid {
			v := frozen.Time
			reward.FrozenUntil = &v
		}
		progress.Rewards = append(progress.Rewards, reward)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var hasClaimable bool
	if err := scanAffiliateRow(ctx, client, `SELECT COALESCE((SELECT rules_version < $3 FROM referral_campaign_user_views WHERE campaign_id=$1 AND user_id=$2),true), EXISTS (SELECT 1 FROM referral_campaign_rewards WHERE campaign_id=$1 AND user_id=$2 AND status='claimable')`, []any{campaignID, userID, campaign.RulesVersion}, &progress.UnseenUpdate, &hasClaimable); err != nil {
		return nil, err
	}
	if hasClaimable {
		progress.Attention = "claimable_reward"
	}
	if progress.Attention == "" && progress.UnseenUpdate {
		progress.Attention = "rules_updated"
	}
	overview, err := r.GetReferralGrowthOverview(ctx, &campaignID, &userID)
	if err != nil {
		return nil, err
	}
	progress.Leaderboard = overview.Ranking
	for i := range overview.Ranking {
		if overview.Ranking[i].IsMe {
			item := overview.Ranking[i]
			progress.Ranking = &item
			break
		}
	}
	return progress, nil
}

func maskReferralEmail(value string) string {
	parts := strings.SplitN(strings.TrimSpace(value), "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}
	localRunes := []rune(parts[0])
	local := ""
	if len(localRunes) == 1 {
		local = string(localRunes[0]) + "*"
	} else if len(localRunes) == 2 {
		local = string(localRunes[0]) + "*"
	} else {
		local = string(localRunes[0]) + "***" + string(localRunes[len(localRunes)-1])
	}
	return local + "@" + parts[1]
}

func adjustAffiliateTransferTotalRecharged(ctx context.Context, client affiliateQueryExecer, userID int64, amount float64) error {
	result, err := client.ExecContext(ctx, `
UPDATE users
SET total_recharged = GREATEST(total_recharged + $1, 0),
    updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL`, amount, userID)
	if err != nil {
		return fmt.Errorf("update total recharged after affiliate transfer: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

func nullableFloat64Ptr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func generateAffiliateCode() (string, error) {
	buf := make([]byte, affiliateCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate affiliate code: %w", err)
	}
	for i := range buf {
		buf[i] = affiliateCodeCharset[int(buf[i])%len(affiliateCodeCharset)]
	}
	return string(buf), nil
}

func isAffiliateUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == "23505"
	}
	return false
}

// UpdateUserAffCode 改写用户的邀请码（自定义专属邀请码）。
// 唯一性冲突返回 ErrAffiliateCodeTaken。
func (r *affiliateRepository) UpdateUserAffCode(ctx context.Context, userID int64, newCode string) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	code := strings.ToUpper(strings.TrimSpace(newCode))
	if code == "" {
		return service.ErrAffiliateCodeInvalid
	}

	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_code = $1,
    aff_code_custom = true,
    updated_at = NOW()
WHERE user_id = $2`, code, userID)
		if err != nil {
			if isAffiliateUniqueViolation(err) {
				return service.ErrAffiliateCodeTaken
			}
			return fmt.Errorf("update aff_code: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}
		return nil
	})
}

// ResetUserAffCode 把 aff_code 还原为系统随机码，并清除 aff_code_custom 标记。
func (r *affiliateRepository) ResetUserAffCode(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", service.ErrUserNotFound
	}
	var newCode string
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		for i := 0; i < affiliateCodeMaxAttempts; i++ {
			candidate, codeErr := generateAffiliateCode()
			if codeErr != nil {
				return codeErr
			}
			res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_code = $1,
    aff_code_custom = false,
    updated_at = NOW()
WHERE user_id = $2`, candidate, userID)
			if err != nil {
				if isAffiliateUniqueViolation(err) {
					continue
				}
				return fmt.Errorf("reset aff_code: %w", err)
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return service.ErrUserNotFound
			}
			newCode = candidate
			return nil
		}
		return fmt.Errorf("reset aff_code: exhausted attempts")
	})
	if err != nil {
		return "", err
	}
	return newCode, nil
}

// SetUserRebateRate 设置或清除用户专属返利比例。ratePercent==nil 表示清除（沿用全局）。
func (r *affiliateRepository) SetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		// nullableArg lets us use a single UPDATE for both "set value" and
		// "clear" cases — database/sql converts nil interface{} to SQL NULL.
		res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_rebate_rate_percent = $1,
    updated_at = NOW()
WHERE user_id = $2`, nullableArg(ratePercent), userID)
		if err != nil {
			return fmt.Errorf("set aff_rebate_rate_percent: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}
		return nil
	})
}

// BatchSetUserRebateRate 批量为多个用户设置专属比例（nil 清除）。
func (r *affiliateRepository) BatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		for _, uid := range userIDs {
			if uid <= 0 {
				continue
			}
			if _, err := ensureUserAffiliateWithClient(txCtx, txClient, uid); err != nil {
				return err
			}
		}
		_, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_rebate_rate_percent = $1,
    updated_at = NOW()
WHERE user_id = ANY($2)`, nullableArg(ratePercent), pq.Array(userIDs))
		if err != nil {
			return fmt.Errorf("batch set aff_rebate_rate_percent: %w", err)
		}
		return nil
	})
}

// nullableArg unwraps a *float64 into an interface{} suitable for SQL parameter
// binding: nil pointer → SQL NULL, non-nil → the float value.
func nullableArg(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt64Arg(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

// ListUsersWithCustomSettings 列出有专属配置（自定义码或专属比例）的用户。
//
// 单一查询同时处理"无搜索"与"按邮箱/用户名模糊搜索"：
// 空 search 时拼接出的 LIKE 模式为 "%%"，匹配所有行；非空时按 ILIKE 子串匹配。
// 这避免了为两种情况维护两份 SQL 模板。
func (r *affiliateRepository) ListUsersWithCustomSettings(ctx context.Context, filter service.AffiliateAdminFilter) ([]service.AffiliateAdminEntry, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	likePattern := "%" + strings.TrimSpace(filter.Search) + "%"

	const baseFrom = `
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id
WHERE (ua.aff_code_custom = true OR ua.aff_rebate_rate_percent IS NOT NULL)
  AND (u.email ILIKE $1 OR u.username ILIKE $1)`

	client := clientFromContext(ctx, r.client)

	total, err := scanInt64(ctx, client, "SELECT COUNT(*)"+baseFrom, likePattern)
	if err != nil {
		return nil, 0, fmt.Errorf("count affiliate admin entries: %w", err)
	}

	listQuery := `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ua.aff_code,
       ua.aff_code_custom,
       ua.aff_rebate_rate_percent,
       ua.aff_count` + baseFrom + `
ORDER BY ua.updated_at DESC
LIMIT $2 OFFSET $3`

	rows, err := client.QueryContext(ctx, listQuery, likePattern, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate admin entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries := make([]service.AffiliateAdminEntry, 0)
	for rows.Next() {
		var e service.AffiliateAdminEntry
		var rebate sql.NullFloat64
		if err := rows.Scan(&e.UserID, &e.Email, &e.Username, &e.AffCode,
			&e.AffCodeCustom, &rebate, &e.AffCount); err != nil {
			return nil, 0, err
		}
		if rebate.Valid {
			v := rebate.Float64
			e.AffRebateRatePercent = &v
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// scanInt64 runs a query expected to return a single int64 column (e.g. COUNT).
func scanInt64(ctx context.Context, client affiliateQueryExecer, query string, args ...any) (int64, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var v int64
	if err := rows.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}
