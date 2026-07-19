-- 205_vip_recharge_bonus_snapshot.sql
-- VIP recharge bonuses are credited to balance at order creation time and
-- frozen here for audit/customer-service explanations.

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS recharge_snapshot JSONB;

INSERT INTO settings (key, value)
VALUES (
    'play_vip_tiers',
    '[{"tier":0,"label":"V0","min_recharge":0,"recharge_bonus_pct":0,"color_key":"neutral"},{"tier":1,"label":"V1","min_recharge":50,"recharge_bonus_pct":2,"color_key":"emerald","perks":["models_vip_tag"]},{"tier":2,"label":"V2","min_recharge":100,"recharge_bonus_pct":4,"color_key":"sky","perks":["models_vip_tag","blindbox_pool_upgrade"]},{"tier":3,"label":"V3","min_recharge":200,"recharge_bonus_pct":6,"color_key":"indigo","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus"]},{"tier":4,"label":"V4","min_recharge":500,"recharge_bonus_pct":8,"color_key":"amber","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]},{"tier":5,"label":"V5","min_recharge":1000,"recharge_bonus_pct":10,"color_key":"gold","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]}]'
)
ON CONFLICT (key) DO NOTHING;

UPDATE settings
SET value = '[{"tier":0,"label":"V0","min_recharge":0,"recharge_bonus_pct":0,"color_key":"neutral"},{"tier":1,"label":"V1","min_recharge":50,"recharge_bonus_pct":2,"color_key":"emerald","perks":["models_vip_tag"]},{"tier":2,"label":"V2","min_recharge":100,"recharge_bonus_pct":4,"color_key":"sky","perks":["models_vip_tag","blindbox_pool_upgrade"]},{"tier":3,"label":"V3","min_recharge":200,"recharge_bonus_pct":6,"color_key":"indigo","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus"]},{"tier":4,"label":"V4","min_recharge":500,"recharge_bonus_pct":8,"color_key":"amber","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]},{"tier":5,"label":"V5","min_recharge":1000,"recharge_bonus_pct":10,"color_key":"gold","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]}]'
WHERE key = 'play_vip_tiers'
  AND value::jsonb = '[{"tier":0,"label":"V0","min_recharge":0},{"tier":1,"label":"V1","min_recharge":50,"perks":["models_vip_tag"]},{"tier":2,"label":"V2","min_recharge":200,"perks":["models_vip_tag","blindbox_pool_upgrade"]},{"tier":3,"label":"V3","min_recharge":500,"perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]}]'::jsonb;
