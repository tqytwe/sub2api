//go:build unit

package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type fakeImageStudioFeatureRuntime struct {
	enabled bool
	running bool
}

func (f *fakeImageStudioFeatureRuntime) IsEnabled(context.Context) bool {
	return f.enabled
}

func (f *fakeImageStudioFeatureRuntime) Running() bool {
	return f.running
}

func TestImageRuntimesHealth_ImageStudioUsesFeatureFlag(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectPing()
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id::text, status, created_at
		FROM image_studio_jobs
		WHERE status IN ('pending', 'running')
		ORDER BY created_at ASC
		LIMIT 1`)).WillReturnRows(sqlmock.NewRows([]string{"id", "status", "created_at"}))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending'),
			COUNT(*) FILTER (WHERE status = 'running')
		FROM image_studio_jobs`)).WillReturnRows(sqlmock.NewRows([]string{"pending", "running"}).AddRow(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT error_message, COALESCE(finished_at, created_at)
		FROM image_studio_jobs
		WHERE error_message IS NOT NULL AND btrim(error_message) <> ''
		ORDER BY COALESCE(finished_at, created_at) DESC
		LIMIT 1`)).WillReturnRows(sqlmock.NewRows([]string{"error_message", "created_at"}))

	svc := &ImageRuntimesHealthService{
		db:  db,
		cfg: &config.Config{},
	}
	svc.SetImageStudioRuntime(&fakeImageStudioFeatureRuntime{enabled: false, running: true})

	health := svc.imageStudioHealth(context.Background())

	require.False(t, health.Enabled)
	require.False(t, health.Ready)
	require.True(t, health.StorageReady)
	require.True(t, health.WorkerRunning)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageRuntimesHealth_BatchIncludesOperationalCounters(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Now().UTC()
	mock.ExpectQuery("SELECT batch_id, status, created_at").
		WillReturnRows(sqlmock.NewRows([]string{"batch_id", "status", "created_at"}).
			AddRow("imgbatch_oldest", BatchImageJobStatusRunning, now.Add(-time.Hour)))
	mock.ExpectQuery("COUNT\\(\\*\\) FILTER").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"stale_balance_holds",
			"settlement_retries",
			"provider_failures",
			"result_cleanup_pending",
		}).AddRow(2, 3, 4, 5))
	mock.ExpectQuery("SELECT last_error_code, last_error_message, updated_at").
		WillReturnRows(sqlmock.NewRows([]string{"code", "message", "updated_at"}).
			AddRow("PROVIDER_STATUS_FAILED", "provider failed", now))

	health := ImageRuntimeComponentHealth{}
	svc := &ImageRuntimesHealthService{
		db: db,
		cfg: &config.Config{BatchImage: config.BatchImageConfig{
			StaleActiveAfterSeconds: 60,
		}},
	}

	svc.loadBatchDatabaseHealth(context.Background(), &health)

	require.Equal(t, int64(2), health.StaleBalanceHolds)
	require.Equal(t, int64(3), health.SettlementRetries)
	require.Equal(t, int64(4), health.ProviderFailures)
	require.Equal(t, int64(5), health.ResultCleanupPending)
	require.Equal(t, "imgbatch_oldest", health.OldestTask.ID)
	require.Equal(t, "PROVIDER_STATUS_FAILED", health.RecentError.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
