package requestledger

import (
	"context"
	"database/sql"
)

type BillingIntent struct {
	RequestID                            string
	APIKeyID, UserID                     int64
	Fingerprint                          string
	SubscriptionID, PackageEntitlementID *int64
}

// PrepareBilling adds a reference before the existing billing transaction. No
// monetary values or usage are invented, and this never calls billing itself.
func PrepareBilling(ctx context.Context, in BillingIntent) error {
	h := FromContext(ctx)
	if h == nil {
		return nil
	}
	if in.RequestID == "" || len(in.RequestID) > 256 || in.Fingerprint == "" || len(in.Fingerprint) > 128 {
		return ErrUnavailable
	}
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	tx, err := h.ledger.db.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	var owned bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM gateway_requests WHERE id=$1 AND user_id=$2 AND api_key_id=$3)`, h.ID, in.UserID, in.APIKeyID).Scan(&owned)
	if err != nil || !owned {
		return ErrUnavailable
	}
	var fp string
	err = tx.QueryRowContext(ctx, `INSERT INTO gateway_request_billing_links(request_id,billing_request_id,api_key_id,request_fingerprint,subscription_id,package_entitlement_id)
 VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(request_id,billing_request_id,api_key_id) DO UPDATE SET request_fingerprint=gateway_request_billing_links.request_fingerprint
 RETURNING request_fingerprint`, h.ID, in.RequestID, in.APIKeyID, in.Fingerprint, in.SubscriptionID, in.PackageEntitlementID).Scan(&fp)
	if err != nil || fp != in.Fingerprint {
		return ErrUnavailable
	}
	_, err = tx.ExecContext(ctx, `UPDATE gateway_requests r SET settlement_state=CASE WHEN EXISTS(SELECT 1 FROM gateway_request_billing_links b WHERE b.request_id=r.id AND NOT b.settlement_verified) THEN 'settlement_pending' ELSE 'settled' END WHERE id=$1`, h.ID)
	if err != nil {
		return ErrUnavailable
	}
	if err = tx.Commit(); err != nil {
		return ErrUnavailable
	}
	return nil
}

// VerifyBillingTx runs in the SAME transaction as existing dedup and monetary
// effects. It only annotates evidence. Duplicate delivery never charges again.
func VerifyBillingTx(ctx context.Context, tx *sql.Tx, in BillingIntent, applied bool) error {
	h := FromContext(ctx)
	if h == nil {
		return nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE gateway_request_billing_links SET settlement_verified=TRUE,settlement_applied=settlement_applied OR $5
 WHERE request_id=$1 AND billing_request_id=$2 AND api_key_id=$3 AND request_fingerprint=$4
 AND (SELECT COUNT(*)>0 AND BOOL_AND(proof.request_fingerprint=$4) FROM (
 SELECT request_fingerprint FROM usage_billing_dedup WHERE request_id=$2 AND api_key_id=$3
 UNION ALL SELECT request_fingerprint FROM usage_billing_dedup_archive WHERE request_id=$2 AND api_key_id=$3) proof)`, h.ID, in.RequestID, in.APIKeyID, in.Fingerprint, applied)
	if err != nil {
		return ErrUnavailable
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return ErrUnavailable
	}
	_, err = tx.ExecContext(ctx, `UPDATE gateway_requests r SET settlement_state=CASE WHEN EXISTS(
 SELECT 1 FROM gateway_request_billing_links b WHERE b.request_id=r.id AND NOT b.settlement_verified)
 THEN 'settlement_pending' ELSE 'settled' END WHERE r.id=$1`, h.ID)
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

// ObserveUsage receives only an affirmative upstream observation, not inferred
// token counts. Missing usage remains pending/unknown even if transport succeeds.
func ObserveUsage(ctx context.Context, accountID ...int64) error {
	h := FromContext(ctx)
	if h == nil || h.kind == "ws_turn" {
		// WS retries can outlive a detached billing task. The relay records
		// observed usage against its exact attempt before submitting that task.
		return nil
	}
	attempt := CurrentAttempt(ctx)
	if attempt == nil || attempt.handle != h || (len(accountID) > 0 && accountID[0] != attempt.accountID) {
		return ErrUnavailable
	}
	return attempt.ObserveUsage(ctx)
}

func (a *Attempt) ObserveUsage(ctx context.Context) error {
	if a == nil {
		return nil
	}
	return observeAttemptUsage(ctx, a.handle, a.Number)
}

func observeAttemptUsage(ctx context.Context, h *Handle, attemptNo int) error {
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	tx, err := h.ledger.db.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	// Lock the parent first, matching admission/finalization. This is lock-order
	// hardening; no claim that a production deadlock was reproduced.
	var id string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM gateway_requests WHERE id=$1 FOR UPDATE`, h.ID).Scan(&id); err != nil {
		return ErrUnavailable
	}
	// Attribution is frozen by the caller, never selected by a latest-row query.
	_, err = tx.ExecContext(ctx, `UPDATE gateway_request_attempts SET usage_state='known' WHERE request_id=$1 AND attempt_no=$2 AND phase IN ('request','ws_observed','ws_input','external_search')`, h.ID, attemptNo)
	if err != nil {
		return ErrUnavailable
	}
	_, err = tx.ExecContext(ctx, `UPDATE gateway_requests r SET usage_state=CASE WHEN EXISTS(
 SELECT 1 FROM gateway_request_attempts a WHERE a.request_id=r.id AND a.usage_state IN ('pending','usage_unknown'))
 THEN 'usage_unknown' ELSE 'known' END WHERE r.id=$1`, h.ID)
	if err != nil {
		return ErrUnavailable
	}
	if tx.Commit() != nil {
		return ErrUnavailable
	}
	return nil
}
