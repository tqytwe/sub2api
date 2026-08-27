package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMobileVideoJobsMigrationKeepsRequestsPrivateAndExtendsTaskKinds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("259_mobile_video_jobs.sql"))
	require.NoError(t, err)
	text := string(body)
	require.Contains(t, text, "SET LOCAL lock_timeout = '5s'")
	require.Contains(t, text, "'video'")
	require.Contains(t, text, "CREATE TABLE IF NOT EXISTS mobile_video_jobs")
	require.Contains(t, text, "execution_api_key_id")
	require.Contains(t, text, "submission_unknown")
	require.NotContains(t, text, "UPDATE groups")
	require.NotContains(t, text, "UPDATE accounts")
}

func TestMobileVideoExecutionBillingMigrationIsAdditiveAndReplaySafe(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("261_mobile_video_execution_billing.sql"))
	require.NoError(t, err)
	text := string(body)
	require.Contains(t, text, "SET LOCAL lock_timeout = '5s'")
	require.Contains(t, text, "ADD COLUMN IF NOT EXISTS execution_snapshot JSONB")
	require.Contains(t, text, "ADD COLUMN IF NOT EXISTS billing_state VARCHAR(24) NOT NULL DEFAULT 'funding'")
	require.Contains(t, text, "'funding', 'reserved', 'releasing', 'captured', 'released', 'not_required'")
	require.Contains(t, text, "client_cancelled_at TIMESTAMPTZ")
	require.Contains(t, text, "execution_snapshot IS NULL")
	require.Contains(t, text, "unit_price_usd IS NULL")
	require.Contains(t, text, "execution_snapshot IS NOT NULL")
	require.Contains(t, text, "'funding', 'queued', 'submitting', 'submission_unknown', 'polling', 'completed', 'failed', 'cancelled'")
	require.Contains(t, text, "DROP CONSTRAINT IF EXISTS mobile_video_jobs_billing_state_check")
	require.Contains(t, text, "idx_mobile_video_jobs_funding_recovery")
	require.Contains(t, text, "idx_mobile_video_jobs_releasing_recovery")
	require.Contains(t, text, "idx_mobile_video_jobs_user_active")
	require.NotContains(t, text, "UPDATE groups")
	require.NotContains(t, text, "UPDATE api_keys")
}
