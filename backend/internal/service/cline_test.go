//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func clineTestAccount() *Account {
	return &Account{ID: 12, Platform: "cline", Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "cline-test"}}
}

func TestClineResponsesUseChatWithVendorModelPrefix(t *testing.T) {
	body := []byte(`{"model":"anthropic/claude-sonnet-4-6","input":"hello"}`)
	c, _ := newTestContext()
	upstream := &httpUpstreamRecorder{err: errors.New("stop after capture")}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	_, _ = svc.Forward(c.Request.Context(), c, clineTestAccount(), body)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.cline.bot/api/v1/chat/completions", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer cline-test", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "anthropic/claude-sonnet-4-6", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
}

func TestClineChatAndMessagesUseChatEndpoint(t *testing.T) {
	for _, inbound := range []struct {
		name string
		body string
	}{
		{"chat", `{"model":"anthropic/claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}]}`},
		{"messages", `{"model":"anthropic/claude-sonnet-4-6","max_tokens":64,"messages":[{"role":"user","content":"hello"}]}`},
	} {
		t.Run(inbound.name, func(t *testing.T) {
			c, _ := newTestContext()
			upstream := &httpUpstreamRecorder{err: errors.New("stop after capture")}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			if inbound.name == "chat" {
				_, _ = svc.ForwardAsChatCompletions(c.Request.Context(), c, clineTestAccount(), []byte(inbound.body), "", "")
			} else {
				_, _ = svc.ForwardAsAnthropic(c.Request.Context(), c, clineTestAccount(), []byte(inbound.body), "", "")
			}
			require.Len(t, upstream.requests, 1)
			require.Equal(t, "https://api.cline.bot/api/v1/chat/completions", upstream.requests[0].URL.String())
			require.Equal(t, "Bearer cline-test", upstream.requests[0].Header.Get("Authorization"))
			require.Equal(t, "anthropic/claude-sonnet-4-6", gjson.GetBytes(upstream.lastBody, "model").String())
			require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
		})
	}
}

func TestClineAdminDiscoversModelIDsFromOfficialEndpoint(t *testing.T) {
	svc := &AccountTestService{cfg: rawChatCompletionsTestConfig()}
	req, err := svc.buildUpstreamModelsRequest(context.Background(), clineTestAccount())
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, req.Method)
	require.Equal(t, "https://api.cline.bot/api/v1/models", req.URL.String())
	require.Equal(t, "Bearer cline-test", req.Header.Get("Authorization"))
}

func TestClineSelectionAndConnectionProbe(t *testing.T) {
	account := clineTestAccount()
	require.True(t, account.IsOpenAICompatible())
	require.Equal(t, "cline", NormalizeOpenAICompatiblePlatform(account.Platform))
	account.Extra = map[string]any{"cline_5h_used_percent": float64(0)}
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNChatTestResponse())
	c, recorder := newTestContext()
	require.NoError(t, svc.TestAccountConnection(c, account.ID, "", "hi", AccountTestModeDefault))
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.cline.bot/api/v1/chat/completions", upstream.requests[0].URL.String())
	require.Equal(t, "cline-pass/glm-5.3-flash", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
}

func TestClineQuotaProbePreservesOtherWalletsAndScopes(t *testing.T) {
	ctx := context.Background()
	account := clineTestAccount()
	now := time.Now().UTC()
	reset := now.Add(4 * time.Hour).Truncate(time.Second)
	repo := &clineProbeRepo{}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		clineJSON(http.StatusOK, fmt.Sprintf(`{"success":true,"data":{"limits":[{"type":"five_hour","percentUsed":100,"resetsAt":%q},{"type":"weekly","percentUsed":40},{"type":"monthly","percentUsed":50}]}}`, reset.Format(time.RFC3339))),
		clineJSON(http.StatusOK, `{"success":true,"data":{"id":"user_123"}}`),
		clineJSON(http.StatusOK, `{"success":true,"data":{"balance":12345678}}`),
	}}
	svc := &CNProviderQuotaService{accountRepo: repo, httpUpstream: upstream}
	result, err := svc.QueryUsageForAccount(ctx, account)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.True(t, result.Persisted)
	require.Equal(t, "ClinePass", result.PlanLevel)
	require.Len(t, result.Tiers, 3)
	require.Equal(t, 12.3457, result.Balance.Balance)
	require.Equal(t, reset, repo.limits[clinePassRateLimitKey])
	require.Equal(t, float64(50), repo.extra["cline_monthly_used_percent"])
	require.Equal(t, 12.3457, repo.extra["cline_balance"])
	require.Len(t, upstream.requests, 3)
	require.Equal(t, "/api/v1/users/user_123/balance", upstream.requests[2].URL.Path)
	for _, req := range upstream.requests {
		require.Equal(t, "api.cline.bot", req.URL.Host)
		require.Equal(t, "Bearer cline-test", req.Header.Get("Authorization"))
	}
	require.Positive(t, account.GetModelRateLimitRemainingTime("cline-pass/glm-5.3-flash"))
	require.Zero(t, account.GetModelRateLimitRemainingTime("anthropic/claude-sonnet-4-6"))
	require.Zero(t, account.GetModelRateLimitRemainingTime("cline-free/example"))
}

