-- 182_image_studio_asset_storage.sql
-- Persist Image Studio assets on local disk; avoid storing upstream temp URLs / huge base64 in DB.

ALTER TABLE image_studio_assets
    ADD COLUMN IF NOT EXISTS storage_key TEXT,
    ADD COLUMN IF NOT EXISTS content_type TEXT,
    ADD COLUMN IF NOT EXISTS byte_size BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN image_studio_assets.storage_key IS 'Relative path under data/image-studio/';
COMMENT ON COLUMN image_studio_assets.content_type IS 'MIME type for stored blob';
COMMENT ON COLUMN image_studio_assets.byte_size IS 'Stored file size in bytes';
