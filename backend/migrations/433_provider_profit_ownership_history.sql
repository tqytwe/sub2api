-- Adapted from ranxi2001/sub2api #913 (1c345720d), migration 383.
-- Capture new provenance at the time of the change. Earlier bindings can be
-- reported as legacy estimates, but are never backdated as observed history.
ALTER TABLE provider_profit_links ADD COLUMN identity_revision BIGINT NOT NULL DEFAULT 1 CHECK (identity_revision > 0);
ALTER TABLE provider_profit_links ADD COLUMN identity_from TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp();
UPDATE provider_profit_links l SET identity_revision=p.revision FROM mature_providers p WHERE p.id=l.provider_id;

-- History retains IDs, not credentials or names; a deleted live entity cannot
-- cascade into a past statement. Current FKs above prevent dangling live links.
CREATE TABLE provider_profit_channel_history (
    provider_id BIGINT NOT NULL,
    channel_id BIGINT NOT NULL,
    revision BIGINT NOT NULL,
    identity_from TIMESTAMPTZ NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    effective_until TIMESTAMPTZ CHECK (effective_until >= effective_from),
    PRIMARY KEY (provider_id, revision)
);
CREATE UNIQUE INDEX provider_profit_channel_history_live ON provider_profit_channel_history(provider_id) WHERE effective_until IS NULL;
CREATE INDEX provider_profit_channel_owner_at ON provider_profit_channel_history(channel_id, effective_from DESC);
CREATE INDEX provider_profit_provider_channel_at ON provider_profit_channel_history(provider_id, effective_from DESC);

CREATE FUNCTION provider_profit_record_link() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE t TIMESTAMPTZ := clock_timestamp();
BEGIN
    -- Dashboard invalidation and channel history share the exact same cutoff.
    IF TG_OP='INSERT' THEN
        t := NEW.identity_from;
    ELSIF TG_OP='UPDATE' THEN
        IF NEW.identity_from IS DISTINCT FROM OLD.identity_from THEN
            t := NEW.identity_from;
        END IF;
    END IF;
    IF TG_OP <> 'INSERT' THEN
        UPDATE provider_profit_channel_history SET effective_until=t
        WHERE provider_id=OLD.provider_id AND effective_until IS NULL;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        INSERT INTO provider_profit_channel_history(provider_id, channel_id, revision, identity_from, effective_from)
        VALUES (NEW.provider_id, NEW.channel_id, NEW.revision, NEW.identity_from, t);
    END IF;
    RETURN NULL;
END;
$$;
CREATE TRIGGER provider_profit_record_link AFTER INSERT OR UPDATE OR DELETE ON provider_profit_links
FOR EACH ROW EXECUTE FUNCTION provider_profit_record_link();

INSERT INTO provider_profit_channel_history(provider_id,channel_id,revision,identity_from,effective_from)
SELECT provider_id,channel_id,revision,identity_from,identity_from FROM provider_profit_links;

CREATE OR REPLACE FUNCTION provider_profit_make_link() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    channel_id bigint;
    suffix text;
    attempt integer := 0;
BEGIN
    LOOP
        suffix := NEW.id::text || CASE WHEN attempt = 0 THEN '' ELSE '-' || attempt::text END;
        INSERT INTO upstream_channels(code, name, remark)
        VALUES ('provider-' || suffix,
                left(COALESCE(NULLIF(btrim(NEW.name), ''), 'Provider'), 96 - char_length(suffix)) || ' (#' || suffix || ')',
                'Managed provider profit channel')
        ON CONFLICT DO NOTHING RETURNING id INTO channel_id;
        EXIT WHEN FOUND;
        attempt := attempt + 1;
    END LOOP;
    INSERT INTO provider_profit_links(provider_id,channel_id,identity_revision) VALUES (NEW.id,channel_id,NEW.revision);
    RETURN NULL;
END;
$$;

-- The configuration writer always sets identity columns on an admin edit.
-- Session refresh also rotates ciphertext, but does NOT target those columns:
-- only an admin identity/credential edit starts a new dashboard/key namespace.
CREATE FUNCTION provider_profit_on_identity_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    -- Take the same row lock as a concurrent administrative relink *before*
    -- sampling its boundary; a waited-on timestamp can predate the new link.
    PERFORM 1 FROM provider_profit_links WHERE provider_id=NEW.id FOR UPDATE;
    UPDATE provider_profit_links SET identity_from=clock_timestamp(), identity_revision=NEW.revision, revision=revision+1
    WHERE provider_id=NEW.id;
    RETURN NULL;
