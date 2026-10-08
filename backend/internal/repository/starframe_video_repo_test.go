package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestStarframeDurableLedger(t *testing.T) {
	driver, dsn := "sqlite", filepath.Join(t.TempDir(), "ledger.db")+"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	if postgres := os.Getenv("STARFRAME_TEST_POSTGRES_DSN"); postgres != "" {
		driver, dsn = "postgres", postgres
	}
	db, err := sql.Open(driver, dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	ddl, err := os.ReadFile("../../migrations/272_starframe_video_submissions.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(ddl))
	require.NoError(t, err)
	store := NewStarframeVideoRepository(db)
	task := service.StarframeVideoTask{LocalID: "sfv_" + uuid.NewString(), ClientTaskID: uuid.NewString(), Owner: service.StarframeVideoOwner{UserID: 1, APIKeyID: 2, GroupID: 3}, Model: "pinned", AccountID: 4}
	var wg sync.WaitGroup
	var winners atomic.Int32
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, e := store.Claim(context.Background(), &task)
			if e != nil {
				t.Error(e)
			}
			if ok {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), winners.Load())
	task.UpstreamID = "upstream-original"
	require.NoError(t, store.Complete(context.Background(), &task))
	restarted := NewStarframeVideoRepository(db)
	loaded, err := restarted.Get(context.Background(), task.LocalID, task.Owner)
	require.NoError(t, err)
	require.Equal(t, task, *loaded)
	_, err = restarted.Get(context.Background(), task.LocalID, service.StarframeVideoOwner{UserID: 9, APIKeyID: 2, GroupID: 3})
	require.Error(t, err)
	task.LocalID = "sfv_new"
	ok, err := restarted.Claim(context.Background(), &task)
	require.NoError(t, err)
	require.False(t, ok)
}
