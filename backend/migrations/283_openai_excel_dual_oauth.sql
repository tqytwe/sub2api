-- Keep Codex and Excel grants independent; existing tasks remain Codex.
-- Ported from upstream v2.10.5 271_openai_excel_dual_oauth.sql (renumbered to
-- avoid collision with fork-local 271_subscription_package_grant_sources.sql).
ALTER TABLE openai_oauth_reauth_tasks
    ADD COLUMN IF NOT EXISTS oauth_profile TEXT NOT NULL DEFAULT 'codex'
    CHECK (oauth_profile IN ('codex', 'excel'));

CREATE TABLE IF NOT EXISTS openai_excel_oauth_credentials (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    credentials_ciphertext TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
