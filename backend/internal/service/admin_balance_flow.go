package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	BalanceFlowTypePaymentRecharge = "payment_recharge"
	BalanceFlowTypeUsageCharge     = "usage_charge"
	BalanceFlowTypeRefund          = "refund"
	BalanceFlowTypePromoBonus      = "promo_bonus"
)

type AdminBalanceFlowHistory struct {
	Items    []AdminBalanceFlowItem  `json:"items"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
	Pages    int                     `json:"pages"`
	Summary  AdminBalanceFlowSummary `json:"summary"`
}

type AdminBalanceFlowSummary struct {
	CurrentBalance float64 `json:"current_balance"`
	FrozenBalance  float64 `json:"frozen_balance"`
	TotalIn        float64 `json:"total_in"`
	TotalOut       float64 `json:"total_out"`
	NetDelta       float64 `json:"net_delta"`
	RechargeTotal  float64 `json:"recharge_total"`
}

type AdminBalanceFlowItem struct {
	ID                string         `json:"id"`
	Type              string         `json:"type"`
	SourceType        string         `json:"source_type"`
	SourceID          string         `json:"source_id,omitempty"`
	Amount            float64        `json:"amount"`
	BalanceDelta      float64        `json:"balance_delta"`
	FrozenDelta       float64        `json:"frozen_delta"`
	BalanceBefore     *float64       `json:"balance_before,omitempty"`
	BalanceAfter      *float64       `json:"balance_after,omitempty"`
	FrozenBefore      *float64       `json:"frozen_before,omitempty"`
	FrozenAfter       *float64       `json:"frozen_after,omitempty"`
	OccurredAt        time.Time      `json:"occurred_at"`
	Description       string         `json:"description"`
	ActorType         string         `json:"actor_type"`
	ActorUserID       *int64         `json:"actor_user_id,omitempty"`
	RelatedObjectType string         `json:"related_object_type,omitempty"`
	RelatedObjectID   string         `json:"related_object_id,omitempty"`
	Reference         string         `json:"reference,omitempty"`
	Notes             string         `json:"notes,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	Confidence        string         `json:"confidence"`
}

type AdminBalanceReconciliation struct {
	CurrentBalance    float64                `json:"current_balance"`
	CurrentFrozen     float64                `json:"current_frozen"`
	LedgerBalanceSum  float64                `json:"ledger_balance_sum"`
	LedgerFrozenSum   float64                `json:"ledger_frozen_sum"`
	BalanceDifference float64                `json:"balance_difference"`
	FrozenDifference  float64                `json:"frozen_difference"`
	Recent            []AdminBalanceFlowItem `json:"recent"`
	Warnings          []string               `json:"warnings"`
}

