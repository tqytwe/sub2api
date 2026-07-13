-- 180_site_subtitle_jisudeng.sql
-- Play/Jisudeng: replace upstream English site_subtitle with Chinese branding.

UPDATE settings
SET value = '最安全的大模型中转平台', updated_at = NOW()
WHERE key = 'site_subtitle'
  AND value IN (
    'Subscription to API Conversion Platform',
    'Subscription to API'
  );
