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

	"github.com/stretchr/testify/require"
)

func TestOpenAIPassthroughRetainsMeteredFailureWithoutRetry(t *testing.T) {
	testOpenAIResponsesRetainsMeteredFailureWithoutRetry(t, true)
}

func TestOpenAINativeResponsesRetainsMeteredFailureWithoutRetry(t *testing.T) {
	testOpenAIResponsesRetainsMeteredFailureWithoutRetry(t, false)
}

func testOpenAIResponsesRetainsMeteredFailureWithoutRetry(t *testing.T, passthrough bool) {
	for _, stream := range []bool{true, false} {
		for _, failure := range []string{"failover", "compact", "interrupted"} {
			if !stream && failure == "interrupted" {
				continue
			}
			t.Run(fmt.Sprintf("stream=%v/%s", stream, failure), func(t *testing.T) {
				selected := passthroughAdmissionAccount()
				selected.Extra["openai_passthrough"] = passthrough
				repo := &turnAdmissionRepo{account: selected}
				svc := newTurnAdmissionGateway(repo, false)
				svc.cfg.Gateway.OpenAICompactModel = "gpt-5.4"
				code, message := "rate_limit_exceeded", "rate limit exceeded"
				if failure == "compact" {
					code, message = "context_length_exceeded", "context window exceeded"
				}
				payload := fmt.Sprintf(`{"type":"response.failed","response":{"id":"resp_partial","status":"failed","model":"gpt-5.5","service_tier":"default","error":{"code":%q,"message":%q},"usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}`, code, message)
				if failure == "interrupted" {
					payload = `{"type":"response.in_progress","response":{"id":"resp_partial","model":"gpt-5.5","service_tier":"default","usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}`
				}
				failedBody := &passthroughCloseTrackingReadCloser{Reader: strings.NewReader("data: " + payload + "\n\n")}
				upstream := &httpUpstreamRecorder{responses: []*http.Response{
					{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"request_partial"}}, Body: failedBody},
					passthroughAdmissionResponse(),
				}}
				svc.httpUpstream = upstream
				body := []byte(fmt.Sprintf(`{"model":"gpt-5.5","input":"hello","stream":%v}`, stream))
				if failure == "compact" {
					body = []byte(fmt.Sprintf(`{"model":"gpt-5.5","input":[{"type":"message","role":"user","content":"hello"},{"type":"compaction_trigger"}],"stream":%v}`, stream))
				}
				ctx, pricingAt := WithGatewayTokenRequestPricing(context.Background())
				c := adaptiveProtocolTestContext("/v1/responses", body)
				c.Request = c.Request.WithContext(ctx)
				if failure == "compact" {
					MarkOpenAINativeCompactionV2(c)
				}
				result, err := svc.Forward(ctx, c, selected, body)
				require.Error(t, err)
				require.NotNil(t, result, "upstream-observed usage must survive an unsuccessful attempt")
				require.Len(t, upstream.requests, 1, "metered work must not be replayed by compact fallback")
				require.True(t, failedBody.closed)
				require.Equal(t, 9, result.Usage.InputTokens)
				require.Equal(t, 2, result.Usage.OutputTokens)
				require.Equal(t, 4, result.Usage.CacheReadInputTokens)
				require.Equal(t, "request_partial", result.RequestID)
				require.Equal(t, "resp_partial", result.ResponseID)
				require.Equal(t, "gpt-5.5", result.Model)
				require.Equal(t, "gpt-5.5", result.UpstreamResponseModel)
				if failure == "interrupted" {
					require.Empty(t, result.UpstreamResponseServiceTier, "nonterminal tier is not authoritative billing evidence")
				} else {
					require.Equal(t, "default", result.UpstreamResponseServiceTier)
				}
				require.Equal(t, pricingAt, GatewayTokenRequestPricingAtFromContext(c.Request.Context()))
			})
		}
	}
}

func TestOpenAIPassthroughUnmeteredFailureDoesNotCreateUsage(t *testing.T) {
	selected := passthroughAdmissionAccount()
	svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: selected}, false)
	svc.httpUpstream = &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader("data: " + `{"type":"response.failed","response":{"status":"failed","error":{"code":"rate_limit_exceeded","message":"rate limit exceeded"}}}` + "\n\n")),
	}}
	body := []byte(`{"model":"gpt-5.5","input":"hello","stream":true}`)
	result, err := svc.Forward(context.Background(), adaptiveProtocolTestContext("/v1/responses", body), selected, body)
	require.Error(t, err)
	require.Nil(t, result, "no charge may be inferred from an unmetered failure")
}

