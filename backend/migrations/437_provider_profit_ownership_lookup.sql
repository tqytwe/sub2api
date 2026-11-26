-- Point-in-time provider ownership lookup used by profit reports and billing
-- evidence. Keep the lookup separate from the applied 433 history migration:
-- 433 is already deployed in some databases and is checksum-immutable.
-- A manual decision wins over imported account provenance; an explicit NULL
-- channel is a durable unlink tombstone.
CREATE OR REPLACE FUNCTION provider_profit_account_at(aid bigint, instant timestamptz)
RETURNS TABLE(channel_id bigint, provider_id bigint, key_id text, assignment text)
LANGUAGE sql STABLE PARALLEL SAFE ROWS 1 AS $$
    SELECT CASE WHEN m.account_id IS NOT NULL THEN m.channel_id ELSE h.channel_id END,
           h.provider_id,
           CASE WHEN s.provider_id=h.provider_id AND s.identity_from=h.identity_from THEN s.key_id END,
           CASE WHEN m.account_id IS NULL THEN 'provider_import'
                WHEN m.channel_id IS NULL THEN 'manual_unlinked'
                ELSE 'explicit' END
    FROM (SELECT 1) AS anchor
    LEFT JOIN provider_profit_manual_history AS m
      ON m.account_id=aid
     AND m.effective_from<=instant
     AND (m.effective_until IS NULL OR m.effective_until>instant)
    LEFT JOIN provider_profit_account_source_history AS s
      ON s.account_id=aid
     AND s.effective_from<=instant
     AND (s.effective_until IS NULL OR s.effective_until>instant)
    LEFT JOIN LATERAL (
        SELECT o.provider_id,o.channel_id,o.identity_from
        FROM provider_profit_channel_history AS o
        WHERE m.account_id IS NOT NULL
          AND o.channel_id=m.channel_id
          AND o.effective_from<=instant
          AND (o.effective_until IS NULL OR o.effective_until>instant)
        UNION ALL
        SELECT o.provider_id,o.channel_id,o.identity_from
        FROM provider_profit_channel_history AS o
        WHERE m.account_id IS NULL
          AND o.provider_id=s.provider_id
          AND o.identity_from=s.identity_from
          AND o.effective_from<=instant
          AND (o.effective_until IS NULL OR o.effective_until>instant)
    ) AS h ON true
    WHERE m.account_id IS NOT NULL OR h.channel_id IS NOT NULL
$$;

COMMENT ON FUNCTION provider_profit_account_at(bigint, timestamptz) IS
'Point-in-time ownership. Explicit channel wins. A manual unlink returns channel_id NULL and assignment manual_unlinked; the profit report may then claim a unique site key. OAuth dynamic endpoints are not matched by host.';
