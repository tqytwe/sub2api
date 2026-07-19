ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS output_image_count INTEGER NOT NULL DEFAULT 0;

UPDATE batch_image_jobs
SET output_image_count = COALESCE(NULLIF((
    SELECT SUM(GREATEST(items.image_count, 0))
    FROM batch_image_items AS items
    WHERE items.job_id = batch_image_jobs.batch_id
      AND items.status = 'success'
), 0), success_count)
WHERE output_image_count = 0
  AND success_count > 0;
