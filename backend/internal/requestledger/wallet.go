package requestledger

import "context"

// A task reference was persisted at admission. A wallet row and an existing
// dedup proof are joined by their exact owner/key/idempotency key; no amount,
// usage, historical request or monetary operation is reconstructed here.
const taskWalletEvidence = `FROM gateway_request_tasks t
 JOIN balance_transactions bt ON bt.user_id=t.user_id AND bt.source_id=t.task_ref
 WHERE t.request_id=r.id AND t.user_id=r.user_id AND t.api_key_id=r.api_key_id
 AND ((t.task_kind='mobile_video' AND bt.source_type IN ('video_balance_hold','video_balance_capture','video_balance_release'))
 OR (t.task_kind IN ('image_batch','image_studio') AND bt.source_type IN ('image_balance_hold','image_balance_capture','image_balance_release')))
 AND EXISTS(SELECT 1 FROM (
 SELECT request_id,api_key_id FROM usage_billing_dedup WHERE api_key_id=t.api_key_id AND request_id=
 (CASE t.task_kind WHEN 'mobile_video' THEN 'mobile_video' WHEN 'image_batch' THEN 'batch_image' ELSE 'image_studio' END)||'_'||split_part(bt.source_type,'_',3)||':'||t.task_ref
 UNION ALL SELECT request_id,api_key_id FROM usage_billing_dedup_archive WHERE api_key_id=t.api_key_id AND request_id=
 (CASE t.task_kind WHEN 'mobile_video' THEN 'mobile_video' WHEN 'image_batch' THEN 'batch_image' ELSE 'image_studio' END)||'_'||split_part(bt.source_type,'_',3)||':'||t.task_ref
 ) proof WHERE bt.idempotency_key=bt.source_type||':'||t.api_key_id::text||':'||proof.request_id)`

type WalletReference struct {
	ID        int64  `json:"id"`
	Operation string `json:"operation"`
}

func (l *Ledger) WalletReferences(ctx context.Context, viewer Viewer, id string) ([]WalletReference, error) {
	if _, err := l.Get(ctx, viewer, id); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	rows, err := l.db.QueryContext(ctx, `SELECT DISTINCT wallet.id,wallet.source_type FROM gateway_requests r
 CROSS JOIN LATERAL (SELECT bt.id,bt.source_type `+taskWalletEvidence+`) wallet WHERE r.id=$1 ORDER BY wallet.id`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []WalletReference{}
	for rows.Next() {
		var ref WalletReference
		if err := rows.Scan(&ref.ID, &ref.Operation); err != nil {
			return nil, err
		}
		result = append(result, ref)
	}
	return result, rows.Err()
}

// ReconcileTaskBilling annotates only requests already in this ledger, using
// committed capture/release evidence. A reservation alone is not settlement.
// It never touches balances, invokes a billing service, or infers missing usage.
func (l *Ledger) ReconcileTaskBilling(ctx context.Context) error {
	ctx, cancel := detachedWrite(ctx)
	defer cancel()
	_, err := l.db.ExecContext(ctx, `WITH candidates AS (
 SELECT r.id FROM gateway_requests r WHERE r.settlement_state='settlement_pending'
 ORDER BY r.reconcile_checked_at NULLS FIRST,r.id LIMIT 1000 FOR UPDATE OF r SKIP LOCKED
 ), evidence AS (
 SELECT r.id,NOT EXISTS(SELECT 1 FROM gateway_request_billing_links b WHERE b.request_id=r.id AND NOT b.settlement_verified)
 AND EXISTS(SELECT 1 `+taskWalletEvidence+` AND bt.source_type IN ('video_balance_capture','video_balance_release','image_balance_capture','image_balance_release')) AS verified
 FROM gateway_requests r JOIN candidates c ON c.id=r.id)
 UPDATE gateway_requests r SET reconcile_checked_at=clock_timestamp(),
 settlement_state=CASE WHEN e.verified THEN 'settled' ELSE r.settlement_state END FROM evidence e WHERE r.id=e.id`)
	if err != nil {
		return ErrUnavailable
	}
	return nil
}
