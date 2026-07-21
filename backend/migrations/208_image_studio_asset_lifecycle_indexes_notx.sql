-- Concurrent indexes for Image Studio asset lifecycle cleanup.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_image_studio_assets_expiry_live
    ON image_studio_assets(expires_at, id)
    WHERE expires_at IS NOT NULL
      AND purged_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_image_studio_jobs_record_expiry
    ON image_studio_jobs(expires_at, id)
    WHERE expires_at IS NOT NULL
      AND status IN ('completed', 'partial', 'failed', 'cancelled');
