//go:build integration

package repository

import (
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

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func welfareBlindboxUser(t *testing.T) int64 {
	t.Helper()
	id := newCheckinExplorer(t, false, 8)
	qualifyBlindboxIntegrationUser(t, &service.User{ID: id})
	return id
}

func welfareBlindboxPool(t *testing.T, server *httptest.Server, user int64) checkinContractResponse {
	t.Helper()
	req, err := http.NewRequest("GET", server.URL+"/api/v1/play/blindbox/pool", nil)
	require.NoError(t, err)
	if user > 0 {
		req.Header.Set("Authorization", "Bearer fixture-only")
		req.Header.Set("X-Welfare-Test-User", strconv.FormatInt(user, 10))
	}
	res, err := server.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, 200, res.StatusCode)
	require.Equal(t, "private, no-store", res.Header.Get("Cache-Control"))
	var out checkinContractResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	return out
}

func assertWelfareBlindboxDB(t *testing.T, user int64, count, coupons, codes int, initial, reward float64) {
	t.Helper()
	var opens, snapshots, energy, issuedCoupons, issuedCodes, ledger, balances, linked int
	var balance, cost, cash, net float64
	require.NoError(t, integrationDB.QueryRow(`SELECT
 (SELECT COUNT(*) FROM play_blindbox_opens WHERE user_id=$1),
 (SELECT COUNT(*) FROM play_growth_eligibility_snapshots WHERE user_id=$1),
 (SELECT COUNT(*) FROM play_growth_energy_ledger WHERE user_id=$1),
 (SELECT COUNT(*) FROM user_coupons WHERE user_id=$1),
 (SELECT COUNT(*) FROM redeem_codes WHERE issued_to=$1),
 (SELECT COUNT(*) FROM play_reward_ledger WHERE user_id=$1),
 (SELECT COUNT(*) FROM balance_transactions WHERE user_id=$1),
 (SELECT COALESCE(SUM(cost_amount),0) FROM play_blindbox_opens WHERE user_id=$1),
 (SELECT COALESCE(SUM(reward_amount),0) FROM play_blindbox_opens WHERE user_id=$1),
 (SELECT COALESCE(SUM(amount),0) FROM play_reward_ledger WHERE user_id=$1),balance FROM users WHERE id=$1`, user).
		Scan(&opens, &snapshots, &energy, &issuedCoupons, &issuedCodes, &ledger, &balances, &cost, &cash, &net, &balance))
	require.Equal(t, count, opens)
	require.Equal(t, count, snapshots)
	require.Zero(t, energy)
	require.Equal(t, coupons, issuedCoupons)
	require.Equal(t, codes, issuedCodes)
	require.Equal(t, count, ledger)
	require.Equal(t, count, balances)
	require.InDelta(t, float64(count)*.5, cost, 1e-8)
	require.InDelta(t, reward, cash, 1e-8)
	require.InDelta(t, reward-cost, net, 1e-8)
	require.InDelta(t, initial+reward-cost, balance, 1e-8)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM play_blindbox_opens o JOIN play_growth_eligibility_snapshots s ON s.id=o.growth_eligibility_snapshot_id
 WHERE o.user_id=$1 AND s.user_id=o.user_id AND s.action_id=o.idempotency_key AND s.activity_date=o.open_date`, user).Scan(&linked))
	require.Equal(t, count, linked)
	t.Logf("blindbox DB: opens=%d snapshots=%d coupons=%d codes=%d cost=%g reward=%g balance=%g", opens, snapshots, issuedCoupons, issuedCodes, cost, cash, balance)
}

func TestBlindboxHTTPDatabaseContract(t *testing.T) {
	_ = testEntClient(t)
	for _, kind := range []string{"balance", "coupon", "redeem_code"} {
		t.Run(kind, func(t *testing.T) {
			seedWelfarePool(t, service.CouponRewardActivityBlindbox, kind)
			user := welfareBlindboxUser(t)
			explorer := newCheckinExplorer(t, false, 8)
			server := newWelfareContractServer(t, user, explorer)
			code, state, err := welfareRequest(server, user, "GET", "/play/blindbox/status", nil, "")
			require.NoError(t, err)
			require.Equal(t, 200, code)
			pool := welfareBlindboxPool(t, server, user)
			for _, r := range []checkinContractResponse{state, pool} {
				for _, branch := range []string{"balance", "coupon", "redeem_code"} {
					expected := 0
					if branch == kind {
						expected = 10000
					}
					if _, ok := r.Data[branch+"_weight_bp"]; !ok {
						t.Errorf("%s pool missing published %s_weight_bp=%d", kind, branch, expected)
						continue
					}
					require.Equal(t, expected, welfareField[int](t, r, branch+"_weight_bp"))
				}
			}
			for _, viewer := range []int64{0, explorer} {
				preview := welfareBlindboxPool(t, server, viewer)
				require.Len(t, preview.Data, 2)
				require.True(t, welfareField[bool](t, preview, "enabled"))
			}
			code, hidden, err := welfareRequest(server, explorer, "GET", "/play/blindbox/status", nil, "")
			require.NoError(t, err)
			require.Equal(t, 200, code)
			for _, field := range []string{"pool", "current_pool", "cost_amount", "expected_reward", "coupon_weight_bp", "redeem_code_weight_bp", "balance_weight_bp"} {
				require.NotContains(t, hidden.Data, field)
			}
			var wg sync.WaitGroup
			errors := make(chan error, 6)
			for range 6 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					status, _, err := welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "same-request")
					if err == nil && status != 200 {
						err = fmt.Errorf("status=%d", status)
					}
					errors <- err
				}()
			}
			wg.Wait()
			close(errors)
			for err := range errors {
				require.NoError(t, err)
			}
			cash, coupons, codes := .0, 0, 0
			switch kind {
			case "balance":
				cash = .2
			case "coupon":
				coupons = 1
			case "redeem_code":
				codes = 1
			}
			assertWelfareBlindboxDB(t, user, 1, coupons, codes, 1, cash)
			code, replay, err := welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "same-request")
			require.NoError(t, err)
			require.Equal(t, 200, code)
			require.Equal(t, kind, welfareField[string](t, replay, "reward_type"))
			if kind == "coupon" {
				c := welfareField[map[string]any](t, replay, "coupon")
				expiry, err := time.Parse(time.RFC3339, c["expires_at"].(string))
				require.NoError(t, err)
				require.True(t, expiry.After(time.Now()))
				require.Equal(t, "隔离福利券", c["name"])
			}
			if kind == "redeem_code" {
				c := welfareField[map[string]any](t, replay, "redeem_code")
				expiry, err := time.Parse(time.RFC3339, c["expires_at"].(string))
				require.NoError(t, err)
				require.True(t, expiry.After(time.Now()))
				require.Contains(t, c["code"], "FIXTURE-")
			}
		})
	}
}

func TestBlindboxBoundaryDatabaseContract(t *testing.T) {
	_ = testEntClient(t)
	t.Run("denied_and_unpublished_no_effects", func(t *testing.T) {
		user := welfareBlindboxUser(t)
		explorer := newCheckinExplorer(t, false, 8)
		server := newWelfareContractServer(t, user, explorer)
		for _, c := range []struct {
			id     int64
			code   int
			reason string
		}{{0, 401, "UNAUTHORIZED"}, {explorer, 403, "PLAY_GROWTH_REWARD_INELIGIBLE"}, {user, 503, "COUPON_REWARD_POOL_UNAVAILABLE"}} {
			code, r, err := welfareRequest(server, c.id, "POST", "/play/blindbox/open", nil, "denied")
			require.NoError(t, err)
			require.Equal(t, c.code, code)
			require.Equal(t, c.reason, r.Reason)
		}
		code, unavailable, err := welfareRequest(server, user, "GET", "/play/blindbox/status", nil, "")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		for _, response := range []checkinContractResponse{unavailable, welfareBlindboxPool(t, server, user)} {
			for _, field := range []string{"coupon_weight_bp", "redeem_code_weight_bp", "balance_weight_bp"} {
				if _, exists := response.Data[field]; exists {
					t.Errorf("unpublished pool discloses fallback probability %s", field)
				}
			}
		}
		assertWelfareBlindboxDB(t, user, 0, 0, 0, 1, 0)
		assertWelfareBlindboxDB(t, explorer, 0, 0, 0, 1, 0)
	})
	t.Run("distinct_requests_respect_daily_limit", func(t *testing.T) {
		seedWelfarePool(t, service.CouponRewardActivityBlindbox, "balance")
		user := welfareBlindboxUser(t)
		server := newWelfareContractServer(t, user)
		_, err := integrationDB.Exec(`UPDATE users SET balance=2 WHERE id=$1`, user)
		require.NoError(t, err)
		results := make(chan int, 6)
		errs := make(chan error, 6)
		var wg sync.WaitGroup
		for i := range 6 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				code, _, err := welfareRequest(server, user, "POST", "/play/blindbox/open", nil, fmt.Sprintf("different-%d", i))
				results <- code
				errs <- err
			}(i)
		}
		wg.Wait()
		close(results)
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		success, denied := 0, 0
		for code := range results {
			if code == 200 {
				success++
			} else {
				require.Equal(t, 409, code)
				denied++
			}
		}
		require.Equal(t, 2, success)
		require.Equal(t, 4, denied)
		assertWelfareBlindboxDB(t, user, 2, 0, 0, 2, .4)
	})
	t.Run("insufficient_balance_no_effects", func(t *testing.T) {
		seedWelfarePool(t, service.CouponRewardActivityBlindbox, "balance")
		user := welfareBlindboxUser(t)
		server := newWelfareContractServer(t, user)
		_, err := integrationDB.Exec(`UPDATE users SET balance=0.49 WHERE id=$1`, user)
		require.NoError(t, err)
		code, r, err := welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "insufficient")
		require.NoError(t, err)
		require.Equal(t, 400, code)
		require.Equal(t, "INSUFFICIENT_BALANCE", r.Reason)
		assertWelfareBlindboxDB(t, user, 0, 0, 0, .49, 0)
	})
	for _, kind := range []string{"balance", "coupon", "redeem_code"} {
		t.Run("rollback_"+kind, func(t *testing.T) {
			seedWelfarePool(t, service.CouponRewardActivityBlindbox, kind)
			user := welfareBlindboxUser(t)
			server := newWelfareContractServer(t, user)
			remove := installWelfareLedgerFailure(t, user)
			code, _, err := welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "rollback")
			require.NoError(t, err)
			require.Equal(t, 500, code)
			assertWelfareBlindboxDB(t, user, 0, 0, 0, 1, 0)
			remove()
			code, _, err = welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "rollback")
			require.NoError(t, err)
			require.Equal(t, 200, code)
			cash, coupons, codes := .0, 0, 0
			switch kind {
			case "balance":
				cash = .2
			case "coupon":
				coupons = 1
			case "redeem_code":
				codes = 1
			}
			assertWelfareBlindboxDB(t, user, 1, coupons, codes, 1, cash)
		})
	}
	t.Run("expired_redeem_inventory", func(t *testing.T) {
		pool := seedWelfarePool(t, service.CouponRewardActivityBlindbox, "redeem_code")
		user := welfareBlindboxUser(t)
		server := newWelfareContractServer(t, user)
		_, err := integrationDB.Exec(`UPDATE redeem_codes SET expires_at=NOW()-INTERVAL '1 second' WHERE batch_name=$1`, pool.Version)
		require.NoError(t, err)
		code, _, err := welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "expired")
		require.NoError(t, err)
		require.Equal(t, 503, code)
		assertWelfareBlindboxDB(t, user, 0, 0, 0, 1, 0)
		_, err = integrationDB.Exec(`UPDATE redeem_codes SET expires_at=NOW()+INTERVAL '1 day' WHERE batch_name=$1`, pool.Version)
		require.NoError(t, err)
		code, _, err = welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "expired")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		assertWelfareBlindboxDB(t, user, 1, 0, 1, 1, 0)
	})
	t.Run("paused_coupon_then_retired_pool_replay", func(t *testing.T) {
		pool := seedWelfarePool(t, service.CouponRewardActivityBlindbox, "coupon")
		user := welfareBlindboxUser(t)
		server := newWelfareContractServer(t, user)
		_, err := integrationDB.Exec(`UPDATE coupon_templates SET status='paused' WHERE id IN (SELECT template_id FROM coupon_reward_pool_entries WHERE pool_version_id=$1)`, pool.ID)
		require.NoError(t, err)
		code, _, err := welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "paused")
		require.NoError(t, err)
		require.Equal(t, 503, code)
		assertWelfareBlindboxDB(t, user, 0, 0, 0, 1, 0)
		_, err = integrationDB.Exec(`UPDATE coupon_templates SET status='active' WHERE id IN (SELECT template_id FROM coupon_reward_pool_entries WHERE pool_version_id=$1)`, pool.ID)
		require.NoError(t, err)
		code, first, err := welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "paused")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		_, err = integrationDB.Exec(`UPDATE coupon_reward_pool_versions SET status='retired' WHERE id=$1`, pool.ID)
		require.NoError(t, err)
		code, replay, err := welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "paused")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		require.Equal(t, first.Data["coupon"], replay.Data["coupon"])
		assertWelfareBlindboxDB(t, user, 1, 1, 0, 1, 0)
	})
	t.Run("same_client_key_is_scoped_to_each_user", func(t *testing.T) {
		seedWelfarePool(t, service.CouponRewardActivityBlindbox, "redeem_code")
		a, b := welfareBlindboxUser(t), welfareBlindboxUser(t)
		server := newWelfareContractServer(t, a, b)
		var codes []string
		for _, id := range []int64{a, b} {
			code, r, err := welfareRequest(server, id, "POST", "/play/blindbox/open", nil, "shared-browser-key")
			require.NoError(t, err)
			require.Equal(t, 200, code)
			codes = append(codes, welfareField[map[string]any](t, r, "redeem_code")["code"].(string))
			assertWelfareBlindboxDB(t, id, 1, 0, 1, 1, 0)
		}
		require.NotEqual(t, codes[0], codes[1])
	})
	t.Run("previous_day_does_not_consume_current_limit", func(t *testing.T) {
		seedWelfarePool(t, service.CouponRewardActivityBlindbox, "balance")
		user := welfareBlindboxUser(t)
		server := newWelfareContractServer(t, user)
		_, err := integrationDB.Exec(`INSERT INTO play_blindbox_opens(user_id,open_date,cost_amount,reward_amount,idempotency_key) VALUES($1,CURRENT_DATE-1,0,0,$2)`, user, fmt.Sprintf("historical-%d", user))
		require.NoError(t, err)
		code, r, err := welfareRequest(server, user, "GET", "/play/blindbox/status", nil, "")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		require.Zero(t, welfareField[int](t, r, "opens_today"))
		code, r, err = welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "today")
		require.NoError(t, err)
		require.Equal(t, 200, code)
		require.Equal(t, 1, welfareField[int](t, r, "opens_today"))
		var today, all, ledger int
		var balance float64
		require.NoError(t, integrationDB.QueryRow(`SELECT (SELECT COUNT(*) FROM play_blindbox_opens WHERE user_id=$1 AND open_date=$2),(SELECT COUNT(*) FROM play_blindbox_opens WHERE user_id=$1),(SELECT COUNT(*) FROM balance_transactions WHERE user_id=$1),balance FROM users WHERE id=$1`, user, welfareField[string](t, r, "server_date")).Scan(&today, &all, &ledger, &balance))
		require.Equal(t, 1, today)
		require.Equal(t, 2, all)
		require.Equal(t, 1, ledger)
		require.InDelta(t, .7, balance, 1e-8)
	})
}

func TestBlindboxVueDatabaseContract(t *testing.T) {
	if os.Getenv("WELFARE_FRONTEND_CONTRACT") != "1" {
		t.Skip("run scripts/test-welfare-contract.sh for actual Vue/PG contract")
	}
	_ = testEntClient(t)
	for _, kind := range []string{"balance", "coupon", "redeem_code"} {
		t.Run(kind, func(t *testing.T) {
			seedWelfarePool(t, service.CouponRewardActivityBlindbox, kind)
			user := welfareBlindboxUser(t)
			server := newWelfareContractServer(t, user)
			// Use the first actual draw. The unknown second draw then exhausts both
			// the daily limit and, for these fixture rewards, the remaining balance.
			code, _, err := welfareRequest(server, user, "POST", "/play/blindbox/open", nil, "first-fixture-open")
			require.NoError(t, err)
			require.Equal(t, 200, code)
			cmd := exec.Command("pnpm", "exec", "vitest", "run", "src/views/public/__tests__/BlindboxView.pg.spec.ts")
			cmd.Dir = "../../../frontend"
			cmd.Env = append(os.Environ(), "WELFARE_CONTRACT_BASE_URL="+server.URL+"/api/v1", "WELFARE_CONTRACT_MODE="+kind, "WELFARE_CONTRACT_USER="+strconv.FormatInt(user, 10))
			output, runErr := cmd.CombinedOutput()
			t.Log(string(output))
			cash, coupons, codes := .0, 0, 0
			switch kind {
			case "balance":
				cash = .4
			case "coupon":
				coupons = 2
			case "redeem_code":
				codes = 2
			}
			assertWelfareBlindboxDB(t, user, 2, coupons, codes, 1, cash)
			require.NoError(t, runErr, "actual Vue tests must pass, not skip")
		})
	}
}
