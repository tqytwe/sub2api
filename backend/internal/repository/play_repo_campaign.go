package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) ListActiveCampaigns(ctx context.Context, now time.Time) ([]service.PlayCampaign, error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT id, name, start_at, end_at, rules_json::text, audience_json::text, enabled, created_at
		FROM play_campaigns
		WHERE enabled = TRUE AND start_at <= $1 AND end_at > $1
		ORDER BY start_at ASC, id ASC`, now)
	if err != nil {
		return nil, fmt.Errorf("list active play campaigns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.PlayCampaign, 0)
	for rows.Next() {
		var item service.PlayCampaign
		var rulesRaw, audienceRaw string
		if err := rows.Scan(&item.ID, &item.Name, &item.StartAt, &item.EndAt, &rulesRaw, &audienceRaw, &item.Enabled, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan play campaign: %w", err)
		}
		item.Rules = service.ParsePlayCampaignRules(rulesRaw)
		item.Audience = service.ParsePlayCampaignAudience(audienceRaw)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active play campaigns rows: %w", err)
	}
	return out, nil
}

func (r *playRepository) ListAdminCampaigns(ctx context.Context) ([]service.PlayCampaign, error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT id, name, start_at, end_at, rules_json::text, audience_json::text, enabled, created_at
		FROM play_campaigns
		ORDER BY enabled DESC, start_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list admin play campaigns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.PlayCampaign, 0)
	for rows.Next() {
		item, err := scanPlayCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list admin play campaigns rows: %w", err)
	}
	return out, nil
}

func (r *playRepository) CreateAdminCampaign(ctx context.Context, campaign service.PlayCampaign) (*service.PlayCampaign, error) {
	rulesJSON, err := json.Marshal(campaign.Rules)
	if err != nil {
		return nil, fmt.Errorf("marshal play campaign rules: %w", err)
	}
	audienceJSON, err := json.Marshal(campaign.Audience)
	if err != nil {
		return nil, fmt.Errorf("marshal play campaign audience: %w", err)
	}

	var item service.PlayCampaign
	var rulesRaw, audienceRaw string
	err = scanSingleRow(ctx, r.sqlExec(ctx), `
		INSERT INTO play_campaigns (name, start_at, end_at, rules_json, audience_json, enabled)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6)
		RETURNING id, name, start_at, end_at, rules_json::text, audience_json::text, enabled, created_at`,
		[]any{campaign.Name, campaign.StartAt, campaign.EndAt, string(rulesJSON), string(audienceJSON), campaign.Enabled},
		&item.ID, &item.Name, &item.StartAt, &item.EndAt, &rulesRaw, &audienceRaw, &item.Enabled, &item.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create admin play campaign: %w", err)
	}
	item.Rules = service.ParsePlayCampaignRules(rulesRaw)
	item.Audience = service.ParsePlayCampaignAudience(audienceRaw)
	return &item, nil
}

func (r *playRepository) UpdateAdminCampaign(ctx context.Context, campaign service.PlayCampaign) (*service.PlayCampaign, error) {
	rulesJSON, err := json.Marshal(campaign.Rules)
	if err != nil {
		return nil, fmt.Errorf("marshal play campaign rules: %w", err)
	}
	audienceJSON, err := json.Marshal(campaign.Audience)
	if err != nil {
		return nil, fmt.Errorf("marshal play campaign audience: %w", err)
	}

	var item service.PlayCampaign
	var rulesRaw, audienceRaw string
	err = scanSingleRow(ctx, r.sqlExec(ctx), `
		UPDATE play_campaigns
		SET name = $2, start_at = $3, end_at = $4, rules_json = $5::jsonb, audience_json = $6::jsonb, enabled = $7
		WHERE id = $1
		RETURNING id, name, start_at, end_at, rules_json::text, audience_json::text, enabled, created_at`,
		[]any{campaign.ID, campaign.Name, campaign.StartAt, campaign.EndAt, string(rulesJSON), string(audienceJSON), campaign.Enabled},
		&item.ID, &item.Name, &item.StartAt, &item.EndAt, &rulesRaw, &audienceRaw, &item.Enabled, &item.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("update admin play campaign: %w", err)
	}
	item.Rules = service.ParsePlayCampaignRules(rulesRaw)
	item.Audience = service.ParsePlayCampaignAudience(audienceRaw)
	return &item, nil
}

func (r *playRepository) DeleteAdminCampaign(ctx context.Context, id int64) error {
	res, err := r.sqlExec(ctx).ExecContext(ctx, `DELETE FROM play_campaigns WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete admin play campaign: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete admin play campaign rows affected: %w", err)
	}
	if n == 0 {
		return infraerrors.NotFound("PLAY_CAMPAIGN_NOT_FOUND", "campaign not found")
	}
	return nil
}

