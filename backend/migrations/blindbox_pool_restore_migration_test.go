package migrations_test

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

const approvedBlindboxPoolJSON = `{"version":"season-1-v1","cost":0.5,"rtp_cap":0.9,"tiers":[{"amount":0.05,"weight":4000},{"amount":0.2,"weight":3000},{"amount":0.5,"weight":1800},{"amount":1,"weight":800},{"amount":3,"weight":300},{"amount":10,"weight":90},{"amount":20,"weight":10}]}`

func TestBlindboxPoolRestoreMigrationIsForwardOnlyAndIdempotent(t *testing.T) {
	raw, err := dbmigrations.FS.ReadFile("190_restore_configurable_blindbox_pool.sql")
	require.NoError(t, err)

	sqlText := string(raw)
	upperSQL := strings.ToUpper(sqlText)
	require.Contains(t, sqlText, "play_blindbox_pool_json")
	require.Contains(t, sqlText, approvedBlindboxPoolJSON)
	require.Contains(t, upperSQL, "ON CONFLICT (KEY) DO NOTHING")
	require.Contains(t, upperSQL, "ADD COLUMN IF NOT EXISTS POOL_VERSION")
	require.Contains(t, upperSQL, "ADD COLUMN IF NOT EXISTS OPEN_SOURCE")
	require.Contains(t, sqlText, "legacy-v1")
	require.Contains(t, sqlText, "paid")
	require.NotContains(t, upperSQL, "DROP TABLE")
	require.NotContains(t, upperSQL, "SET COST_AMOUNT")
	require.NotContains(t, upperSQL, "SET REWARD_AMOUNT")
}

func TestPlayRepositoryInsertBlindboxOpenPersistsAuditFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	record := service.PlayBlindboxOpenRecord{
		UserID:         42,
		Date:           time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC),
		Cost:           0.5,
		Reward:         20,
		IdempotencyKey: "blindbox:42:task-2",
		PoolVersion:    "season-1-v1",
		OpenSource:     "paid",
	}
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO play_blindbox_opens (
			user_id, open_date, cost_amount, reward_amount, idempotency_key, pool_version, open_source
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key) DO NOTHING`)).
		WithArgs(
			record.UserID,
			record.Date.Format("2006-01-02"),
			record.Cost,
			record.Reward,
			record.IdempotencyKey,
			record.PoolVersion,
			record.OpenSource,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	repo := repository.NewPlayRepository(nil, db)
	err = repo.InsertBlindboxOpenRecord(context.Background(), record)

	require.NoError(t, err)
}

func TestPlayRepositoryInsertBlindboxOpenReturnsDuplicateOnConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	record := service.PlayBlindboxOpenRecord{
		UserID:         42,
		Date:           time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC),
		Cost:           0.5,
		Reward:         20,
		IdempotencyKey: "blindbox:42:duplicate",
		PoolVersion:    "season-1-v1",
		OpenSource:     "paid",
	}
	expectBlindboxOpenInsert(mock, record, sqlmock.NewResult(0, 0))
	mock.ExpectClose()

	repo := repository.NewPlayRepository(nil, db)
	err = repo.InsertBlindboxOpenRecord(context.Background(), record)

	require.ErrorIs(t, err, service.ErrPlayRewardDuplicate)
}

func TestPlayRepositoryLegacyBlindboxInsertUsesLegacyAuditValues(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	date := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	record := service.PlayBlindboxOpenRecord{
		UserID:         42,
		Date:           date,
		Cost:           0.5,
		Reward:         3,
		IdempotencyKey: "blindbox:42:legacy",
		PoolVersion:    "legacy-v1",
		OpenSource:     "paid",
	}
	expectBlindboxOpenInsert(mock, record, sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	repo := repository.NewPlayRepository(nil, db)
	err = repo.InsertBlindboxOpen(
		context.Background(),
		record.UserID,
		record.Date,
		record.Cost,
		record.Reward,
		record.IdempotencyKey,
	)

	require.NoError(t, err)
}

func expectBlindboxOpenInsert(
	mock sqlmock.Sqlmock,
	record service.PlayBlindboxOpenRecord,
	result sql.Result,
) {
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO play_blindbox_opens (
			user_id, open_date, cost_amount, reward_amount, idempotency_key, pool_version, open_source
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key) DO NOTHING`)).
		WithArgs(
			record.UserID,
			record.Date.Format("2006-01-02"),
			record.Cost,
			record.Reward,
			record.IdempotencyKey,
			record.PoolVersion,
			record.OpenSource,
		).
		WillReturnResult(result)
}
