package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestTeamSettlementUsesShanghaiMonthActualCostAndSnapshotAllocation(t *testing.T) {
	repo := newTeamSettlementRepo()
	repo.contributions = []TeamContribution{
		{UserID: 11, Amount: decimal.RequireFromString("20.00000000")},
		{UserID: 19, Amount: decimal.RequireFromString("10.00000000")},
	}
	svc := &PlayService{repo: repo}

	settlement, err := svc.settleTeamRewardMonth(
		context.Background(),
		7,
		time.Date(2026, time.June, 18, 9, 0, 0, 0, time.UTC),
		defaultTeamRewardConfig(),
	)

	require.NoError(t, err)
	require.NotNil(t, settlement)
	require.Equal(t, "2026-06-01", settlement.PeriodStart.Format("2006-01-02"))
	require.Equal(t, time.Date(2026, time.May, 31, 16, 0, 0, 0, time.UTC), repo.windowStart.UTC())
	require.Equal(t, time.Date(2026, time.June, 30, 16, 0, 0, 0, time.UTC), repo.windowEnd.UTC())
	require.Equal(t, "30.00000000", settlement.TeamSpend.StringFixed(8))
	require.Equal(t, "20.00000000", settlement.ReachedThreshold.StringFixed(8))
	require.Equal(t, "0.02000000", settlement.RewardRate.StringFixed(8))
	require.Equal(t, "0.60000000", settlement.PoolAmount.StringFixed(8))
	require.Equal(t, "250.00000000", settlement.CapAmount.StringFixed(8))
	require.Equal(t, serviceAllocationAmounts{
		11: "0.40000000",
		19: "0.20000000",
	}, rewardAllocationAmounts(repo.allocations))
	require.Equal(t, "team_reward:7:2026-06:11", allocationForUser(t, repo.allocations, 11).IdempotencyKey)
	require.Equal(t, "team_reward:7:2026-06:19", allocationForUser(t, repo.allocations, 19).IdempotencyKey)
}

func TestTeamSettlementSnapshotsHighestTierAndCap(t *testing.T) {
	repo := newTeamSettlementRepo()
	repo.contributions = []TeamContribution{{
		UserID: 11,
		Amount: decimal.RequireFromString("5000.00000000"),
	}}
	svc := &PlayService{repo: repo}

	settlement, err := svc.settleTeamRewardMonth(
		context.Background(),
		7,
		time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		defaultTeamRewardConfig(),
	)

	require.NoError(t, err)
	require.Equal(t, "2000.00000000", settlement.ReachedThreshold.StringFixed(8))
	require.Equal(t, "0.05000000", settlement.RewardRate.StringFixed(8))
	require.Equal(t, "250.00000000", settlement.PoolAmount.StringFixed(8))
	require.Equal(t, "250.00000000", allocationForUser(t, repo.allocations, 11).RewardAmount.StringFixed(8))
}

func TestTeamSettlementBelowFirstTierDoesNotCreateSnapshot(t *testing.T) {
	repo := newTeamSettlementRepo()
	repo.contributions = []TeamContribution{{
		UserID: 11,
		Amount: decimal.RequireFromString("19.99999999"),
	}}
	svc := &PlayService{repo: repo}

	settlement, err := svc.settleTeamRewardMonth(
		context.Background(),
		7,
		time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		defaultTeamRewardConfig(),
	)

	require.NoError(t, err)
	require.Nil(t, settlement)
	require.Zero(t, repo.createCalls)
	require.Empty(t, repo.allocations)
}

func TestTeamSettlementCompletedSnapshotIsNeverRecalculated(t *testing.T) {
	repo := newTeamSettlementRepo()
	repo.contributions = []TeamContribution{{
		UserID: 11,
		Amount: decimal.RequireFromString("30.00000000"),
	}}
	svc := &PlayService{repo: repo}
	period := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

	first, err := svc.settleTeamRewardMonth(
		context.Background(),
		7,
		period,
		defaultTeamRewardConfig(),
	)
	require.NoError(t, err)
	first.Status = PlayTeamSettlementStatusCompleted
	repo.settlement.Status = PlayTeamSettlementStatusCompleted
	repo.contributions = []TeamContribution{{
		UserID: 11,
		Amount: decimal.RequireFromString("10000.00000000"),
	}}

	second, err := svc.settleTeamRewardMonth(
		context.Background(),
		7,
		period,
		defaultTeamRewardConfig(),
	)

	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, "30.00000000", second.TeamSpend.StringFixed(8))
	require.Equal(t, "0.60000000", second.PoolAmount.StringFixed(8))
	require.Equal(t, 1, repo.contributionCalls)
	require.Equal(t, 1, repo.createCalls)
}

