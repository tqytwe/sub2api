//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Real PostgreSQL is required here: sqlmock cannot reject an untyped, unused
// bind parameter in the count query, which caused the production list to 500.
func TestModerationUsersPostgreSQLFiltersAndPagination(t *testing.T) {
	ctx := context.Background()
	repo := NewContentModerationRepository(integrationDB).(*contentModerationRepository)
	user := mustCreateUser(t, integrationEntClient, &service.User{})
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO content_moderation_user_state(user_id,auto_ban_exempt) VALUES($1,TRUE)`, user.ID)
	require.NoError(t, err)

	for _, tc := range []struct {
		filter string
		total  int64
	}{
		{"all", 1}, {"exempt", 1}, {"enforced", 0},
		{"auto_ban_exempt", 1}, {"keyword_exempt", 0},
		{"api_audit_exempt", 0}, {"email_exempt", 0},
	} {
		t.Run(tc.filter, func(t *testing.T) {
			items, total, err := repo.ListModerationUsers(ctx, 1, 1, tc.filter, user.ID, time.Now().Add(-24*time.Hour), true)
			require.NoError(t, err)
			require.Equal(t, tc.total, total)
			require.Len(t, items, int(tc.total))
			if tc.total > 0 {
				require.Equal(t, user.ID, items[0].UserID)
				require.True(t, items[0].AutoBanExempt)
			}
		})
	}

	items, total, err := repo.ListModerationUsers(ctx, 2, 1, "all", user.ID, time.Now().Add(-24*time.Hour), false)
	require.NoError(t, err)
	require.Equal(t, int64(1), total, "pagination must not change the total")
	require.Empty(t, items)

	unseen := mustCreateUser(t, integrationEntClient, &service.User{})
	items, total, err = repo.ListModerationUsers(ctx, 1, 20, "all", unseen.ID, time.Now().Add(-24*time.Hour), false)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Zero(t, items[0].Version)
	require.Nil(t, items[0].UpdatedAt)
}