// ReconcileNewUserGrowthCampaign creates one claimable reward per reached
// milestone. The linked referral campaign remains the source of truth for the
// shared reward budget and wallet freeze/claim lifecycle.
func (r *playRepository) ReconcileNewUserGrowthCampaign(ctx context.Context, campaign service.PlayCampaign, userID int64, now time.Time) error {
	if userID <= 0 || campaign.Rules.ReferralCampaignID <= 0 {
		return nil
	}
	parent, err := r.newUserGrowthParent(ctx, campaign.Rules.ReferralCampaignID, userID)
	if err != nil {
		return err
	}
	metric, fundingConflict, err := r.newUserGrowthMetric(ctx, campaign, userID, now)
	if err != nil {
		return err
	}
	if !parent.Eligible || parent.LegacyRebatePolicy != campaign.Rules.LegacyRebatePolicy || fundingConflict {
		metric = 0
	}

	if campaign.Enabled {
		for _, tier := range campaign.Rules.RewardTiers {
			if metric+1e-9 < tier.RequiredAmount {
				continue
			}
			if err := r.ensureNewUserGrowthReward(ctx, parent, campaign, userID, tier, now); err != nil {
				return err
			}
		}
	}
	return r.revokeUnqualifiedNewUserGrowthRewards(ctx, parent, campaign, userID, metric)
}

