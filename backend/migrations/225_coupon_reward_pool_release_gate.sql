-- Coupon reward pool entries are operator-configured. Do not enable either
-- activity until an administrator has created and published its pool.
INSERT INTO settings (key, value, updated_at)
VALUES
    ('play_blindbox_enabled', 'false', NOW()),
    ('play_quiz_enabled', 'false', NOW())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = EXCLUDED.updated_at;
