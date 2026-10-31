package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func configurePriorityWSTest(s *OpenAIGatewayService, mode string, models ...string) PrioritySchedulingConfig {
	cfg := priorityLatencyAdmissionConfig(mode)
	cfg.Models = models
	s.settingService = &SettingService{settingRepo: &openAIAdvancedSchedulerSettingRepoStub{}, prioritySchedulingRuntimeConfig: cfg, prioritySchedulingRuntimeExpires: time.Now().Add(time.Hour)}
	s.usageLogRepo = &priorityLatencySignalReaderStub{}
	return cfg
}

func seedPriorityWSTest(s *OpenAIGatewayService, cfg PrioritySchedulingConfig, req OpenAIAccountScheduleRequest, id int64, ms float64) {
	s.priorityScheduling.mu.Lock()
	defer s.priorityScheduling.mu.Unlock()
	if s.priorityScheduling.entries == nil {
		s.priorityScheduling.entries = make(map[string]*prioritySignalEntry)
	}
	s.priorityScheduling.entries[prioritySignalCacheKey(req, cfg, []int64{id})] = &prioritySignalEntry{
		accountIDs: []int64{id}, signals: map[int64]PrioritySchedulingSignal{id: {Samples: 3, P90TTFTMs: ms}},
		observedAt: time.Now(), expiresAt: time.Now().Add(time.Hour),
	}
}

func requirePriorityWSDenial(t *testing.T, err error) {
	t.Helper()
	require.ErrorIs(t, err, ErrPriorityLatencyUnavailable)
	require.True(t, IsOpenAITurnAdmissionError(err))
	var closeErr *OpenAIWSClientCloseError
	require.ErrorAs(t, err, &closeErr)
	require.Equal(t, coderws.StatusTryAgainLater, closeErr.StatusCode())
	var upstreamErr *UpstreamFailoverError
	require.NotErrorAs(t, err, &upstreamErr, "local rejection must not trigger upstream failover")
}

func TestPriorityWSAdmissionBlocksFirstWriteInBothNativeModes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{OpenAIWSIngressModePassthrough, OpenAIWSIngressModeCtxPool} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			upstream := newStagedPassthroughConn()
			svc := newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream)
			account := passthroughLifecycleAccount()
			account.Extra["openai_apikey_responses_websockets_v2_mode"] = mode
			pool := newOpenAIWSConnPool(svc.cfg)
			pool.setClientDialerForTest(&stagedPassthroughDialer{conn: upstream})
			svc.openaiWSPool = pool
			defer pool.Close()
			cfg := configurePriorityWSTest(svc, PriorityLatencyStop, "gpt-5.1")
			seedPriorityWSTest(svc, cfg, OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.1"}, account.ID, 2500)
			server, result := startPassthroughLifecycleServer(t, ctx, svc, account)
			defer server.Close()
			client := dialPassthroughLifecycleClient(t, server)
			defer func() { _ = client.CloseNow() }()
			select {
			case err := <-result:
				requirePriorityWSDenial(t, err)
			case <-ctx.Done():
				t.Fatal("latency admission did not stop the first send")
			}
			select {
			case body := <-upstream.writes:
				t.Fatalf("rejected first request reached upstream: %s", body)
			default:
			}
		})
	}
}

func TestPriorityWSAdmissionPassthroughUsesCurrentTurnModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	upstream := newStagedPassthroughConn()
	svc := newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream)
	account := passthroughLifecycleAccount()
	cfg := configurePriorityWSTest(svc, PriorityLatencyStop, "gpt-slow")
	seedPriorityWSTest(svc, cfg, OpenAIAccountScheduleRequest{RequestedModel: "gpt-slow"}, account.ID, 2500)
	initial := OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.1", HistoryModel: "gpt-5.1", Platform: PlatformOpenAI, UseUpstreamTokenCost: true}
	ctx = ContextWithSelectionPriorityRequest(ctx, &AccountSelectionResult{priorityRequest: &initial})
	server, result := startPassthroughLifecycleServer(t, ctx, svc, account)
	defer server.Close()
	client := dialPassthroughLifecycleClient(t, server)
	defer func() { _ = client.CloseNow() }()
	requirePassthroughUpstreamWrite(t, upstream, time.Second)
	upstream.Send(`{"type":"response.completed","response":{"id":"resp_fast","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`)
	_, err := readPassthroughLifecycleFrame(t, client, time.Second)
	require.NoError(t, err)
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-slow"}`)))
	_, err = readPassthroughLifecycleFrame(t, client, time.Second)
	require.Equal(t, coderws.StatusTryAgainLater, coderws.CloseStatus(err))
	select {
	case err = <-result:
		requirePriorityWSDenial(t, err)
	case <-ctx.Done():
		t.Fatal("latency admission did not recheck the new turn model")
	}
	select {
	case body := <-upstream.writes:
		t.Fatalf("slow follow-up reached upstream: %s", body)
	default:
	}
	got, _ := prioritySelectionRequestFromContext(ctx)
	require.Equal(t, initial, got, "per-turn admission must not mutate the connection's selection")
}

