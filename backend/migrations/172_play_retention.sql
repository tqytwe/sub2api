-- 172_play_retention.sql
-- Sprint B: check-in streak, recharge boost, arena settlement support.

ALTER TABLE play_checkins
    ADD COLUMN IF NOT EXISTS streak_count INT NOT NULL DEFAULT 1;

COMMENT ON COLUMN play_checkins.streak_count IS 'Consecutive check-in streak ending on this date';

CREATE TABLE IF NOT EXISTS play_recharge_boosts (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_play_recharge_boosts_user_expires
    ON play_recharge_boosts(user_id, expires_at DESC);

COMMENT ON TABLE play_recharge_boosts IS '24h play perks after balance recharge';

INSERT INTO settings (key, value)
VALUES
    ('play_recharge_boost_enabled', 'false'),
    ('play_recharge_boost_duration_hours', '24'),
    ('play_recharge_boost_checkin_multiplier', '2'),
    ('play_recharge_boost_blindbox_extra_opens', '1'),
    ('play_recharge_boost_arena_multiplier', '1.5'),
    ('play_checkin_makeup_enabled', 'true'),
    ('play_checkin_streak_milestones', '[{"days":7,"bonus":1},{"days":14,"bonus":2},{"days":30,"bonus":5}]'),
    ('play_arena_settlement_rewards', '[{"rank_max":1,"amount":50},{"rank_max":3,"amount":20},{"rank_max":10,"amount":5}]')
ON CONFLICT (key) DO NOTHING;
