//go:build integration

package service_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerLiveAuditFailurePreservesRedisAndSettlement(t *testing.T) {
	db := ledgertest.New(t)
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	for _, q := range []string{
		`INSERT INTO users(id,email,password_hash,balance) VALUES(101,'synthetic-live@example.invalid','disabled-fixture',100)`,
		`INSERT INTO groups(id,name,platform) VALUES(44,'synthetic-live','openai')`,
		`INSERT INTO api_keys(id,user_id,key,name,group_id) VALUES(201,101,'not-a-credential-live-fixture','synthetic-live',44)`,
		`INSERT INTO accounts(id,name,platform,type) VALUES(234,'synthetic-live','openai','oauth')`,
	} {
		_, err := db.Exec(q)
		require.NoError(t, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	name := "ledger-live-redis-" + uuid.NewString()
	out, err := exec.CommandContext(ctx, "docker", "run", "--rm", "-d", "--name", name, "-p", "127.0.0.1::6379", "redis:8.4-alpine").CombinedOutput()
	require.NoError(t, err, "disposable Redis startup: %s", out)
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	out, err = exec.CommandContext(ctx, "docker", "port", name, "6379/tcp").Output()
	require.NoError(t, err)
	address := strings.TrimSpace(string(out))
	require.True(t, strings.HasPrefix(address, "127.0.0.1:"))
	rdb := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = rdb.Close() })
	require.Eventually(t, func() bool { return rdb.Ping(ctx).Err() == nil }, 30*time.Second, 100*time.Millisecond)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	service.RequestLedgerLiveReviewFixture(t, db, repository.NewGatewayCache(rdb), repository.NewUsageLogRepository(client, db), repository.NewUsageBillingRepositoryWithLedger(client, db, service.NewBalanceLedgerService(db, nil, nil)), repository.NewLiveSettlementOutboxRepository(db))
}
