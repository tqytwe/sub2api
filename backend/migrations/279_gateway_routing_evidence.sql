-- Additive only: old writers keep not_recorded defaults, old readers ignore the
-- new columns. No history is backfilled. Parent contention must fail without
-- queuing an exclusive lock ahead of live ledger admission/finalization.
-- Secondary lock waits and actual DDL execution are bounded as well. A busy
-- installation retries this transaction; it does not drain normal traffic.
SET LOCAL lock_timeout = '50ms';
SET LOCAL statement_timeout = '500ms';
LOCK TABLE gateway_requests IN ACCESS EXCLUSIVE MODE NOWAIT;
ALTER TABLE gateway_requests
 ADD COLUMN IF NOT EXISTS routing_coverage_version text NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS routing_summary jsonb NOT NULL DEFAULT '{"status":"not_recorded"}'::jsonb;
CREATE TABLE IF NOT EXISTS gateway_request_routing (
 request_id uuid PRIMARY KEY REFERENCES gateway_requests(id) ON DELETE CASCADE,
 events jsonb NOT NULL CHECK (jsonb_typeof(events)='array' AND jsonb_array_length(events)<=128 AND octet_length(events::text)<=32768),
 event_count integer NOT NULL CHECK (event_count BETWEEN 0 AND 128 AND event_count=jsonb_array_length(events)),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
