package service

import (
	"context"
	"encoding/json"
	"time"
)

type subscriptionEntitlementBefore struct {
	ID             int64     `json:"id"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expires_at"`
	QuotaAvailable bool      `json:"quota_available"`
}

// The caller already holds the subscription lock. Bulk entitlement locks are
// acquired in ID order; their before-state survives revoke and later Restore.
func (s *SubscriptionService) recordSubscriptionLifecycle(ctx context.Context, sub *UserSubscription, kind string) error {
	if s.entClient == nil {
		return nil
	}
	runner := s.packageQuotaRunner(ctx)
	rows, err := runner.QueryContext(ctx, `SELECT id,status,expires_at,
 (request_limit IS NULL OR request_used<request_limit) AND (amount_limit_usd IS NULL OR amount_used_usd<amount_limit_usd)
 AND (token_limit IS NULL OR token_used<token_limit)
 FROM subscription_package_entitlements WHERE user_id=$1 AND group_id=$2 ORDER BY id FOR UPDATE`, sub.UserID, sub.GroupID)
	if err != nil {
		return err
	}
	states := make([]subscriptionEntitlementBefore, 0)
	for rows.Next() {
		var e subscriptionEntitlementBefore
		if err = rows.Scan(&e.ID, &e.Status, &e.ExpiresAt, &e.QuotaAvailable); err != nil {
			_ = rows.Close()
			return err
		}
		states = append(states, e)
	}
	iterationErr := rows.Err()
	closeErr := rows.Close()
	if iterationErr != nil {
		return iterationErr
	}
	if closeErr != nil {
		return closeErr
	}
	payload, err := json.Marshal(states)
	if err != nil {
		return err
	}
	windowSub, group, now, err := subscriptionWindowSnapshot(ctx, runner, sub.ID)
	if err != nil {
		return err
	}
	windowAvailable := subscriptionEffectiveQuotaAvailable(windowSub, group, now)
	_, err = runner.ExecContext(ctx, `INSERT INTO gateway_subscription_events(subscription_id,user_id,group_id,event_order,kind,previous_status,previous_expires_at,entitlement_states,previous_window_quota_available)
 VALUES($1,$2,$3,nextval('gateway_subscription_order_seq'),$4,$5,$6,$7::jsonb,$8)`, sub.ID, sub.UserID, sub.GroupID, kind, sub.Status, sub.ExpiresAt, string(payload), windowAvailable)
	return err
}

func (s *SubscriptionService) lockSubscriptionIncludingDeleted(ctx context.Context, id int64) error {
	if s.entClient == nil {
		return nil
	}
	rows, err := s.packageQuotaRunner(ctx).QueryContext(ctx, `SELECT id FROM user_subscriptions WHERE id=$1 FOR UPDATE`, id)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return ErrSubscriptionNotFound
	}
	return nil
}
