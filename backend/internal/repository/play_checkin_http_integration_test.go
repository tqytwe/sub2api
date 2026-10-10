//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// These tests use the harness's disposable PostgreSQL with all production
// migrations, repositories, transactions, ledger, service and route registration.
// Only authentication is replaced with an allowlist of synthetic fixture users.
func newCheckinContractServer(t *testing.T, enabled bool, users ...int64) *httptest.Server {
	t.Helper()
	settings := service.NewSettingService(&blindboxIntegrationSettingRepo{values: map[string]string{
		service.SettingKeyPlayCheckinEnabled:          strconv.FormatBool(enabled),
		service.SettingKeyPlayCheckinDailyReward:      "0.5",
		service.SettingKeyPlayCheckinStreakMilestones: `[{"days":7,"bonus":1}]`,
	}}, nil)
	userRepo := NewUserRepository(integrationEntClient, integrationDB)
	svc := service.ProvidePlayService(NewPlayRepository(integrationEntClient, integrationDB), userRepo, nil,
		settings, nil, integrationEntClient, service.NewBalanceLedgerService(integrationDB, nil, nil),
		nil, service.NewCouponService(NewCouponRepository(integrationDB)), nil)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	authFixture := func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.GetHeader("X-Checkin-Test-User"), 10, 64)
		for _, allowed := range users {
			if id == allowed {
				// Explicit GET fault injection for the live Vue recovery tests. All
				// successful reads and every grant still use the real service and PG.
				fault, _ := strconv.Atoi(c.GetHeader("X-Checkin-Test-Fault"))
				if c.Request.Method == http.MethodGet && (fault == 403 || fault == 503) {
					c.AbortWithStatusJSON(fault, gin.H{"code": fault, "reason": "FIXTURE_READ_FAILURE"})
					return
				}
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: id})
				c.Next()
				return
			}
		}
		c.AbortWithStatus(http.StatusUnauthorized)
	}
	routes.RegisterPlayRoutes(router.Group("/api/v1"), &handler.Handlers{Play: handler.NewPlayHandler(svc, nil)}, authFixture)
	authHandler := handler.NewAuthHandler(nil, nil, service.NewUserService(userRepo, nil, nil, nil), nil, nil, nil, nil, nil)
	router.GET("/api/v1/auth/me", authFixture, authHandler.GetCurrentUser)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func newCheckinExplorer(t *testing.T, verified bool, ageDays int) int64 {
	t.Helper()
	ctx := context.Background()
	user, err := integrationEntClient.User.Create().
		SetEmail(fmt.Sprintf("checkin-contract-%d@example.test", time.Now().UnixNano())).
		SetPasswordHash("test-only-unusable-hash").SetBalance(1).
		SetCreatedAt(time.Now().UTC().AddDate(0, 0, -ageDays)).Save(ctx)
	require.NoError(t, err)
	if verified {
		_, err = integrationDB.ExecContext(ctx, `INSERT INTO auth_identities
   (user_id, provider_type, provider_key, provider_subject, verified_at)
   VALUES ($1, 'email', 'checkin-contract', $2, NOW())`, user.ID, user.Email)
		require.NoError(t, err)
	}
	return user.ID
}

