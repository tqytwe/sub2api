//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCheckinStreakRebuildUsesDatesAndStopsAtGap(t *testing.T) {
	userID := newCheckinExplorer(t, false, 30)
	otherUserID := newCheckinExplorer(t, false, 30)
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	// Persisted counters deliberately disagree with the real sequence.
	for day := 0; day < 6; day++ {
		require.NoError(t, repo.InsertCheckin(context.Background(), userID, end.AddDate(0, 0, -day), 0, 0))
	}
	require.NoError(t, repo.InsertCheckin(context.Background(), userID, end.AddDate(0, 0, -7), 0, 99))
	require.NoError(t, repo.InsertCheckin(context.Background(), otherUserID, end.AddDate(0, 0, -6), 0, 20))
	got, err := repo.CountConsecutiveCheckinsEndingAt(context.Background(), userID, end)
	require.NoError(t, err)
	require.Equal(t, 6, got)
	got, err = repo.CountConsecutiveCheckinsEndingAt(context.Background(), userID, end.AddDate(0, 0, 1))
	require.NoError(t, err)
	require.Zero(t, got)
	// A makeup record bridges the gap and includes the older day.
	require.NoError(t, repo.InsertCheckin(context.Background(), userID, end.AddDate(0, 0, -6), 0, 0))
	got, err = repo.CountConsecutiveCheckinsEndingAt(context.Background(), userID, end)
	require.NoError(t, err)
	require.Equal(t, 8, got)
	var changed int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM play_checkins WHERE user_id=$1 AND streak_count<>0 AND checkin_date<>$2::date`, userID, end.AddDate(0, 0, -7).Format("2006-01-02")).Scan(&changed))
	require.Zero(t, changed, "rebuilding must not mutate historical records")
}

func TestAdminCheckinReportUsesActualDatesAndStablePagination(t *testing.T) {
	userID := newCheckinExplorer(t, false, 30)
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	day := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		require.NoError(t, repo.InsertCheckin(context.Background(), userID, day.AddDate(0, 0, i), 0, 0))
	}
	filter := service.PlayAdminCheckinFilter{UserID: userID, StartDate: "2026-11-01", EndDate: "2026-11-03", Page: 1, PageSize: 2, AsOf: time.Now().UTC().Format(time.RFC3339Nano)}
	report, err := repo.ListAdminCheckins(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, 3, report.Total)
	require.Len(t, report.Items, 2)
	require.Equal(t, "2026-11-03", report.Items[0].Date)
	require.Equal(t, 3, report.Items[0].ObservedStreak)
	require.Equal(t, "unknown", report.Items[0].RewardMode)
	require.Equal(t, "legacy_evidence_missing", report.Items[0].EvidenceStatus)
	filter.Page = 2
	report, err = repo.ListAdminCheckins(context.Background(), filter)
	require.NoError(t, err)
	require.Len(t, report.Items, 1)
	require.Equal(t, 1, report.Items[0].ObservedStreak)
	filter.Page = 3
	report, err = repo.ListAdminCheckins(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, 3, report.Total)
	require.Empty(t, report.Items)
	filter.Page = 1
	filter.RewardMode = "redeemable"
	report, err = repo.ListAdminCheckins(context.Background(), filter)
	require.NoError(t, err)
	require.Zero(t, report.Total)
	require.Empty(t, report.Items)
}
