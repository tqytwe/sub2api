package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type subscriptionSettlementProof struct {
	outbox  *requestledger.SubscriptionOutbox
	revoked bool
}
type subscriptionSettlementKey struct{}

func validateSubscriptionSettlement(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, newCharge bool) (*subscriptionSettlementProof, error) {
	if cmd.SubscriptionReceiptID == "" {
		return nil, nil
	}
	o, err := requestledger.LoadSubscriptionOutbox(ctx, tx, cmd.SubscriptionReceiptID)
	if err != nil {
		return nil, err
	}
	r := o.Receipt
	h := requestledger.FromContext(ctx)
	command, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	if h == nil || h.ID != r.RequestID || o.Command == "" || requestledger.SubscriptionDigest(string(command)) != o.CommandHash || cmd.BalanceCost != 0 ||
		cmd.UserID != r.UserID || cmd.APIKeyID != r.APIKeyID || cmd.AccountID != r.AccountID || cmd.RequestID != r.BillingRequestID || cmd.RequestPayloadHash != r.RequestPayloadHash ||
		cmd.SubscriptionID == nil || *cmd.SubscriptionID != r.SubscriptionID || cmd.APIKeyGroupID == nil || *cmd.APIKeyGroupID != r.GroupID ||
		!sameSubscriptionID(cmd.PackageEntitlementID, r.PackageEntitlementID) {
		return nil, service.ErrUsageBillingOwnershipMismatch
	}
	var deleted bool
	var status string
	var expiry time.Time
	err = tx.QueryRowContext(ctx, `SELECT deleted_at IS NOT NULL,status,expires_at FROM user_subscriptions
 WHERE id=$1 AND user_id=$2 AND group_id=$3 FOR UPDATE`, r.SubscriptionID, r.UserID, r.GroupID).Scan(&deleted, &status, &expiry)
	if err != nil {
		return nil, service.ErrUsageBillingOwnershipMismatch
	}
	entStatus := ""
	quotaAvailable := true
	var entExpiry time.Time
	if r.PackageEntitlementID != nil {
		err = tx.QueryRowContext(ctx, `SELECT status,expires_at,
 (request_limit IS NULL OR request_used<request_limit) AND (amount_limit_usd IS NULL OR amount_used_usd<amount_limit_usd) AND (token_limit IS NULL OR token_used<token_limit)
 FROM subscription_package_entitlements WHERE id=$1 AND user_id=$2 AND group_id=$3 FOR UPDATE`, *r.PackageEntitlementID, r.UserID, r.GroupID).Scan(&entStatus, &entExpiry, &quotaAvailable)
		if err != nil {
			return nil, service.ErrUsageBillingOwnershipMismatch
		}
	}
	p := &subscriptionSettlementProof{outbox: o, revoked: entStatus == "revoked"}
	if !newCharge {
		return p, nil
	} // Exact committed proof survives expiry/revoke.
	var now time.Time
	if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return nil, err
	}
	if status != "active" || !expiry.After(now) || !r.SubscriptionExpiresAt.After(now) {
		return nil, service.ErrSubscriptionExpired
	}
	if r.PackageEntitlementID != nil && (!quotaAvailable || !entExpiry.After(now) || !r.EntitlementExpiresAt.Valid || !r.EntitlementExpiresAt.Time.After(now)) {
		return nil, service.ErrPackageQuotaExhausted
	}
	if r.PackageEntitlementID == nil {
		if err := service.SubscriptionWindowLimitsTx(ctx, tx, r.SubscriptionID); err != nil {
			return nil, err
		}
	}
	var beforeWindowAvailable bool
	var beforeStatus string
	var beforeExpiry time.Time
	var states []byte
	// Select the relevant first revoke after this exact admission, not a later
	// Restore/revoke event that could hide an exhausted or expired original row.
	err = tx.QueryRowContext(ctx, `SELECT previous_status,previous_expires_at,entitlement_states,previous_window_quota_available FROM gateway_subscription_events
 WHERE subscription_id=$1 AND user_id=$2 AND group_id=$3 AND kind='revoke' AND event_order>$4 ORDER BY event_order LIMIT 1`, r.SubscriptionID, r.UserID, r.GroupID, r.Order).Scan(&beforeStatus, &beforeExpiry, &states, &beforeWindowAvailable)
	if errors.Is(err, sql.ErrNoRows) {
		if deleted || (r.PackageEntitlementID != nil && entStatus != "active") {
			return nil, service.ErrUsageBillingOwnershipMismatch
		}
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	if beforeStatus != "active" || !beforeExpiry.After(now) {
		return nil, service.ErrSubscriptionExpired
	}
	if r.PackageEntitlementID == nil && !beforeWindowAvailable {
		return nil, service.ErrDailyLimitExceeded
	}
	if r.PackageEntitlementID != nil {
		var prior []struct {
			ID             int64     `json:"id"`
			Status         string    `json:"status"`
			ExpiresAt      time.Time `json:"expires_at"`
			QuotaAvailable bool      `json:"quota_available"`
		}
		if json.Unmarshal(states, &prior) != nil {
			return nil, requestledger.ErrUnavailable
		}
		eligible := false
		for _, e := range prior {
			if e.ID == *r.PackageEntitlementID {
				eligible = e.Status == "active" && e.QuotaAvailable && e.ExpiresAt.After(now)
				break
			}
		}
		if !eligible || (entStatus != "active" && entStatus != "revoked") {
			return nil, service.ErrPackageQuotaExhausted
		}
	}
	return p, nil
}

