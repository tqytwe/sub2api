//go:build vipcreditintegration

package repository

import (
	"context"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"errors"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"os"
	"sync/atomic"
	"testing"
)

func TestVipCreditRedisFailureRecoveryAndLatePublisher(t *testing.T) {
	ctx := context.Background()
	db := creditDB(t)
	uid := creditUser(t, db, "0")
	configureCredit(t, db, uid, "1", "2", "4")
	addr := os.Getenv("VIP_CREDIT_TEST_REDIS_ADDR")
	require.NotEmpty(t, addr, "explicit disposable Redis required")
	rdb := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	require.NoError(t, rdb.Ping(ctx).Err())
	hook := &vipCreditRedisFailureHook{}
	hook.reject.Store(true)
	rdb.AddHook(hook)
	cache := NewBillingCache(rdb)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	svc := service.NewBillingCacheService(cache, NewUserRepository(client, db), nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	keys := service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})
	keys.SetAutoCredit(NewAutoCreditRepository(db), svc)
	issued, err := NewAutoCreditRepository(db).Evaluate(ctx, uid)
	require.NoError(t, err)
	require.True(t, issued.Issued)
	require.NoError(t, cache.SetUserBalance(ctx, uid, -3))
	err = svc.RecoverAutoCreditInvalidations(ctx)
	require.Error(t, err)
	require.ErrorContains(t, err, "fixture Redis DEL failure")
	var stage int
	require.NoError(t, db.QueryRow(`SELECT stage FROM auto_credit_cache_outbox WHERE user_id=$1`, uid).Scan(&stage))
	require.Zero(t, stage, "failed Redis invalidation cannot ack durable receipt")
	hook.reject.Store(false)
	require.NoError(t, svc.RecoverAutoCreditInvalidations(ctx))
	require.NoError(t, cache.SetUserBalance(ctx, uid, -5)) // delayed precommit publisher between passes
	_, err = db.Exec(`UPDATE auto_credit_cache_outbox SET available_at=NOW() WHERE user_id=$1`, uid)
	require.NoError(t, err)
	require.NoError(t, svc.RecoverAutoCreditInvalidations(ctx))
	_, err = cache.GetUserBalance(ctx, uid)
	require.ErrorIs(t, err, redis.Nil)
	require.NoError(t, cache.SetUserBalance(ctx, uid, -9)) // arbitrarily late publisher after the second pass
	balance, err := svc.GetUserBalance(ctx, uid)
	require.NoError(t, err)
	require.Equal(t, float64(2), balance, "VIP admission must use authoritative DB even after a late stale cache publisher")
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM auto_credit_ledger WHERE user_id=$1`, uid).Scan(&count))
	require.Equal(t, 1, count, "cache retries must not issue money")
}

// Keep the real Redis cache and inject a failure only during invalidation.
type vipCreditRedisFailureHook struct{ reject atomic.Bool }

func (*vipCreditRedisFailureHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (*vipCreditRedisFailureHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *vipCreditRedisFailureHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if h.reject.Load() && cmd.Name() == "del" {
			return errors.New("fixture Redis DEL failure")
		}
		return next(ctx, cmd)
	}
}
