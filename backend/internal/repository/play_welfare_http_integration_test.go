//go:build integration

package repository

import (
	"bytes"
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

// Real route -> handler -> service -> repositories -> disposable PostgreSQL.
// Only the synthetic identity allowlist and deterministic fixture configuration
// replace infrastructure; coupon and redeem issuance use the production services.
func newWelfareContractServer(t *testing.T, users ...int64) *httptest.Server {
	t.Helper()
	pool, err := json.Marshal(service.PlayBlindboxPool{Version: "welfare-fixture", Cost: 0.5, RTPCap: 0.9,
		Tiers: []service.PlayBlindboxTier{{Amount: 0.2, Weight: 10000}}})
	require.NoError(t, err)
	settings := service.NewSettingService(&blindboxIntegrationSettingRepo{values: map[string]string{
		service.SettingKeyPlayQuizEnabled: "true", service.SettingKeyPlayQuizQuestionsPerDay: "2",
		service.SettingKeyPlayQuizRewardPerCorrect: "0.1", service.SettingKeyPlayBlindboxEnabled: "true",
		service.SettingKeyPlayBlindboxDailyLimit: "2", service.SettingKeyPlayBlindboxPoolJSON: string(pool),
	}}, nil)
	userRepo := NewUserRepository(integrationEntClient, integrationDB)
	ledger := service.NewBalanceLedgerService(integrationDB, nil, nil)
	redeem := service.NewRedeemService(NewRedeemCodeRepository(integrationEntClient), userRepo, nil, nil, nil, integrationEntClient, nil, nil, ledger)
	svc := service.ProvidePlayService(NewPlayRepository(integrationEntClient, integrationDB), userRepo, nil, settings, nil, integrationEntClient, ledger, nil, service.NewCouponService(NewCouponRepository(integrationDB)), redeem)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	fixtureAuth := func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.GetHeader("X-Welfare-Test-User"), 10, 64)
		for _, allowed := range users {
			if id == allowed {
				if fault := c.GetHeader("X-Welfare-Test-Read-Fault"); c.Request.Method == http.MethodGet && (fault == "403" || fault == "503") {
					code, _ := strconv.Atoi(fault)
					c.AbortWithStatusJSON(code, gin.H{"code": code, "reason": "FIXTURE_READ_FAILURE"})
					return
				}
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: id})
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "reason": "UNAUTHORIZED"})
	}
	routes.RegisterPlayRoutes(router.Group("/api/v1"), &handler.Handlers{Play: handler.NewPlayHandler(svc, nil)}, fixtureAuth)
	authHandler := handler.NewAuthHandler(nil, nil, service.NewUserService(userRepo, nil, nil, nil), nil, nil, nil, nil, nil)
	router.GET("/api/v1/auth/me", fixtureAuth, authHandler.GetCurrentUser)
	var lostMu sync.Mutex
	lost := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-Welfare-Test-Lose-Response") != "1" {
			router.ServeHTTP(w, r)
			return
		}
		// Settle via real HTTP handlers/PG, then close the transport before the
		// response arrives. A retry really has an unknown committed outcome.
		recorded := httptest.NewRecorder()
		router.ServeHTTP(recorded, r)
		key := r.Header.Get("X-Welfare-Test-User") + r.URL.Path
		lostMu.Lock()
		drop := recorded.Code == 200 && !lost[key]
		if drop {
			lost[key] = true
		}
		lostMu.Unlock()
		if drop {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		for key, values := range recorded.Header() {
			for _, v := range values {
				w.Header().Add(key, v)
			}
		}
		w.WriteHeader(recorded.Code)
		_, _ = w.Write(recorded.Body.Bytes())
	}))
	t.Cleanup(server.Close)
	return server
}