func TestPriorityWSAdmissionHTTPBridgeStopsBeforeRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{PriorityLatencyStop, PriorityLatencyDeprioritize} {
		t.Run(mode, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_ok\",\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"))}}
			svc := &OpenAIGatewayService{cfg: passthroughLifecycleConfig(), httpUpstream: upstream}
			account := passthroughLifecycleAccount()
			cfg := configurePriorityWSTest(svc, mode, "gpt-5.1")
			seedPriorityWSTest(svc, cfg, OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.1"}, account.ID, 2500)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
			body := []byte(`{"type":"response.create","model":"gpt-5.1","input":"hi"}`)
			result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "test-token", body, len(body), "gpt-5.1", "", "", "", "", 1, func([]byte) error { return nil })
			if mode == PriorityLatencyStop {
				requirePriorityWSDenial(t, err)
				require.Nil(t, result, "no observed usage may be manufactured by a local rejection")
				require.Empty(t, upstream.requests)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, upstream.requests, 1)
			}
		})
	}
}

type priorityWSRetryUpstream struct {
	*httpUpstreamRecorder
	afterSend func()
}

func (u *priorityWSRetryUpstream) Do(req *http.Request, proxy string, id int64, concurrency int) (*http.Response, error) {
	resp, err := u.httpUpstreamRecorder.Do(req, proxy, id, concurrency)
	u.afterSend()
	return resp, err
}

func TestPriorityWSAdmissionHTTPBridgeRechecksRejectedFieldRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: passthroughLifecycleConfig()}
	account := passthroughLifecycleAccount()
	cfg := configurePriorityWSTest(svc, PriorityLatencyStop, "gpt-5.1")
	req := OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.1"}
	seedPriorityWSTest(svc, cfg, req, account.ID, 100)
	upstream := &priorityWSRetryUpstream{
		httpUpstreamRecorder: &httpUpstreamRecorder{resp: &http.Response{StatusCode: 400, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","code":"invalid_parameter","param":"truncation","message":"Unsupported parameter: truncation"}}`))}},
		afterSend:            func() { seedPriorityWSTest(svc, cfg, req, account.ID, 2500) },
	}
	svc.httpUpstream = upstream
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	body := []byte(`{"type":"response.create","model":"gpt-5.1","input":"hi","truncation":"auto"}`)
	writes := 0
	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "test-token", body, len(body), "gpt-5.1", "", "", "", "", 2, func([]byte) error { writes++; return nil })
	requirePriorityWSDenial(t, err)
	require.Nil(t, result)
	require.Len(t, upstream.requests, 1, "retry must recheck history before sending again")
	require.Zero(t, writes, "local rejection must not become a visible upstream failure event")
}

func TestPriorityWSAdmissionPreservesResolvedScopeWithoutAccountAlias(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := passthroughLifecycleAccount()
	cfg := configurePriorityWSTest(svc, PriorityLatencyStop, "routed-model")
	group := int64(7)
	cfg.GroupIDs = []int64{group}
	svc.settingService.prioritySchedulingRuntimeConfig = cfg
	selected := OpenAIAccountScheduleRequest{GroupID: &group, Platform: PlatformOpenAI, RequestedModel: "routed-model", HistoryModel: "public-alias", InboundEndpoint: "/v1/responses", UseUpstreamTokenCost: false}
	ctx := WithCompositeRouteDecision(context.Background(), CompositeRouteDecision{Matched: true, TargetPlatform: PlatformOpenAI, PublicModel: "public-alias", UpstreamModel: "routed-model"})
	ctx = ContextWithSelectionPriorityRequest(ctx, &AccountSelectionResult{priorityRequest: &selected})
	seedPriorityWSTest(svc, cfg, selected, account.ID, 2500)
	// The first selection was an image turn. This text turn must not inherit
	// its token-cost exemption, nor the account-specific wire model as scope.
	err := svc.admitPriorityWSSend(ctx, nil, account, "public-alias", []byte(`{"model":"account-wire-model","input":"hi"}`))
	requirePriorityWSDenial(t, err)
	got, _ := prioritySelectionRequestFromContext(ctx)
	require.Equal(t, selected, got)
	require.Equal(t, group, *got.GroupID)
}

func TestPriorityWSAdmissionUnknownAndImageIntent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		deny bool
	}{
		{"text unknown", `{"model":"gpt-5.1","input":"hi"}`, true},
		{"native image", `{"model":"gpt-5.1","tools":[{"type":"image_generation"}]}`, false},
		{"passive image namespace", `{"model":"gpt-5.1","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"text2im"}]}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &OpenAIGatewayService{}
			configurePriorityWSTest(svc, PriorityLatencyStop, "gpt-5.1")
			err := svc.admitPriorityWSSend(context.Background(), nil, passthroughLifecycleAccount(), "gpt-5.1", []byte(tc.body))
			if tc.deny {
				requirePriorityWSDenial(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
