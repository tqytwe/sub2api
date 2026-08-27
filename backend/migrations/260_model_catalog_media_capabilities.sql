-- Server-owned model media declarations. NULL intentionally represents a
-- legacy catalog row that has not yet been reviewed, so this migration neither
-- guesses capabilities nor changes any existing model/group configuration.

-- ADD COLUMN needs an ACCESS EXCLUSIVE table lock even though this nullable
-- column is metadata-only. Bound the wait so a busy catalog table causes this
-- transactional migration to retry on a later deployment rather than blocking
-- the active service indefinitely.
SET LOCAL lock_timeout = '5s';

ALTER TABLE site_model_catalog
    ADD COLUMN IF NOT EXISTS media_capabilities JSONB;

-- These two IDs already have exact, reviewed execution adapters in this
-- release. Seed only a missing declaration so an administrator's later edit
-- is never replaced. This intentionally does not touch group membership,
-- pricing, account mappings, or any access-control setting.
UPDATE site_model_catalog
SET media_capabilities = CASE LOWER(model_name)
    WHEN 'sensenova-u1.5-lite' THEN '{
        "version":"2026-08-26.1",
        "adapter":"sensenova",
        "modalities":["image"],
        "image":{
            "operations":["create","edit"],
            "sizing_kind":"custom_dimensions",
            "min_dimension":512,
            "max_dimension":4096,
            "dimension_step":32,
            "max_aspect_ratio":3,
            "supported_output_formats":["png","jpeg","webp"],
            "max_reference_images":4
        }
    }'::jsonb
    WHEN 'sensenova-u1-fast' THEN '{
        "version":"2026-08-26.1",
        "adapter":"sensenova",
        "modalities":["image"],
        "image":{
            "operations":["create"],
            "sizing_kind":"fixed",
            "supported_sizes":[
                "1664x2496","2496x1664","1760x2368","2368x1760",
                "1824x2272","2272x1824","2048x2048","2752x1536",
                "1536x2752","3072x1376","1344x3136"
            ],
            "max_reference_images":0
        }
    }'::jsonb
END
WHERE LOWER(platform) = 'openai'
  AND LOWER(model_name) IN ('sensenova-u1.5-lite', 'sensenova-u1-fast')
  AND media_capabilities IS NULL;

-- Some older installations may not have imported the two catalog rows at all.
-- Insert exactly those missing rows, while preserving every field on an
-- existing row except an absent declaration. The case-insensitive guard avoids
-- creating a second row in installations that predate platform normalization.
WITH sensenova_seed (model_name, display_name, media_capabilities) AS (
    VALUES
        ('sensenova-u1.5-lite', 'SenseNova U1.5 Lite', '{
            "version":"2026-08-26.1",
            "adapter":"sensenova",
            "modalities":["image"],
            "image":{
                "operations":["create","edit"],
                "sizing_kind":"custom_dimensions",
                "min_dimension":512,
                "max_dimension":4096,
                "dimension_step":32,
                "max_aspect_ratio":3,
                "supported_output_formats":["png","jpeg","webp"],
                "max_reference_images":4
            }
        }'::jsonb),
        ('sensenova-u1-fast', 'SenseNova U1 Fast', '{
            "version":"2026-08-26.1",
            "adapter":"sensenova",
            "modalities":["image"],
            "image":{
                "operations":["create"],
                "sizing_kind":"fixed",
                "supported_sizes":[
                    "1664x2496","2496x1664","1760x2368","2368x1760",
                    "1824x2272","2272x1824","2048x2048","2752x1536",
                    "1536x2752","3072x1376","1344x3136"
                ],
                "max_reference_images":0
            }
        }'::jsonb)
)
INSERT INTO site_model_catalog (
    model_name, platform, display_name, visible_auth, source, media_capabilities
)
SELECT
    seed.model_name, 'openai', seed.display_name, TRUE, 'system', seed.media_capabilities
FROM sensenova_seed AS seed
WHERE NOT EXISTS (
    SELECT 1
    FROM site_model_catalog AS existing
    WHERE LOWER(existing.platform) = 'openai'
      AND LOWER(existing.model_name) = LOWER(seed.model_name)
)
ON CONFLICT (model_name, platform) DO UPDATE
SET media_capabilities = COALESCE(
    site_model_catalog.media_capabilities,
    EXCLUDED.media_capabilities
);

