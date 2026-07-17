-- 194_image_studio_asset_derivatives.sql
-- Real output dimensions, private thumbnails, and stable Image Studio gallery pagination.
-- The migration runner runs each statement in its own recoverable phase with lock_timeout.
-- Keep every statement idempotent and keep long validation work separate from DDL.

ALTER TABLE image_studio_assets
    ADD COLUMN IF NOT EXISTS width INT,
    ADD COLUMN IF NOT EXISTS height INT,
    ADD COLUMN IF NOT EXISTS thumbnail_storage_key TEXT,
    ADD COLUMN IF NOT EXISTS thumbnail_content_type TEXT,
    ADD COLUMN IF NOT EXISTS thumbnail_byte_size BIGINT;

ALTER TABLE image_studio_assets
    DROP CONSTRAINT IF EXISTS image_studio_assets_width_chk_upgrade,
    DROP CONSTRAINT IF EXISTS image_studio_assets_height_chk_upgrade,
    DROP CONSTRAINT IF EXISTS image_studio_assets_thumbnail_size_chk_upgrade,
    DROP CONSTRAINT IF EXISTS image_studio_assets_dimensions_pair_chk_upgrade,
    DROP CONSTRAINT IF EXISTS image_studio_assets_thumbnail_pair_chk_upgrade;

ALTER TABLE image_studio_assets
    ADD CONSTRAINT image_studio_assets_width_chk_upgrade
        CHECK (width IS NULL OR width > 0) NOT VALID,
    ADD CONSTRAINT image_studio_assets_height_chk_upgrade
        CHECK (height IS NULL OR height > 0) NOT VALID,
    ADD CONSTRAINT image_studio_assets_thumbnail_size_chk_upgrade
        CHECK (thumbnail_byte_size IS NULL OR thumbnail_byte_size > 0) NOT VALID,
    ADD CONSTRAINT image_studio_assets_dimensions_pair_chk_upgrade
        CHECK ((width IS NULL) = (height IS NULL)) NOT VALID,
    ADD CONSTRAINT image_studio_assets_thumbnail_pair_chk_upgrade
        CHECK (
            (thumbnail_storage_key IS NULL AND thumbnail_content_type IS NULL AND thumbnail_byte_size IS NULL)
            OR (
                thumbnail_storage_key IS NOT NULL
                AND btrim(thumbnail_storage_key) <> ''
                AND thumbnail_content_type IS NOT NULL
                AND btrim(thumbnail_content_type) <> ''
                AND thumbnail_byte_size IS NOT NULL
                AND thumbnail_byte_size > 0
            )
        ) NOT VALID;

ALTER TABLE image_studio_assets
    VALIDATE CONSTRAINT image_studio_assets_width_chk_upgrade;

ALTER TABLE image_studio_assets
    VALIDATE CONSTRAINT image_studio_assets_height_chk_upgrade;

ALTER TABLE image_studio_assets
    VALIDATE CONSTRAINT image_studio_assets_thumbnail_size_chk_upgrade;

ALTER TABLE image_studio_assets
    VALIDATE CONSTRAINT image_studio_assets_dimensions_pair_chk_upgrade;

ALTER TABLE image_studio_assets
    VALIDATE CONSTRAINT image_studio_assets_thumbnail_pair_chk_upgrade;

ALTER TABLE image_studio_assets
    DROP CONSTRAINT IF EXISTS image_studio_assets_width_chk,
    DROP CONSTRAINT IF EXISTS image_studio_assets_height_chk,
    DROP CONSTRAINT IF EXISTS image_studio_assets_thumbnail_size_chk,
    DROP CONSTRAINT IF EXISTS image_studio_assets_dimensions_pair_chk,
    DROP CONSTRAINT IF EXISTS image_studio_assets_thumbnail_pair_chk;

ALTER TABLE image_studio_assets
    RENAME CONSTRAINT image_studio_assets_width_chk_upgrade
    TO image_studio_assets_width_chk;

ALTER TABLE image_studio_assets
    RENAME CONSTRAINT image_studio_assets_height_chk_upgrade
    TO image_studio_assets_height_chk;

ALTER TABLE image_studio_assets
    RENAME CONSTRAINT image_studio_assets_thumbnail_size_chk_upgrade
    TO image_studio_assets_thumbnail_size_chk;

ALTER TABLE image_studio_assets
    RENAME CONSTRAINT image_studio_assets_dimensions_pair_chk_upgrade
    TO image_studio_assets_dimensions_pair_chk;

ALTER TABLE image_studio_assets
    RENAME CONSTRAINT image_studio_assets_thumbnail_pair_chk_upgrade
    TO image_studio_assets_thumbnail_pair_chk;

COMMENT ON COLUMN image_studio_assets.thumbnail_storage_key IS
    'Private storage key for the bounded gallery thumbnail, never exposed directly';
