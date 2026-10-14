package requestledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

type SubscriptionObservation struct {
	Command, Usage, Log string
	PendingReason       string
}

type SubscriptionOutbox struct {
	Receipt *SubscriptionReceipt
	SubscriptionObservation
	CommandHash, PayloadHash string
	Completed                bool
}

type subscriptionLeaseKey struct{}
type subscriptionLease struct{ ReceiptID, Token string }

func subscriptionPayloadHash(price string, in SubscriptionObservation) string {
	return SubscriptionDigest("subscription-v1\x00" + price + "\x00" + in.Command + "\x00" + in.Usage + "\x00" + in.Log + "\x00" + in.PendingReason)
}

// PersistSubscriptionUsage is synchronous and must precede any worker enqueue
// or Apply. The immutable numbers and known marker commit together. An empty
// command is explicit pending evidence, never an inferred free settlement.
func PersistSubscriptionUsage(ctx context.Context, in SubscriptionObservation, project func(context.Context, *sql.Tx) error) error {
	r := CurrentSubscriptionReceipt(ctx)
	h := FromContext(ctx)
	if r == nil || h == nil || h.ID != r.RequestID || !json.Valid([]byte(in.Usage)) ||
		(in.Command != "" && (project == nil || !json.Valid([]byte(in.Command)) || !json.Valid([]byte(in.Log)))) {
		return ErrUnavailable
	}
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	tx, err := h.ledger.db.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	// Pending usage has a subscription FK. Acquire its parent before the
	// request, matching Apply/revoke, so its FK cannot reverse their lock order.
	// These are ownership locks only: observed usage survives expiry/revocation.
	var parentID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM user_subscriptions WHERE id=$1 AND user_id=$2 AND group_id=$3 FOR UPDATE`, r.SubscriptionID, r.UserID, r.GroupID).Scan(&parentID); err != nil {
		return ErrUnavailable
	}
	if r.PackageEntitlementID != nil {
		if err := tx.QueryRowContext(ctx, `SELECT id FROM subscription_package_entitlements WHERE id=$1 AND user_id=$2 AND group_id=$3 FOR UPDATE`, *r.PackageEntitlementID, r.UserID, r.GroupID).Scan(&parentID); err != nil {
			return ErrUnavailable
		}
	}
	var id string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM gateway_requests WHERE id=$1 AND user_id=$2 AND api_key_id=$3 FOR UPDATE`, h.ID, r.UserID, r.APIKeyID).Scan(&id); err != nil {
		return ErrUnavailable
	}
	hash := subscriptionPayloadHash(r.PricingHash, in)
	var existing string
	if err := tx.QueryRowContext(ctx, `SELECT payload_hash FROM gateway_subscription_outbox WHERE receipt_id=$1`, r.ID).Scan(&existing); err == nil {
		if existing != hash {
			return ErrUnavailable
		}
		return nil // The first observation and pending projection already committed.
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ErrUnavailable
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gateway_request_attempts SET usage_state='known' WHERE request_id=$1 AND attempt_no=$2`, h.ID, r.AttemptNo); err != nil {
		return ErrUnavailable
	}
	commandHash := ""
	if in.Command != "" {
		commandHash = SubscriptionDigest(in.Command)
	}
	var stored string
	err = tx.QueryRowContext(ctx, `INSERT INTO gateway_subscription_outbox
 (receipt_id,request_id,api_key_id,billing_request_id,version,command_payload,command_hash,usage_payload,log_payload,payload_hash,pending_reason)
 VALUES($1,$2,$3,$4,1,NULLIF($5,''),NULLIF($6,''),$7,NULLIF($8,''),$9,$10)
 ON CONFLICT(api_key_id,billing_request_id) DO UPDATE SET payload_hash=gateway_subscription_outbox.payload_hash
 WHERE gateway_subscription_outbox.receipt_id=EXCLUDED.receipt_id AND gateway_subscription_outbox.payload_hash=EXCLUDED.payload_hash
 RETURNING payload_hash`, r.ID, h.ID, r.APIKeyID, r.BillingRequestID, in.Command, commandHash, in.Usage, in.Log, hash, in.PendingReason).Scan(&stored)
	if err != nil || stored != hash {
		return ErrUnavailable
	}
	if project != nil {
		if err := project(ctx, tx); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE gateway_requests r SET usage_state=CASE WHEN EXISTS(SELECT 1 FROM gateway_request_attempts a WHERE a.request_id=r.id AND a.usage_state IN ('pending','usage_unknown')) THEN 'usage_unknown' ELSE 'known' END,settlement_state='settlement_pending' WHERE r.id=$1 AND settlement_state<>'settled'`, h.ID); err != nil {
		return ErrUnavailable
	}
	if tx.Commit() != nil {
		return ErrUnavailable
	}
	return nil
}

type subscriptionQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func LoadSubscriptionOutbox(ctx context.Context, q subscriptionQuerier, receiptID string) (*SubscriptionOutbox, error) {
	r := &SubscriptionReceipt{}
	o := &SubscriptionOutbox{Receipt: r}
	var entitlement sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT r.id,r.request_id,r.attempt_no,r.attempt_id,r.user_id,r.api_key_id,r.group_id,r.subscription_id,
 r.package_entitlement_id,r.account_id,r.billing_request_id,r.request_payload_hash,r.admission_order,r.subscription_expires_at,
 r.entitlement_expires_at,r.pricing_payload,r.pricing_hash,COALESCE(o.command_payload,''),COALESCE(o.command_hash,''),o.usage_payload,
 COALESCE(o.log_payload,''),o.payload_hash,o.pending_reason,o.completed_at IS NOT NULL
 FROM gateway_subscription_receipts r JOIN gateway_subscription_outbox o ON o.receipt_id=r.id
 WHERE r.id=$1 AND r.version=1 AND o.version=1 AND r.scope='responses_http_text_v1'
 AND o.request_id=r.request_id AND o.api_key_id=r.api_key_id AND o.billing_request_id=r.billing_request_id`, receiptID).Scan(
		&r.ID, &r.RequestID, &r.AttemptNo, &r.AttemptID, &r.UserID, &r.APIKeyID, &r.GroupID, &r.SubscriptionID, &entitlement, &r.AccountID,
		&r.BillingRequestID, &r.RequestPayloadHash, &r.Order, &r.SubscriptionExpiresAt, &r.EntitlementExpiresAt, &r.PricingPayload, &r.PricingHash,
		&o.Command, &o.CommandHash, &o.Usage, &o.Log, &o.PayloadHash, &o.PendingReason, &o.Completed)
	if err != nil {
		return nil, ErrUnavailable
	}
	if entitlement.Valid {
		r.PackageEntitlementID = &entitlement.Int64
	}
	if SubscriptionDigest(r.PricingPayload) != r.PricingHash || subscriptionPayloadHash(r.PricingHash, o.SubscriptionObservation) != o.PayloadHash ||
		(o.Command != "" && SubscriptionDigest(o.Command) != o.CommandHash) {
		return nil, ErrUnavailable
	}
	return o, nil
}

// Claim is a separate, committed short transaction. No task lock is carried
// into Apply's dedup -> subscription -> entitlement ordering.
func ClaimSubscriptionOutbox(ctx context.Context, db *sql.DB, receiptID string) (context.Context, error) {
	token := uuid.NewString()
	var id string
	err := db.QueryRowContext(ctx, `UPDATE gateway_subscription_outbox SET lease_token=$2,lease_until=clock_timestamp()+interval '60 seconds',attempt_count=attempt_count+1
 WHERE receipt_id=$1 AND completed_at IS NULL AND command_payload IS NOT NULL
 AND (lease_until IS NULL OR lease_until<clock_timestamp()) RETURNING receipt_id`, receiptID, token).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ctx, ErrUnavailable
	}
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, subscriptionLeaseKey{}, subscriptionLease{receiptID, token}), nil
}

func ClaimCurrentSubscriptionOutbox(ctx context.Context) (context.Context, error) {
	h := FromContext(ctx)
	r := CurrentSubscriptionReceipt(ctx)
	if h == nil || r == nil {
		return ctx, ErrUnavailable
	}
	return ClaimSubscriptionOutbox(ctx, h.ledger.db, r.ID)
}

// Resume reconstructs only retained attribution for settlement. It creates no
// request, attempt, receipt or upstream send, and does not acquire admission.
func ResumeSubscriptionOutbox(ctx context.Context, db *sql.DB, o *SubscriptionOutbox) context.Context {
	r := o.Receipt
	h := &Handle{ledger: New(db), ID: r.RequestID, userID: r.UserID, keyID: r.APIKeyID, kind: "http"}
	a := &Attempt{handle: h, ID: r.AttemptID, Number: r.AttemptNo, accountID: r.AccountID, subscriptionReceipt: r}
	h.currentAttempt = a
	return context.WithValue(WithHandle(ctx, h), attemptKey{}, a)
}

func CompleteSubscriptionOutboxTx(ctx context.Context, tx *sql.Tx, o *SubscriptionOutbox) error {
	if o.Completed {
		return nil
	}
	lease, ok := ctx.Value(subscriptionLeaseKey{}).(subscriptionLease)
	if !ok || lease.ReceiptID != o.Receipt.ID {
		return ErrUnavailable
	}
	result, err := tx.ExecContext(ctx, `UPDATE gateway_subscription_outbox SET completed_at=clock_timestamp(),lease_token=NULL,lease_until=NULL,pending_reason=''
 WHERE receipt_id=$1 AND payload_hash=$2 AND completed_at IS NULL AND lease_token=$3 AND lease_until>clock_timestamp()`, o.Receipt.ID, o.PayloadHash, lease.Token)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return ErrUnavailable
	}
	return nil
}

func ReleaseSubscriptionOutbox(ctx context.Context) {
	h := FromContext(ctx)
	lease, ok := ctx.Value(subscriptionLeaseKey{}).(subscriptionLease)
	if h == nil || !ok {
		return
	}
	writeCtx, cancel := detachedWrite(ctx)
	defer cancel()
	_, _ = h.ledger.db.ExecContext(writeCtx, `UPDATE gateway_subscription_outbox SET lease_token=NULL,lease_until=NULL,next_attempt_at=clock_timestamp()+interval '30 seconds'
 WHERE receipt_id=$1 AND lease_token=$2 AND completed_at IS NULL`, lease.ReceiptID, lease.Token)
}
