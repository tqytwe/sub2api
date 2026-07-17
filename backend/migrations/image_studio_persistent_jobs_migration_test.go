package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioPersistentJobsMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("192_image_studio_persistent_jobs.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))
	normalized := strings.Join(strings.Fields(sql), " ")

	for _, fragment := range []string{
		"not valid",
		"validate constraint chk_image_studio_jobs_status_upgrade",
		"validate constraint image_studio_jobs_active_payload_chk_upgrade",
		"'cancelled'",
		"'partial'",
		"request_payload_encrypted",
		"model",
		"quality",
		"hold_amount",
		"hold_id",
		"success_count",
		"fail_count",
		"cancel_requested_at",
		"started_at",
		"finished_at",
		"heartbeat_at",
		"lease_owner",
		"lease_expires_at",
		"idempotency_key_hash",
		"idempotency_fingerprint",
		"create table if not exists image_studio_items",
		"'pending'",
		"'running'",
		"'success'",
		"'failed'",
		"'cancelled'",
		"actual_cost",
		"asset_id",
		"attempt_count",
		"'persisting'",
		"checkpoint_data",
		"checkpoint_content_type",
		"checkpoint_actual_cost",
	} {
		require.Contains(t, normalized, fragment)
	}

	require.NotContains(t, normalized, "create index")
	require.NotContains(t, normalized, "create unique index")
	require.NotContains(t, normalized, "begin")
	require.NotContains(t, normalized, "commit")
	require.NotContains(t, sql, "user_prompt")
	require.NotContains(t, sql, "expert_prompt")

	raw, err = os.ReadFile("192_image_studio_persistent_jobs_indexes_notx.sql")
	require.NoError(t, err)
	indexSQL := strings.ToLower(string(raw))
	indexNormalized := strings.Join(strings.Fields(indexSQL), " ")
	for _, fragment := range []string{
		"create index concurrently if not exists idx_image_studio_jobs_claim",
		"create index concurrently if not exists idx_image_studio_jobs_user_active",
		"create unique index concurrently if not exists uq_image_studio_jobs_user_idempotency",
		"create index concurrently if not exists idx_image_studio_items_job_status",
	} {
		require.Contains(t, indexNormalized, fragment)
	}
	require.NotContains(t, indexNormalized, "drop index")
	require.NotContains(t, indexNormalized, "begin")
	require.NotContains(t, indexNormalized, "commit")
}