const adminBalanceFlowCTE = `
WITH flow_union AS (
	SELECT
		('payment_order:' || po.id::text) AS flow_id,
		po.id AS sort_id,
		'payment_recharge'::text AS type,
		'payment_orders'::text AS source_type,
		po.id::text AS source_id,
		po.amount::double precision AS amount,
		po.amount::double precision AS balance_delta,
		0::double precision AS frozen_delta,
		NULL::double precision AS balance_before,
		NULL::double precision AS balance_after,
		NULL::double precision AS frozen_before,
		NULL::double precision AS frozen_after,
		COALESCE(po.completed_at, po.paid_at, po.updated_at, po.created_at) AS occurred_at,
		'订单充值'::text AS description,
		'system'::text AS actor_type,
		NULL::bigint AS actor_user_id,
		'payment_order'::text AS related_object_type,
		po.id::text AS related_object_id,
		COALESCE(NULLIF(po.out_trade_no, ''), NULLIF(po.payment_trade_no, ''), po.recharge_code, po.id::text) AS reference,
		COALESCE(po.refund_reason, '')::text AS notes,
		jsonb_build_object(
			'order_id', po.id,
			'out_trade_no', po.out_trade_no,
			'payment_type', po.payment_type,
			'pay_amount', po.pay_amount,
			'recharge_code', po.recharge_code,
			'status', po.status
		) AS metadata,
		TRUE AS affects_balance,
		TRUE AS is_recharge,
		'high'::text AS confidence
	FROM payment_orders po
	WHERE po.user_id = $1
	  AND po.order_type = 'balance'
	  AND po.status = 'COMPLETED'
	  AND po.amount <> 0

	UNION ALL

	SELECT
		('redeem_code:' || rc.id::text) AS flow_id,
		rc.id AS sort_id,
		rc.type::text AS type,
		'redeem_codes'::text AS source_type,
		rc.id::text AS source_id,
		rc.value::double precision AS amount,
		CASE WHEN rc.type IN ('balance', 'admin_balance') THEN rc.value ELSE 0 END::double precision AS balance_delta,
		0::double precision AS frozen_delta,
		NULL::double precision AS balance_before,
		NULL::double precision AS balance_after,
		NULL::double precision AS frozen_before,
		NULL::double precision AS frozen_after,
		COALESCE(rc.used_at, rc.created_at) AS occurred_at,
		CASE
			WHEN rc.type = 'admin_balance' AND rc.value < 0 THEN '管理员扣减余额'
			WHEN rc.type = 'admin_balance' THEN '管理员增加余额'
			WHEN rc.type = 'concurrency' THEN '兑换码增加并发'
			WHEN rc.type = 'admin_concurrency' THEN '管理员调整并发'
			ELSE '兑换码充值'
		END AS description,
		CASE WHEN rc.type IN ('admin_balance', 'admin_concurrency') THEN 'admin' ELSE 'user' END AS actor_type,
		rc.used_by AS actor_user_id,
		'redeem_code'::text AS related_object_type,
		rc.id::text AS related_object_id,
		rc.code AS reference,
		COALESCE(rc.notes, '')::text AS notes,
		jsonb_build_object(
			'code', rc.code,
			'status', rc.status,
			'value', rc.value,
			'group_id', rc.group_id,
			'validity_days', rc.validity_days
		) AS metadata,
		rc.type IN ('balance', 'admin_balance') AS affects_balance,
		rc.type IN ('balance', 'admin_balance') AND rc.value > 0 AS is_recharge,
		'high'::text AS confidence
	FROM redeem_codes rc
	WHERE rc.used_by = $1
	  AND rc.status = 'used'
	  AND rc.type IN ('balance', 'admin_balance', 'concurrency', 'admin_concurrency')
	  AND NOT EXISTS (
		SELECT 1
		FROM payment_orders po
		WHERE po.user_id = rc.used_by
		  AND po.order_type = 'balance'
		  AND po.status = 'COMPLETED'
		  AND po.recharge_code = rc.code
	  )

	UNION ALL

	SELECT
		('affiliate_transfer:' || ual.id::text) AS flow_id,
		ual.id AS sort_id,
		'affiliate_balance'::text AS type,
		'user_affiliate_ledger'::text AS source_type,
		ual.id::text AS source_id,
		ual.amount::double precision AS amount,
		ual.amount::double precision AS balance_delta,
		0::double precision AS frozen_delta,
		NULL::double precision AS balance_before,
		ual.balance_after::double precision AS balance_after,
		NULL::double precision AS frozen_before,
		NULL::double precision AS frozen_after,
		ual.created_at AS occurred_at,
		'返利余额转入'::text AS description,
		'user'::text AS actor_type,
		ual.user_id AS actor_user_id,
		'user_affiliate_ledger'::text AS related_object_type,
		ual.id::text AS related_object_id,
		COALESCE(ual.source_order_id::text, '') AS reference,
		''::text AS notes,
		jsonb_build_object(
			'action', ual.action,
			'source_order_id', ual.source_order_id,
			'aff_quota_after', ual.aff_quota_after,
			'aff_frozen_quota_after', ual.aff_frozen_quota_after,
			'aff_history_quota_after', ual.aff_history_quota_after
		) AS metadata,
		TRUE AS affects_balance,
		FALSE AS is_recharge,
		'high'::text AS confidence
	FROM user_affiliate_ledger ual
	WHERE ual.user_id = $1
	  AND ual.action = 'transfer'

	UNION ALL

	SELECT
		('play_reward:' || prl.id::text) AS flow_id,
		prl.id AS sort_id,
		prl.source::text AS type,
		'play_reward_ledger'::text AS source_type,
		prl.id::text AS source_id,
		prl.amount::double precision AS amount,
		prl.amount::double precision AS balance_delta,
		0::double precision AS frozen_delta,
		NULL::double precision AS balance_before,
		NULL::double precision AS balance_after,
		NULL::double precision AS frozen_before,
		NULL::double precision AS frozen_after,
		prl.created_at AS occurred_at,
		CASE prl.source
			WHEN 'checkin' THEN '签到奖励'
			WHEN 'checkin_makeup' THEN '补签奖励'
			WHEN 'quiz' THEN '答题奖励'
			WHEN 'blindbox' THEN '盲盒净变动'
			WHEN 'arena_settlement' THEN '竞技场结算'
			WHEN 'arena_daily_settlement' THEN '日榜竞技场结算'
			WHEN 'team_shared_reward' THEN '组队共享奖励'
			ELSE '玩法奖励'
		END AS description,
		'system'::text AS actor_type,
		NULL::bigint AS actor_user_id,
		CASE WHEN b.id IS NOT NULL THEN 'play_blindbox_open' ELSE 'play_reward_ledger' END AS related_object_type,
		COALESCE(b.id::text, prl.id::text) AS related_object_id,
		prl.idempotency_key AS reference,
		''::text AS notes,
		COALESCE(prl.detail, '{}'::jsonb) ||
			jsonb_strip_nulls(jsonb_build_object(
				'ledger_id', prl.id,
				'idempotency_key', prl.idempotency_key,
				'blindbox_open_id', b.id,
				'cost_amount', b.cost_amount,
				'reward_amount', b.reward_amount,
				'net_amount', CASE WHEN b.id IS NOT NULL THEN b.reward_amount - b.cost_amount ELSE NULL END,
				'pool_version', b.pool_version,
				'open_source', b.open_source
			)) AS metadata,
		TRUE AS affects_balance,
		FALSE AS is_recharge,
		'high'::text AS confidence
	FROM play_reward_ledger prl
	LEFT JOIN play_blindbox_opens b
	  ON b.user_id = prl.user_id
	 AND b.idempotency_key = prl.idempotency_key
	WHERE prl.user_id = $1
	  AND prl.source <> 'team_affiliate_bonus'

	UNION ALL

	SELECT
		('blindbox_open_orphan:' || b.id::text) AS flow_id,
		b.id AS sort_id,
		'blindbox'::text AS type,
		'play_blindbox_opens'::text AS source_type,
		b.id::text AS source_id,
		(b.reward_amount - b.cost_amount)::double precision AS amount,
		(b.reward_amount - b.cost_amount)::double precision AS balance_delta,
		0::double precision AS frozen_delta,
		NULL::double precision AS balance_before,
		NULL::double precision AS balance_after,
		NULL::double precision AS frozen_before,
		NULL::double precision AS frozen_after,
		b.created_at AS occurred_at,
		'盲盒净变动'::text AS description,
		'system'::text AS actor_type,
		NULL::bigint AS actor_user_id,
		'play_blindbox_open'::text AS related_object_type,
		b.id::text AS related_object_id,
		b.idempotency_key AS reference,
		''::text AS notes,
		jsonb_build_object(
			'blindbox_open_id', b.id,
			'open_date', b.open_date,
			'cost_amount', b.cost_amount,
			'reward_amount', b.reward_amount,
			'net_amount', b.reward_amount - b.cost_amount,
			'pool_version', b.pool_version,
			'open_source', b.open_source,
			'missing_play_reward_ledger', true
		) AS metadata,
		TRUE AS affects_balance,
		FALSE AS is_recharge,
		'medium'::text AS confidence
	FROM play_blindbox_opens b
	WHERE b.user_id = $1
	  AND NOT EXISTS (
		SELECT 1
		FROM play_reward_ledger prl
		WHERE prl.user_id = b.user_id
		  AND prl.idempotency_key = b.idempotency_key
	  )

	UNION ALL

	SELECT
		('promo_code_usage:' || pcu.id::text) AS flow_id,
		pcu.id AS sort_id,
		'promo_bonus'::text AS type,
		'promo_code_usages'::text AS source_type,
		pcu.id::text AS source_id,
		pcu.bonus_amount::double precision AS amount,
		pcu.bonus_amount::double precision AS balance_delta,
		0::double precision AS frozen_delta,
		NULL::double precision AS balance_before,
		NULL::double precision AS balance_after,
		NULL::double precision AS frozen_before,
		NULL::double precision AS frozen_after,
		pcu.used_at AS occurred_at,
		'优惠码奖励'::text AS description,
		'user'::text AS actor_type,
		pcu.user_id AS actor_user_id,
		'promo_code'::text AS related_object_type,
		pcu.promo_code_id::text AS related_object_id,
		pc.code AS reference,
		COALESCE(pc.notes, '')::text AS notes,
		jsonb_build_object(
			'promo_code_id', pcu.promo_code_id,
			'code', pc.code,
			'bonus_amount', pcu.bonus_amount
		) AS metadata,
		TRUE AS affects_balance,
		FALSE AS is_recharge,
		'high'::text AS confidence
	FROM promo_code_usages pcu
	JOIN promo_codes pc ON pc.id = pcu.promo_code_id
	WHERE pcu.user_id = $1
	  AND pcu.bonus_amount <> 0

	UNION ALL

	SELECT
		('usage_log:' || ul.id::text) AS flow_id,
		ul.id AS sort_id,
		'usage_charge'::text AS type,
		'usage_logs'::text AS source_type,
		ul.id::text AS source_id,
		(-ul.actual_cost)::double precision AS amount,
		(-ul.actual_cost)::double precision AS balance_delta,
		0::double precision AS frozen_delta,
		NULL::double precision AS balance_before,
		NULL::double precision AS balance_after,
		NULL::double precision AS frozen_before,
		NULL::double precision AS frozen_after,
		ul.created_at AS occurred_at,
		'API 消耗扣费'::text AS description,
		'system'::text AS actor_type,
		NULL::bigint AS actor_user_id,
		'usage_log'::text AS related_object_type,
		ul.id::text AS related_object_id,
		COALESCE(NULLIF(ul.request_id, ''), ul.id::text) AS reference,
		''::text AS notes,
		jsonb_build_object(
			'usage_log_id', ul.id,
			'request_id', ul.request_id,
			'api_key_id', ul.api_key_id,
			'model', ul.model,
			'requested_model', ul.requested_model,
			'upstream_model', ul.upstream_model,
			'billing_mode', ul.billing_mode,
			'actual_cost', ul.actual_cost,
			'total_cost', ul.total_cost,
			'input_tokens', ul.input_tokens,
			'output_tokens', ul.output_tokens
		) AS metadata,
		TRUE AS affects_balance,
		FALSE AS is_recharge,
		'high'::text AS confidence
	FROM usage_logs ul
	WHERE ul.user_id = $1
	  AND ul.billing_type = 0
	  AND ul.actual_cost > 0

	UNION ALL

	SELECT
		('payment_refund:' || pal.id::text) AS flow_id,
		pal.id AS sort_id,
		'refund'::text AS type,
		'payment_audit_logs'::text AS source_type,
		pal.id::text AS source_id,
		(-COALESCE(pal.balance_deducted, po.refund_amount, 0))::double precision AS amount,
		(-COALESCE(pal.balance_deducted, po.refund_amount, 0))::double precision AS balance_delta,
		0::double precision AS frozen_delta,
		NULL::double precision AS balance_before,
		NULL::double precision AS balance_after,
		NULL::double precision AS frozen_before,
		NULL::double precision AS frozen_after,
		pal.created_at AS occurred_at,
		'退款扣回'::text AS description,
		COALESCE(NULLIF(pal.operator, ''), 'admin')::text AS actor_type,
		NULL::bigint AS actor_user_id,
		'payment_order'::text AS related_object_type,
		po.id::text AS related_object_id,
		COALESCE(NULLIF(po.out_trade_no, ''), po.id::text) AS reference,
		COALESCE(po.refund_reason, '')::text AS notes,
		jsonb_build_object(
			'audit_log_id', pal.id,
			'order_id', po.id,
			'action', pal.action,
			'detail_raw', pal.detail,
			'balance_deducted', pal.balance_deducted,
			'refund_amount', po.refund_amount
		) AS metadata,
		TRUE AS affects_balance,
		FALSE AS is_recharge,
		'high'::text AS confidence
	FROM (
		SELECT
			payment_audit_logs.*,
			((regexp_match(
				COALESCE(detail, ''),
				'"balanceDeducted"[[:space:]]*:[[:space:]]*"?(-?[0-9]+([.][0-9]+)?)"?'
			))[1])::numeric AS balance_deducted
		FROM payment_audit_logs
		WHERE action = 'REFUND_SUCCESS'
	) pal
	JOIN payment_orders po ON po.id::text = pal.order_id
	WHERE po.user_id = $1
	  AND po.order_type = 'balance'
	  AND COALESCE(pal.balance_deducted, po.refund_amount, 0) <> 0
),
filtered AS (
	SELECT *
	FROM flow_union
	WHERE (
		($2 = '' AND affects_balance = TRUE)
		OR ($2 <> '' AND (type = $2 OR source_type = $2))
	)
)
`

