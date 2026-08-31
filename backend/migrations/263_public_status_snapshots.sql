-- Immutable, public-safe operational snapshots. The writer only INSERTs a new
-- completed hourly window (ON CONFLICT DO NOTHING); public HTTP reads this table
-- and never performs percentile scans on request traffic.

CREATE TABLE IF NOT EXISTS public_status_snapshots (
    window_end TIMESTAMPTZ PRIMARY KEY,
    total_requests BIGINT NOT NULL CHECK (total_requests >= 0),
    availability_pct DOUBLE PRECISION NULL CHECK (availability_pct IS NULL OR (availability_pct >= 0 AND availability_pct <= 100)),
    availability_sample_count BIGINT NOT NULL DEFAULT 0 CHECK (availability_sample_count >= 0),
    ttft_p50_ms DOUBLE PRECISION NULL CHECK (ttft_p50_ms IS NULL OR ttft_p50_ms >= 0),
    ttft_p95_ms DOUBLE PRECISION NULL CHECK (ttft_p95_ms IS NULL OR ttft_p95_ms >= 0),
    ttft_sample_count BIGINT NOT NULL DEFAULT 0 CHECK (ttft_sample_count >= 0),
    data_through TIMESTAMPTZ NULL,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_public_status_snapshots_latest
    ON public_status_snapshots (window_end DESC);

CREATE OR REPLACE FUNCTION reject_public_status_snapshot_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'public_status_snapshots are immutable after insertion';
END;
$$;

DROP TRIGGER IF EXISTS public_status_snapshots_immutable ON public_status_snapshots;
CREATE TRIGGER public_status_snapshots_immutable
    BEFORE UPDATE OR DELETE ON public_status_snapshots
    FOR EACH ROW
    EXECUTE FUNCTION reject_public_status_snapshot_mutation();

COMMENT ON TABLE public_status_snapshots IS
    'Append-only public status snapshots. A database trigger rejects updates and deletes after insertion.';
