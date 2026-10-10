package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func responsesToolsCapabilityAccounts() (Account, Account) {
	chat := Account{ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6.1-sol": "gpt-6.1-sol", "public-alias": "gpt-6.1-sol", "channel-alias": "gpt-6.1-sol"}},
		Extra:       map[string]any{"openai_responses_mode": "force_chat_completions"}}
	responses := chat
	responses.ID = 82
	responses.Priority = 5
	responses.Extra = map[string]any{"openai_responses_mode": "force_responses"}
	return chat, responses
}

func TestResponsesToolsCapabilityFallbackErrorCompletion(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		if streaming {
			c.Header("Content-Type", "text/event-stream")
			_, err := c.Writer.WriteString(": ping\n\n")
			require.NoError(t, err)
			c.Writer.Flush()
		}
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", "requires Responses for tool calls")
		body := rec.Body.String()
		require.True(t, IsResponseCommitted(c))
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", "must not append")
		require.Equal(t, body, rec.Body.String())
		if streaming {
			require.Equal(t, 1, strings.Count(body, "event: response.failed\n"))
			require.True(t, strings.HasPrefix(body, ": ping\n\nevent: response.failed\n"))
			require.Equal(t, http.StatusOK, rec.Code)
		} else {
			require.JSONEq(t, `{"error":{"type":"invalid_request_error","message":"requires Responses for tool calls"}}`, body)
			require.Equal(t, http.StatusBadRequest, rec.Code)
		}
	}
}

func TestResponsesToolsCapabilityRouting(t *testing.T) {
	for _, mode := range []string{"legacy", "legacy_load", "advanced"} {
		for _, alias := range []string{"gpt-6.1-sol", "public-alias", "channel-alias"} {
			for _, body := range []string{
				`{"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}],"input":"hello"}`,
				`{"tools":[{"type":"web_search"}],"input":"hello"}`,
				`{"input":[{"type":"additional_tools","tools":[{"type":"function","name":"lookup"}]}]}`,
				`{"tools":[{"type":"tool_search","execution":"client"}],"input":[{"type":"tool_search_output","call_id":"search_1","status":"completed","execution":"client","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}]}`,
				`{"tools":[{"type":"function","name":"lookup"}],"tool_choice":"none"}`,
			} {
				t.Run(mode+"/"+alias+"/"+body, func(t *testing.T) {
					chat, responses := responsesToolsCapabilityAccounts()
					cfg := &config.Config{}
					cfg.Gateway.Scheduling.LoadBatchEnabled = mode == "legacy_load"
					svc := &OpenAIGatewayService{cfg: cfg,
						accountRepo:        compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{chat, responses}}},
						concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
					if mode == "advanced" {
						svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
					}
					ctx, err := WithOpenAIResponsesToolRequirements(context.Background(), []byte(body))
					require.NoError(t, err)
					requested := alias
					if alias == "channel-alias" {
						// The channel changes a normally Chat-compatible mapping
						// into sol; ignoring the forward context must fail this case.
						mapping, ok := chat.Credentials["model_mapping"].(map[string]any)
						require.True(t, ok)
						mapping["public-alias"] = "gpt-5.4"
						requested = "public-alias"
						ctx = WithOpenAIForwardModel(ctx, alias, false)
					}
					selection, _, err := svc.SelectAccountWithScheduler(ctx, nil, "", "", requested, nil, OpenAIUpstreamTransportAny, false)
					require.NoError(t, err)
					require.NotNil(t, selection)
					if selection.ReleaseFunc != nil {
						selection.ReleaseFunc()
					}
					require.Equal(t, responses.ID, selection.Account.ID)
					// The only compatible account is excluded: fail locally with an
					// actionable capability error before Forward or any upstream I/O.
					selection, _, err = svc.SelectAccountWithScheduler(ctx, nil, "", "", requested, map[int64]struct{}{responses.ID: {}}, OpenAIUpstreamTransportAny, false)
					require.Nil(t, selection)
					require.ErrorIs(t, err, ErrNoAvailableAccounts)
					require.ErrorContains(t, err, "Responses tool")
				})
			}
		}
	}
}

