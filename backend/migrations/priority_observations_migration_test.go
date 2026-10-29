package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPriorityObservationsMigrationIsBoundedAndIdentityScoped(t *testing.T) {
	path := filepath.Join("310_gateway_request_priority_observations.sql")
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	sql := string(b)
	require.Contains(t, sql, "attempt_id UUID PRIMARY KEY REFERENCES gateway_request_attempt_evidence(attempt_id)")
	require.Contains(t, sql, "created_at TIMESTAMPTZ NOT NULL")
	require.Contains(t, sql, "inbound_endpoint")
	require.Contains(t, sql, "group_id")
	require.Contains(t, sql, "ON DELETE CASCADE")
	require.Contains(t, sql, "created_at, attempt_id")
	require.NotContains(t, strings.ToLower(sql), "request_body")
	require.NotContains(t, strings.ToLower(sql), "response_body")
}
