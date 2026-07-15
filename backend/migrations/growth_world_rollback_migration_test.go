package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrowthWorldRollbackMigrationIsForwardOnlyAndGuarded(t *testing.T) {
	content, err := FS.ReadFile("189_restore_growth_rollback_defaults.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "blindbox_pool_upgrade")
	require.Contains(t, sql, `value::jsonb @> '[{"tier":2,"label":"V2","min_recharge":200},{"tier":3,"label":"V3","min_recharge":500}]'::jsonb`)
	require.Contains(t, sql, "WITH ORDINALITY")
	require.NotContains(t, strings.ToUpper(sql), "DROP TABLE")
	require.NotContains(t, strings.ToUpper(sql), "DELETE FROM")
}