func TestResponsesToolsCapabilitySkipsChatOnlyStickyAccount(t *testing.T) {
	for _, mode := range []string{"legacy", "legacy_load", "advanced"} {
		t.Run(mode, func(t *testing.T) {
			chat, responses := responsesToolsCapabilityAccounts()
			cache := &stubGatewayCache{}
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = mode == "legacy_load"
			svc := &OpenAIGatewayService{cfg: cfg, cache: cache,
				accountRepo:        compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{chat, responses}}},
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
			if mode == "advanced" {
				svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
			}
			ctx, err := WithOpenAIResponsesToolRequirements(context.Background(), []byte(`{"tools":[{"type":"function","name":"lookup"}]}`))
			require.NoError(t, err)
			require.NoError(t, svc.setStickySessionAccountID(ctx, nil, "tools-session", chat.ID, time.Hour))
			selection, decision, err := svc.SelectAccountWithScheduler(ctx, nil, "", "tools-session", "public-alias", nil, OpenAIUpstreamTransportAny, false)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, responses.ID, selection.Account.ID)
			require.False(t, decision.StickySessionHit)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
		})
	}
}

func TestResponsesToolsCapabilityKeepsOrdinaryRequests(t *testing.T) {
	for _, tc := range []struct{ name, body, requested, mapped string }{
		{"no tools", `{"input":"hello"}`, "gpt-6.1-sol", ""},
		{"empty tools", `{"tools":[],"input":"hello"}`, "gpt-6.1-sol", ""},
		{"discovery without tool-search declaration", `{"input":[{"type":"tool_search_output","call_id":"search_1","status":"completed","execution":"client","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]}]}`, "gpt-6.1-sol", ""},
		{"mapped away from sol", `{"tools":[{"type":"function","name":"lookup"}]}`, "gpt-6.1-sol", "gpt-5.4"},
		{"another model", `{"tools":[{"type":"function","name":"lookup"}]}`, "gpt-5.4", ""},
		{"legacy messages without tools", `{"messages":[{"role":"user","content":"hello"}]}`, "gpt-6.1-sol", ""},
		{"native input overrides legacy functions", `{"input":"native","messages":[{"role":"user","content":"legacy"}],"functions":[{"name":"lookup","parameters":{"type":"object"}}]}`, "gpt-6.1-sol", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chat, _ := responsesToolsCapabilityAccounts()
			chat.Credentials = map[string]any{}
			if tc.mapped != "" {
				chat.Credentials["model_mapping"] = map[string]any{tc.requested: tc.mapped}
			}
			ctx, err := WithOpenAIResponsesToolRequirements(context.Background(), []byte(tc.body))
			require.NoError(t, err)
			svc := &OpenAIGatewayService{}
			selected, _, _ := svc.selectBestAccount(ctx, nil, PlatformOpenAI, []Account{chat}, tc.requested, nil, false, "", false)
			require.NotNil(t, selected)
		})
	}
}

func TestResponsesToolsCapabilityLegacyFunctions(t *testing.T) {
	chat, responses := responsesToolsCapabilityAccounts()
	body := []byte(`{"messages":[{"role":"user","content":"hello"}],"functions":[{"name":"lookup","parameters":{"type":"object"}}]}`)
	ctx, err := WithOpenAIResponsesToolRequirements(context.Background(), body)
	require.NoError(t, err)
	svc := &OpenAIGatewayService{}
	selected, _, _ := svc.selectBestAccount(ctx, nil, PlatformOpenAI, []Account{chat, responses}, "public-alias", nil, false, "", false)
	require.NotNil(t, selected)
	require.Equal(t, responses.ID, selected.ID)
	selected, _, stats := svc.selectBestAccount(ctx, nil, PlatformOpenAI, []Account{chat}, "public-alias", nil, false, "", false)
	require.Nil(t, selected)
	require.ErrorIs(t, stats.noAvailableError("public-alias", false, ""), ErrNoAvailableResponsesToolsAccounts)
}

