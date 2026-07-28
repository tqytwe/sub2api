package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type couponHandlerRewardPoolRepo struct {
	service.CouponRepository
	saved *service.CouponRewardPoolVersion
}

func (r *couponHandlerRewardPoolRepo) SaveCouponRewardPool(_ context.Context, pool service.CouponRewardPoolVersion) (*service.CouponRewardPoolVersion, error) {
	clone := pool
	clone.ID = 77
	r.saved = &clone
	return &clone, nil
}

func TestCouponHandlerRejectsInvalidTemplatePayloadBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewCouponHandler(nil)
	router.POST("/templates", handler.CreateTemplate)

	request := httptest.NewRequest(http.MethodPost, "/templates", strings.NewReader(`{
		"key":"bad-template",
		"name":"Bad template",
		"status":"draft",
		"benefit_type":"fixed_amount",
		"benefit_value":1,
		"currency":"CNY",
		"applicable_scopes":[],
		"validity_mode":"relative_days",
		"validity_days":1
	}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestCouponHandlerCreatesCheckinRewardPoolWithRedeemAndBalanceConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &couponHandlerRewardPoolRepo{}
	router := gin.New()
	handler := NewCouponHandler(service.NewCouponService(repo))
	router.POST("/pools", handler.CreateRewardPool)

	body := `{
		"activity":"checkin",
		"version":"签到奖励8月-v1",
		"coupon_weight_bp":7000,
		"redeem_code_weight_bp":2000,
		"balance_weight_bp":1000,
		"reward_config":{
			"redeem_entries":[
				{"name":"签到奖励0.5元","batch_name":"签到奖励8月-0.5元","code_type":"balance","weight_bp":4000,"enabled":true,"delivery_mode":"issue_code"}
			],
			"balance_entries":[
				{"name":"签到奖励0.5元","amount":0.5,"weight_bp":7000,"enabled":true},
				{"name":"签到奖励1元","amount":1,"weight_bp":2000,"enabled":true},
				{"name":"签到奖励2元","amount":2,"weight_bp":1000,"enabled":true}
			]
		},
		"fallback_template_id":10,
		"entries":[
			{"template_id":10,"weight_bp":1,"enabled":true},
			{"template_id":11,"weight_bp":10000,"enabled":true,"stock_cap":100,"per_user_issue_limit":1}
		]
	}`
	request := httptest.NewRequest(http.MethodPost, "/pools", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	require.NotNil(t, repo.saved)
	require.Equal(t, service.CouponRewardActivityCheckin, repo.saved.Activity)
	require.Equal(t, 7000, repo.saved.CouponWeightBP)
	require.Equal(t, 2000, repo.saved.RedeemCodeWeightBP)
	require.Equal(t, 1000, repo.saved.BalanceWeightBP)
	require.Len(t, repo.saved.RewardConfig.RedeemEntries, 1)
	require.Equal(t, "签到奖励8月-0.5元", repo.saved.RewardConfig.RedeemEntries[0].BatchName)
	require.Len(t, repo.saved.RewardConfig.BalanceEntries, 3)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Contains(t, payload, "data")
}