// ListNewUserGrowthCampaignsForUser includes already rewarded campaigns after
// their public end time so a later refund can still revoke the correct tier.
func (r *playRepository) ListNewUserGrowthCampaignsForUser(ctx context.Context, userID int64, now time.Time) ([]service.PlayCampaign, error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
SELECT p.id,p.name,p.start_at,p.end_at,p.rules_json::text,p.audience_json::text,p.enabled,p.created_at
FROM play_campaigns p
JOIN referral_campaign_attributions a
  ON a.campaign_id=(p.rules_json->>'referral_campaign_id')::bigint
 AND a.invitee_id=$1
WHERE p.start_at <= $2
  AND p.rules_json->>'campaign_type' IN ('new_user_growth','hybrid')
  AND (p.enabled=TRUE OR EXISTS (
    SELECT 1 FROM play_campaign_reward_snapshots s
    JOIN referral_campaign_rewards r ON r.id=s.reward_id
    WHERE s.play_campaign_id=p.id AND r.user_id=$1
  ))
  AND (p.end_at > $2 OR a.qualification_to_snapshot > $2 OR EXISTS (
    SELECT 1 FROM play_campaign_reward_snapshots s
    JOIN referral_campaign_rewards r ON r.id=s.reward_id
    WHERE s.play_campaign_id=p.id AND r.user_id=$1
  ))
ORDER BY p.start_at ASC,p.id ASC`, userID, now)
	if err != nil {
		return nil, fmt.Errorf("list new user growth campaigns: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PlayCampaign, 0)
	for rows.Next() {
		item, scanErr := scanPlayCampaign(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list new user growth campaign rows: %w", err)
	}
	return out, nil
}

func (r *playRepository) GetReferralCampaignLegacyRebatePolicy(ctx context.Context, campaignID int64) (string, error) {
	var policy string
	if err := scanSingleRow(ctx, r.sqlExec(ctx), `SELECT legacy_rebate_policy FROM referral_campaigns WHERE id=$1`, []any{campaignID}, &policy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", infraerrors.NotFound("REFERRAL_CAMPAIGN_NOT_FOUND", "referral campaign not found")
		}
		return "", fmt.Errorf("load referral campaign rebate policy: %w", err)
	}
	return policy, nil
}

func (r *playRepository) GetNewUserGrowthProgress(ctx context.Context, campaign service.PlayCampaign, userID int64, now time.Time) (service.PlayNewUserGrowthProgress, error) {
	progress := service.PlayNewUserGrowthProgress{ReferralCampaignID: campaign.Rules.ReferralCampaignID, QualificationMetric: campaign.Rules.QualificationMetric, Rewards: make([]service.PlayNewUserGrowthRewardProgress, 0, len(campaign.Rules.RewardTiers))}
	parent, err := r.newUserGrowthParent(ctx, campaign.Rules.ReferralCampaignID, userID)
	if err != nil {
		return progress, err
	}
	progress.Eligible = parent.Eligible && parent.LegacyRebatePolicy == campaign.Rules.LegacyRebatePolicy
	progress.ReferralVersion = parent.CampaignVersion
	metric, fundingConflict, err := r.newUserGrowthMetric(ctx, campaign, userID, now)
	if err != nil {
		return progress, err
	}
	progress.FundingConflict = fundingConflict
	if !progress.Eligible {
		metric = 0
	}
	if fundingConflict {
		metric = 0
	}
	progress.QualifiedAmount = metric

	type rewardRow struct {
		id     int64
		tier   int
		status string
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
SELECT r.id,r.tier_no,r.status
FROM play_campaign_reward_snapshots s
JOIN referral_campaign_rewards r ON r.id=s.reward_id
WHERE s.play_campaign_id=$1 AND r.user_id=$2 AND r.reward_type='new_user_tier'`, campaign.ID, userID)
	if err != nil {
		return progress, fmt.Errorf("list new user growth rewards: %w", err)
	}
	defer func() { _ = rows.Close() }()
	rewards := map[int]rewardRow{}
	for rows.Next() {
		var row rewardRow
		if scanErr := rows.Scan(&row.id, &row.tier, &row.status); scanErr != nil {
			return progress, scanErr
		}
		rewards[row.tier] = row
	}
	if err := rows.Err(); err != nil {
		return progress, err
	}
	for _, tier := range campaign.Rules.RewardTiers {
		item := service.PlayNewUserGrowthRewardProgress{Tier: tier.Tier, RequiredAmount: tier.RequiredAmount, RewardAmount: tier.RewardAmount, Currency: strings.ToUpper(strings.TrimSpace(tier.Currency))}
		if reward, ok := rewards[tier.Tier]; ok {
			item.RewardID, item.Status = reward.id, reward.status
		}
		progress.Rewards = append(progress.Rewards, item)
	}
	return progress, nil
}

type newUserGrowthParent struct {
	CampaignID         int64
	Eligible           bool
	ClaimDeadline      time.Time
	RiskHoldHours      int
	RulesVersion       int64
	CampaignVersion    int64
	LegacyRebatePolicy string
}

