-- 208_image_studio_asset_lifecycle.sql
-- Split Image Studio task-record retention from generated asset-byte retention.
-- Active jobs keep no record expiry; terminal jobs keep metadata for 7 days.
-- Generated asset bytes expire after 24 hours and are purged by an explicit runtime switch.

ALTER TABLE image_studio_jobs
    ADD COLUMN IF NOT EXISTS group_id BIGINT,
    ADD COLUMN IF NOT EXISTS platform TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS capability_profile_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS capability_revision TEXT NOT NULL DEFAULT '';

UPDATE image_studio_jobs
SET expires_at = NULL
WHERE status IN ('pending', 'running')
  AND expires_at IS NOT NULL;

UPDATE image_studio_jobs
SET expires_at = COALESCE(finished_at, created_at) + INTERVAL '7 days'
WHERE status IN ('completed', 'partial', 'failed', 'cancelled');

ALTER TABLE image_studio_assets
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS purged_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS filename TEXT;

UPDATE image_studio_assets
SET expires_at = created_at + INTERVAL '24 hours'
WHERE expires_at IS NULL;

INSERT INTO settings (key, value)
VALUES ('image_studio_asset_purge_enabled', 'false')
ON CONFLICT (key) DO NOTHING;

COMMENT ON COLUMN image_studio_jobs.expires_at IS
    'Task record expiry. Active jobs keep NULL; terminal Image Studio jobs are retained for 7 days from finished_at.';
COMMENT ON COLUMN image_studio_jobs.group_id IS
    'Snapshot of the API key group used for this Image Studio job; not inferred from model name.';
COMMENT ON COLUMN image_studio_jobs.platform IS
    'Snapshot of the provider platform used for this Image Studio job; legacy rows keep empty string.';
COMMENT ON COLUMN image_studio_jobs.capability_profile_id IS
    'Snapshot of the image capability profile used to build the provider request.';
COMMENT ON COLUMN image_studio_jobs.capability_revision IS
    'Snapshot of the image capability revision used to build the provider request.';
COMMENT ON COLUMN image_studio_assets.expires_at IS
    'Generated asset byte expiry. Image bytes are retained for 24 hours from asset creation.';
COMMENT ON COLUMN image_studio_assets.purged_at IS
    'Set when generated asset bytes have been purged while task metadata remains.';
COMMENT ON COLUMN image_studio_assets.filename IS
    'Stable download filename for a generated Image Studio asset.';