func TestClineRecognizedRequestLimitsKeepOtherModelsAvailable(t *testing.T) {
	account := clineTestAccount()
	repo := &clineProbeRepo{}
	svc := &RateLimitService{accountRepo: repo}
	require.False(t, svc.HandleUpstreamError(context.Background(), account, http.StatusPaymentRequired, nil,
		[]byte(`{"error":{"code":"insufficient_credits","message":"Insufficient credits"}}`), "anthropic/claude-sonnet-4-6"))
	require.Positive(t, account.GetModelRateLimitRemainingTime("deepseek/deepseek-v4-flash"))
	require.Zero(t, account.GetModelRateLimitRemainingTime("cline-pass/glm-5.3-flash"))
	require.False(t, svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, nil,
		[]byte(`{"error":{"message":"Free limit reached on model cline-free/example. Try again in 3h 20m"}}`), "cline-free/example"))
	require.Positive(t, account.GetModelRateLimitRemainingTime("cline-free/example"))
	require.Zero(t, account.GetModelRateLimitRemainingTime("cline-free/other"))
}

func TestClineRecoveredBalanceAndPassReleaseOnlyTheirOwnCooldowns(t *testing.T) {
	now := time.Now().UTC()
	account := clineTestAccount()
	for _, limit := range []struct{ key, reason string }{
		{clineCreditsRateLimitKey, clineCreditsReason},
		{clinePassRateLimitKey, clinePassLimitReason},
		{"cline-free/example", clineFreeModelLimitReason},
	} {
		setAccountModelRateLimitSnapshot(account, limit.key, now.Add(time.Hour), limit.reason, now)
	}
	svc := &CNProviderQuotaService{accountRepo: &clineProbeRepo{}}
	svc.applyClineWallets(context.Background(), account, true, []CNQuotaTier{{Window: "5h", UsedPercent: 30}}, clineBalanceResult(10, now), now)
	require.Zero(t, account.GetModelRateLimitRemainingTime("deepseek/deepseek-v4-flash"))
	require.Zero(t, account.GetModelRateLimitRemainingTime("cline-pass/glm-5.3-flash"))
	require.Positive(t, account.GetModelRateLimitRemainingTime("cline-free/example"))
}

func TestCline402PersistenceFailureDoesNotDisableAccount(t *testing.T) {
	account := clineTestAccount()
	svc := &RateLimitService{accountRepo: &clineProbeRepo{limitErr: errors.New("database unavailable")}}
	body := []byte(`{"error":{"code":"insufficient_credits"}}`)
	require.False(t, svc.HandleUpstreamError(context.Background(), account, http.StatusPaymentRequired, nil, body, "deepseek/deepseek-v4-flash"))
	require.Zero(t, account.GetModelRateLimitRemainingTime("cline-pass/glm-5.3-flash"))
}

func TestClineRelayCannotUseOfficialAccountProbes(t *testing.T) {
	account := clineTestAccount()
	account.Credentials["base_url"] = "https://custom-relay.example/v1"
	upstream := &httpUpstreamRecorder{}
	repo := &clineProbeRepo{}
	usage := &CNProviderQuotaService{accountRepo: repo, httpUpstream: upstream}
	balance := &CNProviderBalanceService{accountRepo: repo, httpUpstream: upstream}
	_, usageErr := usage.QueryUsageForAccount(context.Background(), account)
	_, balanceErr := balance.QueryBalanceForAccount(context.Background(), account)
	require.Error(t, usageErr)
	require.Error(t, balanceErr)
	require.Empty(t, upstream.requests)
}

type clineProbeRepo struct {
	AccountRepository
	extra    map[string]any
	limits   map[string]time.Time
	limitErr error
}

func (r *clineProbeRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.extra = updates
	return nil
}

func (r *clineProbeRepo) SetModelRateLimit(_ context.Context, _ int64, scope string, until time.Time, _ ...string) error {
	if r.limitErr != nil {
		return r.limitErr
	}
	if r.limits == nil {
		r.limits = make(map[string]time.Time)
	}
	r.limits[scope] = until
	return nil
}

func clineJSON(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}
