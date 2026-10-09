package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIWSHTTPBridgeInterruptedTextPreservesObservedUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, observed := range []bool{false, true} {
		name := "without_usage"
		if observed {
			name = "with_usage"
		}
		t.Run(name, func(t *testing.T) {
			body := "data: {\"type\":\"response.output_text.delta\",\"response_id\":\"resp_partial\",\"delta\":\"partial text\"}\n\n"
			if observed {
				body += "data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_partial\",\"usage\":{\"input_tokens\":9,\"output_tokens\":2}}}\n\n"
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)),
			}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
			payload := []byte(`{"type":"response.create","model":"gpt-5","input":"hi"}`)
			result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "test-key", payload, len(payload), "gpt-5", "", "", "", "", 1, func([]byte) error { return nil })
			require.ErrorContains(t, err, "before terminal event")
			require.NotNil(t, result)
			require.Zero(t, result.ImageCount)
			if observed {
				require.Equal(t, 9, result.Usage.InputTokens)
				require.Equal(t, 2, result.Usage.OutputTokens)
			} else {
				require.Equal(t, OpenAIUsage{}, result.Usage, "visible text must not become estimated billable tokens")
			}
		})
	}
}

func TestOpenAIWSHTTPBridgeProgressiveAggregateSurvivesDetailOnlyFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, platform := range []string{PlatformOpenAI, PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			body := "data: " + `{"type":"response.output_text.delta","delta":"partial"}` + "\n\n" + "data: " + `{"type":"response.in_progress","response":{"id":"resp_partial","usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}` + "\n\n" +
				"data: " + `{"type":"response.failed","response":{"id":"resp_partial","status":"failed","error":{"code":"rate_limit_exceeded","message":"rate limit"},"usage":{"input_tokens_details":{"cached_tokens":2,"cache_write_tokens":3}}}}` + "\n\n"
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{ID: 11, Platform: platform, Type: AccountTypeAPIKey, Concurrency: 1}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
			payload := []byte(`{"type":"response.create","model":"gpt-5","input":"hi"}`)
			result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "test-key", payload, len(payload), "gpt-5", "", "", "", "", 1, func([]byte) error { return nil })
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, upstream.requests, 1)
			if platform == PlatformOpenAI {
				require.Equal(t, OpenAIUsage{InputTokens: 9, OutputTokens: 2, CacheReadInputTokens: 4}, result.Usage)
			} else {
				require.Equal(t, OpenAIUsage{CacheReadInputTokens: 2, CacheCreationInputTokens: 3}, result.Usage, "compatible providers retain legacy parser behavior")
			}
		})
	}
}

