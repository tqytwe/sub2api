package requestledger

// Batch captures use the existing hold/capture transaction, not ordinary usage
// billing. Join only retained admission/worker ownership and exact committed
// capture fingerprints. No link is backfilled and no amount is reconstructed.
// Keep detail and drill authorization/evidence identical through this CTE.
const batchCaptureUsageEvidence = `FROM gateway_request_tasks t
 JOIN gateway_requests owner ON owner.id=t.request_id
 JOIN usage_logs u ON u.request_id='batch_image_capture:'||t.task_ref
 AND u.api_key_id=t.api_key_id AND u.user_id=t.user_id
 WHERE t.task_kind='image_batch'
 AND t.user_id=owner.user_id AND t.api_key_id=owner.api_key_id
 AND u.billing_mode='image' AND u.billing_settled AND COALESCE(u.billing_request_fingerprint,'')<>''
 AND (SELECT COUNT(*)>0 AND BOOL_AND(proof.request_fingerprint=u.billing_request_fingerprint) FROM (
 SELECT request_fingerprint FROM usage_billing_dedup WHERE request_id=u.request_id AND api_key_id=u.api_key_id
 UNION ALL SELECT request_fingerprint FROM usage_billing_dedup_archive WHERE request_id=u.request_id AND api_key_id=u.api_key_id
 ) proof)`

const usageEvidence = `WITH usage_evidence AS (
 SELECT b.request_id,b.billing_request_id,b.api_key_id,b.request_fingerprint,
 b.subscription_id,b.package_entitlement_id,b.settlement_verified,b.settlement_applied,
 NULL::text AS batch_id,b.created_at
 FROM gateway_request_billing_links b WHERE b.request_id=$1
 UNION ALL
 SELECT t.request_id,u.request_id,t.api_key_id,u.billing_request_fingerprint,
 NULL::bigint,NULL::bigint,TRUE,FALSE,t.task_ref,t.created_at ` + batchCaptureUsageEvidence + `
 AND t.request_id=$1
 AND NOT EXISTS(SELECT 1 FROM gateway_request_billing_links b
 WHERE b.request_id=t.request_id AND b.billing_request_id=u.request_id AND b.api_key_id=u.api_key_id)
) `
