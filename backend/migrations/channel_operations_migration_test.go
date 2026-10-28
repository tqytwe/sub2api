//go:build unit

package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelOperationsMigrationContract(t *testing.T) {
	const migration = "282_channel_operations.sql"
	content, err := FS.ReadFile(migration)
	require.NoError(t, err)
	requireMigrationChecksum(t, content, "aa8ca6209d84eece240a0b5dd8d6d923f500b818efe1befe98c9390598a9a45d")

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.True(t, strings.HasPrefix(sql, "SET LOCAL lock_timeout = '2s'; SET LOCAL statement_timeout = '30s';"))
	require.Contains(t, sql, "ALTER TABLE channels ADD COLUMN IF NOT EXISTS operations_revision BIGINT NOT NULL DEFAULT 0")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS channel_operation_upstreams")
	require.Contains(t, sql, "UNIQUE (channel_id, name)")
	require.Contains(t, sql, "UNIQUE (id, channel_id)")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS channel_operation_members")
	require.Contains(t, sql, "PRIMARY KEY (channel_id, account_id)")
	require.Contains(t, sql, "FOREIGN KEY (upstream_id, channel_id) REFERENCES channel_operation_upstreams(id, channel_id) ON DELETE CASCADE")
	require.Contains(t, sql, "ON channel_operation_members(upstream_id, account_id)")
	require.Contains(t, sql, "does not change account_groups, scheduler, or historical usage attribution")

	for _, forbidden := range []string{
		"INSERT INTO usage_logs",
		"UPDATE usage_logs",
		"DELETE FROM usage_logs",
		"UPDATE account_groups",
		"DELETE FROM account_groups",
		"CREATE TABLE IF NOT EXISTS account_token_guard_v2_accounts",
	} {
		require.NotContains(t, sql, forbidden)
	}
}

func TestChannelOperationsMigrationUsesUniqueFullFilename(t *testing.T) {
	files, err := FS.ReadDir(".")
	require.NoError(t, err)

	count := 0
	for _, file := range files {
		if file.Name() == "282_channel_operations.sql" {
			count++
		}
		require.NotEqual(t, "309_channel_operations.sql", file.Name())
		require.NotEqual(t, "309_credential_operation_refresh_failure.sql", file.Name())
	}
	require.Equal(t, 1, count)
}

func TestOAuthRefreshFailureMigrationContract(t *testing.T) {
	const migration = "283_account_oauth_refresh_failures.sql"
	content, err := FS.ReadFile(migration)
	require.NoError(t, err)
	requireMigrationChecksum(t, content, "8d5e6cc30f6c32e8238feeb4816f047e91ffb0071d0776016e3ab515a55eb559")

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.True(t, strings.HasPrefix(sql, "SET LOCAL lock_timeout = '2s'; SET LOCAL statement_timeout = '30s';"))
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS account_oauth_refresh_failures")
	require.Contains(t, sql, "account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE")
	require.Contains(t, sql, "failure_code TEXT NOT NULL CHECK (failure_code IN (")
	require.Contains(t, sql, "last_failure_at TIMESTAMPTZ NOT NULL")
	allowlist := regexp.MustCompile(`failure_code TEXT NOT NULL CHECK \(failure_code IN \(([^)]*)\)\)`).FindStringSubmatch(sql)
	require.Len(t, allowlist, 2, "failure code constraint must remain an explicit allowlist")
	codeMatches := regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(allowlist[1], -1)
	codes := make([]string, 0, len(codeMatches))
	for _, match := range codeMatches {
		codes = append(codes, match[1])
	}
	require.Equal(t, []string{
		"phone_verification_required",
		"account_deactivated",
		"account_deleted",
		"account_retired",
		"requires_relogin",
		"password_rejected",
		"mfa_required",
		"security_check",
		"authorization_denied",
		"rate_limited",
		"timeout",
		"transport_error",
		"service_error",
		"unknown",
	}, codes)
	require.Contains(t, sql, "no response text, credentials, scheduling or billing effects")

	for _, forbidden := range []string{
		"response_body",
		"error_message",
		"access_token",
		"refresh_token",
		"UPDATE accounts",
		"INSERT INTO usage_logs",
		"UPDATE usage_logs",
		"account_token_guard_v2_accounts",
	} {
		require.NotContains(t, sql, forbidden)
	}
}

func requireMigrationChecksum(t *testing.T, content []byte, expected string) {
	t.Helper()
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(content))))
	require.Equal(t, expected, hex.EncodeToString(sum[:]))
}
