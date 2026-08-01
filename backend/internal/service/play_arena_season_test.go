package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type arenaSeasonRepo struct {
	PlayRepository

	monthly      *PlayArenaPeriod
	daily        *PlayArenaPeriod
	rows         []PlayArenaScoreRow
	history      []PlayArenaSeasonHistory
	rules        string
	historyCalls int
	frozenRules  string
	frozenID     int64
	dailyHistory *PlayArenaPeriod
	dailyLedger  []PlayArenaDailyRewardLedgerRow
}

func (r *arenaSeasonRepo) EnsureMonthlyArenaPeriod(context.Context, time.Time) (*PlayArenaPeriod, error) {
	return r.monthly, nil
}

func (r *arenaSeasonRepo) EnsureDailyArenaPeriod(context.Context, time.Time) (*PlayArenaPeriod, error) {
	return r.daily, nil
}

func (r *arenaSeasonRepo) ListArenaLeaderboard(context.Context, time.Time, time.Time, int) ([]PlayArenaScoreRow, error) {
	return append([]PlayArenaScoreRow(nil), r.rows...), nil
}

func (r *arenaSeasonRepo) GetUserArenaScore(_ context.Context, userID int64, _ time.Time, _ time.Time) (int64, int, error) {
	for _, row := range r.rows {
		if row.UserID == userID {
			return row.TokenSum, row.Rank, nil
		}
	}
	return 0, 0, nil
}

func (r *arenaSeasonRepo) GetArenaTokensToPrevRank(_ context.Context, _ int64, _ time.Time, _ time.Time, rank int, tokenSum int64) (int64, error) {
	if rank <= 1 {
		return 0, nil
	}
	for _, row := range r.rows {
		if row.Rank == rank-1 {
			return row.TokenSum - tokenSum, nil
		}
	}
	return 0, nil
}

func (r *arenaSeasonRepo) GetArenaRewardRulesSnapshot(context.Context, int64) (string, error) {
	return r.rules, nil
}

func (r *arenaSeasonRepo) FreezeArenaRewardRules(_ context.Context, periodID int64, rulesJSON string) error {
	r.frozenID = periodID
	r.frozenRules = rulesJSON
	return nil
}

func (r *arenaSeasonRepo) ListArenaSeasonHistory(context.Context, string, int) ([]PlayArenaSeasonHistory, error) {
	r.historyCalls++
	return append([]PlayArenaSeasonHistory(nil), r.history...), nil
}

func (r *arenaSeasonRepo) GetLatestSettledDailyArenaPeriod(context.Context) (*PlayArenaPeriod, error) {
	return r.dailyHistory, nil
}

func (r *arenaSeasonRepo) ListArenaDailyRewardLedger(context.Context, int64) ([]PlayArenaDailyRewardLedgerRow, error) {
	return append([]PlayArenaDailyRewardLedgerRow(nil), r.dailyLedger...), nil
}

func TestArenaSeasonUsesFrozenRewardRules(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	period := &PlayArenaPeriod{ID: 7, StartAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, shanghai), EndAt: time.Date(2026, time.September, 1, 0, 0, 0, 0, shanghai), PeriodType: "monthly", Status: "active"}
	frozen, err := json.Marshal([]PlayArenaSettlementTier{{RankMax: 1, Amount: 1000}, {RankMax: 10, Amount: 100}})
	require.NoError(t, err)

	repo := &arenaSeasonRepo{monthly: period, rules: string(frozen)}
	settings := NewSettingService(&dailyRewardSummarySettingRepo{values: map[string]string{
		SettingKeyPlayArenaEnabled:           "true",
		SettingKeyPlayArenaSettlementRewards: `[{"rank_max":1,"amount":1}]`,
	}}, nil)
	svc := NewPlayService(repo, nil, nil, settings, nil, nil)

	tiers, err := svc.arenaRewardTiersForPeriod(context.Background(), period)

	require.NoError(t, err)
	require.Equal(t, []PlayArenaSettlementTier{{RankMax: 1, Amount: 1000}, {RankMax: 10, Amount: 100}}, tiers)
}