const adminBalanceTransactionFlowCTE = `
WITH filtered AS (
	SELECT
		('balance_transaction:' || bt.id::text) AS flow_id,
		bt.id AS sort_id,
		bt.source_type::text AS type,
		'balance_transactions'::text AS source_type,
		bt.source_id::text AS source_id,
		bt.balance_delta::double precision AS amount,
		bt.balance_delta::double precision AS balance_delta,
		bt.frozen_delta::double precision AS frozen_delta,
		bt.balance_before::double precision AS balance_before,
		bt.balance_after::double precision AS balance_after,
		bt.frozen_before::double precision AS frozen_before,
		bt.frozen_after::double precision AS frozen_after,
		bt.created_at AS occurred_at,
		bt.description::text AS description,
		bt.actor_type::text AS actor_type,
		bt.actor_user_id AS actor_user_id,
		bt.source_type::text AS related_object_type,
		bt.source_id::text AS related_object_id,
		bt.idempotency_key::text AS reference,
		COALESCE(bt.metadata->>'notes', '')::text AS notes,
		bt.metadata AS metadata,
		TRUE AS affects_balance,
		bt.source_type IN ('payment_recharge', 'balance', 'admin_balance') AND bt.balance_delta > 0 AS is_recharge,
		bt.confidence::text AS confidence
	FROM balance_transactions bt
	WHERE bt.user_id = $1
	  AND ($2 = '' OR bt.source_type = $2)
)
`

