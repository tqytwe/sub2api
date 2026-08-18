//go:build unit

package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorAccountSoftDeleteMigration(t *testing.T) {
	content, err := FS.ReadFile("227_channel_monitor_account_soft_delete_unbind.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE OR REPLACE FUNCTION clear_channel_monitor_account_on_soft_delete()")
	require.Contains(t, sql, "OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL")
	require.Contains(t, sql, "UPDATE channel_monitors SET account_id = NULL WHERE account_id = NEW.id")
	require.Contains(t, sql, "DROP TRIGGER IF EXISTS accounts_channel_monitor_soft_delete_unbind ON accounts")
	require.Contains(t, sql, "AFTER UPDATE OF deleted_at ON accounts")
	require.Contains(t, sql, "UPDATE channel_monitors AS monitors")
	require.Contains(t, sql, "WHERE accounts.deleted_at IS NOT NULL")
}
