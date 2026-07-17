-- Remove generic Image Studio template thumbnails from the curated prompt seed.
-- These assets are product entry templates, not per-prompt example images.
DELETE FROM prompt_media media
USING prompts prompt, prompt_sources source
WHERE media.prompt_id = prompt.id
  AND source.prompt_id = prompt.id
  AND source.version = media.version
  AND source.source_key = 'jisudeng-gpt-image-2-curated-seed-20260717'
  AND prompt.brand_type = 'curated'
  AND media.url IN (
      '/image-studio/templates/ecom-white-bg.webp',
      '/image-studio/templates/free-create.webp',
      '/image-studio/templates/xhs-cover.webp'
  );