func TestOpenAIWSHTTPBridgePreOutputMeteredFailureStopsReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, platform := range []string{PlatformOpenAI, PlatformGrok} {
		for _, expense := range []string{"aggregate", "completed_image", "unmetered"} {
			t.Run(platform+"/"+expense, func(t *testing.T) {
				observed := ""
				if expense == "aggregate" {
					observed = `,"usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}`
				}
				if expense == "completed_image" {
					observed = `,"output":[{"type":"image_generation_call","id":"image1","status":"completed","result":"image-data","size":"1024x1024"}]`
				}
				body := "data: " + `{"type":"response.failed","response":{"id":"resp_paid","status":"failed","error":{"code":"rate_limit_exceeded","message":"rate limit"}` + observed + "}}\n\n"
				upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}}
				svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
				account := &Account{ID: 11, Platform: platform, Type: AccountTypeAPIKey, Concurrency: 1}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
				payload := []byte(`{"type":"response.create","model":"gpt-5","input":"hi"}`)
				writes := 0
				result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "test-key", payload, len(payload), "gpt-5", "", "", "", "", 1, func([]byte) error { writes++; return nil })

				if platform == PlatformGrok {
					var legacy *UpstreamFailoverError
					require.True(t, errors.As(err, &legacy))
					require.Equal(t, http.StatusTooManyRequests, legacy.StatusCode)
					require.True(t, legacy.ShouldRetryNextAccount())
					require.Nil(t, result)
					require.Len(t, upstream.requests, 1)
					require.Zero(t, writes)
					return
				}
				var failover *UpstreamFailoverError
				require.True(t, errors.As(err, &failover), "%v", err)
				require.Equal(t, http.StatusTooManyRequests, failover.StatusCode)
				require.Equal(t, "rate limit", gjson.GetBytes(failover.ResponseBody, "error.message").String())
				originalBody := append([]byte(nil), failover.ResponseBody...)
				wrapped := newOpenAIWSCurrentTurnFailoverError(err, []byte(`{"type":"response.create","input":"retry"}`))
				var wrappedPolicy *UpstreamFailoverError
				require.True(t, errors.As(wrapped, &wrappedPolicy))
				require.Same(t, failover, wrappedPolicy)
				require.Equal(t, originalBody, wrappedPolicy.ResponseBody)
				require.Equal(t, http.StatusTooManyRequests, wrappedPolicy.StatusCode)
				require.Len(t, upstream.requests, 1)
				require.Zero(t, writes)
				if expense == "unmetered" {
					require.Nil(t, result)
					require.True(t, failover.ShouldRetryNextAccount())
					return
				}
				require.NotNil(t, result)
				require.True(t, result.HasObservedUsage())
				require.False(t, wrappedPolicy.ShouldRetryNextAccount(), "paid turn cannot be replayed through the later-turn wrapper")
				require.Equal(t, NextAccountStop, wrappedPolicy.NextAccountAction)
				require.False(t, failover.RetryableOnSameAccount)
				require.True(t, failover.SameAccountRetryDeadline.IsZero())
				require.Equal(t, "resp_paid", result.RequestID)
				if expense == "aggregate" {
					require.Equal(t, OpenAIUsage{InputTokens: 9, OutputTokens: 2, CacheReadInputTokens: 4}, result.Usage)
				} else {
					require.Equal(t, 1, result.ImageCount)
				}
			})
		}
	}
}

func TestOpenAIWSHTTPBridgePreemptedObservedTurnSettlesBeforeExit(t *testing.T) {
	for _, tc := range []struct {
		name, platform string
		result         *OpenAIForwardResult
		err            error
		wantCalls      int
	}{
		{name: "openai_metered_error", platform: PlatformOpenAI, result: &OpenAIForwardResult{Usage: OpenAIUsage{InputTokens: 9}}, err: errors.New("upstream failed"), wantCalls: 1},
		{name: "openai_completed_image_error", platform: PlatformOpenAI, result: &OpenAIForwardResult{ImageCount: 1}, err: errors.New("upstream failed"), wantCalls: 1},
		{name: "openai_unmetered_error", platform: PlatformOpenAI, result: &OpenAIForwardResult{}, err: errors.New("upstream failed")},
		{name: "grok_metered_error", platform: PlatformGrok, result: &OpenAIForwardResult{Usage: OpenAIUsage{InputTokens: 9}}, err: errors.New("upstream failed")},
		{name: "openai_success", platform: PlatformOpenAI, result: &OpenAIForwardResult{Usage: OpenAIUsage{InputTokens: 9}}, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			cancel(errOpenAIWSSessionPreempted)
			calls := 0
			hooks := &OpenAIWSIngressHooks{AfterTurn: func(turn int, result *OpenAIForwardResult, turnErr error) {
				calls++
				require.Equal(t, 2, turn)
				require.Same(t, tc.result, result)
				require.Equal(t, tc.err, turnErr)
			}}
			err := finishOpenAIWSHTTPBridgeTurn(ctx, &Account{Platform: tc.platform}, hooks, 2, tc.result, tc.err)
			if tc.err != nil {
				require.ErrorIs(t, err, errOpenAIWSSessionPreempted)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantCalls, calls)
		})
	}
}

