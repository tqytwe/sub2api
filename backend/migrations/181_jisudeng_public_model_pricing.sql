-- 181_jisudeng_public_model_pricing.sql
-- Enable public model catalog and default reference multiplier for /models page.

INSERT INTO settings (key, value)
VALUES
    ('public_models_enabled', 'true'),
    ('public_model_rate_multiplier', '1')
ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value
WHERE settings.value IS DISTINCT FROM EXCLUDED.value;