func TestOpenAIForwardObservedUsageExcludesUnmeteredDetails(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result *OpenAIForwardResult
		want   bool
	}{
		{name: "nil"},
		{name: "zero", result: &OpenAIForwardResult{}},
		{name: "detail_subsets", result: &OpenAIForwardResult{Usage: OpenAIUsage{InputAudioTokens: 2, OutputAudioTokens: 3, ImageInputTokens: 4, ImageOutputTokens: 5}}},
		{name: "input", result: &OpenAIForwardResult{Usage: OpenAIUsage{InputTokens: 9}}, want: true},
		{name: "output", result: &OpenAIForwardResult{Usage: OpenAIUsage{OutputTokens: 2}}, want: true},
		{name: "cache_read_details_without_aggregate", result: &OpenAIForwardResult{Usage: OpenAIUsage{CacheReadInputTokens: 4}}},
		{name: "cache_write_details_without_aggregate", result: &OpenAIForwardResult{Usage: OpenAIUsage{CacheCreationInputTokens: 4}}},
		{name: "image", result: &OpenAIForwardResult{ImageCount: 1}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, tc.result.HasObservedUsage()) })
	}
}

func TestOpenAIResponsesTerminalImageExpenseStopsReplay(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, stream := range []bool{true, false} {
			for _, failure := range []string{"rate_limit", "compact"} {
				for _, imageExpense := range []bool{true, false} {
					t.Run(fmt.Sprintf("passthrough=%v/stream=%v/%s/image=%v", passthrough, stream, failure, imageExpense), func(t *testing.T) {
						selected := passthroughAdmissionAccount()
						selected.Extra["openai_passthrough"] = passthrough
						svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: selected}, false)
						svc.cfg.Gateway.OpenAICompactModel = "gpt-5.4"
						code, message := "rate_limit_exceeded", "rate limit exceeded"
						if failure == "compact" {
							code, message = "context_length_exceeded", "context window exceeded"
						}
						evidence := `"output":[{"id":"image_complete","type":"image_generation_call","status":"completed","result":"synthetic-final-image","size":"1024x1024"}]`
						if !imageExpense {
							evidence = `"usage":{"input_tokens_details":{"cached_tokens":4,"cache_write_tokens":3}}`
						}
						payload := fmt.Sprintf(`{"type":"response.failed","response":{"id":"resp_image_partial","model":"gpt-5.5","status":"failed","error":{"code":%q,"message":%q},%s}}`, code, message, evidence)
						failedBody := &passthroughCloseTrackingReadCloser{Reader: strings.NewReader("data: " + payload + "\n\n")}
						successResponse := passthroughAdmissionResponse()
						if stream {
							successResponse.Header.Set("Content-Type", "text/event-stream")
							successResponse.Body = io.NopCloser(strings.NewReader("data: " + `{"type":"response.completed","response":{"id":"resp_admitted","model":"gpt-5.4","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":2}}}` + "\n\n"))
						}
						upstream := &httpUpstreamRecorder{responses: []*http.Response{
							{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"request_image_partial"}}, Body: failedBody},
							successResponse,
						}}
						svc.httpUpstream = upstream
						body := []byte(fmt.Sprintf(`{"model":"gpt-5.5","input":"hello","stream":%v}`, stream))
						if failure == "compact" {
							// A native marker alone does not authorize retry;
							// the wire request must carry the explicit trigger.
							body = []byte(fmt.Sprintf(`{"model":"gpt-5.5","input":[{"type":"message","role":"user","content":"hello"},{"type":"compaction_trigger"}],"stream":%v}`, stream))
						}
						c := adaptiveProtocolTestContext("/v1/responses", body)
						if failure == "compact" {
							MarkOpenAINativeCompactionV2(c)
						}
						result, err := svc.Forward(context.Background(), c, selected, body)
						if !imageExpense {
							if failure == "compact" {
								require.Len(t, upstream.requests, 2, "detail-only usage must not take expense ownership from the ordinary compact retry")
								require.NoError(t, err)
								require.NotNil(t, result)
								require.Equal(t, OpenAIUsage{InputTokens: 3, OutputTokens: 2}, result.Usage, "only the second attempt's actual aggregate owns expense")
								require.True(t, failedBody.closed)
							} else {
								require.Error(t, err)
								require.Nil(t, result, "nested cache details without aggregate usage must not create settlement")
							}
							return
						}
						require.Error(t, err)
						require.NotNil(t, result, "a completed image is observed expense even without token usage")
						require.Len(t, upstream.requests, 1, "image expense must stop compact and account replay")
						require.True(t, failedBody.closed)
						require.Equal(t, 1, result.ImageCount)
						require.Equal(t, []string{"1024x1024"}, result.ImageOutputSizes)
						require.Equal(t, OpenAIUsage{}, result.Usage, "image capture must not estimate token usage")
						if failure == "compact" {
							_, ok := asOpenAICompactFallbackSignal(err)
							require.True(t, ok, "retain original compact signal without replay")
						} else {
							var failoverErr *UpstreamFailoverError
							require.ErrorAs(t, err, &failoverErr)
							require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
						}
					})
				}
			}
		}
	}
}

