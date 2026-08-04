package handler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	mobileWebSearchBudgetUserRPMDefault     = 12
	mobileWebSearchBudgetUserDailyDefault   = 120
	mobileWebSearchBudgetGlobalRPMDefault   = 600
	mobileWebSearchBudgetGlobalDailyDefault = 10000
	mobileWebSearchBudgetMaxLimit           = 1000000
	mobileWebSearchBudgetStoreTimeout       = time.Second
)

var errMobileWebSearchBudgetUnavailable = errors.New("mobile web search budget store unavailable")

type mobileWebSearchBudgetExceededError struct {
	retryAfter time.Duration
}

func (e *mobileWebSearchBudgetExceededError) Error() string {
	return "mobile web search budget exceeded"
}

func newMobileWebSearchBudgetExceededError(retryAfter time.Duration) error {
	if retryAfter <= 0 {
		retryAfter = time.Minute
	}
	return &mobileWebSearchBudgetExceededError{retryAfter: retryAfter}
}

type mobileWebSearchBudget interface {
	Reserve(context.Context, int64) (time.Duration, error)
}

type mobileWebSearchBudgetConfig struct {
	UserRPM     int
	UserDaily   int
	GlobalRPM   int
	GlobalDaily int
}

func mobileWebSearchBudgetConfigFromEnvironment() mobileWebSearchBudgetConfig {
	return mobileWebSearchBudgetConfig{
		UserRPM:     mobileWebSearchPositiveEnvInt("MOBILE_WEB_SEARCH_USER_RPM", mobileWebSearchBudgetUserRPMDefault),
		UserDaily:   mobileWebSearchPositiveEnvInt("MOBILE_WEB_SEARCH_USER_DAILY_LIMIT", mobileWebSearchBudgetUserDailyDefault),
		GlobalRPM:   mobileWebSearchPositiveEnvInt("MOBILE_WEB_SEARCH_GLOBAL_RPM", mobileWebSearchBudgetGlobalRPMDefault),
		GlobalDaily: mobileWebSearchPositiveEnvInt("MOBILE_WEB_SEARCH_GLOBAL_DAILY_LIMIT", mobileWebSearchBudgetGlobalDailyDefault),
	}
}

func mobileWebSearchPositiveEnvInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 || value > mobileWebSearchBudgetMaxLimit {
		return fallback
	}
	return value
}

// mobileWebSearchBudgetScript checks all four counters before incrementing any
// of them. This prevents a rejected request from partially consuming a second
// budget and keeps the decision atomic when several API instances race.
var mobileWebSearchBudgetScript = redis.NewScript(`
local limits = {
  tonumber(ARGV[1]), tonumber(ARGV[2]), tonumber(ARGV[3]), tonumber(ARGV[4])
}
for i = 1, 4 do
  local current = tonumber(redis.call('GET', KEYS[i]) or '0')
  if current >= limits[i] then
    local ttl = redis.call('PTTL', KEYS[i])
    return {0, i, ttl}
  end
end
for i = 1, 4 do
  local current = redis.call('INCR', KEYS[i])
  local ttl = redis.call('PTTL', KEYS[i])
  if current == 1 or ttl == -1 then
    redis.call('PEXPIRE', KEYS[i], ARGV[4 + i])
  end
end
return {1, 0, 0}
`)

type redisMobileWebSearchBudget struct {
	client *redis.Client
	config mobileWebSearchBudgetConfig
	now    func() time.Time
}

func newRedisMobileWebSearchBudget(client *redis.Client) mobileWebSearchBudget {
	return &redisMobileWebSearchBudget{
		client: client,
		config: mobileWebSearchBudgetConfigFromEnvironment(),
		now:    time.Now,
	}
}

// Reserve charges an upstream attempt immediately before the provider call.
// It intentionally does not refund a timeout or 5xx: after either outcome the
// server cannot prove that Exa did not receive or bill the request. Successful
// retries are instead replayed by the idempotency coordinator before Reserve
// is reached; a confirmed failed retry is a new provider attempt.
func (b *redisMobileWebSearchBudget) Reserve(ctx context.Context, userID int64) (time.Duration, error) {
	if b == nil || b.client == nil || userID <= 0 {
		return 0, errMobileWebSearchBudgetUnavailable
	}
	now := time.Now()
	if b.now != nil {
		now = b.now()
	}
	now = now.UTC()
	minuteStamp := now.Format("200601021504")
	dayStamp := now.Format("20060102")
	keys := []string{
		"{mobile-web-search}:user:" + strconv.FormatInt(userID, 10) + ":minute:" + minuteStamp,
		"{mobile-web-search}:user:" + strconv.FormatInt(userID, 10) + ":day:" + dayStamp,
		"{mobile-web-search}:global:minute:" + minuteStamp,
		"{mobile-web-search}:global:day:" + dayStamp,
	}
	minuteTTL := durationUntilNextMinute(now)
	dayTTL := durationUntilNextDay(now)
	args := []any{
		b.config.UserRPM,
		b.config.UserDaily,
		b.config.GlobalRPM,
		b.config.GlobalDaily,
		minuteTTL.Milliseconds(),
		dayTTL.Milliseconds(),
		minuteTTL.Milliseconds(),
		dayTTL.Milliseconds(),
	}
	budgetCtx, cancel := context.WithTimeout(ctx, mobileWebSearchBudgetStoreTimeout)
	defer cancel()
	values, err := mobileWebSearchBudgetScript.Run(budgetCtx, b.client, keys, args...).Slice()
	if err != nil {
		return 0, errMobileWebSearchBudgetUnavailable
	}
	if len(values) < 3 {
		return 0, errMobileWebSearchBudgetUnavailable
	}
	allowed, err := mobileWebSearchBudgetInt64(values[0])
	if err != nil {
		return 0, errMobileWebSearchBudgetUnavailable
	}
	if allowed == 1 {
		return 0, nil
	}
	retryMillis, err := mobileWebSearchBudgetInt64(values[2])
	if err != nil {
		return 0, errMobileWebSearchBudgetUnavailable
	}
	if retryMillis <= 0 {
		retryMillis = minuteTTL.Milliseconds()
	}
	retryAfter := time.Duration(retryMillis) * time.Millisecond
	return retryAfter, newMobileWebSearchBudgetExceededError(retryAfter)
}

func mobileWebSearchBudgetInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected budget value type %T", value)
	}
}

func durationUntilNextMinute(now time.Time) time.Duration {
	return positiveBudgetTTL(now.Truncate(time.Minute).Add(time.Minute).Sub(now) + time.Second)
}

func durationUntilNextDay(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return positiveBudgetTTL(next.Sub(now) + time.Second)
}

func positiveBudgetTTL(value time.Duration) time.Duration {
	if value <= 0 {
		return time.Millisecond
	}
	return value
}

// The unmetered implementation is used only by the package-level test
// constructor. Production construction always injects the Redis-backed
// implementation above, which fails closed when Redis is unavailable.
type unmeteredMobileWebSearchBudget struct{}

func (unmeteredMobileWebSearchBudget) Reserve(context.Context, int64) (time.Duration, error) {
	return 0, nil
}