func TestResponsesToolsCapabilityLegacyFunctionsMalformed(t *testing.T) {
	_, err := WithOpenAIResponsesToolRequirements(context.Background(), []byte(`{"messages":[{"role":"user","content":"hello"}],"functions":"invalid"}`))
	require.Error(t, err, "a failed legacy conversion must not cache no-tools")
}

func TestResponsesToolsCapabilityPreservesPreviousBindingAndRechecksDB(t *testing.T) {
	chat, responses := responsesToolsCapabilityAccounts()
	ctx, err := WithOpenAIResponsesToolRequirements(context.Background(), []byte(`{"tools":[{"type":"function","name":"lookup"}]}`))
	require.NoError(t, err)
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	chat.ID = responses.ID
	svc := &OpenAIGatewayService{cfg: &config.Config{}, cache: cache, openaiWSStateStore: store,
		accountRepo:       compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{chat}}},
		schedulerSnapshot: &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{accountsByID: map[int64]*Account{responses.ID: &responses}}}}
	require.NoError(t, store.BindResponseAccount(ctx, 0, "resp_tools", responses.ID, time.Hour))
	id, selected, _, _ := svc.resolveAccountByPreviousResponseIDForCapability(ctx, nil, "resp_tools", "public-alias", nil, "", false)
	require.Zero(t, id)
	require.Nil(t, selected)
	boundID, err := store.GetResponseAccount(ctx, 0, "resp_tools")
	require.NoError(t, err)
	require.Equal(t, responses.ID, boundID, "a per-request capability mismatch must not erase another valid continuation")
}

func TestResponsesToolsCapabilityPreservesNativeValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		tools      bool
	}{
		{"native input extension", `{"input":[{"type":"message","content":[{"type":"input_text","nonce":1e1000000}]}]}`, false},
		{"native tool extension", `{"tools":[{"type":"image_generation","nonce":1e1000000}]}`, true},
		{"malformed top-level tools", `{"tools":{}}`, true},
		{"malformed additional tools", `{"input":[{"type":"additional_tools","tools":{}}]}`, true},
		{"empty additional tools", `{"input":[{"type":"additional_tools","tools":[]}]}`, false},
		{"null tools", `{"tools":null,"input":[{"type":"additional_tools","tools":null}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, err := WithOpenAIResponsesToolRequirements(context.Background(), []byte(tc.body))
			require.NoError(t, err, "routing must preserve the existing native validation boundary")
			chat, responses := responsesToolsCapabilityAccounts()
			svc := &OpenAIGatewayService{}
			require.Equal(t, !tc.tools, svc.openAIResponsesToolsProtocolCompatible(ctx, &chat, "public-alias", false))
			require.True(t, svc.openAIResponsesToolsProtocolCompatible(ctx, &responses, "public-alias", false))
		})
	}
}

func TestResponsesToolsCapabilityAdmissionUsesMappedModel(t *testing.T) {
	chat, _ := responsesToolsCapabilityAccounts()
	svc := &OpenAIGatewayService{accountRepo: compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{chat}}}}
	ctx, err := WithOpenAIResponsesToolRequirements(context.Background(), []byte(`{"tools":[{"type":"function","name":"lookup"}]}`))
	require.NoError(t, err)
	ctx = WithOpenAIForwardModel(ctx, "public-alias", false)
	_, err = svc.AdmitOpenAITurn(ctx, nil, &chat, "public-alias")
	var denied *OpenAITurnAdmissionError
	require.ErrorAs(t, err, &denied)
	require.Equal(t, "responses_tools_protocol_mismatch", denied.Reason)
}
