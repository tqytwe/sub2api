package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicStatusOpsAggregationWatermarkMigrationIsMonotonicAndScoped(t *testing.T) {
	raw, err := os.ReadFile("267_public_status_ops_aggregation_watermark.sql")
	require.NoError(t, err)
	sql := strings.ToUpper(string(raw))
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS OPS_AGGREGATION_WATERMARKS",
		"JOB_NAME TEXT PRIMARY KEY",
		"COMPLETED_THROUGH TIMESTAMPTZ NOT NULL",
		"CHECK (MOD(EXTRACT(EPOCH FROM COMPLETED_THROUGH)::BIGINT, 3600) = 0)",
		"CHECK (JOB_NAME <> '')",
		"UPDATED_AT TIMESTAMPTZ NOT NULL DEFAULT NOW()",
	} {
		require.Contains(t, sql, fragment)
	}
	require.NotContains(t, sql, "DROP ")
}
