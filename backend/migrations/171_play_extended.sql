-- 171_play_extended.sql
-- Blindbox opens, quiz questions/attempts, agent teams.

CREATE TABLE IF NOT EXISTS play_blindbox_opens (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    open_date   DATE NOT NULL,
    cost_amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    reward_amount DECIMAL(20, 8) NOT NULL DEFAULT 0,
    idempotency_key VARCHAR(128) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_blindbox_opens_idempotency UNIQUE (idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_play_blindbox_opens_user_date ON play_blindbox_opens(user_id, open_date);

CREATE TABLE IF NOT EXISTS play_quiz_questions (
    id             BIGSERIAL PRIMARY KEY,
    prompt         TEXT NOT NULL,
    options        JSONB NOT NULL,
    correct_index  INT NOT NULL,
    sort_order     INT NOT NULL DEFAULT 0,
    active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS play_quiz_attempts (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    attempt_date   DATE NOT NULL,
    score          INT NOT NULL DEFAULT 0,
    total          INT NOT NULL DEFAULT 0,
    reward_amount  DECIMAL(20, 8) NOT NULL DEFAULT 0,
    answers        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_quiz_attempts_user_date UNIQUE (user_id, attempt_date)
);

CREATE TABLE IF NOT EXISTS play_teams (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(64) NOT NULL,
    captain_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invite_code     VARCHAR(16) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_teams_invite_code UNIQUE (invite_code)
);

CREATE INDEX IF NOT EXISTS idx_play_teams_captain ON play_teams(captain_user_id);

CREATE TABLE IF NOT EXISTS play_team_members (
    id         BIGSERIAL PRIMARY KEY,
    team_id    BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_play_team_members_user UNIQUE (user_id),
    CONSTRAINT uq_play_team_members_team_user UNIQUE (team_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_play_team_members_team ON play_team_members(team_id);

INSERT INTO settings (key, value)
VALUES
    ('play_blindbox_cost', '0.5'),
    ('play_blindbox_daily_limit', '10'),
    ('play_quiz_reward_per_correct', '0.1'),
    ('play_quiz_questions_per_day', '5')
ON CONFLICT (key) DO NOTHING;

INSERT INTO play_quiz_questions (prompt, options, correct_index, sort_order)
SELECT * FROM (VALUES
('Which HTTP method is typically used to send a chat completion request?', '["GET", "POST", "DELETE", "OPTIONS"]'::jsonb, 1, 1),
('What does API stand for?', '["Application Programming Interface", "Automated Program Integration", "Advanced Protocol Instance", "Applied Process Input"]'::jsonb, 0, 2),
('In token-based billing, which metric is most commonly metered for LLM APIs?', '["Request count only", "Tokens consumed", "Server CPU time", "Database rows"]'::jsonb, 1, 3),
('What is a common purpose of an API gateway?', '["Replace all databases", "Route, authenticate, and meter upstream API traffic", "Host static websites only", "Compile source code"]'::jsonb, 1, 4),
('Which field usually carries the user message in an OpenAI-style chat request?', '["messages", "headers", "cookies", "fonts"]'::jsonb, 0, 5),
('Why do providers cache prompt prefixes?', '["To increase token price", "To reduce latency and cost for repeated context", "To disable streaming", "To block JSON output"]'::jsonb, 1, 6),
('What does RPM commonly limit?', '["Requests per minute", "Rows per model", "Random password minimum", "Refund policy months"]'::jsonb, 0, 7),
('Which status code indicates authentication is required or failed?', '["200", "301", "401", "418"]'::jsonb, 2, 8),
('In a REST API, idempotency keys help prevent what problem?', '["Duplicate side effects from retried requests", "Slow DNS lookup", "Missing CSS files", "Large image uploads only"]'::jsonb, 0, 9),
('What is streaming mainly used for in chat APIs?', '["Batch deleting users", "Delivering partial model output as it is generated", "Encrypting invoices", "Compiling Go code"]'::jsonb, 1, 10)
) AS seed(prompt, options, correct_index, sort_order)
WHERE NOT EXISTS (SELECT 1 FROM play_quiz_questions LIMIT 1);
