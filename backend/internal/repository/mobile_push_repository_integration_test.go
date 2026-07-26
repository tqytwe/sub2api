//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMobilePushRepositoryConcurrentRegistrationHasSingleOwner(t *testing.T) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			require.NoError(t, err, "Docker must be available for repository integration tests in CI")
		}
		t.Skip("Docker is unavailable")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("sub2api_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(`CREATE TABLE users (id BIGINT PRIMARY KEY); INSERT INTO users (id) VALUES (1), (2)`)
	require.NoError(t, err)
	migration, err := dbmigrations.FS.ReadFile("221_mobile_push.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)

	repo := NewMobilePushRepository(db, time.Minute, 8)
	installationID := uuid.NewString()
	now := time.Now().UTC()
	writes := []service.MobileDeviceWrite{
		{UserID: 1, InstallationID: installationID, Platform: "android", PushProvider: "fcm", TokenCiphertext: "cipher-1", TokenHash: "shared-hash", Locale: "zh-CN", LastSeenAt: now},
		{UserID: 2, InstallationID: installationID, Platform: "android", PushProvider: "fcm", TokenCiphertext: "cipher-2", TokenHash: "shared-hash", Locale: "zh-CN", LastSeenAt: now},
	}
	start := make(chan struct{})
	errs := make(chan error, len(writes))
	var wg sync.WaitGroup
	for _, write := range writes {
		write := write
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := repo.UpsertDevice(ctx, write)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var activeCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM mobile_devices WHERE installation_id = $1::uuid AND token_hash = $2 AND enabled = TRUE`, installationID, "shared-hash").Scan(&activeCount))
	require.Equal(t, 1, activeCount)
}