func seedWelfarePool(t *testing.T, activity service.CouponRewardActivity, kind string) *service.CouponRewardPoolVersion {
	t.Helper()
	var templateID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO coupon_templates
  (template_key,name,status,benefit_type,benefit_value,validity_mode,validity_days,applicable_scopes)
  VALUES($1,'隔离福利券','active','fixed_amount',1,'relative_days',3,'["balance"]'::jsonb) RETURNING id`, fmt.Sprintf("welfare-%d", time.Now().UnixNano())).Scan(&templateID))
	version := fmt.Sprintf("welfare-%d", time.Now().UnixNano())
	pool := service.CouponRewardPoolVersion{Activity: activity, Version: version, Status: service.CouponRewardPoolStatusPublished, FallbackTemplateID: templateID}
	switch kind {
	case "coupon":
		pool.CouponWeightBP = 10000
		var fallbackID int64
		require.NoError(t, integrationDB.QueryRow(`INSERT INTO coupon_templates
 (template_key,name,status,benefit_type,benefit_value,validity_mode,validity_days,applicable_scopes)
 VALUES($1,'隔离保底券','active','fixed_amount',1,'relative_days',3,'["balance"]'::jsonb) RETURNING id`, version+"-fallback").Scan(&fallbackID))
		pool.FallbackTemplateID = fallbackID
		pool.Entries = []service.CouponRewardPoolEntry{{TemplateID: templateID, WeightBP: 10000, Enabled: true}, {TemplateID: fallbackID, WeightBP: 1, Enabled: true}}
	case "redeem_code":
		pool.RedeemCodeWeightBP = 10000
		pool.RewardConfig.RedeemEntries = []service.RedeemRewardPoolEntry{{Name: "隔离兑换码", BatchName: version, CodeType: "balance", WeightBP: 10000, Enabled: true, DeliveryMode: "code"}}
		expires := time.Now().UTC().Add(72 * time.Hour)
		for i := 0; i < 4; i++ {
			require.NoError(t, NewRedeemCodeRepository(integrationEntClient).Create(context.Background(), &service.RedeemCode{
				Code: fmt.Sprintf("FIXTURE-%d-%d", time.Now().UnixNano(), i), Type: "balance", Value: 0.3, Status: service.StatusUnused, BatchName: version, ExpiresAt: &expires,
			}))
		}
	default:
		pool.BalanceWeightBP = 10000
		pool.RewardConfig.BalanceEntries = []service.BalanceRewardPoolEntry{{Name: "隔离余额奖励", Amount: 0.2, WeightBP: 10000, Enabled: true}}
	}
	require.NoError(t, service.ValidateCouponRewardPool(pool))
	saved, err := NewCouponRepository(integrationDB).SaveCouponRewardPool(context.Background(), pool)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`UPDATE coupon_reward_pool_versions SET status='retired' WHERE id=$1`, saved.ID)
		require.NoError(t, err)
	})
	return saved
}

func welfareRequest(server *httptest.Server, user int64, method, path string, payload any, key string) (int, checkinContractResponse, error) {
	var out checkinContractResponse
	var data []byte
	var err error
	if payload != nil {
		data, err = json.Marshal(payload)
		if err != nil {
			return 0, out, err
		}
	}
	req, err := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(data))
	if err != nil {
		return 0, out, err
	}
	req.Header.Set("X-Welfare-Test-User", strconv.FormatInt(user, 10))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "zh")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := server.Client().Do(req)
	if err != nil {
		return 0, out, err
	}
	defer func() { _ = res.Body.Close() }()
	err = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out, err
}
func welfareField[T any](t *testing.T, r checkinContractResponse, key string) T {
	t.Helper()
	var v T
	require.NotEmpty(t, r.Data[key], "missing field %s", key)
	require.NoError(t, json.Unmarshal(r.Data[key], &v))
	return v
}
func welfareQuizAnswers(t *testing.T, server *httptest.Server, user int64) []service.PlayQuizAnswer {
	t.Helper()
	code, r, err := welfareRequest(server, user, "GET", "/play/quiz/today", nil, "")
	require.NoError(t, err)
	require.Equal(t, 200, code)
	questions := welfareField[[]service.PlayQuizQuestion](t, r, "questions")
	require.Len(t, questions, 2)
	answers := make([]service.PlayQuizAnswer, 0, len(questions))
	for _, q := range questions {
		var correct int
		require.NoError(t, integrationDB.QueryRow(`SELECT correct_index FROM play_quiz_questions WHERE id=$1`, q.ID).Scan(&correct))
		answers = append(answers, service.PlayQuizAnswer{QuestionID: q.ID, ChoiceIndex: correct})
	}
	return answers
}

func welfareQuizPayload(answers []service.PlayQuizAnswer) map[string]any {
	wire := make([]map[string]any, 0, len(answers))
	for _, a := range answers {
		wire = append(wire, map[string]any{"question_id": a.QuestionID, "choice_index": a.ChoiceIndex})
	}
	return map[string]any{"answers": wire}
}

func assertWelfareQuizDB(t *testing.T, user int64, count, energy, coupons, codes int, cash float64) {
	t.Helper()
	var attempts, snapshots, storedEnergy, issuedCoupons, issuedCodes, ledgerEntries, balanceEntries int
	var balance, reward, ledgerTotal float64
	require.NoError(t, integrationDB.QueryRow(`SELECT
 (SELECT COUNT(*) FROM play_quiz_attempts WHERE user_id=$1),
 (SELECT COUNT(*) FROM play_growth_eligibility_snapshots WHERE user_id=$1),
 (SELECT COALESCE(SUM(amount),0) FROM play_growth_energy_ledger WHERE user_id=$1),
 (SELECT COUNT(*) FROM user_coupons WHERE user_id=$1),
 (SELECT COUNT(*) FROM redeem_codes WHERE issued_to=$1),
 (SELECT COUNT(*) FROM play_reward_ledger WHERE user_id=$1),
 (SELECT COUNT(*) FROM balance_transactions WHERE user_id=$1),
 (SELECT COALESCE(SUM(reward_amount),0) FROM play_quiz_attempts WHERE user_id=$1),
 (SELECT COALESCE(SUM(amount),0) FROM play_reward_ledger WHERE user_id=$1),balance FROM users WHERE id=$1`, user).
		Scan(&attempts, &snapshots, &storedEnergy, &issuedCoupons, &issuedCodes, &ledgerEntries, &balanceEntries, &reward, &ledgerTotal, &balance))
	require.Equal(t, count, attempts)
	require.Equal(t, count, snapshots)
	require.Equal(t, energy, storedEnergy)
	require.Equal(t, coupons, issuedCoupons)
	require.Equal(t, codes, issuedCodes)
	expectedCashEntries := 0
	if cash > 0 {
		expectedCashEntries = 1
	}
	require.Equal(t, expectedCashEntries, ledgerEntries)
	require.Equal(t, expectedCashEntries, balanceEntries)
	require.InDelta(t, cash, reward, 1e-8)
	require.InDelta(t, cash, ledgerTotal, 1e-8)
	require.InDelta(t, 1+cash, balance, 1e-8)
	var linked int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM play_quiz_attempts a JOIN play_growth_eligibility_snapshots s ON s.id=a.growth_eligibility_snapshot_id AND s.user_id=a.user_id AND s.activity_date=a.attempt_date WHERE a.user_id=$1 AND s.source='quiz' AND s.action_id='quiz:' || a.user_id || ':' || a.attempt_date`, user).Scan(&linked))
	require.Equal(t, count, linked)
	t.Logf("quiz DB: attempts=%d snapshots=%d energy=%d coupons=%d codes=%d reward=%g balance=%g", attempts, snapshots, storedEnergy, issuedCoupons, issuedCodes, reward, balance)
}

