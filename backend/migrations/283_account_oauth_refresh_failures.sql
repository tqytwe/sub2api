SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

-- Latest safe classification only. Never persist provider bodies or credentials.
CREATE TABLE IF NOT EXISTS account_oauth_refresh_failures (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    failure_code TEXT NOT NULL CHECK (failure_code IN (
        'phone_verification_required',
        'account_deactivated',
        'account_deleted',
        'account_retired',
        'requires_relogin',
        'password_rejected',
        'mfa_required',
        'security_check',
        'authorization_denied',
        'rate_limited',
        'timeout',
        'transport_error',
        'service_error',
        'unknown'
    )),
    last_failure_at TIMESTAMPTZ NOT NULL
);

COMMENT ON TABLE account_oauth_refresh_failures IS
    'Latest allowlisted OAuth refresh failure classification; no response text, credentials, scheduling or billing effects';
