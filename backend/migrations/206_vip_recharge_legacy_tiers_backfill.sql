-- 206_vip_recharge_legacy_tiers_backfill.sql
-- Backfill legacy 4-tier VIP defaults missed by migration 205 when the stored
-- JSON array order differs. Operator-customized VIP settings are preserved.

WITH default_tiers(value) AS (
    VALUES (
        '[{"tier":0,"label":"V0","min_recharge":0,"recharge_bonus_pct":0,"color_key":"neutral"},{"tier":1,"label":"V1","min_recharge":50,"recharge_bonus_pct":2,"color_key":"emerald","perks":["models_vip_tag"]},{"tier":2,"label":"V2","min_recharge":100,"recharge_bonus_pct":4,"color_key":"sky","perks":["models_vip_tag","blindbox_pool_upgrade"]},{"tier":3,"label":"V3","min_recharge":200,"recharge_bonus_pct":6,"color_key":"indigo","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus"]},{"tier":4,"label":"V4","min_recharge":500,"recharge_bonus_pct":8,"color_key":"amber","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]},{"tier":5,"label":"V5","min_recharge":1000,"recharge_bonus_pct":10,"color_key":"gold","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]}]'
    )
),
legacy_play_vip_tiers AS (
    SELECT s.key
    FROM settings AS s
    WHERE s.key = 'play_vip_tiers'
      AND jsonb_typeof(s.value::jsonb) = 'array'
      AND jsonb_array_length(s.value::jsonb) = 4
      AND NOT EXISTS (
          SELECT 1
          FROM jsonb_array_elements(s.value::jsonb) AS tier(value)
          WHERE tier.value ? 'recharge_bonus_pct'
             OR tier.value ? 'color_key'
      )
      AND EXISTS (
          SELECT 1
          FROM jsonb_array_elements(s.value::jsonb) AS tier(value)
          WHERE tier.value @> '{"tier":0,"label":"V0","min_recharge":0}'::jsonb
            AND tier.value <@ '{"tier":0,"label":"V0","min_recharge":0}'::jsonb
      )
      AND EXISTS (
          SELECT 1
          FROM jsonb_array_elements(s.value::jsonb) AS tier(value)
          WHERE tier.value @> '{"tier":1,"label":"V1","min_recharge":50,"perks":["models_vip_tag"]}'::jsonb
            AND tier.value <@ '{"tier":1,"label":"V1","min_recharge":50,"perks":["models_vip_tag"]}'::jsonb
      )
      AND EXISTS (
          SELECT 1
          FROM jsonb_array_elements(s.value::jsonb) AS tier(value)
          WHERE tier.value @> '{"tier":2,"label":"V2","min_recharge":200,"perks":["models_vip_tag","blindbox_pool_upgrade"]}'::jsonb
            AND tier.value <@ '{"tier":2,"label":"V2","min_recharge":200,"perks":["models_vip_tag","blindbox_pool_upgrade"]}'::jsonb
      )
      AND EXISTS (
          SELECT 1
          FROM jsonb_array_elements(s.value::jsonb) AS tier(value)
          WHERE tier.value @> '{"tier":3,"label":"V3","min_recharge":500,"perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]}'::jsonb
            AND tier.value <@ '{"tier":3,"label":"V3","min_recharge":500,"perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]}'::jsonb
      )
)
UPDATE settings AS s
SET value = default_tiers.value
FROM default_tiers, legacy_play_vip_tiers AS legacy
WHERE s.key = legacy.key;