-- Repair legacy Grok rows that existed before the catalog carried a
-- declaration. This is NULL-only: any administrator-reviewed JSON remains
-- authoritative on migration replay.
UPDATE site_model_catalog
SET media_capabilities = CASE LOWER(model_name)
    WHEN 'grok-imagine-video' THEN '{
        "version":"2026-08-26.1",
        "adapter":"grok_video",
        "modalities":["video"],
        "video":{
            "operations":["generate"],
            "supported_resolutions":["480p","720p"],
            "supported_aspect_ratios":["16:9"],
            "durations_seconds":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]
        }
    }'::jsonb
    WHEN 'grok-imagine-video-1.5' THEN '{
        "version":"2026-08-26.1",
        "adapter":"grok_video",
        "modalities":["video"],
        "video":{
            "operations":["generate"],
            "supported_resolutions":["480p","720p","1080p"],
            "supported_aspect_ratios":["16:9"],
            "durations_seconds":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]
        }
    }'::jsonb
END
WHERE LOWER(platform) = 'grok'
  AND LOWER(model_name) IN ('grok-imagine-video', 'grok-imagine-video-1.5')
  AND media_capabilities IS NULL;

-- Agnes V2.0 has a different upstream contract from Grok. Its declaration
-- exposes only the documented mobile request presets; group membership,
-- pricing, allowlists and account mappings are intentionally untouched.
WITH agnes_video_seed (model_name, display_name, media_capabilities) AS (
    VALUES
        ('agnes-video-v2.0', 'Agnes Video V2.0', '{
            "version":"2026-08-26.1",
            "adapter":"agnes_video",
            "modalities":["video"],
            "video":{
                "operations":["generate"],
                "supported_resolutions":["480p"],
                "supported_aspect_ratios":["16:9"],
                "durations_seconds":[3,5,10,18]
            }
        }'::jsonb)
)
INSERT INTO site_model_catalog (
    model_name, platform, display_name, visible_auth, source, media_capabilities
)
SELECT
    seed.model_name, 'openai', seed.display_name, TRUE, 'system', seed.media_capabilities
FROM agnes_video_seed AS seed
WHERE NOT EXISTS (
    SELECT 1
    FROM site_model_catalog AS existing
    WHERE LOWER(existing.platform) = 'openai'
      AND LOWER(existing.model_name) = LOWER(seed.model_name)
)
ON CONFLICT (model_name, platform) DO UPDATE
SET media_capabilities = COALESCE(
    site_model_catalog.media_capabilities,
    EXCLUDED.media_capabilities
);

UPDATE site_model_catalog
SET media_capabilities = '{
    "version":"2026-08-26.1",
    "adapter":"agnes_video",
    "modalities":["video"],
    "video":{
        "operations":["generate"],
        "supported_resolutions":["480p"],
        "supported_aspect_ratios":["16:9"],
        "durations_seconds":[3,5,10,18]
    }
}'::jsonb
WHERE LOWER(platform) = 'openai'
  AND LOWER(model_name) = 'agnes-video-v2.0'
  AND media_capabilities IS NULL;

-- The existing Grok video gateway has already supported these exact public
-- model IDs. Declare only the text-to-video controls that the mobile worker
-- can currently execute; reference-image/video transport remains deliberately
-- absent until that worker path is implemented. Group eligibility, account
-- mappings, scheduler availability and each declared resolution's price are
-- still checked at request time.
WITH grok_video_seed (model_name, display_name, media_capabilities) AS (
    VALUES
        ('grok-imagine-video', 'Grok Imagine Video', '{
            "version":"2026-08-26.1",
            "adapter":"grok_video",
            "modalities":["video"],
            "video":{
                "operations":["generate"],
                "supported_resolutions":["480p","720p"],
                "supported_aspect_ratios":["16:9"],
                "durations_seconds":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]
            }
        }'::jsonb),
        ('grok-imagine-video-1.5', 'Grok Imagine Video 1.5', '{
            "version":"2026-08-26.1",
            "adapter":"grok_video",
            "modalities":["video"],
            "video":{
                "operations":["generate"],
                "supported_resolutions":["480p","720p","1080p"],
                "supported_aspect_ratios":["16:9"],
                "durations_seconds":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]
            }
        }'::jsonb)
)
INSERT INTO site_model_catalog (
    model_name, platform, display_name, visible_auth, source, media_capabilities
)
SELECT
    seed.model_name, 'grok', seed.display_name, TRUE, 'system', seed.media_capabilities
FROM grok_video_seed AS seed
WHERE NOT EXISTS (
    SELECT 1
    FROM site_model_catalog AS existing
    WHERE LOWER(existing.platform) = 'grok'
      AND LOWER(existing.model_name) = LOWER(seed.model_name)
)
ON CONFLICT (model_name, platform) DO UPDATE
SET media_capabilities = COALESCE(
    site_model_catalog.media_capabilities,
    EXCLUDED.media_capabilities
);
