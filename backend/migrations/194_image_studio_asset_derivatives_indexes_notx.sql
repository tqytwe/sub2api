-- Online stable-pagination index for the Image Studio gallery.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_image_studio_jobs_user_created_id
    ON image_studio_jobs(user_id, created_at DESC, id DESC);