func (s *adminServiceImpl) GetUserBalanceHistory(ctx context.Context, userID int64, page, pageSize int, flowType string) (*AdminBalanceFlowHistory, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "user_id must be greater than 0")
	}
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	flowType = strings.TrimSpace(flowType)

	flowCTE, err := s.adminBalanceFlowQuerySource(ctx, userID, flowType)
	if err != nil {
		return nil, err
	}
	summary, total, err := s.getAdminBalanceFlowSummary(ctx, userID, flowType, flowCTE)
	if err != nil {
		return nil, err
	}
	items, err := s.listAdminBalanceFlowItems(ctx, userID, flowType, params, flowCTE)
	if err != nil {
		return nil, err
	}
	pages := int((total + int64(params.Limit()) - 1) / int64(params.Limit()))
	if pages < 1 {
		pages = 1
	}
	return &AdminBalanceFlowHistory{
		Items:    items,
		Total:    total,
		Page:     params.Page,
		PageSize: params.Limit(),
		Pages:    pages,
		Summary:  summary,
	}, nil
}

func (s *adminServiceImpl) GetUserBalanceReconciliation(ctx context.Context, userID int64) (*AdminBalanceReconciliation, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "user_id must be greater than 0")
	}
	flowCTE, err := s.adminBalanceFlowQuerySource(ctx, userID, "")
	if err != nil {
		return nil, err
	}
	summary, _, err := s.getAdminBalanceFlowSummary(ctx, userID, "", flowCTE)
	if err != nil {
		return nil, err
	}
	ledgerBalance, ledgerFrozen, ledgerExists, err := s.sumBalanceTransactions(ctx, userID)
	if err != nil {
		return nil, err
	}
	recent, err := s.listAdminBalanceFlowItems(ctx, userID, "", pagination.PaginationParams{Page: 1, PageSize: 20}, flowCTE)
	if err != nil {
		return nil, err
	}
	if !ledgerExists {
		ledgerBalance = summary.NetDelta
		ledgerFrozen = 0
	}
	warnings := make([]string, 0, 2)
	if !ledgerExists {
		warnings = append(warnings, "balance_transactions is not available yet; reconciliation uses legacy source aggregation")
	}
	if absFloat(summary.CurrentBalance-ledgerBalance) > 0.000001 {
		warnings = append(warnings, "current balance does not match aggregated ledger total")
	}
	if absFloat(summary.FrozenBalance-ledgerFrozen) > 0.000001 {
		warnings = append(warnings, "current frozen balance does not match aggregated ledger total")
	}
	return &AdminBalanceReconciliation{
		CurrentBalance:    summary.CurrentBalance,
		CurrentFrozen:     summary.FrozenBalance,
		LedgerBalanceSum:  ledgerBalance,
		LedgerFrozenSum:   ledgerFrozen,
		BalanceDifference: summary.CurrentBalance - ledgerBalance,
		FrozenDifference:  summary.FrozenBalance - ledgerFrozen,
		Recent:            recent,
		Warnings:          warnings,
	}, nil
}