func TestQuizHTTPDatabaseContract(t *testing.T) {
	_ = testEntClient(t)
	for _, kind := range []string{"energy", "balance", "coupon", "redeem_code"} {
		t.Run(kind, func(t *testing.T) {
			if kind != "energy" {
				seedWelfarePool(t, service.CouponRewardActivityQuiz, kind)
			}
			user := newCheckinExplorer(t, false, 8)
			if kind != "energy" {
				qualifyBlindboxIntegrationUser(t, &service.User{ID: user})
			}
			server := newWelfareContractServer(t, user)
			answers := welfareQuizAnswers(t, server, user)
			payload := welfareQuizPayload(answers)
			// Multi-tab / same-day concurrent submission: exactly one commits.
			const workers = 6
			var wg sync.WaitGroup
			results := make(chan struct {
				code int
				body checkinContractResponse
				err  error
			}, workers)
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					code, body, err := welfareRequest(server, user, "POST", "/play/quiz/submit", payload, "")
					results <- struct {
						code int
						body checkinContractResponse
						err  error
					}{code, body, err}
				}()
			}
			wg.Wait()
			close(results)
			wins := 0
			var granted checkinContractResponse
			for r := range results {
				require.NoError(t, r.err)
				if r.code == 200 {
					wins++
					granted = r.body
				} else {
					require.Equal(t, "PLAY_QUIZ_ALREADY_DONE", r.body.Reason)
				}
			}
			require.Equal(t, 1, wins)
			require.Equal(t, 2, welfareField[int](t, granted, "score"))
			require.Equal(t, 2, welfareField[int](t, granted, "total"))
			code, reloaded, err := welfareRequest(server, user, "GET", "/play/quiz/today", nil, "")
			require.NoError(t, err)
			require.Equal(t, 200, code)
			require.True(t, welfareField[bool](t, reloaded, "already_submitted"))
			require.Equal(t, 2, welfareField[int](t, reloaded, "previous_score"))
			require.Empty(t, welfareField[[]service.PlayQuizQuestion](t, reloaded, "questions"))
			switch kind {
			case "energy":
				require.Equal(t, 1, welfareField[int](t, granted, "growth_energy"))
				require.Equal(t, 1, welfareField[int](t, reloaded, "previous_growth_energy"))
				assertWelfareQuizDB(t, user, 1, 1, 0, 0, 0)
			case "balance":
				require.Equal(t, 0.2, welfareField[float64](t, reloaded, "previous_reward"))
				assertWelfareQuizDB(t, user, 1, 0, 0, 0, 0.2)
			case "coupon":
				coupon := welfareField[service.PlayCouponRewardSummary](t, granted, "coupon")
				restored := welfareField[service.PlayCouponRewardSummary](t, reloaded, "previous_coupon")
				require.Equal(t, coupon, restored)
				require.True(t, coupon.ExpiresAt.After(coupon.ValidFrom))
				assertWelfareQuizDB(t, user, 1, 0, 1, 0, 0)
			case "redeem_code":
				reward := welfareField[service.PlayRedeemCodeRewardSummary](t, granted, "redeem_code")
				require.Equal(t, reward, welfareField[service.PlayRedeemCodeRewardSummary](t, reloaded, "previous_redeem_code"))
				require.Equal(t, service.StatusIssued, reward.Status)
				require.NotNil(t, reward.ExpiresAt)
				assertWelfareQuizDB(t, user, 1, 0, 0, 1, 0)
			}
		})
	}
	t.Run("invalid_and_zero_score", func(t *testing.T) {
		seedWelfarePool(t, service.CouponRewardActivityQuiz, "balance")
		user := newCheckinExplorer(t, false, 8)
		qualifyBlindboxIntegrationUser(t, &service.User{ID: user})
		server := newWelfareContractServer(t, user)
		answers := welfareQuizAnswers(t, server, user)
		for _, bad := range [][]service.PlayQuizAnswer{nil, answers[:1], {answers[0], answers[0]}, {{QuestionID: answers[0].QuestionID, ChoiceIndex: 99}, answers[1]}, {{QuestionID: -1, ChoiceIndex: 0}, answers[1]}} {
			code, _, err := welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(bad), "")
			require.NoError(t, err)
			require.Equal(t, 400, code)
			assertWelfareQuizDB(t, user, 0, 0, 0, 0, 0)
		}
		for i := range answers {
			answers[i].ChoiceIndex = (answers[i].ChoiceIndex + 1) % 2
		}
		code, r, err := welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(answers), "")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		require.Equal(t, 0, welfareField[int](t, r, "score"))
		assertWelfareQuizDB(t, user, 1, 0, 0, 0, 0)
	})
	t.Run("authentication_and_unpublished_pool", func(t *testing.T) {
		user := newCheckinExplorer(t, false, 8)
		qualifyBlindboxIntegrationUser(t, &service.User{ID: user})
		server := newWelfareContractServer(t, user)
		for _, method := range []string{"GET", "POST"} {
			path := "/play/quiz/today"
			if method == "POST" {
				path = "/play/quiz/submit"
			}
			code, _, err := welfareRequest(server, 0, method, path, nil, "")
			require.NoError(t, err)
			require.Equal(t, 401, code)
		}
		code, r, err := welfareRequest(server, user, "GET", "/play/quiz/today", nil, "")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		require.False(t, welfareField[bool](t, r, "coupon_pool_ready"))
		code, r, err = welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload([]service.PlayQuizAnswer{{QuestionID: 1, ChoiceIndex: 0}}), "")
		require.NoError(t, err)
		require.Equal(t, 503, code)
		require.Equal(t, "COUPON_REWARD_POOL_UNAVAILABLE", r.Reason)
		assertWelfareQuizDB(t, user, 0, 0, 0, 0, 0)
	})
	t.Run("stale_questions_and_completion_reward", func(t *testing.T) {
		seedWelfarePool(t, service.CouponRewardActivityQuiz, "balance")
		user := newCheckinExplorer(t, false, 8)
		qualifyBlindboxIntegrationUser(t, &service.User{ID: user})
		server := newWelfareContractServer(t, user)
		old := welfareQuizAnswers(t, server, user)
		_, err := integrationDB.Exec(`UPDATE play_quiz_questions SET active=false WHERE id=$1`, old[0].QuestionID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := integrationDB.Exec(`UPDATE play_quiz_questions SET active=true WHERE id=$1`, old[0].QuestionID)
			require.NoError(t, err)
		})
		code, _, err := welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(old), "")
		require.NoError(t, err)
		require.Equal(t, 400, code)
		assertWelfareQuizDB(t, user, 0, 0, 0, 0, 0)
		fresh := welfareQuizAnswers(t, server, user)
		fresh[0].ChoiceIndex = (fresh[0].ChoiceIndex + 1) % 2
		code, r, err := welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(fresh), "")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		require.Equal(t, 1, welfareField[int](t, r, "score"))
		// Existing rule: one correct answer earns the complete two-question reward.
		require.Equal(t, 0.2, welfareField[float64](t, r, "reward_amount"))
		assertWelfareQuizDB(t, user, 1, 0, 0, 0, 0.2)
	})
	t.Run("expired_inventory_rolls_back_attempt", func(t *testing.T) {
		pool := seedWelfarePool(t, service.CouponRewardActivityQuiz, "redeem_code")
		user := newCheckinExplorer(t, false, 8)
		qualifyBlindboxIntegrationUser(t, &service.User{ID: user})
		server := newWelfareContractServer(t, user)
		answers := welfareQuizAnswers(t, server, user)
		_, err := integrationDB.Exec(`UPDATE redeem_codes SET expires_at=NOW()-INTERVAL '1 second' WHERE batch_name=$1`, pool.Version)
		require.NoError(t, err)
		code, r, err := welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(answers), "")
		require.NoError(t, err)
		require.Equal(t, 503, code)
		require.Equal(t, "COUPON_REWARD_POOL_UNAVAILABLE", r.Reason)
		assertWelfareQuizDB(t, user, 0, 0, 0, 0, 0)
		_, err = integrationDB.Exec(`UPDATE redeem_codes SET expires_at=NOW()+INTERVAL '1 day' WHERE batch_name=$1`, pool.Version)
		require.NoError(t, err)
		code, _, err = welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(answers), "")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		assertWelfareQuizDB(t, user, 1, 0, 0, 1, 0)
	})
	t.Run("paused_coupon_pool_and_historical_replay", func(t *testing.T) {
		pool := seedWelfarePool(t, service.CouponRewardActivityQuiz, "coupon")
		user := newCheckinExplorer(t, false, 8)
		qualifyBlindboxIntegrationUser(t, &service.User{ID: user})
		server := newWelfareContractServer(t, user)
		answers := welfareQuizAnswers(t, server, user)
		_, err := integrationDB.Exec(`UPDATE coupon_templates SET status='paused' WHERE id IN (SELECT template_id FROM coupon_reward_pool_entries WHERE pool_version_id=$1)`, pool.ID)
		require.NoError(t, err)
		code, r, err := welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(answers), "")
		require.NoError(t, err)
		require.Equal(t, 503, code)
		require.Equal(t, "COUPON_REWARD_POOL_UNAVAILABLE", r.Reason)
		assertWelfareQuizDB(t, user, 0, 0, 0, 0, 0)
		_, err = integrationDB.Exec(`UPDATE coupon_templates SET status='active' WHERE id IN (SELECT template_id FROM coupon_reward_pool_entries WHERE pool_version_id=$1)`, pool.ID)
		require.NoError(t, err)
		code, granted, err := welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(answers), "")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		_, err = integrationDB.Exec(`UPDATE coupon_reward_pool_versions SET status='retired' WHERE id=$1`, pool.ID)
		require.NoError(t, err)
		code, reloaded, err := welfareRequest(server, user, "GET", "/play/quiz/today", nil, "")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		require.True(t, welfareField[bool](t, reloaded, "already_submitted"))
		require.False(t, welfareField[bool](t, reloaded, "coupon_pool_ready"))
		require.Equal(t, welfareField[service.PlayCouponRewardSummary](t, granted, "coupon"), welfareField[service.PlayCouponRewardSummary](t, reloaded, "previous_coupon"))
		assertWelfareQuizDB(t, user, 1, 0, 1, 0, 0)
	})
	t.Run("ledger_failure_rolls_back_before_retry", func(t *testing.T) {
		seedWelfarePool(t, service.CouponRewardActivityQuiz, "balance")
		user := newCheckinExplorer(t, false, 8)
		qualifyBlindboxIntegrationUser(t, &service.User{ID: user})
		server := newWelfareContractServer(t, user)
		answers := welfareQuizAnswers(t, server, user)
		remove := installWelfareLedgerFailure(t, user)
		code, _, err := welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(answers), "")
		require.NoError(t, err)
		require.Equal(t, 500, code)
		assertWelfareQuizDB(t, user, 0, 0, 0, 0, 0)
		remove()
		code, _, err = welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(answers), "")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		assertWelfareQuizDB(t, user, 1, 0, 0, 0, 0.2)
	})
	t.Run("previous_day_does_not_consume_today", func(t *testing.T) {
		seedWelfarePool(t, service.CouponRewardActivityQuiz, "balance")
		user := newCheckinExplorer(t, false, 8)
		qualifyBlindboxIntegrationUser(t, &service.User{ID: user})
		server := newWelfareContractServer(t, user)
		_, err := integrationDB.Exec(`INSERT INTO play_quiz_attempts(user_id,attempt_date,score,total,reward_amount,answers) VALUES($1,CURRENT_DATE-1,0,2,0,'{}')`, user)
		require.NoError(t, err)
		answers := welfareQuizAnswers(t, server, user)
		code, r, err := welfareRequest(server, user, "POST", "/play/quiz/submit", welfareQuizPayload(answers), "")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		var today, total, ledger int
		var balance float64
		require.NoError(t, integrationDB.QueryRow(`SELECT (SELECT COUNT(*) FROM play_quiz_attempts WHERE user_id=$1 AND attempt_date=$2),(SELECT COUNT(*) FROM play_quiz_attempts WHERE user_id=$1),(SELECT COUNT(*) FROM balance_transactions WHERE user_id=$1),balance FROM users WHERE id=$1`, user, welfareField[string](t, r, "server_date")).Scan(&today, &total, &ledger, &balance))
		require.Equal(t, 1, today)
		require.Equal(t, 2, total)
		require.Equal(t, 1, ledger)
		require.InDelta(t, 1.2, balance, 1e-8)
	})

}

