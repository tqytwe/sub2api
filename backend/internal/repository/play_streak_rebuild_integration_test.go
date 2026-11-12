//go:build integration

package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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

func TestAdminCheckinAccountAndContinuityEvidence(t *testing.T) {
	ctx := context.Background()
	userID := newCheckinExplorer(t, false, 30)
	otherID := newCheckinExplorer(t, false, 30)
	name := fmt.Sprintf("evidence_%%_%d", userID)
	_, err := integrationDB.Exec(`UPDATE users SET username=$1 WHERE id=$2`, name, userID)
	require.NoError(t, err)
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	anchor := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	// Nov 1 is a real gap. Nov 3 was only made up after the Nov 4 reward.
	for _, d := range []string{"2026-10-31", "2026-11-02", "2026-11-03", "2026-11-04"} {
		created := anchor.Add(-time.Hour)
		if d == "2026-11-03" {
			created = anchor.Add(time.Hour)
		}
		if d == "2026-11-04" {
			created = anchor
		}
		_, err = integrationDB.Exec(`INSERT INTO play_checkins(user_id,checkin_date,streak_count,reward_amount,created_at) VALUES($1,$2::date,99,7,$3)`, userID, d, created)
		require.NoError(t, err)
	}
	require.NoError(t, repo.InsertCheckin(ctx, otherID, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), 0, 100))
	for source, amount := range map[string]int{"checkin": 2, "checkin_milestone": 5} {
		_, err = integrationDB.Exec(`INSERT INTO play_reward_ledger(user_id,source,amount,idempotency_key,detail,created_at) VALUES($1,$2,$3,$4,'{"checkin_date":"2026-11-04"}',$5)`, userID, source, amount, fmt.Sprintf("evidence:%d:%s", userID, source), anchor)
		require.NoError(t, err)
	}
	filter := service.PlayAdminCheckinFilter{Account: strings.ToUpper(name), StartDate: "2026-11-04", EndDate: "2026-11-04", Page: 1, PageSize: 1, AsOf: anchor.Add(2 * time.Hour).Format(time.RFC3339Nano)}
	report, err := repo.ListAdminCheckins(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 1, report.Total)
	require.Len(t, report.Items, 1)
	row := report.Items[0]
	require.Equal(t, userID, row.UserID)
	require.Equal(t, name, row.Username)
	require.Equal(t, 99, row.StoredStreak)
	require.Equal(t, 3, row.ObservedStreak)
	require.Equal(t, 1, row.StreakAtCheckin, "later makeup must not rewrite reward-time continuity")
	require.Equal(t, "2026-11-02", row.StreakStartDate)
	require.Equal(t, "2026-10-31", *row.PreviousCheckinDate)
	require.Equal(t, "2026-11-01", *row.GapStartDate)
	require.Equal(t, "2026-11-01", *row.GapEndDate)
	require.Equal(t, 1, row.LateRecordedDays)
	require.Len(t, row.Continuity, 3, "evidence crosses date filter and page boundary")
	require.Equal(t, "2026-11-02", row.Continuity[0].Date)
	require.True(t, row.Continuity[1].CreatedAt.After(row.CreatedAt))
	require.InDelta(t, 2, decimalAmount(t, row.DailyLedgerAmount), 0.000001)
	require.InDelta(t, 5, decimalAmount(t, row.MilestoneLedgerAmount), 0.000001)
	require.InDelta(t, 7, decimalAmount(t, row.LedgerAmount), 0.000001)
	filter.Account = strings.ToUpper(row.Email)
	report, err = repo.ListAdminCheckins(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 1, report.Total)
	filter.Account = "definitely-no-such-account"
	report, err = repo.ListAdminCheckins(ctx, filter)
	require.NoError(t, err)
	require.Zero(t, report.Total)
	filter.Account = name
	filter.AsOf = anchor.Add(30 * time.Minute).Format(time.RFC3339Nano)
	report, err = repo.ListAdminCheckins(ctx, filter)
	require.NoError(t, err)
	row = report.Items[0]
	require.Equal(t, 1, row.ObservedStreak, "as-of excludes the later makeup from all evidence")
	require.Equal(t, 1, row.StreakAtCheckin)
	require.Zero(t, row.LateRecordedDays)
	require.Len(t, row.Continuity, 1)
	require.Equal(t, "2026-11-03", *row.GapStartDate)
	filter.StartDate, filter.EndDate = "2026-10-31", "2026-10-31"
	report, err = repo.ListAdminCheckins(ctx, filter)
	require.NoError(t, err)
	require.Nil(t, report.Items[0].PreviousCheckinDate)
	require.Nil(t, report.Items[0].GapStartDate, "do not invent gaps before the earliest known check-in")
}

func decimalAmount(t *testing.T, value string) float64 {
	t.Helper()
	amount, err := strconv.ParseFloat(value, 64)
	require.NoError(t, err)
	return amount
}