func TestOpenAIWSHTTPBridgePostObservationFailuresRetainExpense(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, platform := range []string{PlatformOpenAI, PlatformGrok} {
		for _, observed := range []bool{false, true} {
			for _, failure := range []string{"sentinel", "local_admission", "eof", "done", "staging_overflow", "write"} {
				t.Run(platform+map[bool]string{false: "/unmetered/", true: "/metered/"}[observed]+failure, func(t *testing.T) {
					usage := ""
					if observed {
						usage = `,"usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}`
					}
					body := "data: " + `{"type":"response.in_progress","response":{"id":"resp_progress","model":"gpt-5"` + usage + "}}\n\n"
					if failure == "done" {
						body += "data: [DONE]\n\n"
					}
					if failure == "staging_overflow" {
						body += "data: " + `{"type":"response.in_progress","padding":"` + strings.Repeat("x", openAIFirstOutputStageMaxBytes+1) + `"}` + "\n\n"
					}
					if failure == "write" {
						body += "data: " + `{"type":"response.output_text.delta","delta":"partial"}` + "\n\n"
					}
					sentinel := errors.New("synthetic bridge reader/write failure")
					readErr := sentinel
					if failure == "local_admission" {
						readErr = denyOpenAITurn("changed")
					}
					var reader io.Reader = strings.NewReader(body)
					if failure == "sentinel" || failure == "local_admission" {
						reader = io.MultiReader(reader, iotest.ErrReader(readErr))
					}
					upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(reader)}}
					svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: 2 * openAIFirstOutputStageMaxBytes}}, httpUpstream: upstream}
					account := &Account{ID: 11, Platform: platform, Type: AccountTypeAPIKey, Concurrency: 1}
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
					payload := []byte(`{"type":"response.create","model":"gpt-5","input":"hi"}`)
					result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "test-key", payload, len(payload), "gpt-5", "", "", "", "", 1, func([]byte) error {
						if failure == "write" {
							return sentinel
						}
						return nil
					})
					require.Error(t, err)
					require.Len(t, upstream.requests, 1)
					wantResult := platform == PlatformOpenAI && observed || platform == PlatformGrok && failure != "write"
					if wantResult {
						require.NotNil(t, result)
						if observed {
							require.Equal(t, OpenAIUsage{InputTokens: 9, OutputTokens: 2, CacheReadInputTokens: 4}, result.Usage)
						} else {
							require.Equal(t, OpenAIUsage{}, result.Usage)
						}
					} else {
						require.Nil(t, result)
					}
					if platform == PlatformOpenAI && observed {
						var policy *UpstreamFailoverError
						if errors.As(err, &policy) {
							require.False(t, policy.ShouldRetryNextAccount())
							require.False(t, policy.RetryableOnSameAccount)
							require.True(t, policy.SameAccountRetryDeadline.IsZero())
						}
						if failure == "sentinel" || failure == "local_admission" {
							require.ErrorIs(t, err, readErr)
						}
					}
					if failure == "write" {
						require.ErrorIs(t, err, sentinel)
					}
					if failure == "local_admission" && platform == PlatformOpenAI {
						require.True(t, IsOpenAITurnAdmissionError(err))
						_, hasOps := c.Get(OpsUpstreamErrorsKey)
						require.False(t, hasOps)
					}
					if platform == PlatformOpenAI && !observed && failure != "write" && failure != "local_admission" {
						var policy *UpstreamFailoverError
						require.True(t, errors.As(err, &policy))
						require.True(t, policy.ShouldRetryNextAccount())
					}
				})
			}
		}
	}
}

func TestOpenAIWSHTTPBridgeImageReadErrorPreservesLocalAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	local := denyOpenAITurn("changed")
	body := "data: " + `{"type":"response.output_item.done","item":{"type":"image_generation_call","id":"image1","status":"completed","result":"image-data","size":"1024x1024"}}` + "\n\n"
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(io.MultiReader(strings.NewReader(body), iotest.ErrReader(local)))}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	payload := []byte(`{"type":"response.create","model":"gpt-5","input":"hi"}`)
	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "test-key", payload, len(payload), "gpt-5", "", "", "", "", 1, func([]byte) error { return nil })
	require.ErrorIs(t, err, local)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, OpenAIUsage{}, result.Usage)
}
