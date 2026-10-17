//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type turnAdmissionRepo struct {
	AccountRepository
	account, parent *Account
	err             error
	reads           int
	afterRead       func(int, *Account)
}

func (r *turnAdmissionRepo) GetOpenAITurnAdmission(ctx context.Context, id int64) (*Account, *Account, error) {
	r.reads++
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if r.afterRead != nil {
		r.afterRead(r.reads, r.account)
	}
	return r.account, r.parent, r.err
}
func newTurnAdmissionGateway(repo AccountRepository, live bool) *OpenAIGatewayService {
	cfg := rawChatCompletionsTestConfig()
	if live {
		return NewOpenAIGatewayServiceWithLiveBilling(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	}
	return NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}
func turnAdmissionAccount() *Account {
	a := rawChatCompletionsTestAccount()
	a.Status = StatusActive
	a.Schedulable = true
	return a
}
func TestOpenAITurnAdmissionHTTPRejectsBeforeSend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{"disabled initial", "database failure", "disabled final", "route drift final"} {
		t.Run(mode, func(t *testing.T) {
			selected := turnAdmissionAccount()
			latest := *selected
			repo := &turnAdmissionRepo{account: &latest}
			switch mode {
			case "disabled initial":
				latest.Status = "disabled"
			case "database failure":
				repo.err = errors.New("database unavailable")
			case "disabled final":
				repo.afterRead = func(n int, a *Account) {
					if n == 2 {
						copy := *a
						copy.Status = "disabled"
						repo.account = &copy
					}
				}
			case "route drift final":
				repo.afterRead = func(n int, a *Account) {
					if n == 2 {
						copy := *a
						copy.Credentials = map[string]any{"api_key": "sk-test", "base_url": "http://changed.example"}
						repo.account = &copy
					}
				}
			}
			svc := newTurnAdmissionGateway(repo, false)
			upstream := &httpUpstreamRecorder{err: errors.New("must not send")}
			svc.httpUpstream = upstream
			body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false}`)
			result, err := svc.Forward(context.Background(), adaptiveProtocolTestContext("/v1/responses", body), selected, body)
			require.ErrorContains(t, err, "request admission denied")
			require.Nil(t, result)
			require.Nil(t, upstream.lastReq)
			var failover *UpstreamFailoverError
			require.False(t, errors.As(err, &failover), "local admission rejection is not an upstream failure")
		})
	}
}

type turnAdmissionNonReader struct{ AccountRepository }

func TestOpenAITurnAdmissionConstructorsFailClosed(t *testing.T) {
	for _, live := range []bool{false, true} {
		for _, repo := range []AccountRepository{nil, &turnAdmissionNonReader{}} {
			svc := newTurnAdmissionGateway(repo, live)
			require.True(t, svc.requireLatestTurnAdmission)
			upstream := &httpUpstreamRecorder{err: errors.New("must not send")}
			svc.httpUpstream = upstream
			body := []byte(`{"model":"gpt-5.4","input":"hello"}`)
			_, err := svc.Forward(context.Background(), adaptiveProtocolTestContext("/v1/responses", body), turnAdmissionAccount(), body)
			require.True(t, IsOpenAITurnAdmissionError(err))
			require.Nil(t, upstream.lastReq)
		}
	}
	_, err := (&OpenAIGatewayService{}).AdmitOpenAITurn(context.Background(), nil, turnAdmissionAccount(), "gpt-5.4")
	require.True(t, IsOpenAITurnAdmissionError(err), "exported boundary never enables fixture mode")
	// Explicitly document the legacy direct-struct predicate fixture boundary.
	selected := turnAdmissionAccount()
	selected.Schedulable = false
	got, err := (&OpenAIGatewayService{}).admitOpenAITurnForRequest(context.Background(), nil, selected, "gpt-5.4", "gpt-5.4")
	require.NoError(t, err)
	require.Same(t, selected, got)
}

func TestOpenAITurnAdmissionLatestEligibility(t *testing.T) {
	for _, change := range []string{"disabled", "paused", "expired", "id mismatch", "removed group", "missing group", "inactive group", "global model restriction", "route", "proxy", "parent missing", "parent disabled", "runtime block", "model runtime block", "model rate limit", "credentials refresh"} {
		t.Run(change, func(t *testing.T) {
			selected := turnAdmissionAccount()
			selected.GroupIDs = []int64{9}
			selected.Groups = []*Group{{ID: 9, Status: StatusActive, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}}}
			if change == "parent missing" || change == "parent disabled" {
				id := int64(8)
				selected.ParentAccountID = &id
			}
			latest := *selected
			latest.Extra = map[string]any{}
			latest.Credentials = maps.Clone(selected.Credentials)
			repo := &turnAdmissionRepo{account: &latest}
			svc := newTurnAdmissionGateway(repo, false)
			groupID := int64(9)
			c := adaptiveProtocolTestContext("/v1/responses", nil)
			c.Set("api_key", &APIKey{GroupID: &groupID})
			switch change {
			case "disabled":
				latest.Status = "disabled"
			case "paused":
				latest.Schedulable = false
			case "expired":
				past := time.Now().Add(-time.Minute)
				latest.ExpiresAt = &past
				latest.AutoPauseOnExpired = true
			case "id mismatch":
				latest.ID++
			case "removed group":
				latest.GroupIDs = []int64{10}
			case "missing group":
				latest.Groups = nil
			case "inactive group":
				group := *latest.Groups[0]
				group.Status = "disabled"
				latest.Groups = []*Group{&group}
			case "global model restriction":
				group := *latest.Groups[0]
				group.ModelAllowlist.Models = []string{"different"}
				latest.Groups = []*Group{&group}
			case "route":
				latest.Credentials["base_url"] = "http://changed.example"
			case "proxy":
				id := int64(12)
				latest.ProxyID = &id
			case "parent disabled":
				repo.parent = &Account{ID: 8, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: "disabled"}
			case "runtime block":
				svc.openaiAccountRuntimeBlockUntil.Store(latest.ID, time.Now().Add(time.Minute))
			case "model runtime block":
				svc.recordOpenAIAccountModelTransientFailure(&latest, "gpt-5.4", time.Now())
				svc.recordOpenAIAccountModelTransientFailure(&latest, "gpt-5.4", time.Now())
			case "model rate limit":
				latest.Extra[modelRateLimitsKey] = map[string]any{"gpt-5.4": map[string]any{"rate_limit_reset_at": time.Now().Add(time.Minute).Format(time.RFC3339)}}
			case "credentials refresh":
				latest.Credentials["api_key"] = "rotated-synthetic-key"
			}
			got, err := svc.admitOpenAITurnForRequest(context.Background(), c, selected, "public-model", "gpt-5.4")
			if change == "credentials refresh" {
				require.NoError(t, err)
				require.NotSame(t, &latest, got)
				require.Equal(t, latest.ID, got.ID)
				require.Equal(t, latest.Credentials, got.Credentials)
				require.True(t, got.openAITurnCredentialsAdmitted)
				require.False(t, latest.openAITurnCredentialsAdmitted)
			} else {
				require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
				require.Nil(t, got)
			}
		})
	}
}

func TestOpenAITurnAdmissionPreservesPricingAndSimpleMode(t *testing.T) {
	selected := turnAdmissionAccount()
	selected.GroupIDs = []int64{9}
	currentGroup := &Group{ID: 9, Status: StatusActive, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}}
	selected.Groups = []*Group{currentGroup}
	latest := *selected
	repo := &turnAdmissionRepo{account: &latest}
	svc := newTurnAdmissionGateway(repo, false)
	parentBilling := &Group{ID: 20, Hydrated: true, Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 2}
	ctx, pricingAt := WithGatewayTokenRequestPricing(context.WithValue(context.Background(), ctxkey.Group, parentBilling))
	groupID := int64(9)
	authGroup := &Group{ID: 9, RateMultiplier: 3}
	key := &APIKey{GroupID: &groupID, Group: authGroup}
	c := adaptiveProtocolTestContext("/v1/responses", nil)
	c.Request = c.Request.WithContext(ctx)
	c.Set("api_key", key)
	got, err := svc.admitOpenAITurnForRequest(ctx, c, selected, "public-model", "mapped-upstream-model")
	require.NoError(t, err)
	require.NotSame(t, &latest, got)
	require.Equal(t, latest.ID, got.ID)
	require.Equal(t, latest.Credentials, got.Credentials)
	require.True(t, got.openAITurnCredentialsAdmitted)
	require.False(t, latest.openAITurnCredentialsAdmitted)
	require.Same(t, key, getAPIKeyFromContext(c))
	require.Same(t, authGroup, key.Group)
	require.Same(t, parentBilling, gatewayTokenRequestBillingGroupFromContext(c.Request.Context()))
	require.Equal(t, pricingAt, GatewayTokenRequestPricingAtFromContext(c.Request.Context()))
	svc.cfg.RunMode = config.RunModeSimple
	latest.GroupIDs = []int64{10}
	latest.Groups = nil
	_, err = svc.admitOpenAITurnForRequest(ctx, c, selected, "public-model", "mapped-upstream-model")
	require.NoError(t, err, "simple mode intentionally schedules across groups")
}

func TestOpenAITurnAdmissionCancellationAndDeadline(t *testing.T) {
	for _, expired := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if expired {
			cancel()
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		} else {
			cancel()
		}
		defer cancel()
		selected := turnAdmissionAccount()
		repo := &turnAdmissionRepo{account: selected}
		svc := newTurnAdmissionGateway(repo, false)
		upstream := &httpUpstreamRecorder{}
		svc.httpUpstream = upstream
		body := []byte(`{"model":"gpt-5.4","input":"hello"}`)
		_, err := svc.Forward(ctx, adaptiveProtocolTestContext("/v1/responses", body), selected, body)
		require.ErrorIs(t, err, ctx.Err())
		require.Zero(t, repo.reads)
		require.Nil(t, upstream.lastReq)
	}
}

func TestOpenAITurnAdmissionHTTPPreservesPublicModelScope(t *testing.T) {
	selected := turnAdmissionAccount()
	selected.GroupIDs = []int64{9}
	selected.Groups = []*Group{{ID: 9, Status: StatusActive, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}}}
	repo := &turnAdmissionRepo{account: selected}
	svc := newTurnAdmissionGateway(repo, false)
	upstream := &httpUpstreamRecorder{err: errors.New("stop after capture")}
	svc.httpUpstream = upstream
	// Channel/composite routing rewrites the body before Forward, while the
	// immutable request context retains the original client model.
	body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false}`)
	ctx := context.WithValue(context.Background(), ctxkey.Model, "public-model")
	c := adaptiveProtocolTestContext("/v1/responses", body)
	c.Request = c.Request.WithContext(ctx)
	groupID := int64(9)
	c.Set("api_key", &APIKey{GroupID: &groupID})
	_, err := svc.Forward(ctx, c, selected, body)
	require.False(t, IsOpenAITurnAdmissionError(err), "public alias must not be compared with the mapped upstream model")
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, 2, repo.reads, "both initial and final-send checks retain the public model")
}

