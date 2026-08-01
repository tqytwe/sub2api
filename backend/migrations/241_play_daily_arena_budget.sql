INSERT INTO settings (key, value)
VALUES ('play_daily_arena_daily_budget', '50')
ON CONFLICT (key) DO NOTHING;