func seedCheckinCashPool(t *testing.T) {
	t.Helper()
	// Persist a deterministic, 100% balance pool. No draw, payment or paid API is mocked or called.
	var templateID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO coupon_templates
  (template_key, name, status, benefit_type, benefit_value, validity_mode, validity_days)
  VALUES ($1, 'checkin contract fixture', 'active', 'fixed_amount', 1, 'relative_days', 1)
  RETURNING id`, fmt.Sprintf("checkin-contract-%d", time.Now().UnixNano())).Scan(&templateID))
	repo := NewCouponRepository(integrationDB)
	pool, err := repo.SaveCouponRewardPool(context.Background(), service.CouponRewardPoolVersion{
		Activity: service.CouponRewardActivityCheckin, Version: fmt.Sprintf("contract-%d", time.Now().UnixNano()),
		Status: service.CouponRewardPoolStatusPublished, BalanceWeightBP: 10000, FallbackTemplateID: templateID,
		RewardConfig: service.CouponRewardPoolConfig{BalanceEntries: []service.BalanceRewardPoolEntry{
			{Name: "contract cash", Amount: 0.5, WeightBP: 10000, Enabled: true},
		}},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`DELETE FROM coupon_reward_pool_versions WHERE id = $1`, pool.ID)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`DELETE FROM coupon_templates WHERE id = $1`, templateID)
		require.NoError(t, err)
	})
}

type checkinContractResponse struct {
	Code   int                        `json:"code"`
	Reason string                     `json:"reason"`
	Data   map[string]json.RawMessage `json:"data"`
}

func checkinContractRequest(server *httptest.Server, userID int64, method, path string) (int, checkinContractResponse, error) {
	var body checkinContractResponse
	req, err := http.NewRequest(method, server.URL+"/api/v1/play/checkin"+path, nil)
	if err != nil {
		return 0, body, err
	}
	req.Header.Set("X-Checkin-Test-User", strconv.FormatInt(userID, 10))
	res, err := server.Client().Do(req)
	if err != nil {
		return 0, body, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusUnauthorized {
		err = json.NewDecoder(res.Body).Decode(&body)
	}
	return res.StatusCode, body, err
}

func assertCheckinDatabase(t *testing.T, userID int64, count int, energy int64, cash float64) {
	t.Helper()
	var checkins, snapshots, energyEntries, cashEntries, balanceEntries int
	var storedEnergy int64
	var balance, reward, credited float64
	err := integrationDB.QueryRow(`SELECT
  (SELECT COUNT(*) FROM play_checkins WHERE user_id=$1),
  (SELECT COUNT(*) FROM play_growth_eligibility_snapshots WHERE user_id=$1),
  (SELECT COUNT(*) FROM play_growth_energy_ledger WHERE user_id=$1),
  (SELECT COALESCE(SUM(amount),0) FROM play_growth_energy_ledger WHERE user_id=$1),
  (SELECT COUNT(*) FROM play_reward_ledger WHERE user_id=$1),
  (SELECT COALESCE(SUM(amount),0) FROM play_reward_ledger WHERE user_id=$1),
  (SELECT COUNT(*) FROM balance_transactions WHERE user_id=$1),
  (SELECT COALESCE(SUM(reward_amount),0) FROM play_checkins WHERE user_id=$1),
  balance FROM users WHERE id=$1`, userID).
		Scan(&checkins, &snapshots, &energyEntries, &storedEnergy, &cashEntries, &credited, &balanceEntries, &reward, &balance)
	require.NoError(t, err)
	require.Equal(t, count, checkins)
	require.Equal(t, count, snapshots)
	require.Equal(t, energy, storedEnergy)
	require.InDelta(t, cash, reward, 1e-8)
	require.InDelta(t, cash, credited, 1e-8)
	require.InDelta(t, 1+cash, balance, 1e-8)
	expectedCashEntries := 0
	if cash > 0 {
		expectedCashEntries = 1
	}
	require.Equal(t, expectedCashEntries, cashEntries)
	require.Equal(t, expectedCashEntries, balanceEntries)
	expectedEnergyEntries := 0
	if energy > 0 {
		expectedEnergyEntries = 1
	}
	require.Equal(t, expectedEnergyEntries, energyEntries)
	if count > 0 {
		var linked int
		require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM play_checkins c
   JOIN play_growth_eligibility_snapshots s ON s.id=c.growth_eligibility_snapshot_id
    AND s.user_id=c.user_id AND s.activity_date=c.checkin_date
   WHERE c.user_id=$1 AND s.source='checkin' AND s.action_id='checkin:' || c.user_id || ':' || c.checkin_date
    AND s.rule_version='v1'
    AND (($2::bigint > 0 AND EXISTS (SELECT 1 FROM play_growth_energy_ledger e
      WHERE e.eligibility_snapshot_id=s.id AND e.user_id=c.user_id AND e.action_id=s.action_id AND e.amount=$2))
     OR ($3::numeric > 0 AND EXISTS (SELECT 1 FROM play_reward_ledger r
      WHERE r.growth_eligibility_snapshot_id=s.id AND r.user_id=c.user_id AND r.amount=$3)))`, userID, energy, cash).Scan(&linked))
		require.Equal(t, 1, linked, "activity, immutable snapshot and ledger must identify the same grant")
	}
	t.Logf("DB reconciliation: checkins=%d snapshots=%d energy=%d cash=%g balance=%g", checkins, snapshots, storedEnergy, credited, balance)
}

