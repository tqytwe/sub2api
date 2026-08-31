-- Supports the bounded 24-hour first-token percentile calculation performed by
-- the hourly snapshot worker. This must remain non-transactional because it
-- creates the index concurrently on the production usage_logs table.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_public_status_ttft_window
    ON usage_logs (created_at DESC) INCLUDE (first_token_ms)
    WHERE first_token_ms IS NOT NULL;