func TestTeamPayoutRetryPaysOnlyFailedAllocationAndReconcilesExactly(t *testing.T) {
	repo := newTeamSettlementRepo()
	repo.settlement = &PlayTeamSettlement{
		ID:          41,
		TeamID:      7,
		PeriodStart: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		TeamSpend:   decimal.RequireFromString("30.00000000"),
		PoolAmount:  decimal.RequireFromString("0.60000000"),
		Status:      PlayTeamSettlementStatusPending,
	}
	repo.allocations = []PlayTeamRewardAllocation{
		{
			ID:             101,
			SettlementID:   41,
			UserID:         11,
			Contribution:   decimal.RequireFromString("20.00000000"),
			Ratio:          decimal.RequireFromString("0.66666667"),
			RewardAmount:   decimal.RequireFromString("0.40000000"),
			PayoutStatus:   PlayTeamRewardAllocationStatusPending,
			IdempotencyKey: "team_reward:7:2026-06:11",
		},
		{
			ID:             102,
			SettlementID:   41,
			UserID:         19,
			Contribution:   decimal.RequireFromString("10.00000000"),
			Ratio:          decimal.RequireFromString("0.33333333"),
			RewardAmount:   decimal.RequireFromString("0.20000000"),
			PayoutStatus:   PlayTeamRewardAllocationStatusPending,
			IdempotencyKey: "team_reward:7:2026-06:19",
		},
	}
	repo.failPaidOnceForUser = 19

	db, mock, client := newTeamSettlementServiceTestClient(t)
	_ = db
	svc := NewPlayService(repo, nil, nil, nil, nil, client)

	mock.ExpectBegin()
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectRollback()

	first, err := svc.PayoutTeamRewardSettlement(context.Background(), 41)

	require.ErrorContains(t, err, "simulated allocation payout failure")
	require.Equal(t, PlayTeamSettlementStatusPartial, first.Status)
	require.Equal(t, PlayTeamRewardAllocationStatusPaid, allocationForUser(t, repo.allocations, 11).PayoutStatus)
	require.Equal(t, PlayTeamRewardAllocationStatusFailed, allocationForUser(t, repo.allocations, 19).PayoutStatus)
	require.Equal(t, "0.40000000", repo.balances[11].StringFixed(8))
	require.True(t, repo.balances[19].IsZero())
	require.Equal(t, 1, ledgerCount(repo.ledgerEntries, "team_reward:7:2026-06:11"))
	require.Equal(t, 0, ledgerCount(repo.ledgerEntries, "team_reward:7:2026-06:19"))

	mock.ExpectBegin()
	mock.ExpectCommit()

	second, err := svc.PayoutTeamRewardSettlement(context.Background(), 41)

	require.NoError(t, err)
	require.Equal(t, PlayTeamSettlementStatusCompleted, second.Status)
	require.Equal(t, PlayTeamRewardAllocationStatusPaid, allocationForUser(t, repo.allocations, 11).PayoutStatus)
	require.Equal(t, PlayTeamRewardAllocationStatusPaid, allocationForUser(t, repo.allocations, 19).PayoutStatus)
	require.Equal(t, "0.40000000", repo.balances[11].StringFixed(8))
	require.Equal(t, "0.20000000", repo.balances[19].StringFixed(8))
	require.Equal(t, 1, ledgerCount(repo.ledgerEntries, "team_reward:7:2026-06:11"))
	require.Equal(t, 1, ledgerCount(repo.ledgerEntries, "team_reward:7:2026-06:19"))
	require.Equal(t, "0.60000000", sumLedger(repo.ledgerEntries).StringFixed(8))
	require.Equal(t, "0.60000000", sumBalances(repo.balances).StringFixed(8))
	require.Equal(t, []int64{11, 19, 19}, repo.claimedUsers)
	for _, entry := range repo.ledgerEntries {
		require.Equal(t, PlayRewardSourceTeamSharedReward, entry.Source)
		require.Positive(t, entry.UserID)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserTeamRewardSettlementsReturnOnlyCurrentUserAllocationAcrossTeams(t *testing.T) {
	paidAt := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	repo := newTeamSettlementRepo()
	repo.userSettlementRecords = []PlayUserTeamSettlementRecord{
		{
			Settlement: PlayTeamSettlement{
				ID:          41,
				TeamID:      7,
				PeriodStart: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				WindowStart: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				WindowEnd:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
				TeamSpend:   decimal.RequireFromString("300.00000000"),
				PoolAmount:  decimal.RequireFromString("15.00000000"),
				Status:      PlayTeamSettlementStatusCompleted,
			},
			TeamName: "old team",
			Allocation: PlayTeamRewardAllocation{
				ID:           101,
				SettlementID: 41,
				UserID:       11,
				Contribution: decimal.RequireFromString("120.00000000"),
				Ratio:        decimal.RequireFromString("0.40000000"),
				RewardAmount: decimal.RequireFromString("6.00000000"),
				PayoutStatus: PlayTeamRewardAllocationStatusPaid,
				PaidAt:       &paidAt,
			},
		},
		{
			Settlement: PlayTeamSettlement{
				ID:          42,
				TeamID:      9,
				PeriodStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
				WindowStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
				WindowEnd:   time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
				TeamSpend:   decimal.RequireFromString("500.00000000"),
				PoolAmount:  decimal.RequireFromString("25.00000000"),
				Status:      PlayTeamSettlementStatusProcessing,
			},
			TeamName: "new team",
			Allocation: PlayTeamRewardAllocation{
				ID:           102,
				SettlementID: 42,
				UserID:       11,
				Contribution: decimal.RequireFromString("80.00000000"),
				Ratio:        decimal.RequireFromString("0.16000000"),
				RewardAmount: decimal.RequireFromString("4.00000000"),
				PayoutStatus: PlayTeamRewardAllocationStatusProcessing,
			},
		},
	}
	svc := &PlayService{repo: repo}

	records, err := svc.ListUserTeamRewardSettlements(context.Background(), 11, 24)

	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, []int64{11, 11}, []int64{records[0].Allocation.UserID, records[1].Allocation.UserID})
	require.Equal(t, "old team", records[0].TeamName)
	require.Equal(t, "6.00000000", records[0].Allocation.RewardAmount.StringFixed(8))
	require.Equal(t, &paidAt, records[0].Allocation.PaidAt)
	require.Equal(t, int64(11), repo.userSettlementQueryUserID)
	require.Equal(t, 24, repo.userSettlementQueryLimit)
}

type serviceAllocationAmounts map[int64]string

type teamSettlementRepo struct {
	PlayRepository

	contributions             []TeamContribution
	contributionCalls         int
	snapshotLockCalls         int
	snapshotLockHeld          bool
	windowStart               time.Time
	windowEnd                 time.Time
	createCalls               int
	settlement                *PlayTeamSettlement
	allocations               []PlayTeamRewardAllocation
	nextSettlementID          int64
	nextAllocationID          int64
	failPaidOnceForUser       int64
	claimedUsers              []int64
	ledgerEntries             []PlayRewardLedgerEntry
	balances                  map[int64]decimal.Decimal
	userSettlementRecords     []PlayUserTeamSettlementRecord
	userSettlementQueryUserID int64
	userSettlementQueryLimit  int
}

func (r *teamSettlementRepo) WithTeamRewardSnapshotLock(
	ctx context.Context,
	_ int64,
	fn func(context.Context) error,
) error {
	r.snapshotLockCalls++
	r.snapshotLockHeld = true
	defer func() { r.snapshotLockHeld = false }()
	return fn(ctx)
}

func newTeamSettlementRepo() *teamSettlementRepo {
	return &teamSettlementRepo{
		nextSettlementID: 41,
		nextAllocationID: 101,
		balances:         map[int64]decimal.Decimal{},
	}
}

func (r *teamSettlementRepo) ListTeamRewardContributions(
	_ context.Context,
	_ int64,
	start time.Time,
	end time.Time,
) ([]TeamContribution, error) {
	if !r.snapshotLockHeld {
		panic("team reward contributions must be read while the team snapshot lock is held")
	}
	r.contributionCalls++
	r.windowStart = start
	r.windowEnd = end
	return append([]TeamContribution(nil), r.contributions...), nil
}

func (r *teamSettlementRepo) GetTeamRewardSettlementByTeamPeriod(
	_ context.Context,
	teamID int64,
	periodStart time.Time,
) (*PlayTeamSettlement, error) {
	if r.settlement == nil ||
		r.settlement.TeamID != teamID ||
		r.settlement.PeriodStart.Format("2006-01") != periodStart.Format("2006-01") {
		return nil, nil
	}
	copy := *r.settlement
	return &copy, nil
}

func (r *teamSettlementRepo) CreateTeamRewardSnapshot(
	_ context.Context,
	settlement PlayTeamSettlement,
	allocations []PlayTeamRewardAllocation,
) (*PlayTeamSettlement, bool, error) {
	if !r.snapshotLockHeld {
		panic("team reward snapshot must be created while the team snapshot lock is held")
	}
	r.createCalls++
	if r.settlement != nil {
		copy := *r.settlement
		return &copy, false, nil
	}
	settlement.ID = r.nextSettlementID
	settlement.Status = PlayTeamSettlementStatusPending
	r.nextSettlementID++
	r.settlement = &settlement
	r.allocations = append([]PlayTeamRewardAllocation(nil), allocations...)
	for i := range r.allocations {
		r.allocations[i].ID = r.nextAllocationID
		r.allocations[i].SettlementID = settlement.ID
		r.nextAllocationID++
	}
	copy := settlement
	return &copy, true, nil
}

func (r *teamSettlementRepo) GetTeamRewardSettlement(
	_ context.Context,
	settlementID int64,
) (*PlayTeamSettlement, error) {
	if r.settlement == nil || r.settlement.ID != settlementID {
		return nil, nil
	}
	copy := *r.settlement
	return &copy, nil
}

func (r *teamSettlementRepo) ListUnpaidTeamRewardAllocations(
	_ context.Context,
	settlementID int64,
) ([]PlayTeamRewardAllocation, error) {
	var out []PlayTeamRewardAllocation
	for _, allocation := range r.allocations {
		if allocation.SettlementID == settlementID &&
			(allocation.PayoutStatus == PlayTeamRewardAllocationStatusPending ||
				allocation.PayoutStatus == PlayTeamRewardAllocationStatusFailed) {
			out = append(out, allocation)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out, nil
}

func (r *teamSettlementRepo) MarkTeamRewardSettlementProcessing(
	_ context.Context,
	settlementID int64,
) error {
	if r.settlement == nil || r.settlement.ID != settlementID {
		return errors.New("settlement not found")
	}
	r.settlement.Status = PlayTeamSettlementStatusProcessing
	return nil
}

func (r *teamSettlementRepo) ClaimTeamRewardAllocation(
	_ context.Context,
	allocationID int64,
) (bool, error) {
	for i := range r.allocations {
		if r.allocations[i].ID != allocationID {
			continue
		}
		if r.allocations[i].PayoutStatus != PlayTeamRewardAllocationStatusPending &&
			r.allocations[i].PayoutStatus != PlayTeamRewardAllocationStatusFailed {
			return false, nil
		}
		r.allocations[i].PayoutStatus = PlayTeamRewardAllocationStatusProcessing
		r.claimedUsers = append(r.claimedUsers, r.allocations[i].UserID)
		return true, nil
	}
	return false, nil
}

func (r *teamSettlementRepo) MarkTeamRewardAllocationPaid(
	ctx context.Context,
	allocationID int64,
) error {
	requireTeamRewardTransaction(ctx)
	for i := range r.allocations {
		if r.allocations[i].ID != allocationID {
			continue
		}
		if r.failPaidOnceForUser == r.allocations[i].UserID {
			r.failPaidOnceForUser = 0
			return errors.New("simulated allocation payout failure")
		}
		r.allocations[i].PayoutStatus = PlayTeamRewardAllocationStatusPaid
		return nil
	}
	return errors.New("allocation not found")
}

func (r *teamSettlementRepo) MarkTeamRewardAllocationFailed(
	_ context.Context,
	allocationID int64,
	message string,
) error {
	for i := range r.allocations {
		if r.allocations[i].ID == allocationID {
			r.allocations[i].PayoutStatus = PlayTeamRewardAllocationStatusFailed
			r.allocations[i].LastError = message
			return nil
		}
	}
	return errors.New("allocation not found")
}

func (r *teamSettlementRepo) RefreshTeamRewardSettlementStatus(
	_ context.Context,
	settlementID int64,
) (*PlayTeamSettlement, error) {
	var paid, processing, failed, total int
	for _, allocation := range r.allocations {
		if allocation.SettlementID != settlementID || !allocation.RewardAmount.IsPositive() {
			continue
		}
		total++
		switch allocation.PayoutStatus {
		case PlayTeamRewardAllocationStatusPaid:
			paid++
		case PlayTeamRewardAllocationStatusProcessing:
			processing++
		case PlayTeamRewardAllocationStatusFailed:
			failed++
		}
	}
	switch {
	case total > 0 && paid == total:
		r.settlement.Status = PlayTeamSettlementStatusCompleted
	case paid > 0:
		r.settlement.Status = PlayTeamSettlementStatusPartial
	case processing > 0:
		r.settlement.Status = PlayTeamSettlementStatusProcessing
	case failed > 0:
		r.settlement.Status = PlayTeamSettlementStatusFailed
	default:
		r.settlement.Status = PlayTeamSettlementStatusPending
	}
	copy := *r.settlement
	return &copy, nil
}

func (r *teamSettlementRepo) ListUserTeamRewardSettlements(
	_ context.Context,
	userID int64,
	limit int,
) ([]PlayUserTeamSettlementRecord, error) {
	r.userSettlementQueryUserID = userID
	r.userSettlementQueryLimit = limit
	return append([]PlayUserTeamSettlementRecord(nil), r.userSettlementRecords...), nil
}

func (r *teamSettlementRepo) InsertRewardLedger(
	ctx context.Context,
	entry PlayRewardLedgerEntry,
) error {
	requireTeamRewardTransaction(ctx)
	if entry.UserID <= 0 {
		return errors.New("invalid reward user")
	}
	for _, existing := range r.ledgerEntries {
		if existing.IdempotencyKey == entry.IdempotencyKey {
			return ErrPlayRewardDuplicate
		}
	}
	r.ledgerEntries = append(r.ledgerEntries, entry)
	return nil
}

func (r *teamSettlementRepo) UpdatePlayBalance(
	ctx context.Context,
	userID int64,
	amount float64,
) error {
	requireTeamRewardTransaction(ctx)
	if userID <= 0 {
		return errors.New("invalid balance user")
	}
	r.balances[userID] = r.balances[userID].
		Add(decimal.NewFromFloat(amount).Round(teamRewardAmountScale))
	return nil
}

func requireTeamRewardTransaction(ctx context.Context) {
	if dbent.TxFromContext(ctx) == nil {
		panic("team reward payout must use the grant balance transaction")
	}
}

func newTeamSettlementServiceTestClient(
	t *testing.T,
) (*sql.DB, sqlmock.Sqlmock, *dbent.Client) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return db, mock, client
}

func allocationForUser(
	t *testing.T,
	allocations []PlayTeamRewardAllocation,
	userID int64,
) PlayTeamRewardAllocation {
	t.Helper()
	for _, allocation := range allocations {
		if allocation.UserID == userID {
			return allocation
		}
	}
	t.Fatalf("missing allocation for user %d", userID)
	return PlayTeamRewardAllocation{}
}

func rewardAllocationAmounts(allocations []PlayTeamRewardAllocation) serviceAllocationAmounts {
	out := make(serviceAllocationAmounts, len(allocations))
	for _, allocation := range allocations {
		out[allocation.UserID] = allocation.RewardAmount.StringFixed(teamRewardAmountScale)
	}
	return out
}

func ledgerCount(entries []PlayRewardLedgerEntry, key string) int {
	count := 0
	for _, entry := range entries {
		if entry.IdempotencyKey == key {
			count++
		}
	}
	return count
}

func sumLedger(entries []PlayRewardLedgerEntry) decimal.Decimal {
	total := decimal.Zero
	for _, entry := range entries {
		total = total.Add(decimal.NewFromFloat(entry.Amount).Round(teamRewardAmountScale))
	}
	return total
}

func sumBalances(balances map[int64]decimal.Decimal) decimal.Decimal {
	total := decimal.Zero
	for _, balance := range balances {
		total = total.Add(balance)
	}
	return total
}
