-- 174_play_team_affiliate.sql
-- Sprint C: Agent Team x Affiliate linkage.

INSERT INTO settings (key, value)
VALUES
    ('play_team_affiliate_enabled', 'false'),
    ('play_team_affiliate_token_threshold', '1000000'),
    ('play_team_affiliate_captain_bonus', '5')
ON CONFLICT (key) DO NOTHING;