END;
$$;
CREATE TRIGGER provider_profit_identity_change AFTER UPDATE OF base_url, site_type, auth_mode, account, user_id ON mature_providers
FOR EACH ROW WHEN (
    (OLD.base_url,OLD.site_type,OLD.auth_mode,OLD.account,OLD.user_id,OLD.credential_cipher)
    IS DISTINCT FROM (NEW.base_url,NEW.site_type,NEW.auth_mode,NEW.account,NEW.user_id,NEW.credential_cipher)
) EXECUTE FUNCTION provider_profit_on_identity_change();

-- Retain the explicit NULL unlink tombstone separately from no manual decision.
CREATE TABLE provider_profit_manual_history (
    account_id BIGINT NOT NULL,
    channel_id BIGINT,
    revision BIGINT NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    effective_until TIMESTAMPTZ CHECK (effective_until >= effective_from),
    legacy_at_migration boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX provider_profit_manual_live ON provider_profit_manual_history(account_id) WHERE effective_until IS NULL;
CREATE INDEX provider_profit_manual_at ON provider_profit_manual_history(account_id, effective_from DESC);
CREATE FUNCTION provider_profit_record_manual() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE t timestamptz := clock_timestamp();
BEGIN
    IF TG_OP <> 'INSERT' THEN
        UPDATE provider_profit_manual_history SET effective_until=t
        WHERE account_id=OLD.account_id AND effective_until IS NULL;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        INSERT INTO provider_profit_manual_history(account_id,channel_id,revision,effective_from)
        VALUES (NEW.account_id,NEW.channel_id,NEW.revision,t);
    END IF;
    RETURN NULL;
END;
$$;
CREATE TRIGGER provider_profit_record_manual AFTER INSERT OR UPDATE OR DELETE ON upstream_channel_bindings
FOR EACH ROW EXECUTE FUNCTION provider_profit_record_manual();

CREATE TABLE provider_profit_account_source_history (
    account_id BIGINT NOT NULL,
    provider_id BIGINT NOT NULL,
    key_id TEXT NOT NULL,
    origin_hash text NOT NULL,
    -- An unversioned legacy import can compete for a request at this site,
    -- but cannot establish which wallet (or channel) owned it.
    identity_from TIMESTAMPTZ,
    effective_from TIMESTAMPTZ NOT NULL,
    effective_until TIMESTAMPTZ CHECK (effective_until >= effective_from)
);
CREATE UNIQUE INDEX provider_profit_source_live ON provider_profit_account_source_history(account_id) WHERE effective_until IS NULL;
CREATE INDEX provider_profit_source_at ON provider_profit_account_source_history(account_id,effective_from DESC);
CREATE INDEX provider_profit_source_key_at ON provider_profit_account_source_history(provider_id,key_id,effective_from DESC);

-- Reject malformed, negative, or overflowing imported JSON values as unknown
-- without breaking unrelated account writes or persisting arbitrary JSON.
CREATE FUNCTION provider_profit_positive_id(v text) RETURNS bigint LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
    SELECT CASE WHEN v ~ '^[1-9][0-9]{0,18}$' AND (length(v)<19 OR v COLLATE "C" <= '9223372036854775807') THEN v::bigint END
$$;
-- Only called with the catalog destination; both capture and reconciliation use
-- this same namespace. Never persist the address in an account's history.
CREATE FUNCTION provider_profit_origin(base text) RETURNS text LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
    SELECT encode(sha256(convert_to(regexp_replace(regexp_replace(lower(rtrim(base,'/') COLLATE "C"), ':443$', ''), '\.(:[0-9]+)?$', '\1'), 'UTF8')), 'hex')
$$;
CREATE FUNCTION provider_profit_import_source(binding jsonb, account_type text, platform text, base text)
RETURNS TABLE(provider_id bigint,key_id text,origin_hash text,identity_from timestamptz) LANGUAGE sql STABLE AS $$
    SELECT l.provider_id, binding->>'key_id', provider_profit_origin(p.base_url),
           CASE WHEN provider_profit_positive_id(binding->>'provider_revision') BETWEEN l.identity_revision AND p.revision THEN l.identity_from END
    FROM provider_profit_links l JOIN mature_providers p ON p.id=l.provider_id
    WHERE l.provider_id=provider_profit_positive_id(binding->>'provider_id') AND account_type='apikey'
      AND binding->>'key_id' ~ '^[A-Za-z0-9_-]{1,128}$'
      -- Validate the claimed provider's destination; never use the URL alone to
      -- assign a provider. Stale imports after an identity change are unknown.
      AND base = rtrim(p.base_url,'/') || CASE WHEN platform IN ('grok','composite') THEN '/v1' ELSE '' END
$$;
CREATE FUNCTION provider_profit_record_source() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE t timestamptz := clock_timestamp();
BEGIN
    IF TG_OP <> 'INSERT' THEN
        UPDATE provider_profit_account_source_history SET effective_until=t
        WHERE account_id=OLD.id AND effective_until IS NULL;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        -- Changing credentials underneath an unchanged provenance claim does not
        -- prove which key now serves that account: close it, do not reopen it.
        IF TG_OP='UPDATE' AND (OLD.extra#>>'{provider_binding,provider_id}',OLD.extra#>>'{provider_binding,key_id}',OLD.extra#>>'{provider_binding,provider_revision}')
            IS NOT DISTINCT FROM (NEW.extra#>>'{provider_binding,provider_id}',NEW.extra#>>'{provider_binding,key_id}',NEW.extra#>>'{provider_binding,provider_revision}') THEN
            RETURN NULL;
        END IF;
        INSERT INTO provider_profit_account_source_history(account_id,provider_id,key_id,origin_hash,identity_from,effective_from)
        SELECT NEW.id,s.provider_id,s.key_id,s.origin_hash,s.identity_from,t
        FROM provider_profit_import_source(NEW.extra->'provider_binding',NEW.type,NEW.platform,NEW.credentials->>'base_url') s
        WHERE NULLIF(NEW.credentials->>'api_key','') IS NOT NULL;
    END IF;
    RETURN NULL;
END;
$$;
CREATE TRIGGER provider_profit_source_create AFTER INSERT ON accounts
FOR EACH ROW WHEN (NEW.extra ? 'provider_binding') EXECUTE FUNCTION provider_profit_record_source();
CREATE TRIGGER provider_profit_source_delete AFTER DELETE ON accounts
FOR EACH ROW WHEN (OLD.extra ? 'provider_binding') EXECUTE FUNCTION provider_profit_record_source();
CREATE TRIGGER provider_profit_source_edit AFTER UPDATE OF extra,credentials,type,platform ON accounts
FOR EACH ROW WHEN ((OLD.extra ? 'provider_binding' OR NEW.extra ? 'provider_binding') AND
    (OLD.extra#>>'{provider_binding,provider_id}',OLD.extra#>>'{provider_binding,key_id}',OLD.extra#>>'{provider_binding,provider_revision}',OLD.credentials->>'api_key',OLD.credentials->>'base_url',OLD.type,OLD.platform)
    IS DISTINCT FROM
    (NEW.extra#>>'{provider_binding,provider_id}',NEW.extra#>>'{provider_binding,key_id}',NEW.extra#>>'{provider_binding,provider_revision}',NEW.credentials->>'api_key',NEW.credentials->>'base_url',NEW.type,NEW.platform)
) EXECUTE FUNCTION provider_profit_record_source();

-- Migration is atomic. Trigger creation blocks writers to its source table
-- until commit. Seed only the *observed current state*, never an old event.
INSERT INTO provider_profit_manual_history(account_id,channel_id,revision,effective_from,legacy_at_migration)
SELECT account_id,channel_id,revision,clock_timestamp(),true FROM upstream_channel_bindings;
INSERT INTO provider_profit_account_source_history(account_id,provider_id,key_id,origin_hash,identity_from,effective_from)
SELECT a.id,s.provider_id,s.key_id,s.origin_hash,s.identity_from,clock_timestamp() FROM accounts a
CROSS JOIN LATERAL provider_profit_import_source(a.extra->'provider_binding',a.type,a.platform,a.credentials->>'base_url') s
WHERE a.extra ? 'provider_binding' AND NULLIF(a.credentials->>'api_key','') IS NOT NULL;
