package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

type dailyRewardSummarySettingRepo struct {
	SettingRepository
	values map[string]string
}

func (r *dailyRewardSummarySettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

type dailyRewardSummaryRepo struct {
	PlayRepository

	latestSettled          *PlayArenaPeriod
	currentPeriod          *PlayArenaPeriod
	settledLedgerRows      []PlayArenaDailyRewardLedgerRow
	settledLeaderboardRows []PlayArenaScoreRow
	currentLeaderboardRows []PlayArenaScoreRow
	leaderboardLimits      []int
}

func (r *dailyRewardSummaryRepo) GetLatestSettledDailyArenaPeriod(context.Context) (*PlayArenaPeriod, error) {
	return r.latestSettled, nil
}

func (r *dailyRewardSummaryRepo) ListArenaDailyRewardLedger(_ context.Context, periodID int64) ([]PlayArenaDailyRewardLedgerRow, error) {
	if r.latestSettled == nil || periodID != r.latestSettled.ID {
		return nil, nil
	}
	return append([]PlayArenaDailyRewardLedgerRow(nil), r.settledLedgerRows...), nil
}

func (r *dailyRewardSummaryRepo) EnsureDailyArenaPeriod(context.Context, time.Time) (*PlayArenaPeriod, error) {
	return r.currentPeriod, nil
}

func (r *dailyRewardSummaryRepo) ListArenaLeaderboard(_ context.Context, start, _ time.Time, limit int) ([]PlayArenaScoreRow, error) {
	r.leaderboardLimits = append(r.leaderboardLimits, limit)
	if r.latestSettled != nil && start.Equal(r.latestSettled.StartAt) {
		return append([]PlayArenaScoreRow(nil), r.settledLeaderboardRows...), nil
	}
	return append([]PlayArenaScoreRow(nil), r.currentLeaderboardRows...), nil
}

func TestDailyArenaRewardSummaryCombinesSettledLedgerAndCurrentEstimate(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, time.July, 21, 10, 30, 0, 0, shanghai)
	settledAt := time.Date(2026, time.July, 21, 0, 8, 0, 0, shanghai)
	settledPeriod := &PlayArenaPeriod{
		ID:         100,
		Name:       "2026-07-20",
		StartAt:    time.Date(2026, time.July, 20, 0, 0, 0, 0, shanghai),
		EndAt:      time.Date(2026, time.July, 21, 0, 0, 0, 0, shanghai),
		Status:     "settled",
		PeriodType: "daily",
		SettledAt:  &settledAt,
	}
	currentPeriod := &PlayArenaPeriod{
		ID:         101,
		Name:       "2026-07-21",
		StartAt:    time.Date(2026, time.July, 21, 0, 0, 0, 0, shanghai),
		EndAt:      time.Date(2026, time.July, 22, 0, 0, 0, 0, shanghai),
		Status:     "active",
		PeriodType: "daily",
	}
	repo := &dailyRewardSummaryRepo{
		latestSettled: settledPeriod,
		currentPeriod: currentPeriod,
		settledLedgerRows: []PlayArenaDailyRewardLedgerRow{
			{UserID: 501, DisplayName: "星河工作流", Amount: 0.5, Rank: 0, TokenSum: 0},
			{UserID: 502, DisplayName: "Daily Two", Amount: 0.2, Rank: 2, TokenSum: 8000},
		},
		settledLeaderboardRows: []PlayArenaScoreRow{
			{UserID: 501, DisplayName: "星河工作流", Rank: 1, TokenSum: 12000},
			{UserID: 502, DisplayName: "Daily Two", Rank: 2, TokenSum: 8000},
		},
		currentLeaderboardRows: []PlayArenaScoreRow{
			{UserID: 701, DisplayName: "Current One", Rank: 1, TokenSum: 22000},
			{UserID: 702, DisplayName: "Current Two", Rank: 2, TokenSum: 17000},
			{UserID: 703, DisplayName: "Current Three", Rank: 3, TokenSum: 9000},
		},
	}
	tiers, err := json.Marshal([]PlayArenaSettlementTier{
		{RankMax: 1, Amount: 0.5},
		{RankMax: 3, Amount: 0.2},
		{RankMax: 10, Amount: 0.1},
	})
	require.NoError(t, err)
	settings := NewSettingService(&dailyRewardSummarySettingRepo{values: map[string]string{
		SettingKeyPlayArenaEnabled:         "true",
		SettingKeyPlayDailyArenaEnabled:    "true",
		SettingKeyPlayDailyArenaTopRewards: string(tiers),
	}}, nil)
	svc := NewPlayService(repo, nil, nil, settings, nil, nil)
	svc.now = func() time.Time { return now }

	summary, err := svc.GetDailyArenaRewardSummary(context.Background())

	require.NoError(t, err)
	require.True(t, summary.Enabled)
	require.NotNil(t, summary.Recent)
	require.Equal(t, settledPeriod.ID, summary.Recent.Period.ID)
	require.Equal(t, &settledAt, summary.Recent.SettledAt)
	require.True(t, summary.Recent.PaidToday)
	require.Equal(t, 2, summary.Recent.WinnersCount)
	require.InDelta(t, 0.7, summary.Recent.TotalAmount, 0.00000001)
	require.Len(t, summary.Recent.Winners, 2)
	require.Equal(t, 1, summary.Recent.Winners[0].Rank)
	require.Equal(t, int64(12000), summary.Recent.Winners[0].TokenSum)
	require.Equal(t, "星河工作流", summary.Recent.Winners[0].DisplayName)

	require.NotNil(t, summary.Current)
	require.Equal(t, currentPeriod.ID, summary.Current.Period.ID)
	require.Len(t, summary.Current.Rows, 3)
	require.Equal(t, 1, summary.Current.Rows[0].Rank)
	require.Equal(t, int64(22000), summary.Current.Rows[0].TokenSum)
	require.InDelta(t, 0.5, summary.Current.Rows[0].EstimatedReward, 0.00000001)
	require.InDelta(t, 0.2, summary.Current.Rows[2].EstimatedReward, 0.00000001)
	require.Contains(t, repo.leaderboardLimits, 10)
}

