-- Port of v2.21.2 migration 242, with a unique Fork revision.
-- Diagnostics only; usage_logs can be partitioned, so retention removes orphans.
CREATE TABLE IF NOT EXISTS request_timing_details (
    usage_log_id BIGINT NOT NULL,
    trace_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    detail JSONB NOT NULL,
    PRIMARY KEY (usage_log_id, trace_id)
);
CREATE INDEX IF NOT EXISTS request_timing_details_created_at_idx ON request_timing_details(created_at);
-- Ops reports deduplicate unbilled traces by trace_id independently of a usage
-- row; the composite primary key cannot support that lookup by its second key.
CREATE INDEX IF NOT EXISTS request_timing_details_trace_id_idx ON request_timing_details(trace_id);