func (s *adminServiceImpl) adminBalanceFlowQuerySource(ctx context.Context, userID int64, flowType string) (string, error) {
	if s == nil || s.entClient == nil {
		return adminBalanceFlowCTE, nil
	}
	rows, err := s.entClient.QueryContext(ctx, `SELECT to_regclass('public.balance_transactions') IS NOT NULL`)
	if err != nil {
		return "", err
	}
	var exists bool
	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			_ = rows.Close()
			return "", err
		}
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	if !exists {
		return adminBalanceFlowCTE, nil
	}

	rows, err = s.entClient.QueryContext(ctx, `
SELECT EXISTS (
	SELECT 1
	FROM balance_transactions
	WHERE user_id = $1
	  AND ($2 = '' OR source_type = $2)
)`, userID, flowType)
	if err != nil {
		return "", err
	}
	var hasRows bool
	if rows.Next() {
		if err := rows.Scan(&hasRows); err != nil {
			_ = rows.Close()
			return "", err
		}
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	if hasRows {
		return adminBalanceTransactionFlowCTE, nil
	}
	return adminBalanceFlowCTE, nil
}

func (s *adminServiceImpl) getAdminBalanceFlowSummary(ctx context.Context, userID int64, flowType string, flowCTE string) (AdminBalanceFlowSummary, int64, error) {
	if s == nil || s.entClient == nil {
		return AdminBalanceFlowSummary{}, 0, nil
	}
	currentBalance, frozenBalance, err := loadAdminUserBalanceSnapshot(ctx, s.entClient, userID)
	if err != nil {
		return AdminBalanceFlowSummary{}, 0, err
	}
	query := flowCTE + `
SELECT
	COUNT(*)::bigint,
	COALESCE(SUM(CASE WHEN balance_delta > 0 THEN balance_delta ELSE 0 END), 0)::double precision,
	COALESCE(SUM(CASE WHEN balance_delta < 0 THEN -balance_delta ELSE 0 END), 0)::double precision,
	COALESCE(SUM(balance_delta), 0)::double precision,
	COALESCE(SUM(CASE WHEN is_recharge AND balance_delta > 0 THEN balance_delta ELSE 0 END), 0)::double precision
FROM filtered`
	rows, err := s.entClient.QueryContext(ctx, query, userID, flowType)
	if err != nil {
		return AdminBalanceFlowSummary{}, 0, err
	}
	defer func() { _ = rows.Close() }()

	var total int64
	summary := AdminBalanceFlowSummary{
		CurrentBalance: currentBalance,
		FrozenBalance:  frozenBalance,
	}
	if rows.Next() {
		if err := rows.Scan(&total, &summary.TotalIn, &summary.TotalOut, &summary.NetDelta, &summary.RechargeTotal); err != nil {
			return AdminBalanceFlowSummary{}, 0, err
		}
	}
	if err := rows.Err(); err != nil {
		return AdminBalanceFlowSummary{}, 0, err
	}
	return summary, total, nil
}

