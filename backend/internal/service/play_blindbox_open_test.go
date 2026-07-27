package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

type blindboxOpenSettingRepo struct {
	SettingRepository
	values map[string]string
	err    error
}

func (r *blindboxOpenSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

type blindboxOpenRepo struct {
	PlayRepository
	opens             int
	countRecords      bool
	lockedBalance     float64
	records           []PlayBlindboxOpenRecord
	replayLookups     int
	replayInTx        bool
	legacyInsertCalls int
	ledgerEntries     []PlayRewardLedgerEntry
	balanceUpdates    []float64
	lockInTx          bool
	countInTx         bool
	recordInTx        bool
	ledgerInTx        bool
	balanceInTx       bool
}

func (r *blindboxOpenRepo) LockBlindboxOpenUser(ctx context.Context, _ int64) (float64, error) {
	r.lockInTx = dbent.TxFromContext(ctx) != nil
	return r.lockedBalance, nil
}

func (r *blindboxOpenRepo) UpdatePlayBalance(ctx context.Context, _ int64, amount float64) error {
	r.balanceInTx = dbent.TxFromContext(ctx) != nil
	r.balanceUpdates = append(r.balanceUpdates, amount)
	return nil
}

func (r *blindboxOpenRepo) CountBlindboxOpens(ctx context.Context, _ int64, _ time.Time) (int, error) {
	r.countInTx = dbent.TxFromContext(ctx) != nil
	if r.countRecords {
		return r.opens + len(r.records), nil
	}
	return r.opens, nil
}

func (r *blindboxOpenRepo) FindBlindboxOpenByIdempotency(ctx context.Context, userID int64, idempotencyKey string) (*PlayBlindboxOpenRecord, error) {
	r.replayLookups++
	r.replayInTx = dbent.TxFromContext(ctx) != nil
	for _, record := range r.records {
		if record.UserID == userID && record.IdempotencyKey == idempotencyKey {
			copy := record
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *blindboxOpenRepo) InsertBlindboxOpen(
	_ context.Context,
	userID int64,
	date time.Time,
	cost float64,
	reward float64,
	idempotencyKey string,
) error {
	r.legacyInsertCalls++
	r.records = append(r.records, PlayBlindboxOpenRecord{
		UserID:         userID,
		Date:           date,
		Cost:           cost,
		Reward:         reward,
		IdempotencyKey: idempotencyKey,
		PoolVersion:    "legacy-v1",
		OpenSource:     "paid",
	})
	return nil
}

func (r *blindboxOpenRepo) InsertBlindboxOpenRecord(ctx context.Context, record PlayBlindboxOpenRecord) error {
	r.recordInTx = dbent.TxFromContext(ctx) != nil
	r.records = append(r.records, record)
	return nil
}

func (r *blindboxOpenRepo) InsertRewardLedger(ctx context.Context, entry PlayRewardLedgerEntry) error {
	r.ledgerInTx = dbent.TxFromContext(ctx) != nil
	r.ledgerEntries = append(r.ledgerEntries, entry)
	return nil
}

type blindboxOpenUserRepo struct {
	UserRepository
	user           *User
	balanceUpdates []float64
}

func (r *blindboxOpenUserRepo) GetByID(context.Context, int64) (*User, error) {
	return r.user, nil
}

func (r *blindboxOpenUserRepo) UpdateBalance(_ context.Context, _ int64, amount float64) error {
	r.balanceUpdates = append(r.balanceUpdates, amount)
	return nil
}

func TestBlindboxOpenUsesConfiguredPoolAndPersistsAudit(t *testing.T) {
	pool := defaultBlindboxPool()
	poolJSON, err := json.Marshal(pool)
	require.NoError(t, err)

	settings := NewSettingService(&blindboxOpenSettingRepo{values: map[string]string{
		SettingKeyPlayBlindboxEnabled:    "true",
		SettingKeyPlayBlindboxCost:       "0.1",
		SettingKeyPlayBlindboxPoolJSON:   string(poolJSON),
		SettingKeyPlayBlindboxDailyLimit: "10",
	}}, nil)
	repo := &blindboxOpenRepo{lockedBalance: 1}
	userRepo := &blindboxOpenUserRepo{user: &User{ID: 42, Balance: 1}}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })

	svc := NewPlayService(repo, userRepo, nil, settings, nil, client)
	svc.rewardDrawSource = func(max int64) (int64, error) {
		require.Equal(t, int64(couponWeightBasisPoints), max)
		return 6000, nil // keep legacy audit tests on the 40% balance branch
	}
	svc.blindboxDrawSource = func(max int64) (int64, error) {
		require.Equal(t, blindboxWeightTotal, max)
		return max - 1, nil
	}

	mock.ExpectBegin()
	mock.ExpectCommit()

	rawKey := "blindbox-open-task-3"
	result, err := svc.OpenBlindbox(context.Background(), 42, rawKey)
	require.NoError(t, err)
	require.Equal(t, pool.Cost, result.CostAmount)
	require.Equal(t, 20.0, result.RewardAmount)
	require.Equal(t, 19.5, result.NetAmount)
	require.Equal(t, pool.Version, result.PoolVersion)
	require.Equal(t, "paid", result.OpenSource)

	expectedKey := fmt.Sprintf("blindbox:%d:%x", 42, sha256.Sum256([]byte(rawKey)))
	require.Zero(t, repo.legacyInsertCalls)
	require.Len(t, repo.records, 1)
	require.Equal(t, PlayBlindboxOpenRecord{
		UserID:         42,
		Date:           repo.records[0].Date,
		Cost:           pool.Cost,
		Reward:         20,
		IdempotencyKey: expectedKey,
		PoolVersion:    pool.Version,
		OpenSource:     "paid",
	}, repo.records[0])
	require.Len(t, repo.ledgerEntries, 1)
	require.Equal(t, expectedKey, repo.ledgerEntries[0].IdempotencyKey)
	require.Equal(t, pool.Version, repo.ledgerEntries[0].Detail["pool_version"])
	require.Equal(t, "paid", repo.ledgerEntries[0].Detail["open_source"])
	require.Equal(t, []float64{19.5}, repo.balanceUpdates)
	require.Empty(t, userRepo.balanceUpdates)
	require.True(t, repo.lockInTx)
	require.True(t, repo.countInTx)
	require.True(t, repo.recordInTx)
	require.True(t, repo.ledgerInTx)
	require.True(t, repo.balanceInTx)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlindboxStatusSelectsCurrentAndNextVIPBlindboxPools(t *testing.T) {
	pool := defaultBlindboxPool()
	poolJSON, err := json.Marshal(pool)
	require.NoError(t, err)

	settings := NewSettingService(&blindboxOpenSettingRepo{values: map[string]string{
		SettingKeyPlayBlindboxEnabled:    "true",
		SettingKeyPlayBlindboxPoolJSON:   string(poolJSON),
		SettingKeyPlayBlindboxDailyLimit: "5",
	}}, nil)
	repo := &blindboxOpenRepo{}
	userRepo := &blindboxOpenUserRepo{user: &User{ID: 42, Balance: 10, TotalRecharged: 200}}
	svc := NewPlayService(repo, userRepo, nil, settings, nil, nil)

	status, err := svc.GetBlindboxStatus(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, 3, status.VIPTier.Tier)
	require.Equal(t, "V3", status.VIPTier.Label)
	require.Equal(t, "season-1-vip-v3", status.BlindboxPool.Version)
	require.Equal(t, status.BlindboxPool, status.CurrentPool)
	require.NotNil(t, status.NextPool)
	require.Equal(t, "season-1-vip-v4", status.NextPool.Version)
	require.InDelta(t, status.BlindboxPool.ExpectedReward(), status.ExpectedReward, 1e-12)
	require.Equal(t, status.BlindboxPool.Version, status.PoolVersion)
	require.Equal(t, status.BlindboxPool.RTPCap, status.RTPCap)
}

func TestBlindboxOpenUsesVIPPoolAndReturnsCelebrationContext(t *testing.T) {
	pool := defaultBlindboxPool()
	poolJSON, err := json.Marshal(pool)
	require.NoError(t, err)

	settings := NewSettingService(&blindboxOpenSettingRepo{values: map[string]string{
		SettingKeyPlayBlindboxEnabled:    "true",
		SettingKeyPlayBlindboxPoolJSON:   string(poolJSON),
		SettingKeyPlayBlindboxDailyLimit: "10",
	}}, nil)
	repo := &blindboxOpenRepo{lockedBalance: 2}
	userRepo := &blindboxOpenUserRepo{user: &User{ID: 42, Balance: 2, TotalRecharged: 1000}}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })

	svc := NewPlayService(repo, userRepo, nil, settings, nil, client)
	svc.rewardDrawSource = func(max int64) (int64, error) {
		require.Equal(t, int64(couponWeightBasisPoints), max)
		return 6000, nil // keep legacy audit tests on the 40% balance branch
	}
	svc.blindboxDrawSource = func(max int64) (int64, error) {
		require.Equal(t, blindboxWeightTotal, max)
		return max - 1, nil
	}

	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.OpenBlindbox(context.Background(), 42, "vip-pool-open")

	require.NoError(t, err)
	require.Equal(t, 5, result.VIPTier.Tier)
	require.Equal(t, "V5", result.VIPTier.Label)
	require.Equal(t, "season-1-vip-v5", result.PoolVersion)
	require.Equal(t, "season-1-vip-v5", repo.records[0].PoolVersion)
	require.Equal(t, "V5", repo.ledgerEntries[0].Detail["vip_label"])
	require.Equal(t, float64(5), repo.ledgerEntries[0].Detail["vip_tier"])
	require.Equal(t, result.ExpectedReward, repo.ledgerEntries[0].Detail["expected_reward"])
	require.Equal(t, result.RTPCap, repo.ledgerEntries[0].Detail["rtp_cap"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlindboxIdempotencyKeyIsHashedAndScopedByUser(t *testing.T) {
	const raw = " shared-client-key "
	first, err := scopeBlindboxIdempotencyKey(42, raw)
	require.NoError(t, err)
	second, err := scopeBlindboxIdempotencyKey(43, raw)
	require.NoError(t, err)

	sum := sha256.Sum256([]byte("shared-client-key"))
	require.Equal(t, fmt.Sprintf("blindbox:42:%x", sum), first)
	require.Equal(t, fmt.Sprintf("blindbox:43:%x", sum), second)
	require.NotEqual(t, first, second)
	require.NotContains(t, first, "shared-client-key")
	require.LessOrEqual(t, len(first), 128)
}

func TestBlindboxIdempotencyKeyRejectsInvalidClientKey(t *testing.T) {
	_, err := scopeBlindboxIdempotencyKey(42, "bad\nkey")
	require.ErrorIs(t, err, ErrIdempotencyKeyInvalid)

	_, err = scopeBlindboxIdempotencyKey(42, string(make([]byte, 129)))
	require.ErrorIs(t, err, ErrIdempotencyKeyInvalid)
}

func TestBlindboxOpenReplaysCompletedBalanceOpenBeforeFeatureAndDailyLimitChecks(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	rawKey := "retry-after-lost-response"
	scopedKey, err := scopeBlindboxIdempotencyKey(42, rawKey)
	require.NoError(t, err)

	expectedReward := 0.45
	rtpCap := 0.9
	repo := &blindboxOpenRepo{
		opens: 5,
		records: []PlayBlindboxOpenRecord{{
			UserID:         42,
			Date:           now,
			Cost:           0.5,
			Reward:         9,
			IdempotencyKey: scopedKey,
			PoolVersion:    "season-1-v1",
			OpenSource:     "paid",
			VIPTierSnapshot: &PlayVIPStatus{
				Tier:             3,
				Label:            "V3",
				RechargeBonusPct: 6,
				ColorKey:         "indigo",
				Perks:            []string{"blindbox_pool_upgrade"},
				NextTier:         4,
				NextLabel:        "V4",
				NextMinRecharge:  500,
				AmountToNext:     300,
			},
			ExpectedReward: &expectedReward,
			RTPCap:         &rtpCap,
		}},
	}
	svc := NewPlayService(repo, nil, nil, NewSettingService(&blindboxOpenSettingRepo{}, nil), nil, nil)
	svc.now = func() time.Time { return now }
	svc.rewardDrawSource = func(int64) (int64, error) {
		t.Fatal("completed blindbox retry must not redraw the reward branch")
		return 0, nil
	}
	svc.blindboxDrawSource = func(int64) (int64, error) {
		t.Fatal("completed blindbox retry must not redraw the balance pool")
		return 0, nil
	}

	result, err := svc.OpenBlindbox(context.Background(), 42, rawKey)

	require.NoError(t, err)
	require.Equal(t, PlayRewardTypeBalance, result.RewardType)
	require.InDelta(t, 0.5, result.CostAmount, 1e-12)
	require.InDelta(t, 9, result.RewardAmount, 1e-12)
	require.InDelta(t, 8.5, result.NetAmount, 1e-12)
	require.Equal(t, 5, result.OpensToday)
	require.Equal(t, "2026-07-27", result.ServerDate)
	require.Equal(t, "season-1-v1", result.PoolVersion)
	require.Equal(t, 3, result.VIPTier.Tier)
	require.Equal(t, "V3", result.VIPTier.Label)
	require.Equal(t, []string{"blindbox_pool_upgrade"}, result.VIPTier.Perks)
	require.InDelta(t, expectedReward, result.ExpectedReward, 1e-12)
	require.InDelta(t, rtpCap, result.RTPCap, 1e-12)
	require.Equal(t, 1, repo.replayLookups)
	require.False(t, repo.replayInTx)
	require.False(t, repo.lockInTx)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
}

func TestBlindboxOpenSameKeyReplaysWithoutSecondDrawOrBalanceMutation(t *testing.T) {
	pool := defaultBlindboxPool()
	settings := newCouponRewardSettingService(t, pool, true)
	repo := &blindboxOpenRepo{lockedBalance: 1, countRecords: true}
	client, mock := newCouponRewardEntClient(t)
	svc := NewPlayService(repo, nil, nil, settings, nil, client)
	branchDraws := 0
	balanceDraws := 0
	svc.rewardDrawSource = func(int64) (int64, error) {
		branchDraws++
		return 6000, nil
	}
	svc.blindboxDrawSource = func(max int64) (int64, error) {
		balanceDraws++
		return max - 1, nil
	}

	mock.ExpectBegin()
	mock.ExpectCommit()
	first, err := svc.OpenBlindbox(context.Background(), 42, "same-logical-open")
	require.NoError(t, err)
	retry, err := svc.OpenBlindbox(context.Background(), 42, "same-logical-open")

	require.NoError(t, err)
	require.Equal(t, first.CostAmount, retry.CostAmount)
	require.Equal(t, first.RewardAmount, retry.RewardAmount)
	require.Equal(t, first.NetAmount, retry.NetAmount)
	require.Equal(t, first.PoolVersion, retry.PoolVersion)
	require.Equal(t, first.OpensToday, retry.OpensToday)
	require.Equal(t, 1, branchDraws)
	require.Equal(t, 1, balanceDraws)
	require.Len(t, repo.records, 1)
	require.Len(t, repo.ledgerEntries, 1)
	require.Equal(t, []float64{first.NetAmount}, repo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrantBalanceUsesPlayBalanceUpdateWithoutRechargeMutation(t *testing.T) {
	repo := &blindboxOpenRepo{}
	userRepo := &blindboxOpenUserRepo{}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })

	svc := NewPlayService(repo, userRepo, nil, nil, nil, client)
	mock.ExpectBegin()
	mock.ExpectCommit()

	err = svc.grantBalance(
		context.Background(),
		42,
		3.5,
		PlayRewardSourceCheckin,
		"checkin:42:2026-07-16",
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, []float64{3.5}, repo.balanceUpdates)
	require.Empty(t, userRepo.balanceUpdates)
	require.True(t, repo.balanceInTx)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlindboxOpenRandomFailureDoesNotGrantBalance(t *testing.T) {
	pool := defaultBlindboxPool()
	poolJSON, err := json.Marshal(pool)
	require.NoError(t, err)

	settings := NewSettingService(&blindboxOpenSettingRepo{values: map[string]string{
		SettingKeyPlayBlindboxEnabled:    "true",
		SettingKeyPlayBlindboxPoolJSON:   string(poolJSON),
		SettingKeyPlayBlindboxDailyLimit: "10",
	}}, nil)
	repo := &blindboxOpenRepo{lockedBalance: 1}
	userRepo := &blindboxOpenUserRepo{user: &User{ID: 42, Balance: 1}}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })

	svc := NewPlayService(repo, userRepo, nil, settings, nil, client)
	svc.rewardDrawSource = func(max int64) (int64, error) {
		require.Equal(t, int64(couponWeightBasisPoints), max)
		return 6000, nil
	}
	svc.blindboxDrawSource = func(int64) (int64, error) {
		return 0, errors.New("random unavailable")
	}

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err = svc.OpenBlindbox(context.Background(), 42, "blindbox-random-failure")
	require.ErrorContains(t, err, "random unavailable")
	require.Empty(t, repo.records)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, userRepo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlindboxOpenPoolReadFailureDoesNotGrantBalance(t *testing.T) {
	repo := &blindboxOpenRepo{lockedBalance: 1}
	userRepo := &blindboxOpenUserRepo{user: &User{ID: 42, Balance: 1}}
	settings := NewSettingService(&blindboxOpenSettingRepo{
		err: errors.New("settings unavailable"),
	}, nil)
	svc := NewPlayService(repo, userRepo, nil, settings, nil, nil)

	_, err := svc.OpenBlindbox(context.Background(), 42, "blindbox-pool-read-failure")
	require.ErrorIs(t, err, ErrPlayFeatureDisabled)
	require.Empty(t, repo.records)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, userRepo.balanceUpdates)
}
