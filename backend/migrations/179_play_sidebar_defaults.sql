-- 179_play_sidebar_defaults.sql
-- Jisudeng Play: enable growth-world sidebar entries by default.
-- Migration 170 inserted these as opt-in (false); Zeabur and fresh installs
-- were left with only image_studio + affiliate visible under「玩法福利」.

UPDATE settings
SET value = 'true', updated_at = NOW()
WHERE key IN (
    'play_checkin_enabled',
    'play_arena_enabled',
    'play_blindbox_enabled',
    'play_quiz_enabled',
    'play_agent_team_enabled'
)
  AND value = 'false';
