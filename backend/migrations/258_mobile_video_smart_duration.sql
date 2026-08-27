-- Seedance accepts -1 as its smart-duration sentinel. All other values remain
-- bounded to the normal 1-15 second video contract.
ALTER TABLE mobile_video_jobs
    DROP CONSTRAINT IF EXISTS mobile_video_jobs_duration_check;

ALTER TABLE mobile_video_jobs
    ADD CONSTRAINT mobile_video_jobs_duration_check
    CHECK (
        duration_seconds = -1
        OR duration_seconds BETWEEN 1 AND 15
    );
