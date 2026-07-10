-- 173_play_vip.sql
-- Sprint C: VIP cumulative recharge tiers (display-only; billing unchanged).

INSERT INTO settings (key, value)
VALUES
    ('play_vip_tiers', '[{"tier":0,"label":"V0","min_recharge":0},{"tier":1,"label":"V1","min_recharge":50,"perks":["models_vip_tag"]},{"tier":2,"label":"V2","min_recharge":200,"perks":["models_vip_tag","blindbox_pool_upgrade"]},{"tier":3,"label":"V3","min_recharge":500,"perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]}]')
ON CONFLICT (key) DO NOTHING;