func TestOpenAIResponsesProgressiveAggregateSurvivesDetailOnlyTerminal(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("passthrough=%v/stream=%v", passthrough, stream), func(t *testing.T) {
				selected := passthroughAdmissionAccount()
				selected.Extra["openai_passthrough"] = passthrough
				svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: selected}, false)
				payload := `data: {"type":"response.in_progress","response":{"id":"resp_progress","usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}` + "\n\n" +
					`data: {"type":"response.failed","response":{"id":"resp_progress","status":"failed","error":{"code":"rate_limit_exceeded","message":"rate limit exceeded"},"usage":{"input_tokens_details":{"cached_tokens":2,"cache_write_tokens":3,"image_tokens":5}}}}` + "\n\n"
				upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(payload))}}
				svc.httpUpstream = upstream
				body := []byte(fmt.Sprintf(`{"model":"gpt-5.5","input":"hello","stream":%v}`, stream))
				result, err := svc.Forward(context.Background(), adaptiveProtocolTestContext("/v1/responses", body), selected, body)
				require.Error(t, err)
				require.NotNil(t, result)
				require.Equal(t, OpenAIUsage{InputTokens: 9, OutputTokens: 2, CacheReadInputTokens: 4}, result.Usage, "terminal detail subsets cannot replace an observed aggregate snapshot")
				require.Len(t, upstream.requests, 1)
			})
		}
	}
}

type observedUsageSentinelReader struct{ err error }

func (r observedUsageSentinelReader) Read([]byte) (int, error) { return 0, r.err }

func TestOpenAIResponsesBufferedReadErrorRetainsCompleteMeteredEvents(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, evidence := range []string{"complete_metered_frame", "complete_json", "truncated_json", "truncated_json_document", "delta_only"} {
			for _, local := range []bool{false, true} {
				t.Run(fmt.Sprintf("passthrough=%v/%s/local=%v", passthrough, evidence, local), func(t *testing.T) {
					selected := passthroughAdmissionAccount()
					selected.Extra["openai_passthrough"] = passthrough
					svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: selected}, false)
					payload := `data: {"type":"response.in_progress","response":{"id":"resp_progress","model":"gpt-5.5","usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}` + "\n\n"
					contentType := "text/event-stream"
					if evidence == "complete_json" {
						contentType = "application/json"
						payload = `{"id":"resp_progress","object":"response","status":"completed","model":"gpt-5.5","usage":{"input_tokens":9,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}`
					}
					if evidence == "truncated_json_document" {
						contentType = "application/json"
						payload = `{"id":"resp_progress","object":"response","usage":{"input_tokens":9`
					}
					if evidence == "truncated_json" {
						payload = `data: {"type":"response.in_progress","response":{"usage":{"input_tokens":9`
					}
					if evidence == "delta_only" {
						payload = `data: {"type":"response.output_text.delta","delta":"visible text"}` + "\n\n"
					}
					var wantErr error = errors.New("synthetic buffered read interruption")
					if local {
						wantErr = &OpenAITurnAdmissionError{Reason: "account_binding_changed"}
					}
					failedBody := &passthroughCloseTrackingReadCloser{Reader: io.MultiReader(strings.NewReader(payload), observedUsageSentinelReader{err: wantErr})}
					upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{contentType}, "X-Request-Id": []string{"request_buffered_partial"}}, Body: failedBody}}
					svc.httpUpstream = upstream
					body := []byte(`{"model":"gpt-5.5","input":"hello","stream":false}`)
					result, err := svc.Forward(context.Background(), adaptiveProtocolTestContext("/v1/responses", body), selected, body)
					require.ErrorIs(t, err, wantErr, "retain the original read or local-admission error")
					require.Len(t, upstream.requests, 1)
					require.True(t, failedBody.closed)
					if evidence != "complete_metered_frame" && evidence != "complete_json" {
						require.Nil(t, result, "incomplete JSON and output deltas do not authorize estimated usage")
						return
					}
					require.NotNil(t, result)
					require.Equal(t, OpenAIUsage{InputTokens: 9, OutputTokens: 2, CacheReadInputTokens: 4}, result.Usage)
					require.Equal(t, "request_buffered_partial", result.RequestID)
					require.Equal(t, "resp_progress", result.ResponseID)
					require.Equal(t, "gpt-5.5", result.UpstreamResponseModel)
				})
			}
		}
	}
}