func TestQuizVueDatabaseContract(t *testing.T) {
	if os.Getenv("WELFARE_FRONTEND_CONTRACT") != "1" {
		t.Skip("run scripts/test-welfare-contract.sh for live Vue/PG contract")
	}
	_ = testEntClient(t)
	for _, kind := range []string{"energy", "balance", "coupon", "redeem_code"} {
		t.Run(kind, func(t *testing.T) {
			if kind != "energy" {
				seedWelfarePool(t, service.CouponRewardActivityQuiz, kind)
			}
			normal := newCheckinExplorer(t, false, 8)
			unknown := newCheckinExplorer(t, false, 8)
			if kind != "energy" {
				for _, id := range []int64{normal, unknown} {
					qualifyBlindboxIntegrationUser(t, &service.User{ID: id})
				}
			}
			server := newWelfareContractServer(t, normal, unknown)
			fixtures := map[string]any{}
			for label, id := range map[string]int64{"normal": normal, "unknown": unknown} {
				fixtures[label] = map[string]any{"user": id, "answers": welfareQuizAnswers(t, server, id)}
			}
			encoded, err := json.Marshal(fixtures)
			require.NoError(t, err)
			cmd := exec.Command("pnpm", "exec", "vitest", "run", "src/views/public/__tests__/QuizQuestView.pg.spec.ts")
			cmd.Dir = "../../../frontend"
			cmd.Env = append(os.Environ(), "WELFARE_CONTRACT_BASE_URL="+server.URL+"/api/v1", "WELFARE_CONTRACT_MODE="+kind, "WELFARE_CONTRACT_FIXTURES="+string(encoded))
			output, runErr := cmd.CombinedOutput()
			t.Log(string(output))
			for _, id := range []int64{normal, unknown} {
				switch kind {
				case "energy":
					assertWelfareQuizDB(t, id, 1, 1, 0, 0, 0)
				case "balance":
					assertWelfareQuizDB(t, id, 1, 0, 0, 0, 0.2)
				case "coupon":
					assertWelfareQuizDB(t, id, 1, 0, 1, 0, 0)
				case "redeem_code":
					assertWelfareQuizDB(t, id, 1, 0, 0, 1, 0)
				}
			}
			require.NoError(t, runErr, "live Vue tests must pass, not skip")
		})
	}
}

func installWelfareLedgerFailure(t *testing.T, user int64) func() {
	t.Helper()
	name := fmt.Sprintf("welfare_ledger_failure_%d", user)
	_, err := integrationDB.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'isolated welfare ledger failure'; END; $$ LANGUAGE plpgsql;
 CREATE TRIGGER %s BEFORE INSERT ON play_reward_ledger FOR EACH ROW WHEN (NEW.user_id=%d) EXECUTE FUNCTION %s();`, name, name, user, name))
	require.NoError(t, err)
	remove := func() {
		_, err := integrationDB.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON play_reward_ledger; DROP FUNCTION IF EXISTS %s();`, name, name))
		require.NoError(t, err)
	}
	t.Cleanup(remove)
	return remove
}
