-- Restore the approved configurable blindbox pool without replacing operator configuration.
INSERT INTO settings (key, value)
VALUES (
    'play_blindbox_pool_json',
    '{"version":"season-1-v1","cost":0.5,"rtp_cap":0.9,"tiers":[{"amount":0.05,"weight":4000},{"amount":0.2,"weight":3000},{"amount":0.5,"weight":1800},{"amount":1,"weight":800},{"amount":3,"weight":300},{"amount":10,"weight":90},{"amount":20,"weight":10}]}'
)
ON CONFLICT (key) DO NOTHING;

ALTER TABLE play_blindbox_opens
    ADD COLUMN IF NOT EXISTS pool_version VARCHAR(64) DEFAULT 'legacy-v1',
    ADD COLUMN IF NOT EXISTS open_source VARCHAR(24) DEFAULT 'paid';

UPDATE play_blindbox_opens
SET pool_version = 'legacy-v1'
WHERE pool_version IS NULL OR BTRIM(pool_version) = '';

UPDATE play_blindbox_opens
SET open_source = 'paid'
WHERE open_source IS NULL OR BTRIM(open_source) = '';

ALTER TABLE play_blindbox_opens
    ALTER COLUMN pool_version SET DEFAULT 'legacy-v1',
    ALTER COLUMN pool_version SET NOT NULL,
    ALTER COLUMN open_source SET DEFAULT 'paid',
    ALTER COLUMN open_source SET NOT NULL;