func assertCheckinSnapshot(t *testing.T, userID int64, eligibility service.PlayGrowthEligibility) {
	t.Helper()
	var stored service.PlayGrowthEligibility
	err := integrationDB.QueryRow(`SELECT s.tier, s.reward_mode, s.primary_reason,
	 s.email_verified, s.account_age_days, s.has_recent_usage,
	 s.net_balance_recharge_30d, s.has_active_subscription
	 FROM play_checkins c JOIN play_growth_eligibility_snapshots s
	 ON s.id=c.growth_eligibility_snapshot_id WHERE c.user_id=$1`, userID).
		Scan(&stored.Tier, &stored.RewardMode, &stored.PrimaryReason, &stored.EmailVerified,
			&stored.AccountAgeDays, &stored.HasRecentUsage, &stored.NetBalanceRecharge30d, &stored.HasActiveSubscription)
	require.NoError(t, err)
	// Progress is derived from the persisted signals, not a snapshot column.
	eligibility.Progress = service.PlayGrowthEligibilityProgress{}
	require.Equal(t, eligibility, stored, "HTTP qualification must match the committed immutable snapshot")
}

func newCheckinMilestoneUser(t *testing.T, makeup bool) int64 {
	t.Helper()
	id := newCheckinExplorer(t, false, 8)
	qualifyBlindboxIntegrationUser(t, &service.User{ID: id})
	daysAgo := -1
	if makeup {
		daysAgo = -2
		// Synthetic completed recharge is eligibility evidence only. It does not
		// invoke a payment gateway or change this fixture user's starting balance.
		_, err := integrationDB.Exec(`INSERT INTO payment_orders
		 (user_id, amount, pay_amount, expires_at, status, order_type, completed_at)
		 VALUES ($1, 1, 1, NOW() + INTERVAL '1 hour', 'COMPLETED', 'balance', NOW())`, id)
		require.NoError(t, err)
	}
	require.NoError(t, NewPlayRepository(integrationEntClient, integrationDB).InsertCheckin(
		context.Background(), id, time.Now().UTC().AddDate(0, 0, daysAgo), 0, 6))
	return id
}

