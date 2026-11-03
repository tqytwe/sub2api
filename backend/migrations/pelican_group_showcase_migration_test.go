package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPelicanGroupShowcaseMigrationsAreAdditiveAndOrdered(t *testing.T) {
	showcase, err := os.ReadFile("343_pelican_showcase_items.sql")
	require.NoError(t, err)
	groupTests, err := os.ReadFile("344_pelican_group_tests.sql")
	require.NoError(t, err)
	costs, err := os.ReadFile("345_pelican_group_test_costs.sql")
	require.NoError(t, err)

	showcaseSQL := string(showcase)
	groupTestsSQL := string(groupTests)
	costsSQL := string(costs)
	require.Contains(t, showcaseSQL, "CREATE TABLE IF NOT EXISTS pelican_showcase_items")
	require.Contains(t, showcaseSQL, "response_text")
	require.Contains(t, groupTestsSQL, "CREATE TABLE IF NOT EXISTS pelican_group_test_plans")
	require.Contains(t, groupTestsSQL, "CREATE TABLE IF NOT EXISTS pelican_group_test_results")
	require.Contains(t, groupTestsSQL, "REFERENCES pelican_group_test_plans(id) ON DELETE CASCADE")
	require.Contains(t, costsSQL, "CREATE TABLE IF NOT EXISTS pelican_group_test_daily_costs")
	require.Contains(t, costsSQL, "ADD COLUMN IF NOT EXISTS cost_usd")
	require.Contains(t, costsSQL, "ADD COLUMN IF NOT EXISTS cost_incomplete")
	require.Contains(t, groupTestsSQL, "jsonb_array_elements_text(value::jsonb -> 'group_ids')::BIGINT")
	require.Contains(t, groupTestsSQL, "FROM pelican_showcase_items i")
	require.Contains(t, groupTestsSQL, "WHERE g.id = ANY(selected)")
	require.Contains(t, groupTestsSQL, "AND NOT EXISTS (SELECT 1 FROM pelican_group_test_plans p WHERE p.group_id = g.id)")
	require.Contains(t, groupTestsSQL, "false,")
	require.Contains(t, groupTestsSQL, "'question_kind', 'pelican'")
	require.NotContains(t, strings.ToLower(showcaseSQL+groupTestsSQL+costsSQL), "drop table")
	require.NotContains(t, strings.ToLower(showcaseSQL+groupTestsSQL+costsSQL), "delete from")
}
