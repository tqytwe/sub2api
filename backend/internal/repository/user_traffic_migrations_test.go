package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestUserTrafficMigrationsHaveUniqueReservedNumbers(t *testing.T) {
	files, err := migrations.FS.ReadDir(".")
	require.NoError(t, err)
	seen := map[string]bool{}
	byNumber := map[string][]string{}
	for _, file := range files {
		seen[file.Name()] = true
		number, _, ok := strings.Cut(file.Name(), "_")
		if ok && strings.HasSuffix(file.Name(), ".sql") {
			byNumber[number] = append(byNumber[number], file.Name())
		}
	}
	for _, name := range []string{
		"381_user_traffic.sql",
		"382_user_traffic_cursor_indexes_notx.sql",
		"383_ops_network_minutes.sql",
	} {
		number, _, _ := strings.Cut(name, "_")
		require.Equal(t, []string{name}, byNumber[number], "missing or conflicting migration %s", name)
	}
	for _, name := range []string{
		"307_user_traffic.sql",
		"308_user_traffic_cursor_indexes_notx.sql",
		"320_ops_network_minutes.sql",
	} {
		require.False(t, seen[name], "upstream migration %s must be renumbered in this fork", name)
	}
}
