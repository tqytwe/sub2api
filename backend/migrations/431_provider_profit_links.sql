-- Explicit provider-to-analytics-channel ownership.
-- This projection is independent from routing/model-pricing channels and never
-- infers ownership from a provider URL or an account credential.
CREATE TABLE IF NOT EXISTS provider_profit_links (
    provider_id BIGINT PRIMARY KEY REFERENCES mature_providers(id) ON DELETE CASCADE,
    channel_id BIGINT NOT NULL UNIQUE REFERENCES upstream_channels(id) ON DELETE RESTRICT,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    cost_mode TEXT NOT NULL DEFAULT 'estimate' CHECK (cost_mode IN ('estimate', 'wallet'))
);

-- Existing providers receive an explicit, deterministic channel that is not
-- allowed to collide with an operator-created channel.
DO $$
DECLARE
    p RECORD;
    channel_id BIGINT;
    suffix TEXT;
    candidate_code TEXT;
    candidate_name TEXT;
    attempt INTEGER;
BEGIN
    FOR p IN SELECT id, name FROM mature_providers ORDER BY id LOOP
        IF EXISTS (SELECT 1 FROM provider_profit_links WHERE provider_id = p.id) THEN
            CONTINUE;
        END IF;
        attempt := 0;
        LOOP
            suffix := p.id::text || CASE WHEN attempt = 0 THEN '' ELSE '-' || attempt::text END;
            candidate_code := 'provider-' || suffix;
            candidate_name := left(COALESCE(NULLIF(btrim(p.name), ''), 'Provider'), 96 - char_length(suffix)) || ' (#' || suffix || ')';
            INSERT INTO upstream_channels(code, name, remark)
            VALUES (candidate_code, candidate_name, 'Managed provider profit channel')
            ON CONFLICT DO NOTHING
            RETURNING id INTO channel_id;
            EXIT WHEN FOUND;
            attempt := attempt + 1;
        END LOOP;
        INSERT INTO provider_profit_links(provider_id, channel_id)
        VALUES (p.id, channel_id)
        ON CONFLICT (provider_id) DO NOTHING;
    END LOOP;
END;
$$;

CREATE OR REPLACE FUNCTION provider_profit_make_link() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    channel_id BIGINT;
    suffix TEXT;
    candidate_code TEXT;
    candidate_name TEXT;
    attempt INTEGER := 0;
BEGIN
    LOOP
        suffix := NEW.id::text || CASE WHEN attempt = 0 THEN '' ELSE '-' || attempt::text END;
        candidate_code := 'provider-' || suffix;
        candidate_name := left(COALESCE(NULLIF(btrim(NEW.name), ''), 'Provider'), 96 - char_length(suffix)) || ' (#' || suffix || ')';
        INSERT INTO upstream_channels(code, name, remark)
        VALUES (candidate_code, candidate_name, 'Managed provider profit channel')
        ON CONFLICT DO NOTHING
        RETURNING id INTO channel_id;
        EXIT WHEN FOUND;
        attempt := attempt + 1;
    END LOOP;
    INSERT INTO provider_profit_links(provider_id, channel_id)
    VALUES (NEW.id, channel_id)
    ON CONFLICT (provider_id) DO NOTHING;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS provider_profit_on_create ON mature_providers;
CREATE TRIGGER provider_profit_on_create
AFTER INSERT ON mature_providers
FOR EACH ROW EXECUTE FUNCTION provider_profit_make_link();

CREATE INDEX IF NOT EXISTS provider_profit_links_channel_idx
    ON provider_profit_links(channel_id);