func (r *playRepository) newUserGrowthParent(ctx context.Context, campaignID, userID int64) (newUserGrowthParent, error) {
	var out newUserGrowthParent
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
SELECT c.id,
       a.status IN ('pending','approved') AND c.status IN ('scheduled','running','settling','closed'),
       c.claim_deadline,c.risk_hold_hours,a.rules_version,c.version,a.legacy_rebate_policy
FROM referral_campaigns c
JOIN referral_campaign_attributions a ON a.campaign_id=c.id AND a.invitee_id=$2
WHERE c.id=$1`, []any{campaignID, userID},
		&out.CampaignID, &out.Eligible, &out.ClaimDeadline, &out.RiskHoldHours, &out.RulesVersion, &out.CampaignVersion, &out.LegacyRebatePolicy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, nil
		}
		return out, fmt.Errorf("load linked referral campaign for new user growth: %w", err)
	}
	return out, nil
}

func (r *playRepository) newUserGrowthMetric(ctx context.Context, campaign service.PlayCampaign, userID int64, now time.Time) (float64, bool, error) {
	start := campaign.StartAt
	if !now.After(start) {
		return 0, false, nil
	}
	end := now
	var metric float64
	var fundingConflict bool
	var query string
	if campaign.Rules.QualificationMetric == service.PlayCampaignMetricConsumption {
		query = `
WITH attribution AS (
  SELECT invitee_id, registered_at, qualification_to_snapshot
  FROM referral_campaign_attributions
  WHERE campaign_id=$1 AND invitee_id=$2
)
SELECT
  COALESCE((
    SELECT SUM(u.actual_cost)
    FROM usage_logs u
    JOIN attribution a ON a.invitee_id=u.user_id
    WHERE u.created_at>=GREATEST(a.registered_at,$3)
      AND u.created_at<LEAST($4,a.qualification_to_snapshot)
  ),0)::double precision,
  EXISTS (
    SELECT 1
    FROM balance_transactions bt
    JOIN attribution a ON a.invitee_id=bt.user_id
    WHERE bt.balance_delta > 0
      AND bt.created_at >= GREATEST(a.registered_at,$3)
      AND bt.created_at < LEAST($4,a.qualification_to_snapshot)
      AND bt.source_type <> 'payment_recharge'
      AND NOT (
        bt.source_type IN ('image_balance_capture','image_balance_release','reversal')
        AND (
          bt.metadata ? 'restore_ledger_key'
          OR bt.metadata ? 'ledger_deduct_key'
          OR bt.metadata ? 'reverses_idempotency_key'
        )
      )
  ) AS funding_conflict`
	} else {
		query = `SELECT COALESCE(SUM(m.net_amount),0)::double precision FROM play_membership_order_contributions m JOIN referral_campaign_attributions a ON a.campaign_id=$1 AND a.invitee_id=m.user_id WHERE m.user_id=$2 AND m.qualification_state='verified' AND m.paid_at>=GREATEST(a.registered_at,$3) AND m.paid_at<LEAST($4,a.qualification_to_snapshot)`
	}
	destinations := []any{&metric}
	if campaign.Rules.QualificationMetric == service.PlayCampaignMetricConsumption {
		destinations = append(destinations, &fundingConflict)
	}
	if err := scanSingleRow(ctx, r.sqlExec(ctx), query, []any{campaign.Rules.ReferralCampaignID, userID, start, end}, destinations...); err != nil {
		return 0, false, fmt.Errorf("calculate new user growth metric: %w", err)
	}
	return metric, fundingConflict, nil
}

func (r *playRepository) ensureNewUserGrowthReward(ctx context.Context, parent newUserGrowthParent, campaign service.PlayCampaign, userID int64, tier service.PlayCampaignRewardTier, now time.Time) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin new user growth reward transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	exec := sqlExecutorFromEntClient(tx.Client())
	idempotency := fmt.Sprintf("new-user-growth:%d:%d:%d", campaign.ID, userID, tier.Tier)
	if !now.Before(parent.ClaimDeadline) {
		return nil
	}
	var lockedCampaignID int64
	if err := scanSingleRow(txCtx, exec, `SELECT id FROM referral_campaigns WHERE id=$1 FOR UPDATE`, []any{parentCampaignID(parent)}, &lockedCampaignID); err != nil {
		return err
	}
	var existing int64
	err = scanSingleRow(txCtx, exec, `SELECT id FROM referral_campaign_rewards WHERE campaign_id=$1 AND user_id=$2 AND reward_type='new_user_tier' AND tier_no=$3 AND generation=1 FOR UPDATE`, []any{parentCampaignID(parent), userID, tier.Tier}, &existing)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var rewardID int64
	claimDeadline := parent.ClaimDeadline
	if claimDeadline.IsZero() {
		claimDeadline = campaign.EndAt
	}
	res, err := exec.ExecContext(txCtx, `UPDATE referral_campaigns SET budget_reserved=budget_reserved+$1,updated_at=NOW() WHERE id=$2 AND budget_total >= budget_reserved+budget_paid+$1`, tier.RewardAmount, parentCampaignID(parent))
	if err != nil {
		return fmt.Errorf("reserve new user growth reward: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrPlayCampaignRewardBudgetExceeded
	}
	err = scanSingleRow(txCtx, exec, `INSERT INTO referral_campaign_rewards (campaign_id,user_id,tier_no,reward_type,amount,currency,status,qualification_id,idempotency_key,claim_deadline,generation) VALUES ($1,$2,$3,'new_user_tier',$4,$5,'claimable',NULL,$6,$7,1) ON CONFLICT (idempotency_key) DO NOTHING RETURNING id`, []any{parentCampaignID(parent), userID, tier.Tier, tier.RewardAmount, strings.ToUpper(strings.TrimSpace(tier.Currency)), idempotency, claimDeadline}, &rewardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("create new user growth reward: %w", err)
	}
	if _, err := exec.ExecContext(txCtx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key,metadata) VALUES ($1,$2,'reserve',$3,$4,$5::jsonb) ON CONFLICT DO NOTHING`, parentCampaignID(parent), rewardID, tier.RewardAmount, idempotency+":reserve", fmt.Sprintf(`{"source":"new_user_growth","play_campaign_id":%d,"risk_hold_hours":%d}`, campaign.ID, parent.RiskHoldHours)); err != nil {
		return fmt.Errorf("record new user growth budget reserve: %w", err)
	}
	rulesSnapshot, err := json.Marshal(campaign.Rules)
	if err != nil {
		return fmt.Errorf("marshal new user growth rules snapshot: %w", err)
	}
	if _, err := exec.ExecContext(txCtx, `INSERT INTO play_campaign_reward_snapshots (reward_id,play_campaign_id,referral_campaign_id,referral_rules_version,qualification_metric,qualified_amount,rules_snapshot) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)`, rewardID, campaign.ID, parent.CampaignID, parent.RulesVersion, campaign.Rules.QualificationMetric, tier.RequiredAmount, string(rulesSnapshot)); err != nil {
		return fmt.Errorf("record new user growth reward snapshot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit new user growth reward: %w", err)
	}
	return nil
}

