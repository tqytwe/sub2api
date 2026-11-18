-- Independent grants retain encrypted credentials; existing account and Excel
-- credentials remain authoritative until an administrator selects a connection.
CREATE TABLE IF NOT EXISTS openai_auth_connections (
    id UUID PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('codex','bps','siwc','api_key')),
    label TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled','unlinked')),
    issuer TEXT NOT NULL DEFAULT '',
    client_id TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    identity_verified BOOLEAN NOT NULL DEFAULT FALSE,
    workspace TEXT NOT NULL DEFAULT '',
    resource TEXT NOT NULL DEFAULT '',
    audience TEXT NOT NULL DEFAULT '',
    scopes JSONB NOT NULL DEFAULT '[]',
    capabilities JSONB NOT NULL DEFAULT '[]',
    host_id TEXT NOT NULL DEFAULT '',
    credentials_ciphertext TEXT NOT NULL DEFAULT '',
    revision BIGINT NOT NULL DEFAULT 1,
    preferred BOOLEAN NOT NULL DEFAULT FALSE,
    cooldown_until TIMESTAMPTZ,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    lease_epoch BIGINT NOT NULL DEFAULT 0,
    refresh_state TEXT NOT NULL DEFAULT 'idle' CHECK (refresh_state IN ('idle','in_progress','uncertain','rejected')),
    refresh_attempt_id TEXT NOT NULL DEFAULT '',
    refresh_candidate_ciphertext TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (jsonb_typeof(scopes) = 'array' AND jsonb_typeof(capabilities) = 'array')
);
CREATE INDEX IF NOT EXISTS openai_auth_connections_account ON openai_auth_connections(account_id);
CREATE UNIQUE INDEX IF NOT EXISTS openai_auth_connections_preferred ON openai_auth_connections(account_id) WHERE preferred;

CREATE TABLE IF NOT EXISTS openai_auth_connection_policies (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS openai_auth_connection_usage (
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    api_key_id BIGINT NOT NULL,
    request_id TEXT NOT NULL,
    connection_id UUID NOT NULL REFERENCES openai_auth_connections(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(account_id,api_key_id,request_id)
);
CREATE INDEX IF NOT EXISTS openai_auth_connection_usage_connection ON openai_auth_connection_usage(connection_id,created_at);

CREATE TABLE IF NOT EXISTS openai_auth_connection_affinity (
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    api_key_id BIGINT NOT NULL,
    session_hash TEXT NOT NULL,
    connection_id UUID NOT NULL REFERENCES openai_auth_connections(id),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(account_id,api_key_id,session_hash)
);
CREATE INDEX IF NOT EXISTS openai_connection_affinity_expiry ON openai_auth_connection_affinity(expires_at);

CREATE OR REPLACE FUNCTION openai_connection_snapshot() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE aid BIGINT; metadata JSONB;
BEGIN
    aid := COALESCE(NEW.account_id, OLD.account_id);
    PERFORM id FROM accounts WHERE id=aid FOR UPDATE;
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'id',id,'kind',kind,'status',status,'capabilities',capabilities,
        'preferred',preferred,'cooldown_until',cooldown_until,'refresh_state',refresh_state,
        'revision',revision)
        ORDER BY created_at,id),'[]'::jsonb)
      INTO metadata FROM openai_auth_connections WHERE account_id=aid;
    UPDATE accounts SET extra=(CASE WHEN jsonb_typeof(extra)='object' THEN extra ELSE '{}'::jsonb END) || jsonb_build_object(
        'openai_connections_managed',EXISTS(SELECT 1 FROM openai_auth_connection_policies WHERE account_id=aid AND enabled),
        'openai_auth_connections',metadata), updated_at=NOW() WHERE id=aid;
    INSERT INTO scheduler_outbox(event_type,account_id) VALUES('account_changed',aid);
    RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS openai_connection_snapshot_trigger ON openai_auth_connections;
CREATE TRIGGER openai_connection_snapshot_trigger AFTER INSERT OR UPDATE OF status,capabilities,preferred,cooldown_until,credentials_ciphertext,refresh_state,revision OR DELETE ON openai_auth_connections
FOR EACH ROW EXECUTE FUNCTION openai_connection_snapshot();

CREATE OR REPLACE FUNCTION preserve_openai_connection_snapshot() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE saved JSONB;
BEGIN
    IF pg_trigger_depth()=1 THEN
        saved := jsonb_strip_nulls(jsonb_build_object(
            'openai_connections_managed',OLD.extra->'openai_connections_managed',
            'openai_auth_connections',OLD.extra->'openai_auth_connections'));
        IF jsonb_typeof(NEW.extra)='object' THEN
            NEW.extra := (NEW.extra - 'openai_connections_managed' - 'openai_auth_connections') || saved;
        ELSIF saved <> '{}'::jsonb THEN
            NEW.extra := saved;
        END IF;
    END IF;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS preserve_openai_connection_snapshot_trigger ON accounts;
CREATE TRIGGER preserve_openai_connection_snapshot_trigger BEFORE UPDATE OF extra ON accounts
FOR EACH ROW EXECUTE FUNCTION preserve_openai_connection_snapshot();
