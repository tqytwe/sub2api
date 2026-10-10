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
	account := int64(0)
	if len(accountID) > 0 {
		account = accountID[0]
	}
	return observeAttemptUsage(ctx, h, account, 0)
}

func (a *Attempt) ObserveUsage(ctx context.Context) error {
	if a == nil {
		return nil
	}
	return observeAttemptUsage(ctx, a.handle, 0, a.Number)
}

func observeAttemptUsage(ctx context.Context, h *Handle, account int64, attemptNo int) error {
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	tx, err := h.ledger.db.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = tx.Rollback() }()
	// Bind evidence to the latest billable attempt on the actual scheduled account.
	// Connection/control frames do not become metered usage observations.
	_, err = tx.ExecContext(ctx, `UPDATE gateway_request_attempts SET usage_state='known' WHERE request_id=$1 AND attempt_no=CASE WHEN $3>0 THEN $3 ELSE (
 SELECT attempt_no FROM gateway_request_attempts WHERE request_id=$1 AND phase IN ('request','ws_observed','ws_input','external_search')
 AND ($2=0 OR account_id=$2) ORDER BY attempt_no DESC LIMIT 1) END`, h.ID, account, attemptNo)
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
