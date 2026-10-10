//go:build integration

// Package ledgertest provisions only disposable loopback fixtures, never app DSNs.
package ledgertest

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func New(t *testing.T) *sql.DB {
	db, _ := NewWithDSN(t)
	return db
}

func NewWithDSN(t *testing.T) (*sql.DB, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	name := "ledger-test-" + uuid.NewString()
	out, err := exec.CommandContext(ctx, "docker", "run", "--rm", "-d", "--name", name,
		"-e", "POSTGRES_HOST_AUTH_METHOD=trust", "-e", "POSTGRES_DB=ledger_test",
		"-p", "127.0.0.1::5432", "postgres:18.1-alpine3.23").CombinedOutput()
	require.NoError(t, err, "disposable postgres startup: %s", out)
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	out, err = exec.CommandContext(ctx, "docker", "port", name, "5432/tcp").Output()
	require.NoError(t, err)
	address := strings.TrimSpace(string(out))
	require.True(t, strings.HasPrefix(address, "127.0.0.1:"))
	dsn := fmt.Sprintf("host=127.0.0.1 port=%s user=postgres dbname=ledger_test sslmode=disable", strings.TrimPrefix(address, "127.0.0.1:"))
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	if !assert.Eventually(t, func() bool { return db.PingContext(ctx) == nil }, 30*time.Second, 100*time.Millisecond) {
		logs, _ := exec.Command("docker", "logs", "--tail", "40", name).CombinedOutput()
		t.Logf("Disposable PostgreSQL startup diagnostics: %s", logs)
		t.FailNow()
	}
	_, source, _, _ := runtime.Caller(0)
	content, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "..", "migrations", "275_gateway_request_ledger.sql"))
	require.NoError(t, err)
	for range 2 {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, string(content))
		if err != nil {
			_ = tx.Rollback()
		}
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
	}
	return db, dsn
}
