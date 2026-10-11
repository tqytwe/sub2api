-- Additive and compatible with older readers. No historical rows or money change.
-- Keep the original pending-only index; this also covers exact batch admissions
-- and worker executions that have no metered upstream attempt yet.
CREATE INDEX CONCURRENTLY IF NOT EXISTS gateway_requests_batch_reconcile_idx
ON gateway_requests (reconcile_checked_at NULLS FIRST, id)
WHERE settlement_state = 'settlement_pending'
   OR (settlement_state = 'not_required'
       AND route IN ('/v1/images/batches', 'async/image_batch'));
