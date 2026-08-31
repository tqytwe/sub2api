-- Record the durable completion boundary of the hourly ops aggregation.
--
-- Public status snapshots are immutable. They may only be written after the
-- aggregation has successfully completed every source window through their
-- target boundary, including an hour with no traffic rows to materialize.
-- The writer advances this watermark monotonically in application code.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS ops_aggregation_watermarks (
    job_name TEXT PRIMARY KEY CHECK (job_name <> ''),
    completed_through TIMESTAMPTZ NOT NULL
        CHECK (MOD(EXTRACT(EPOCH FROM completed_through)::BIGINT, 3600) = 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE ops_aggregation_watermarks IS
    'Monotonic completed-through watermarks for background ops aggregation jobs.';
COMMENT ON COLUMN ops_aggregation_watermarks.completed_through IS
    'Exclusive UTC boundary through which the named aggregation job completed successfully.';