func sameSubscriptionID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func finishSubscriptionSettlement(ctx context.Context, tx *sql.Tx, p *subscriptionSettlementProof) error {
	if p == nil || p.outbox.Completed {
		return nil
	}
	var log service.UsageLog
	if err := json.Unmarshal([]byte(p.outbox.Log), &log); err != nil {
		return err
	}
	var command service.UsageBillingCommand
	if err := json.Unmarshal([]byte(p.outbox.Command), &command); err != nil {
		return err
	}
	// UsageLog intentionally hides these internal proof fields from JSON APIs.
	// Restore them from the separately sealed billing command, not current data.
	log.BillingRequestFingerprint = command.RequestFingerprint
	log.BillingSettled = true
	// Reuse the existing complete usage insert/promotion and fingerprint proof;
	// do not duplicate its pricing fields or reimplement its conflict policy.
	if _, err := (&usageLogRepository{}).createSingle(ctx, tx, &log); err != nil {
		return err
	}
	return requestledger.CompleteSubscriptionOutboxTx(ctx, tx, p.outbox)
}

func incrementAdmittedSubscription(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) error {
	result, err := tx.ExecContext(ctx, `UPDATE user_subscriptions us SET daily_usage_usd=us.daily_usage_usd+$1,weekly_usage_usd=us.weekly_usage_usd+$1,
 monthly_usage_usd=us.monthly_usage_usd+$1,updated_at=NOW() FROM groups g
 WHERE us.id=$2 AND us.user_id=$3 AND us.group_id=g.id AND g.deleted_at IS NULL AND us.status='active' AND us.expires_at>clock_timestamp()`, cmd.SubscriptionCost, *cmd.SubscriptionID, cmd.UserID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return service.ErrSubscriptionNotFound
	}
	return nil
}

func incrementRevokedEntitlement(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) error {
	result, err := tx.ExecContext(ctx, `UPDATE subscription_package_entitlements SET request_used=request_used+1,amount_used_usd=amount_used_usd+$3,token_used=token_used+$4,
 exhausted_reason=CASE WHEN request_limit IS NOT NULL AND request_used+1>=request_limit THEN 'request'
 WHEN amount_limit_usd IS NOT NULL AND amount_used_usd+$3>=amount_limit_usd THEN 'amount'
 WHEN token_limit IS NOT NULL AND token_used+$4>=token_limit THEN 'token' ELSE exhausted_reason END,updated_at=NOW()
 WHERE id=$1 AND user_id=$2 AND status='revoked' AND expires_at>clock_timestamp()
 AND (request_limit IS NULL OR request_used<request_limit) AND (amount_limit_usd IS NULL OR amount_used_usd<amount_limit_usd) AND (token_limit IS NULL OR token_used<token_limit)`, *cmd.PackageEntitlementID, cmd.UserID, cmd.BilledCost, packageEntitlementTokenCount(cmd))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return service.ErrPackageQuotaExhausted
	}
	return nil
}

// RecoverSubscriptionSettlements only replays sealed, new-version commands.
// It neither re-enters admission nor fetches mutable pricing or sends upstream.
func (r *usageBillingRepository) RecoverSubscriptionSettlements(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `SELECT receipt_id FROM gateway_subscription_outbox WHERE completed_at IS NULL AND command_payload IS NOT NULL
 AND next_attempt_at<=clock_timestamp() AND (lease_until IS NULL OR lease_until<clock_timestamp()) ORDER BY next_attempt_at,receipt_id LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	iterationErr := rows.Err()
	closeErr := rows.Close()
	if iterationErr != nil {
		return 0, iterationErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	completed := 0
	for _, id := range ids {
		o, err := requestledger.LoadSubscriptionOutbox(ctx, r.db, id)
		if err != nil {
			r.deferInvalidSubscriptionOutbox(ctx, id)
			continue
		}
		var cmd service.UsageBillingCommand
		if json.Unmarshal([]byte(o.Command), &cmd) != nil || cmd.SubscriptionReceiptID != id {
			r.deferInvalidSubscriptionOutbox(ctx, id)
			continue
		}
		resume := requestledger.ResumeSubscriptionOutbox(ctx, r.db, o)
		claimed, err := requestledger.ClaimSubscriptionOutbox(resume, r.db, id)
		if err != nil {
			continue
		}
		_, err = r.Apply(claimed, &cmd)
		requestledger.ReleaseSubscriptionOutbox(claimed)
		if err == nil {
			completed++
		}
	}
	return completed, nil
}

// A malformed immutable payload remains pending, but cannot monopolize every
// bounded recovery batch and starve unrelated valid commands.
func (r *usageBillingRepository) deferInvalidSubscriptionOutbox(ctx context.Context, id string) {
	_, _ = r.db.ExecContext(ctx, `UPDATE gateway_subscription_outbox SET next_attempt_at=clock_timestamp()+interval '30 minutes' WHERE receipt_id=$1 AND completed_at IS NULL AND (lease_until IS NULL OR lease_until<clock_timestamp())`, id)
}
