package requestledger

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SubscriptionAdmission is populated only by the native HTTP service before
// sending. It is not decoded from public request headers or request IDs.
type SubscriptionAdmission struct {
	UserID, APIKeyID, GroupID, SubscriptionID int64
	PackageEntitlementID                      *int64
	AccountID                                 int64
	BillingRequestID, RequestPayloadHash      string
	PricingPayload                            string
}

type SubscriptionReceipt struct {
	SubscriptionAdmission
	ID, RequestID, AttemptID, PricingHash string
	AttemptNo                             int
	Order                                 int64
	SubscriptionExpiresAt                 time.Time
	EntitlementExpiresAt                  sql.NullTime
}

type subscriptionAdmissionKey struct{}
type subscriptionWindowValidatorKey struct{}
type SubscriptionWindowValidator func(context.Context, *sql.Tx, int64) error

func WithoutSubscriptionAdmission(ctx context.Context) context.Context {
	return context.WithValue(ctx, subscriptionAdmissionKey{}, false)
}

func HasSubscriptionAdmission(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Value(subscriptionAdmissionKey{}).(SubscriptionAdmission)
	return ok
}

func WithSubscriptionAdmission(ctx context.Context, in SubscriptionAdmission, check SubscriptionWindowValidator) context.Context {
	ctx = context.WithValue(ctx, subscriptionWindowValidatorKey{}, check)
	return context.WithValue(ctx, subscriptionAdmissionKey{}, in)
}

func CurrentSubscriptionReceipt(ctx context.Context) *SubscriptionReceipt {
	if a := CurrentAttempt(ctx); a != nil {
		return a.subscriptionReceipt
	}
	return nil
}

func SubscriptionDigest(payload string) string {
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// Lock before gateway_requests: Apply and revocation also start with the
// subscription then entitlement. Admission never holds a task or dedup lock.
func lockSubscriptionAdmission(ctx context.Context, tx *sql.Tx, h *Handle, accountID int64, phase string) (*SubscriptionReceipt, error) {
	in, ok := ctx.Value(subscriptionAdmissionKey{}).(SubscriptionAdmission)
	if !ok || phase != "request" {
		return nil, nil
	}
	if h == nil || h.kind != "http" || in.UserID <= 0 || in.APIKeyID <= 0 || in.GroupID <= 0 || in.SubscriptionID <= 0 || in.AccountID != accountID ||
		!strings.HasPrefix(in.BillingRequestID, "gateway:") || in.RequestPayloadHash == "" || !json.Valid([]byte(in.PricingPayload)) {
		return nil, ErrUnavailable
	}
	r := &SubscriptionReceipt{SubscriptionAdmission: in, ID: uuid.NewString(), RequestID: h.ID, PricingHash: SubscriptionDigest(in.PricingPayload)}
	if err := tx.QueryRowContext(ctx, `SELECT expires_at FROM user_subscriptions
 WHERE id=$1 AND user_id=$2 AND group_id=$3 AND deleted_at IS NULL AND status='active'
 AND starts_at<=clock_timestamp() AND expires_at>clock_timestamp() FOR UPDATE`, in.SubscriptionID, in.UserID, in.GroupID).Scan(&r.SubscriptionExpiresAt); err != nil {
		return nil, ErrUnavailable
	}
	if in.PackageEntitlementID != nil {
		if err := tx.QueryRowContext(ctx, `SELECT expires_at FROM subscription_package_entitlements
 WHERE id=$1 AND user_id=$2 AND group_id=$3 AND status='active'
 AND starts_at<=clock_timestamp() AND expires_at>clock_timestamp()
 AND (request_limit IS NULL OR request_used<request_limit) AND (amount_limit_usd IS NULL OR amount_used_usd<amount_limit_usd) AND (token_limit IS NULL OR token_used<token_limit) FOR UPDATE`, *in.PackageEntitlementID, in.UserID, in.GroupID).Scan(&r.EntitlementExpiresAt); err != nil {
			return nil, ErrUnavailable
		}
	}
	if in.PackageEntitlementID == nil {
		check, ok := ctx.Value(subscriptionWindowValidatorKey{}).(SubscriptionWindowValidator)
		if !ok || check == nil {
			return nil, ErrUnavailable
		}
		if err := check(ctx, tx, in.SubscriptionID); err != nil {
			return nil, err
		}
	}
	var owner, key int64
	if err := tx.QueryRowContext(ctx, `SELECT user_id,api_key_id FROM gateway_requests WHERE id=$1`, h.ID).Scan(&owner, &key); err != nil || owner != in.UserID || key != in.APIKeyID {
		return nil, ErrUnavailable
	}
	// A completed observation bars another send even when the old caller tries
	// to continue a compatibility loop using the same server request.
	var observed bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM gateway_subscription_outbox WHERE request_id=$1)`, h.ID).Scan(&observed); err != nil || observed {
		return nil, ErrUnavailable
	}
	if err := tx.QueryRowContext(ctx, `SELECT nextval('gateway_subscription_order_seq')`).Scan(&r.Order); err != nil {
		return nil, ErrUnavailable
	}
	return r, nil
}

func insertSubscriptionReceipt(ctx context.Context, tx *sql.Tx, r *SubscriptionReceipt, a *Attempt) error {
	if r == nil {
		return nil
	}
	// SELECT ... FOR UPDATE may evaluate expiry before waiting on an unchanged
	// row. Check again after the subscription, entitlement and request locks are
	// all held, immediately before the durable admission is inserted.
	var admittedAt time.Time
	if err := tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&admittedAt); err != nil ||
		!admittedAt.Before(r.SubscriptionExpiresAt) || (r.EntitlementExpiresAt.Valid && !admittedAt.Before(r.EntitlementExpiresAt.Time)) {
		return ErrUnavailable
	}
	r.AttemptNo, r.AttemptID = a.Number, a.ID
	_, err := tx.ExecContext(ctx, `INSERT INTO gateway_subscription_receipts
 (id,request_id,attempt_no,attempt_id,version,scope,user_id,api_key_id,group_id,subscription_id,package_entitlement_id,account_id,
 billing_request_id,request_payload_hash,admission_order,subscription_expires_at,entitlement_expires_at,pricing_payload,pricing_hash)
 VALUES($1,$2,$3,$4,1,'responses_http_text_v1',$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		r.ID, r.RequestID, a.Number, a.ID, r.UserID, r.APIKeyID, r.GroupID, r.SubscriptionID, r.PackageEntitlementID, r.AccountID,
		r.BillingRequestID, r.RequestPayloadHash, r.Order, r.SubscriptionExpiresAt, r.EntitlementExpiresAt, r.PricingPayload, r.PricingHash)
	if err != nil {
		return ErrUnavailable
	}
	return nil
}