func (s *adminServiceImpl) listAdminBalanceFlowItems(ctx context.Context, userID int64, flowType string, params pagination.PaginationParams, flowCTE string) ([]AdminBalanceFlowItem, error) {
	if s == nil || s.entClient == nil {
		return []AdminBalanceFlowItem{}, nil
	}
	query := flowCTE + `
SELECT
	flow_id,
	type,
	source_type,
	source_id,
	amount,
	balance_delta,
	frozen_delta,
	balance_before,
	balance_after,
	frozen_before,
	frozen_after,
	occurred_at,
	description,
	actor_type,
	actor_user_id,
	related_object_type,
	related_object_id,
	reference,
	notes,
	metadata::text,
	confidence
FROM filtered
ORDER BY occurred_at DESC, sort_id DESC
OFFSET $3
LIMIT $4`
	rows, err := s.entClient.QueryContext(ctx, query, userID, flowType, params.Offset(), params.Limit())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]AdminBalanceFlowItem, 0, params.Limit())
	for rows.Next() {
		item, err := scanAdminBalanceFlowItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func scanAdminBalanceFlowItem(rows *sql.Rows) (AdminBalanceFlowItem, error) {
	var item AdminBalanceFlowItem
	var (
		sourceID, description, actorType, relatedType, relatedID, reference, notes, metadataRaw, confidence sql.NullString
		balanceBefore, balanceAfter, frozenBefore, frozenAfter                                              sql.NullFloat64
		actorUserID                                                                                         sql.NullInt64
	)
	if err := rows.Scan(
		&item.ID,
		&item.Type,
		&item.SourceType,
		&sourceID,
		&item.Amount,
		&item.BalanceDelta,
		&item.FrozenDelta,
		&balanceBefore,
		&balanceAfter,
		&frozenBefore,
		&frozenAfter,
		&item.OccurredAt,
		&description,
		&actorType,
		&actorUserID,
		&relatedType,
		&relatedID,
		&reference,
		&notes,
		&metadataRaw,
		&confidence,
	); err != nil {
		return AdminBalanceFlowItem{}, err
	}
	item.SourceID = stringOrEmpty(sourceID)
	item.Description = stringOrEmpty(description)
	item.ActorType = stringOrDefault(actorType, "system")
	item.ActorUserID = int64PtrFromNull(actorUserID)
	item.RelatedObjectType = stringOrEmpty(relatedType)
	item.RelatedObjectID = stringOrEmpty(relatedID)
	item.Reference = stringOrEmpty(reference)
	item.Notes = stringOrEmpty(notes)
	item.Confidence = stringOrDefault(confidence, "high")
	item.BalanceBefore = float64PtrFromNull(balanceBefore)
	item.BalanceAfter = float64PtrFromNull(balanceAfter)
	item.FrozenBefore = float64PtrFromNull(frozenBefore)
	item.FrozenAfter = float64PtrFromNull(frozenAfter)
	if metadataRaw.Valid && strings.TrimSpace(metadataRaw.String) != "" {
		var meta map[string]any
		if err := json.Unmarshal([]byte(metadataRaw.String), &meta); err == nil {
			item.Metadata = meta
		}
	}
	return item, nil
}

func loadAdminUserBalanceSnapshot(ctx context.Context, client *dbent.Client, userID int64) (float64, float64, error) {
	rows, err := client.QueryContext(ctx, `
SELECT balance::double precision, COALESCE(frozen_balance, 0)::double precision
FROM users
WHERE id = $1`, userID)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, err
		}
		return 0, 0, ErrUserNotFound
	}
	var balance, frozen float64
	if err := rows.Scan(&balance, &frozen); err != nil {
		return 0, 0, err
	}
	return balance, frozen, rows.Err()
}