func assertCheckinMilestoneDatabase(t *testing.T, id int64, makeup bool) {
	t.Helper()
	entries := 2
	actionPrefix := "checkin:"
	if makeup {
		entries = 1
		actionPrefix = "checkin_makeup:"
	}
	var checkins, snapshots, rewards, balances, linked, energy int
	var reward, delta, balance float64
	err := integrationDB.QueryRow(`SELECT
	 (SELECT COUNT(*) FROM play_checkins WHERE user_id=$1),
	 (SELECT COUNT(*) FROM play_growth_eligibility_snapshots WHERE user_id=$1),
	 (SELECT COUNT(*) FROM play_reward_ledger WHERE user_id=$1),
	 (SELECT COUNT(*) FROM balance_transactions WHERE user_id=$1),
	 (SELECT COUNT(*) FROM play_growth_energy_ledger WHERE user_id=$1),
	 (SELECT SUM(reward_amount) FROM play_checkins WHERE user_id=$1),
	 (SELECT SUM(amount) FROM play_reward_ledger WHERE user_id=$1),
	 balance FROM users WHERE id=$1`, id).Scan(&checkins, &snapshots, &rewards, &balances, &energy, &reward, &delta, &balance)
	require.NoError(t, err)
	require.Equal(t, 2, checkins, "one zero-reward seed plus one actual grant")
	require.Equal(t, 1, snapshots)
	require.Equal(t, entries, rewards)
	require.Equal(t, entries, balances)
	require.Zero(t, energy)
	require.InDelta(t, 1.5, reward, 1e-8)
	require.InDelta(t, 1.5, delta, 1e-8)
	require.InDelta(t, 2.5, balance, 1e-8)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM play_reward_ledger r
	 JOIN play_growth_eligibility_snapshots s ON s.id=r.growth_eligibility_snapshot_id AND s.user_id=r.user_id
	 JOIN play_checkins c ON c.growth_eligibility_snapshot_id=s.id AND c.user_id=s.user_id AND c.checkin_date=s.activity_date
	 JOIN balance_transactions b ON b.user_id=r.user_id AND b.idempotency_key=r.idempotency_key AND b.balance_delta=r.amount
	 WHERE r.user_id=$1 AND c.streak_count=7 AND s.action_id=$2 || r.user_id || ':' || c.checkin_date`, id, actionPrefix).Scan(&linked))
	require.Equal(t, entries, linked, "reward, qualification, activity and balance must reconcile")
	t.Logf("Milestone reconciliation: makeup=%t checkins=%d snapshots=%d rewards=%d balances=%d reward=%.2f balance=%.2f", makeup, checkins, snapshots, rewards, balances, reward, balance)
}

func TestCheckinHTTPDatabaseContract(t *testing.T) {
	seedCheckinCashPool(t)
	for _, tc := range []struct {
		name, reason string
		verified     bool
		age          int
		cash         bool
	}{
		{"explorer", "no_recent_activity", true, 8, false},
		{"unverified", "email_unverified", false, 8, false},
		{"too_new", "account_too_new", true, 1, false},
		{"cash", "eligible", true, 8, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := newCheckinExplorer(t, tc.verified, tc.age)
			if tc.cash {
				qualifyBlindboxIntegrationUser(t, &service.User{ID: id})
			}
			server := newCheckinContractServer(t, true, id)
			status, before, err := checkinContractRequest(server, id, http.MethodGet, "/status")
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)
			require.JSONEq(t, "false", string(before.Data["checked_in_today"]))
			// A racing retry must lose the unique daily claim before a second reward.
			var wg sync.WaitGroup
			codes := make([]int, 2)
			bodies := make([]checkinContractResponse, 2)
			errs := make([]error, 2)
			start := make(chan struct{})
			for i := range 2 {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					codes[i], bodies[i], errs[i] = checkinContractRequest(server, id, http.MethodPost, "")
				}(i)
			}
			close(start)
			wg.Wait()
			var result checkinContractResponse
			successes := 0
			for i := range 2 {
				require.NoError(t, errs[i])
				if codes[i] == http.StatusOK {
					successes++
					result = bodies[i]
				} else {
					require.Equal(t, http.StatusConflict, codes[i])
					require.Equal(t, "PLAY_CHECKIN_ALREADY_DONE", bodies[i].Reason)
				}
			}
			require.Equal(t, 1, successes)
			energy := int64(1)
			cash := 0.0
			rewardType := "none"
			if tc.cash {
				energy = 0
				cash = 0.5
				rewardType = "balance"
			}
			assertCheckinDatabase(t, id, 1, energy, cash)
			require.Zero(t, result.Code)
			var eligibility service.PlayGrowthEligibility
			require.NoError(t, json.Unmarshal(result.Data["growth_eligibility"], &eligibility))
			require.Equal(t, tc.reason, eligibility.PrimaryReason)
			require.Equal(t, tc.reason, eligibility.Progress.NextAction)
			assertCheckinSnapshot(t, id, eligibility)
			require.JSONEq(t, string(before.Data["growth_eligibility"]), string(result.Data["growth_eligibility"]))
			for _, field := range []string{"reward_amount", "balance_added", "daily_reward_amount"} {
				require.JSONEq(t, fmt.Sprint(cash), string(result.Data[field]), "legacy field %s", field)
			}
			require.JSONEq(t, "0", string(result.Data["milestone_amount"]))
			require.JSONEq(t, strconv.Quote(rewardType), string(result.Data["reward_type"]))
			require.Equal(t, string(before.Data["server_date"]), string(result.Data["server_date"]))
			if tc.cash {
				require.NotContains(t, result.Data, "growth_energy")
				require.JSONEq(t, "1", string(result.Data["streak_count"]))
			} else {
				require.JSONEq(t, "1", string(result.Data["growth_energy"]))
			}
			require.NotContains(t, result.Data, "coupon")
			require.NotContains(t, result.Data, "redeem_code")
			status, duplicate, err := checkinContractRequest(server, id, http.MethodPost, "")
			require.NoError(t, err)
			require.Equal(t, http.StatusConflict, status)
			require.Equal(t, "PLAY_CHECKIN_ALREADY_DONE", duplicate.Reason)
			status, after, err := checkinContractRequest(server, id, http.MethodGet, "/status")
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)
			require.JSONEq(t, "true", string(after.Data["checked_in_today"]))
			assertCheckinDatabase(t, id, 1, energy, cash)
		})
	}
	t.Run("disabled_and_unauthenticated", func(t *testing.T) {
		id := newCheckinExplorer(t, true, 8)
		server := newCheckinContractServer(t, false, id)
		status, body, err := checkinContractRequest(server, id, http.MethodPost, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, status)
		require.Equal(t, "PLAY_FEATURE_DISABLED", body.Reason)
		status, _, err = checkinContractRequest(server, 0, http.MethodPost, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, status)
		assertCheckinDatabase(t, id, 0, 0, 0)
		status, _, err = checkinContractRequest(server, 0, http.MethodPost, "/makeup")
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, status)
		server = newCheckinContractServer(t, true, id)
		status, body, err = checkinContractRequest(server, id, http.MethodPost, "/makeup")
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, status)
		require.Equal(t, "PLAY_CHECKIN_MAKEUP_UNAVAILABLE", body.Reason)
		assertCheckinDatabase(t, id, 0, 0, 0)
	})
	for _, makeup := range []bool{false, true} {
		t.Run(fmt.Sprintf("milestone_makeup_%t", makeup), func(t *testing.T) {
			id := newCheckinMilestoneUser(t, makeup)
			server := newCheckinContractServer(t, true, id)
			path := ""
			if makeup {
				path = "/makeup"
			}
			var wg sync.WaitGroup
			codes := make([]int, 2)
			bodies := make([]checkinContractResponse, 2)
			errs := make([]error, 2)
			start := make(chan struct{})
			for i := range 2 {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					codes[i], bodies[i], errs[i] = checkinContractRequest(server, id, http.MethodPost, path)
				}(i)
			}
			close(start)
			wg.Wait()
			successes := 0
			for i := range 2 {
				require.NoError(t, errs[i])
				if codes[i] == http.StatusOK {
					successes++
					require.JSONEq(t, "1.5", string(bodies[i].Data["balance_added"]))
					require.JSONEq(t, "1", string(bodies[i].Data["milestone_bonus"]))
					require.JSONEq(t, "7", string(bodies[i].Data["streak_count"]))
				} else if makeup {
					require.Contains(t, []int{http.StatusBadRequest, http.StatusConflict}, codes[i])
					require.Contains(t, []string{"PLAY_CHECKIN_MAKEUP_UNAVAILABLE", "PLAY_CHECKIN_MAKEUP_ALREADY_DONE"}, bodies[i].Reason)
				} else {
					require.Equal(t, http.StatusConflict, codes[i])
					require.Equal(t, "PLAY_CHECKIN_ALREADY_DONE", bodies[i].Reason)
				}
			}
			require.Equal(t, 1, successes)
			status, _, err := checkinContractRequest(server, id, http.MethodPost, path)
			require.NoError(t, err)
			require.NotEqual(t, http.StatusOK, status)
			assertCheckinMilestoneDatabase(t, id, makeup)
		})
	}
	t.Run("frontend_live_http", func(t *testing.T) {
		if os.Getenv("CHECKIN_FRONTEND_CONTRACT") != "1" {
			t.Skip("run scripts/test-checkin-contract.sh to include the live frontend")
		}
		explorer := newCheckinExplorer(t, true, 8)
		cashID := newCheckinExplorer(t, false, 8)
		qualifyBlindboxIntegrationUser(t, &service.User{ID: cashID})
		milestoneID := newCheckinMilestoneUser(t, false)
		makeupID := newCheckinMilestoneUser(t, true)
		readOnlyID := newCheckinExplorer(t, true, 8)
		server := newCheckinContractServer(t, true, explorer, cashID, milestoneID, makeupID, readOnlyID)
		disabledServer := newCheckinContractServer(t, false, readOnlyID)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "pnpm", "exec", "vitest", "run", "src/views/user/__tests__/CheckInView.http.spec.ts")
		cmd.Dir = "../../../frontend"
		cmd.Env = append(os.Environ(), "CHECKIN_CONTRACT_BASE_URL="+server.URL+"/api/v1",
			"CHECKIN_CONTRACT_DISABLED_URL="+disabledServer.URL+"/api/v1", "CHECKIN_CONTRACT_READONLY="+strconv.FormatInt(readOnlyID, 10),
			"CHECKIN_CONTRACT_EXPLORER="+strconv.FormatInt(explorer, 10), "CHECKIN_CONTRACT_CASH="+strconv.FormatInt(cashID, 10),
			"CHECKIN_CONTRACT_MILESTONE="+strconv.FormatInt(milestoneID, 10), "CHECKIN_CONTRACT_MAKEUP="+strconv.FormatInt(makeupID, 10))
		output, err := cmd.CombinedOutput()
		t.Log(string(output))
		// Reconcile even on a frontend assertion failure: this is the regression
		// where a committed energy grant used to be rendered as a zero-cash reward.
		assertCheckinDatabase(t, explorer, 1, 1, 0)
		assertCheckinDatabase(t, cashID, 1, 0, 0.5)
		assertCheckinMilestoneDatabase(t, milestoneID, false)
		assertCheckinMilestoneDatabase(t, makeupID, true)
		assertCheckinDatabase(t, readOnlyID, 0, 0, 0)
		require.NoError(t, err)
	})
}