func TestArenaSeasonOverviewKeepsPublicRowsAnonymousAndMarksOnlyMine(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, shanghai)
	period := &PlayArenaPeriod{ID: 9, Name: "2026-08", StartAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, shanghai), EndAt: time.Date(2026, time.September, 1, 0, 0, 0, 0, shanghai), PeriodType: "monthly", Status: "active"}
	repo := &arenaSeasonRepo{
		monthly: period,
		rows: []PlayArenaScoreRow{
			{Rank: 1, UserID: 1001, DisplayName: "al***@example.com", TokenSum: 22000},
			{Rank: 2, UserID: 1002, Anonymous: true, TokenSum: 18000},
		},
		history: []PlayArenaSeasonHistory{{Period: PlayArenaPeriod{ID: 8, Name: "2026-07", Status: "settled", PeriodType: "monthly"}, WinnersCount: 2, TotalAmount: 120}},
	}
	settings := NewSettingService(&dailyRewardSummarySettingRepo{values: map[string]string{
		SettingKeyPlayArenaEnabled:           "true",
		SettingKeyPlayArenaSettlementRewards: `[{"rank_max":1,"amount":100},{"rank_max":10,"amount":10}]`,
	}}, nil)
	svc := NewPlayService(repo, nil, nil, settings, nil, nil)
	svc.now = func() time.Time { return now }

	overview, err := svc.GetArenaSeasonOverview(context.Background(), 1002, "monthly")

	require.NoError(t, err)
	require.True(t, overview.Enabled)
	require.Equal(t, period.ID, overview.Period.ID)
	require.Len(t, overview.Rows, 2)
	require.False(t, overview.Rows[0].IsMine)
	require.True(t, overview.Rows[1].IsMine)
	require.True(t, overview.Rows[1].Anonymous)
	require.Equal(t, int64(0), overview.Rows[0].UserID)
	require.Equal(t, int64(0), overview.Rows[1].UserID)
	require.NotNil(t, overview.Current)
	require.Equal(t, 2, overview.Current.Rank)
	require.Len(t, overview.History, 1)
}

func TestArenaSeasonOverviewDefersHistoricalReadUntilRequested(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	period := &PlayArenaPeriod{ID: 9, Name: "2026-08", StartAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, shanghai), EndAt: time.Date(2026, time.September, 1, 0, 0, 0, 0, shanghai), PeriodType: "monthly", Status: "active"}
	repo := &arenaSeasonRepo{
		monthly: period,
		rows:    []PlayArenaScoreRow{{Rank: 1, UserID: 1001, DisplayName: "al***@example.com", TokenSum: 22000}},
		history: []PlayArenaSeasonHistory{{Period: PlayArenaPeriod{ID: 8, Name: "2026-07", Status: "settled", PeriodType: "monthly"}}},
	}
	settings := NewSettingService(&dailyRewardSummarySettingRepo{values: map[string]string{
		SettingKeyPlayArenaEnabled:           "true",
		SettingKeyPlayArenaSettlementRewards: `[{"rank_max":1,"amount":100}]`,
	}}, nil)
	svc := NewPlayService(repo, nil, nil, settings, nil, nil)

	overview, err := svc.GetArenaSeasonOverview(context.Background(), 0, "monthly", false)

	require.NoError(t, err)
	require.Empty(t, overview.History)
	require.Zero(t, repo.historyCalls)
}

func TestDailyArenaSeasonOverviewPreservesDailyPayoutProof(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	current := &PlayArenaPeriod{ID: 12, Name: "2026-08-02", StartAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, shanghai), EndAt: time.Date(2026, time.August, 3, 0, 0, 0, 0, shanghai), PeriodType: "daily", Status: "active"}
	settledAt := time.Date(2026, time.August, 2, 0, 5, 0, 0, shanghai)
	history := &PlayArenaPeriod{ID: 11, Name: "2026-08-01", StartAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, shanghai), EndAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, shanghai), PeriodType: "daily", Status: "settled", SettledAt: &settledAt}
	repo := &arenaSeasonRepo{
		daily:        current,
		dailyHistory: history,
		rows:         []PlayArenaScoreRow{{Rank: 1, UserID: 7, TokenSum: 1000}},
		dailyLedger: []PlayArenaDailyRewardLedgerRow{{
			Rank:        1,
			DisplayName: "mi***@example.com",
			TokenSum:    900,
			Amount:      0.5,
			CreatedAt:   settledAt,
		}},
	}
	settings := NewSettingService(&dailyRewardSummarySettingRepo{values: map[string]string{
		SettingKeyPlayArenaEnabled:         "true",
		SettingKeyPlayDailyArenaEnabled:    "true",
		SettingKeyPlayDailyArenaTopRewards: `[{"rank_max":1,"amount":0.5}]`,
	}}, nil)
	svc := NewPlayService(repo, nil, nil, settings, nil, nil)

	overview, err := svc.GetArenaSeasonOverview(context.Background(), 0, "daily", true)

	require.NoError(t, err)
	require.Len(t, overview.History, 1)
	require.Equal(t, int64(11), overview.History[0].Period.ID)
	require.Equal(t, 1, overview.History[0].WinnersCount)
	require.InDelta(t, 0.5, overview.History[0].TotalAmount, 1e-9)
	require.Len(t, overview.History[0].Winners, 1)
	require.Equal(t, "mi***@example.com", overview.History[0].Winners[0].DisplayName)
	require.Equal(t, "paid", overview.History[0].Winners[0].PayoutStatus)
}

func TestMonthlyArenaSettlementWaitsUntilShanghaiTenPastMidnight(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	period := &PlayArenaPeriod{EndAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, shanghai), PeriodType: "monthly"}

	require.False(t, isArenaPeriodSettlementDue(period, time.Date(2026, time.August, 1, 0, 9, 59, 0, shanghai)))
	require.True(t, isArenaPeriodSettlementDue(period, time.Date(2026, time.August, 1, 0, 10, 0, 0, shanghai)))
	require.True(t, isArenaPeriodSettlementDue(period, time.Date(2026, time.August, 2, 12, 0, 0, 0, shanghai)))
}