func (s *adminServiceImpl) sumBalanceTransactions(ctx context.Context, userID int64) (float64, float64, bool, error) {
	if s == nil || s.entClient == nil {
		return 0, 0, false, nil
	}
	rows, err := s.entClient.QueryContext(ctx, `
SELECT to_regclass('public.balance_transactions') IS NOT NULL`)
	if err != nil {
		return 0, 0, false, err
	}
	var exists bool
	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			_ = rows.Close()
			return 0, 0, false, err
		}
	}
	if err := rows.Close(); err != nil {
		return 0, 0, false, err
	}
	if !exists {
		return 0, 0, false, nil
	}
	rows, err = s.entClient.QueryContext(ctx, `
SELECT
	COALESCE(SUM(balance_delta), 0)::double precision,
	COALESCE(SUM(frozen_delta), 0)::double precision
FROM balance_transactions
WHERE user_id = $1`, userID)
	if err != nil {
		return 0, 0, true, err
	}
	defer func() { _ = rows.Close() }()
	var balance, frozen float64
	if rows.Next() {
		if err := rows.Scan(&balance, &frozen); err != nil {
			return 0, 0, true, err
		}
	}
	return balance, frozen, true, rows.Err()
}

func stringOrEmpty(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func stringOrDefault(value sql.NullString, fallback string) string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return fallback
	}
	return value.String
}

func int64PtrFromNull(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func float64PtrFromNull(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
