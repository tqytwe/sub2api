-- Make mobile video execution identity immutable and make uncertain submits
-- fail closed. This is a separate migration because 255 may already have run
-- on an environment before the worker safety patch is deployed.

ALTER TABLE mobile_video_jobs
    ADD COLUMN IF NOT EXISTS execution_api_key_id BIGINT;

ALTER TABLE mobile_video_jobs
    ALTER COLUMN state TYPE VARCHAR(32);

ALTER TABLE mobile_video_jobs
    DROP CONSTRAINT IF EXISTS mobile_video_jobs_state_check;

ALTER TABLE mobile_video_jobs
    ADD CONSTRAINT mobile_video_jobs_state_check
    CHECK (state IN ('queued', 'submitting', 'submission_unknown', 'polling', 'completed', 'failed', 'cancelled'));

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'mobile_video_jobs_execution_api_key_fkey'
    ) THEN
        ALTER TABLE mobile_video_jobs
            ADD CONSTRAINT mobile_video_jobs_execution_api_key_fkey
            FOREIGN KEY (execution_api_key_id) REFERENCES api_keys(id) ON DELETE SET NULL;
    END IF;
END $$;
