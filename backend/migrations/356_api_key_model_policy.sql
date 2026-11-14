-- Key-specific administrator policy. Existing keys keep the request's model and effort.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS model_mapping JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS reasoning_effort VARCHAR(16) NOT NULL DEFAULT '';

-- Preserve current behavior until an administrator disables long context per key.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS allow_long_context BOOLEAN NOT NULL DEFAULT TRUE;

-- Zero preserves the existing allow_long_context and model-threshold policy.
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS context_token_limit INTEGER NOT NULL DEFAULT 0
    CHECK (context_token_limit >= 0);

COMMENT ON COLUMN api_keys.context_token_limit IS
    'Administrator-managed input token ceiling; zero inherits the long context policy';
