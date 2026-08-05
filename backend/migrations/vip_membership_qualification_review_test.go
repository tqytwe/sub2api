package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVIPQualificationReviewMigrationIsSchemaOnly(t *testing.T) {
	raw, err := FS.ReadFile("251_vip_membership_qualification_review.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))
	require.Contains(t, sql, "qualification_state")
	require.Contains(t, sql, "pending_review")
	require.Contains(t, sql, "reviewed_by")
	// Historical values are handled by the standalone reconciliation command,
	// never by application startup migration.
	require.NotContains(t, sql, "insert into play_membership_order_contributions")
	require.NotContains(t, sql, "update play_membership_order_contributions")
}
