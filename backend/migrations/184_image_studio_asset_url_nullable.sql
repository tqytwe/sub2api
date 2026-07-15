-- 184_image_studio_asset_url_nullable.sql
-- Local disk storage writes storage_key and leaves url empty.
-- Migration 182 added storage columns but left url NOT NULL — insert failed with:
--   pq: null value in column "url" of relation "image_studio_assets" violates not-null constraint

ALTER TABLE image_studio_assets
    ALTER COLUMN url DROP NOT NULL;

-- At least one of url (legacy remote) or storage_key (local blob) must be present.
ALTER TABLE image_studio_assets
    DROP CONSTRAINT IF EXISTS image_studio_assets_url_or_storage_key_chk;

ALTER TABLE image_studio_assets
    ADD CONSTRAINT image_studio_assets_url_or_storage_key_chk
    CHECK (
        (url IS NOT NULL AND btrim(url) <> '')
        OR (storage_key IS NOT NULL AND btrim(storage_key) <> '')
    );

COMMENT ON COLUMN image_studio_assets.url IS
    'Legacy remote/temp URL; nullable when storage_key points to local blob';
