WITH ranked AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY user_id, api_key_id, idempotency_key
            ORDER BY id ASC
        ) AS duplicate_rank
    FROM batch_image_jobs
    WHERE api_key_id IS NOT NULL
      AND idempotency_key IS NOT NULL
      AND btrim(idempotency_key) <> ''
)
UPDATE batch_image_jobs AS jobs
SET idempotency_key = NULL,
    updated_at = NOW()
FROM ranked
WHERE jobs.id = ranked.id
  AND ranked.duplicate_rank > 1;

CREATE UNIQUE INDEX IF NOT EXISTS batch_image_jobs_owner_idempotency_uq
    ON batch_image_jobs (user_id, api_key_id, idempotency_key)
    WHERE api_key_id IS NOT NULL
      AND idempotency_key IS NOT NULL
      AND btrim(idempotency_key) <> '';
