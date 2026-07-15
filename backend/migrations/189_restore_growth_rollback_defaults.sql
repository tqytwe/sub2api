-- Restore the default V2/V3 blind-box perk removed by migration 188.
-- Existing growth-world tables and custom VIP tier structures are preserved.
UPDATE settings
SET value = (
    SELECT jsonb_agg(
        CASE
            WHEN (tier->>'tier')::int IN (2, 3)
              AND jsonb_typeof(tier->'perks') = 'array'
              AND NOT (tier->'perks' ? 'blindbox_pool_upgrade')
            THEN jsonb_set(
                tier,
                '{perks}',
                tier->'perks' || '["blindbox_pool_upgrade"]'::jsonb
            )
            ELSE tier
        END
        ORDER BY tier_order
    )::text
    FROM jsonb_array_elements(settings.value::jsonb)
         WITH ORDINALITY AS tiers(tier, tier_order)
)
WHERE key = 'play_vip_tiers'
  AND jsonb_typeof(value::jsonb) = 'array'
  AND value::jsonb @> '[{"tier":2,"label":"V2","min_recharge":200},{"tier":3,"label":"V3","min_recharge":500}]'::jsonb;
