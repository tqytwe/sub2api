-- Keep Excel/BPS OAuth tokens separate from the account's native Codex
-- credentials. The payload is AES-GCM ciphertext produced by SecretEncryptor.
CREATE TABLE IF NOT EXISTS openai_excel_oauth_credentials (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    credentials_encrypted TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