func TestDailyArenaRewardSummaryCapsRecentWinnersAndMarksDelayedSettlement(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, time.July, 21, 10, 30, 0, 0, shanghai)
	settledAt := time.Date(2026, time.July, 20, 23, 59, 0, 0, shanghai)
	settledPeriod := &PlayArenaPeriod{
		ID:         99,
		Name:       "2026-07-19",
		StartAt:    time.Date(2026, time.July, 19, 0, 0, 0, 0, shanghai),
		EndAt:      time.Date(2026, time.July, 20, 0, 0, 0, 0, shanghai),
		Status:     "settled",
		PeriodType: "daily",
		SettledAt:  &settledAt,
	}
	repo := &dailyRewardSummaryRepo{latestSettled: settledPeriod}
	for rank := 1; rank <= 12; rank++ {
		repo.settledLedgerRows = append(repo.settledLedgerRows, PlayArenaDailyRewardLedgerRow{
			UserID:      int64(600 + rank),
			DisplayName: "winner",
			Amount:      0.1,
			Rank:        rank,
			TokenSum:    int64(1000 - rank),
		})
	}
	settings := NewSettingService(&dailyRewardSummarySettingRepo{values: map[string]string{
		SettingKeyPlayArenaEnabled:      "true",
		SettingKeyPlayDailyArenaEnabled: "true",
	}}, nil)
	svc := NewPlayService(repo, nil, nil, settings, nil, nil)
	svc.now = func() time.Time { return now }

	summary, err := svc.GetDailyArenaRewardSummary(context.Background())

	require.NoError(t, err)
	require.NotNil(t, summary.Recent)
	require.False(t, summary.Recent.PaidToday)
	require.Equal(t, 12, summary.Recent.WinnersCount)
	require.Len(t, summary.Recent.Winners, 10)
	require.InDelta(t, 1.2, summary.Recent.TotalAmount, 0.00000001)
	require.Nil(t, summary.Current)
}

func TestDailyArenaSettlementWritesPeriodRankAndTokenDetail(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	period := &PlayArenaPeriod{
		ID:         44,
		Name:       "2026-07-20",
		StartAt:    time.Date(2026, time.July, 20, 0, 0, 0, 0, shanghai),
		EndAt:      time.Date(2026, time.July, 21, 0, 0, 0, 0, shanghai),
		Status:     "active",
		PeriodType: "daily",
	}
	repo := &dailyArenaSettlementRepo{
		period: period,
		rows: []PlayArenaScoreRow{
			{UserID: 301, DisplayName: "Daily One", Rank: 1, TokenSum: 9000},
		},
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	mock.ExpectBegin()
	mock.ExpectCommit()

	svc := NewPlayService(repo, nil, nil, nil, nil, client)
	result, err := svc.SettleDailyArenaPeriod(context.Background(), period.ID)

	require.NoError(t, err)
	require.Equal(t, 1, result.WinnersCount)
	require.Len(t, repo.ledgerEntries, 1)
	detail := repo.ledgerEntries[0].Detail
	require.Equal(t, period.ID, detail["period_id"])
	require.Equal(t, period.Name, detail["period_name"])
	require.Equal(t, "daily", detail["period_type"])
	require.Equal(t, period.StartAt.Format(time.RFC3339), detail["period_start"])
	require.Equal(t, period.EndAt.Format(time.RFC3339), detail["period_end"])
	require.Equal(t, 1, detail["rank"])
	require.Equal(t, int64(9000), detail["token"])
	require.True(t, repo.markedSettled)
	require.NoError(t, mock.ExpectationsWereMet())
}

type dailyArenaSettlementRepo struct {
	PlayRepository
	period         *PlayArenaPeriod
	rows           []PlayArenaScoreRow
	ledgerEntries  []PlayRewardLedgerEntry
	balanceUpdates []float64
	markedSettled  bool
}

func (r *dailyArenaSettlementRepo) GetArenaPeriodByID(_ context.Context, periodID int64) (*PlayArenaPeriod, error) {
	if r.period == nil || r.period.ID != periodID {
		return nil, nil
	}
	return r.period, nil
}

func (r *dailyArenaSettlementRepo) ListArenaLeaderboard(context.Context, time.Time, time.Time, int) ([]PlayArenaScoreRow, error) {
	return append([]PlayArenaScoreRow(nil), r.rows...), nil
}

func (r *dailyArenaSettlementRepo) InsertRewardLedger(_ context.Context, entry PlayRewardLedgerEntry) error {
	r.ledgerEntries = append(r.ledgerEntries, entry)
	return nil
}

func (r *dailyArenaSettlementRepo) UpdatePlayBalance(_ context.Context, _ int64, amount float64) error {
	r.balanceUpdates = append(r.balanceUpdates, amount)
	return nil
}

func (r *dailyArenaSettlementRepo) MarkArenaPeriodSettled(_ context.Context, periodID int64) error {
	if r.period != nil && r.period.ID == periodID {
		r.markedSettled = true
		return nil
	}
	return ErrPlayArenaPeriodNotSettleable
}