func (r *playRepository) revokeUnqualifiedNewUserGrowthRewards(ctx context.Context, parent newUserGrowthParent, campaign service.PlayCampaign, userID int64, metric float64) error {
	if parent.CampaignID <= 0 {
		return nil
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin new user growth revoke transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	exec := sqlExecutorFromEntClient(tx.Client())
	rows, err := exec.QueryContext(txCtx, `
SELECT r.id,r.amount::double precision,r.status,
       CASE WHEN l.frozen_until IS NOT NULL THEN 'frozen' ELSE 'available' END
FROM play_campaign_reward_snapshots s
JOIN referral_campaign_rewards r ON r.id=s.reward_id
LEFT JOIN user_affiliate_ledger l ON l.referral_reward_id=r.id AND l.action='accrue'
WHERE s.play_campaign_id=$1 AND r.user_id=$2 AND r.reward_type='new_user_tier'
  AND r.status IN ('claimable','claimed_frozen','available')
  AND COALESCE((
    SELECT (item->>'required_amount')::numeric
    FROM jsonb_array_elements(s.rules_snapshot->'reward_tiers') item
    WHERE (item->>'tier')::integer=r.tier_no
  ),0) > $3
FOR UPDATE OF r`, campaign.ID, userID, metric)
	if err != nil {
		return fmt.Errorf("list unqualified new user growth rewards: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var rewardID int64
		var amount float64
		var status, bucket string
		if err := rows.Scan(&rewardID, &amount, &status, &bucket); err != nil {
			return err
		}
		if err := revokeNewUserGrowthReward(txCtx, exec, parent.CampaignID, userID, rewardID, amount, status, bucket); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit new user growth revoke transaction: %w", err)
	}
	return nil
}

func revokeNewUserGrowthReward(ctx context.Context, exec sqlQueryExecutor, campaignID, userID, rewardID int64, amount float64, status, bucket string) error {
	if status == service.ReferralRewardStatusClaimable {
		res, err := exec.ExecContext(ctx, `UPDATE referral_campaign_rewards SET status='revoked',revoked_at=NOW(),updated_at=NOW(),version=version+1 WHERE id=$1 AND status='claimable'`, rewardID)
		if err != nil {
			return err
		}
		if changed, _ := res.RowsAffected(); changed == 0 {
			return nil
		}
		if _, err := exec.ExecContext(ctx, `UPDATE referral_campaigns SET budget_reserved=GREATEST(budget_reserved-$1,0),updated_at=NOW() WHERE id=$2`, amount, campaignID); err != nil {
			return err
		}
		_, err = exec.ExecContext(ctx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key,metadata) VALUES ($1,$2,'release',$3,$4,$5::jsonb) ON CONFLICT DO NOTHING`, campaignID, rewardID, amount, fmt.Sprintf("new-user-growth:%d:release", rewardID), `{"source":"new_user_growth","reason":"metric_below_tier"}`)
		return err
	}
	column := "aff_quota"
	if bucket == "frozen" {
		column = "aff_frozen_quota"
	}
	res, err := exec.ExecContext(ctx, `UPDATE user_affiliates SET `+column+`=`+column+`-$1,updated_at=NOW() WHERE user_id=$2 AND `+column+` >= $1`, amount, userID)
	if err != nil {
		return err
	}
	if recovered, _ := res.RowsAffected(); recovered == 0 {
		_, err = exec.ExecContext(ctx, `UPDATE referral_campaign_rewards SET status='debt_review',revoked_at=NOW(),updated_at=NOW(),version=version+1 WHERE id=$1 AND status IN ('claimed_frozen','available')`, rewardID)
		return err
	}
	if _, err := exec.ExecContext(ctx, `UPDATE referral_campaign_rewards SET status='revoked',revoked_at=NOW(),updated_at=NOW(),version=version+1 WHERE id=$1 AND status IN ('claimed_frozen','available')`, rewardID); err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, `UPDATE user_affiliate_ledger SET frozen_until=NULL,updated_at=NOW() WHERE referral_reward_id=$1 AND action='accrue'`, rewardID); err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, `INSERT INTO user_affiliate_ledger (user_id,action,amount,referral_reward_id,created_at,updated_at) VALUES ($1,'revoke',$2,$3,NOW(),NOW())`, userID, -amount, rewardID); err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, `UPDATE referral_campaigns SET budget_paid=GREATEST(budget_paid-$1,0),updated_at=NOW() WHERE id=$2`, amount, campaignID); err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, `INSERT INTO referral_campaign_budget_ledger (campaign_id,reward_id,action,amount,idempotency_key,metadata) VALUES ($1,$2,'reverse',$3,$4,$5::jsonb) ON CONFLICT DO NOTHING`, campaignID, rewardID, amount, fmt.Sprintf("new-user-growth:%d:reverse", rewardID), `{"source":"new_user_growth","reason":"metric_below_tier"}`)
	return err
}

func parentCampaignID(parent newUserGrowthParent) int64 { return parent.CampaignID }

type campaignRowScanner interface {
	Scan(dest ...any) error
}

func scanPlayCampaign(row campaignRowScanner) (service.PlayCampaign, error) {
	var item service.PlayCampaign
	var rulesRaw, audienceRaw string
	if err := row.Scan(&item.ID, &item.Name, &item.StartAt, &item.EndAt, &rulesRaw, &audienceRaw, &item.Enabled, &item.CreatedAt); err != nil {
		return item, fmt.Errorf("scan play campaign: %w", err)
	}
	item.Rules = service.ParsePlayCampaignRules(rulesRaw)
	item.Audience = service.ParsePlayCampaignAudience(audienceRaw)
	return item, nil
}
