//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPriorityLatencySeriesPostgresFiltersCompletedModelAndGroup(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	suffix := uuid.NewString()
	user := mustCreateUser(t, client, &service.User{Email: "priority-trend-" + suffix + "@test.invalid"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "priority-trend-" + suffix})
	account := mustCreateAccount(t, client, &service.Account{Name: "priority-trend-" + suffix, Platform: service.PlatformOpenAI})
	group := mustCreateGroup(t, client, &service.Group{Name: "priority-a-" + suffix, Platform: service.PlatformOpenAI})
	other := mustCreateGroup(t, client, &service.Group{Name: "priority-b-" + suffix, Platform: service.PlatformOpenAI})
	repo := newUsageLogRepositoryWithSQL(client, tx)
	base := time.Now().UTC().Truncate(5 * time.Minute).Add(-time.Minute)
	add := func(groupID int64, model string, ttft int, complete, traced bool) {
		t.Helper()
		duration := 2000
		row := &service.UsageLog{
			UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, GroupID: &groupID,
			RequestID: uuid.NewString(), RequestedModel: model, Model: "mapped-upstream-model",
			InputTokens: 10, OutputTokens: 10, DurationMs: &duration, FirstTokenMs: &ttft,
			Stream: true, RateMultiplier: 1, CreatedAt: base.Add(time.Second),
		}
		inserted, err := repo.Create(ctx, row)
		require.NoError(t, err)
		require.True(t, inserted)
		if !traced {
			return
		}
		detail, err := json.Marshal(map[string]any{
			"version": 1, "started_at": base.Format(time.RFC3339Nano),
			"outcome": "success", "status": 200, "body_complete": complete,
			"canceled": false, "truncated": false, "client_disconnect": false,
			"downstream_error": false, "terminal": "completed",
		})
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO request_timing_details(usage_log_id,trace_id,created_at,detail)
 VALUES($1,$2::uuid,$3,$4::jsonb)`, row.ID, uuid.NewString(), row.CreatedAt, string(detail))
		require.NoError(t, err)
	}
	add(group.ID, "public-model", 100, true, true)
	add(group.ID, "public-model", 300, true, true)
	add(other.ID, "public-model", 1500, true, true)
	add(group.ID, "other-model", 1500, true, true)
	add(group.ID, "public-model", 1500, false, true)
	add(group.ID, "public-model", 1500, true, false)

	rows, err := repo.ReadPriorityLatencySeries(ctx, service.PriorityLatencySeriesQuery{
		AccountIDs: []int64{account.ID}, Model: "public-model", GroupID: &group.ID,
		Since: base.Add(-time.Minute), BucketMinutes: 5, SlowTTFTMs: 200,
	})
	require.NoError(t, err, "execute the trend query against migrated PostgreSQL")
	require.Len(t, rows, 1)
	require.Equal(t, 2, rows[0].Samples)
	require.InDelta(t, 200, rows[0].P50TTFTMs, 1e-9)
	require.InDelta(t, 280, rows[0].P90TTFTMs, 1e-9)
	require.Equal(t, 1, rows[0].SlowSamples)
	require.Nil(t, rows[0].ErrorSamples, "successful-only population does not prove zero errors")
}
