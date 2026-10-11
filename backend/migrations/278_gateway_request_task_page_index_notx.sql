-- Read-only detail paging: bound source candidates before testing retained proof.
-- Additive and compatible with old writers/readers. No backfill or data deletion.
CREATE INDEX CONCURRENTLY IF NOT EXISTS gateway_request_tasks_page_idx
ON gateway_request_tasks (request_id, task_kind, task_ref);
