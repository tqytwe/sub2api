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
	lockedBalance     float64
	records           []PlayBlindboxOpenRecord
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
	return r.opens, nil
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
	svc := NewPlayService(repo, userRepo, nil, settings, nil, nil)
	svc.blindboxDrawSource = func(int64) (int64, error) {
		return 0, errors.New("random unavailable")
	}

	_, err = svc.OpenBlindbox(context.Background(), 42, "blindbox-random-failure")
	require.ErrorContains(t, err, "random unavailable")
	require.Empty(t, repo.records)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, userRepo.balanceUpdates)
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