func TestPrepareCurrentArenaMonthlySeasonFreezesRulesWithoutPageTraffic(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	period := &PlayArenaPeriod{ID: 17, Name: "2026-08", StartAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, shanghai), EndAt: time.Date(2026, time.September, 1, 0, 0, 0, 0, shanghai), PeriodType: "monthly", Status: "active"}
	repo := &arenaSeasonRepo{monthly: period}
	settings := NewSettingService(&dailyRewardSummarySettingRepo{values: map[string]string{
		SettingKeyPlayArenaEnabled:           "true",
		SettingKeyPlayArenaSettlementRewards: `[{"rank_max":1,"amount":1000},{"rank_max":10,"amount":100}]`,
	}}, nil)
	svc := NewPlayService(repo, nil, nil, settings, nil, nil)

	prepared, err := svc.PrepareCurrentArenaMonthlySeason(context.Background(), time.Date(2026, time.August, 1, 0, 5, 0, 0, shanghai))

	require.NoError(t, err)
	require.Equal(t, int64(17), prepared.ID)
	require.Equal(t, int64(17), repo.frozenID)
	var tiers []PlayArenaSettlementTier
	require.NoError(t, json.Unmarshal([]byte(repo.frozenRules), &tiers))
	require.Equal(t, []PlayArenaSettlementTier{{RankMax: 1, Amount: 1000}, {RankMax: 10, Amount: 100}}, tiers)
}

type arenaSettlementGuardRepo struct {
	PlayRepository
	period *PlayArenaPeriod
}

func (r *arenaSettlementGuardRepo) GetArenaPeriodByID(context.Context, int64) (*PlayArenaPeriod, error) {
	return r.period, nil
}

func TestMonthlyArenaSettlementRejectsManualEarlyPublication(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	repo := &arenaSettlementGuardRepo{period: &PlayArenaPeriod{
		ID:         12,
		StartAt:    time.Date(2026, time.August, 1, 0, 0, 0, 0, shanghai),
		EndAt:      time.Date(2026, time.September, 1, 0, 0, 0, 0, shanghai),
		PeriodType: "monthly",
		Status:     "active",
	}}
	settings := NewSettingService(&dailyRewardSummarySettingRepo{values: map[string]string{
		SettingKeyPlayArenaEnabled: "true",
	}}, nil)
	svc := NewPlayService(repo, nil, nil, settings, nil, nil)
	svc.now = func() time.Time { return time.Date(2026, time.September, 1, 0, 5, 0, 0, shanghai) }

	_, err := svc.SettleArenaPeriod(context.Background(), 12)

	require.ErrorIs(t, err, ErrPlayArenaPeriodNotSettleable)
}

type arenaExpiredSeasonRepo struct {
	PlayRepository
	periods []PlayArenaPeriod
	loaded  []int64
}

func (r *arenaExpiredSeasonRepo) ListExpiredActiveMonthlyArenaPeriods(context.Context, time.Time) ([]PlayArenaPeriod, error) {
	return append([]PlayArenaPeriod(nil), r.periods...), nil
}

func (r *arenaExpiredSeasonRepo) CreateArenaSeasonSnapshot(context.Context, PlayArenaSeasonSnapshot) error {
	return nil
}

func (r *arenaExpiredSeasonRepo) GetArenaPeriodByID(_ context.Context, periodID int64) (*PlayArenaPeriod, error) {
	r.loaded = append(r.loaded, periodID)
	for index := range r.periods {
		if r.periods[index].ID == periodID {
			return &r.periods[index], nil
		}
	}
	return nil, nil
}

func TestExpiredMonthlyArenaSettlementScansPastAnEarlierFailure(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	repo := &arenaExpiredSeasonRepo{periods: []PlayArenaPeriod{
		{ID: 1, StartAt: time.Date(2026, time.June, 1, 0, 0, 0, 0, shanghai), EndAt: time.Date(2026, time.July, 1, 0, 0, 0, 0, shanghai), PeriodType: "monthly", Status: "active"},
		{ID: 2, StartAt: time.Date(2026, time.July, 1, 0, 0, 0, 0, shanghai), EndAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, shanghai), PeriodType: "monthly", Status: "active"},
	}}
	settings := NewSettingService(&dailyRewardSummarySettingRepo{values: map[string]string{
		SettingKeyPlayArenaEnabled: "true",
	}}, nil)
	svc := NewPlayService(repo, nil, nil, settings, nil, nil)

	settled, err := svc.SettleExpiredMonthlyArenaPeriods(context.Background(), time.Date(2026, time.August, 1, 0, 10, 0, 0, shanghai))

	require.Zero(t, settled)
	require.Error(t, err)
	require.Equal(t, []int64{1, 2}, repo.loaded)
}
