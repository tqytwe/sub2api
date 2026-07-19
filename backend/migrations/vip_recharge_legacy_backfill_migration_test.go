package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const approvedVIPRechargeTiersJSON = `[{"tier":0,"label":"V0","min_recharge":0,"recharge_bonus_pct":0,"color_key":"neutral"},{"tier":1,"label":"V1","min_recharge":50,"recharge_bonus_pct":2,"color_key":"emerald","perks":["models_vip_tag"]},{"tier":2,"label":"V2","min_recharge":100,"recharge_bonus_pct":4,"color_key":"sky","perks":["models_vip_tag","blindbox_pool_upgrade"]},{"tier":3,"label":"V3","min_recharge":200,"recharge_bonus_pct":6,"color_key":"indigo","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus"]},{"tier":4,"label":"V4","min_recharge":500,"recharge_bonus_pct":8,"color_key":"amber","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]},{"tier":5,"label":"V5","min_recharge":1000,"recharge_bonus_pct":10,"color_key":"gold","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]}]`

func TestVIPRechargeLegacyBackfillMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("206_vip_recharge_legacy_tiers_backfill.sql")
	require.NoError(t, err)

	sql := normalizeMigrationSQL(string(content))
	upperSQL := strings.ToUpper(sql)

	require.Contains(t, sql, approvedVIPRechargeTiersJSON)
	require.Contains(t, upperSQL, "UPDATE SETTINGS AS S SET VALUE = DEFAULT_TIERS.VALUE")
	require.Contains(t, upperSQL, "JSONB_ARRAY_LENGTH(S.VALUE::JSONB) = 4")
	require.Contains(t, upperSQL, "WHERE S.KEY = 'PLAY_VIP_TIERS'")
	require.Contains(t, upperSQL, "TIER.VALUE ? 'RECHARGE_BONUS_PCT'")
	require.Contains(t, upperSQL, "TIER.VALUE ? 'COLOR_KEY'")

	for _, expected := range []string{
		`TIER.VALUE @> '{"TIER":0,"LABEL":"V0","MIN_RECHARGE":0}'::JSONB`,
		`TIER.VALUE <@ '{"TIER":0,"LABEL":"V0","MIN_RECHARGE":0}'::JSONB`,
		`TIER.VALUE @> '{"TIER":1,"LABEL":"V1","MIN_RECHARGE":50,"PERKS":["MODELS_VIP_TAG"]}'::JSONB`,
		`TIER.VALUE <@ '{"TIER":1,"LABEL":"V1","MIN_RECHARGE":50,"PERKS":["MODELS_VIP_TAG"]}'::JSONB`,
		`TIER.VALUE @> '{"TIER":2,"LABEL":"V2","MIN_RECHARGE":200,"PERKS":["MODELS_VIP_TAG","BLINDBOX_POOL_UPGRADE"]}'::JSONB`,
		`TIER.VALUE <@ '{"TIER":2,"LABEL":"V2","MIN_RECHARGE":200,"PERKS":["MODELS_VIP_TAG","BLINDBOX_POOL_UPGRADE"]}'::JSONB`,
		`TIER.VALUE @> '{"TIER":3,"LABEL":"V3","MIN_RECHARGE":500,"PERKS":["MODELS_VIP_TAG","BLINDBOX_POOL_UPGRADE","ARENA_SETTLEMENT_BONUS","AFFILIATE_BONUS_5PCT"]}'::JSONB`,
		`TIER.VALUE <@ '{"TIER":3,"LABEL":"V3","MIN_RECHARGE":500,"PERKS":["MODELS_VIP_TAG","BLINDBOX_POOL_UPGRADE","ARENA_SETTLEMENT_BONUS","AFFILIATE_BONUS_5PCT"]}'::JSONB`,
		"AFFILIATE_BONUS_5PCT",
	} {
		require.Contains(t, upperSQL, expected)
	}

	require.NotContains(t, upperSQL, "DELETE FROM")
	require.NotContains(t, upperSQL, "DROP TABLE")
	require.NotContains(t, upperSQL, "TRUNCATE")
}