func TestOpenAITurnAdmissionHTTPPreservesForwardingAndUsage(t *testing.T) {
	selected := turnAdmissionAccount()
	repo := &turnAdmissionRepo{account: selected}
	svc := newTurnAdmissionGateway(repo, false)
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"resp_admitted","object":"response","status":"completed","model":"gpt-5.4","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`))}}
	svc.httpUpstream = upstream
	body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false,"reasoning":{"effort":"high"},"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}`)
	original := string(body)
	result, err := svc.Forward(context.Background(), adaptiveProtocolTestContext("/v1/responses", body), selected, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, repo.reads)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "lookup", gjson.GetBytes(upstream.lastBody, "tools.0.name").String())
	require.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.False(t, result.Stream)
	require.Equal(t, original, string(body))
}

func TestOpenAITurnAdmissionShadowDoesNotInheritParentQuota(t *testing.T) {
	selected := turnAdmissionAccount()
	selected.Type = AccountTypeOAuth
	selected.Credentials = map[string]any{"access_token": "synthetic"}
	id := int64(8)
	selected.ParentAccountID = &id
	until := time.Now().Add(time.Hour)
	parent := &Account{ID: 8, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: false, RateLimitResetAt: &until, OverloadUntil: &until}
	repo := &turnAdmissionRepo{account: selected, parent: parent}
	svc := newTurnAdmissionGateway(repo, false)
	_, err := svc.AdmitOpenAITurn(context.Background(), nil, selected, "gpt-5.4")
	require.NoError(t, err, "a shadow has independent quota; only parent credential/transport eligibility applies")
	parent.TempUnschedulableUntil = &until
	_, err = svc.AdmitOpenAITurn(context.Background(), nil, selected, "gpt-5.4")
	require.True(t, IsOpenAITurnAdmissionError(err), "shared parent credential cooldown still blocks the shadow")
}
func TestOpenAITurnAdmissionUsesCompositePublicModel(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxkey.Model, "channel-mapped")
	ctx = context.WithValue(ctx, ctxkey.RequestedPublicModel, "public-alias")
	require.Equal(t, "public-alias", openAITurnRequestModel(ctx, "outbound-model"))
}
