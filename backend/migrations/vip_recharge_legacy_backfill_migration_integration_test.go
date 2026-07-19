//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const approvedVIPRechargeBackfillJSON = `[{"tier":0,"label":"V0","min_recharge":0,"recharge_bonus_pct":0,"color_key":"neutral"},{"tier":1,"label":"V1","min_recharge":50,"recharge_bonus_pct":2,"color_key":"emerald","perks":["models_vip_tag"]},{"tier":2,"label":"V2","min_recharge":100,"recharge_bonus_pct":4,"color_key":"sky","perks":["models_vip_tag","blindbox_pool_upgrade"]},{"tier":3,"label":"V3","min_recharge":200,"recharge_bonus_pct":6,"color_key":"indigo","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus"]},{"tier":4,"label":"V4","min_recharge":500,"recharge_bonus_pct":8,"color_key":"amber","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]},{"tier":5,"label":"V5","min_recharge":1000,"recharge_bonus_pct":10,"color_key":"gold","perks":["models_vip_tag","blindbox_pool_upgrade","arena_settlement_bonus","affiliate_bonus_5pct"]}]`

func TestVIPRechargeLegacyBackfillMigrationUpdatesReorderedLegacyDefaults(t *testing.T) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			require.NoError(t, err, "Docker must be available for migration integration tests in CI")
		}
		t.Skip("Docker is unavailable")
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(
		ctx,
		"pgvector/pgvector:pg16",
		tcpostgres.WithDatabase("sub2api_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.PingContext(ctx))

	_, err = db.ExecContext(ctx, `
		CREATE TABLE settings (
			key VARCHAR(255) PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	require.NoError(t, err)

	raw, err := dbmigrations.FS.ReadFile("206_vip_recharge_legacy_tiers_backfill.sql")
	require.NoError(t, err)
	migrationSQL := string(raw)

	legacyReordered := `[{"tier":0,"label":"V0","min_recharge":0},{"tier":1,"label":"V1","min_recharge":50,"perks":["models_vip_tag"]},{"tier":2,"label":"V2","min_recharge":200,"perks":["models_vip_tag","blindbox_pool_upgrade"]},{"tier":3,"label":"V3","min_recharge":500,"perks":["models_vip_tag","arena_settlement_bonus","affiliate_bonus_5pct","blindbox_pool_upgrade"]}]`
	setVIPTiers(t, ctx, db, legacyReordered)
	_, err = db.ExecContext(ctx, migrationSQL)
	require.NoError(t, err)
	require.JSONEq(t, approvedVIPRechargeBackfillJSON, getVIPTiers(t, ctx, db))

	operatorCustom := `[{"tier":0,"label":"V0","min_recharge":0},{"tier":1,"label":"V1","min_recharge":50,"perks":["models_vip_tag"]},{"tier":2,"label":"V2","min_recharge":150,"perks":["models_vip_tag","blindbox_pool_upgrade"]},{"tier":3,"label":"V3","min_recharge":500,"perks":["models_vip_tag","arena_settlement_bonus","affiliate_bonus_5pct","blindbox_pool_upgrade"]}]`
	setVIPTiers(t, ctx, db, operatorCustom)
	_, err = db.ExecContext(ctx, migrationSQL)
	require.NoError(t, err)
	require.JSONEq(t, operatorCustom, getVIPTiers(t, ctx, db))

	alreadyNew := approvedVIPRechargeBackfillJSON
	setVIPTiers(t, ctx, db, alreadyNew)
	_, err = db.ExecContext(ctx, migrationSQL)
	require.NoError(t, err)
	require.JSONEq(t, alreadyNew, getVIPTiers(t, ctx, db))
}

func setVIPTiers(t *testing.T, ctx context.Context, db *sql.DB, value string) {
	t.Helper()

	_, err := db.ExecContext(ctx, `
		INSERT INTO settings (key, value)
		VALUES ('play_vip_tiers', $1)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
	`, value)
	require.NoError(t, err)
}

func getVIPTiers(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()

	var value string
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT value FROM settings WHERE key = 'play_vip_tiers'
	`).Scan(&value))
	return value
}
