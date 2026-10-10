//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCompactAdmissionRoutingGlobalChannelRestriction(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	for _, allowed := range []string{"gpt-5.4", "gpt-5.5"} {
		channelSvc := newTestChannelService(makeStandardRepo(Channel{
			ID: 1, Status: StatusActive, GroupIDs: []int64{10}, RestrictModels: true, BillingModelSource: BillingModelSourceUpstream,
			ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{allowed}}},
		}, map[int64]string{10: PlatformOpenAI}))
		svc := &OpenAIGatewayService{channelService: channelSvc, cfg: &config.Config{Gateway: config.GatewayConfig{OpenAICompactModel: "gpt-5.5"}}}
		ctx := WithOpenAIForwardModel(context.Background(), "gpt-5.4", true)
		require.Equal(t, allowed != "gpt-5.5", svc.isUpstreamModelRestrictedByChannel(ctx, 10, account, "gpt-5.4", true))
	}
}

func TestCompactForwardAdmissionLogsPredicateAndCredentialDenialsOnce(t *testing.T) {
	for _, credential := range []bool{false, true} {
		t.Run(map[bool]string{false: "predicate", true: "credential"}[credential], func(t *testing.T) {
			var output bytes.Buffer
			old := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
			t.Cleanup(func() { slog.SetDefault(old) })
			selected := turnAdmissionAccount()
			reason := "account_ineligible"
			if credential {
				selected.Type = AccountTypeOAuth
				selected.Credentials = map[string]any{"access_token": "synthetic-bearer", "expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)}
				reason = "credential_token_expired"
			} else {
				selected.Status = "disabled"
			}
			svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: selected}, false)
			svc.openAITokenProvider = &OpenAITokenProvider{}
			upstream := &httpUpstreamRecorder{}
			svc.httpUpstream = upstream
			body := []byte(`{"model":"gpt-5.4","input":"private-prompt"}`)
			ctx := context.WithValue(context.Background(), ctxkey.RequestID, "private-request-id")
			_, err := svc.Forward(ctx, adaptiveProtocolTestContext("/v1/responses/compact", body), selected, body)
			require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
			require.Empty(t, upstream.requests)
			var events []map[string]any
			for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
				var event map[string]any
				require.NoError(t, json.Unmarshal([]byte(line), &event))
				if event["msg"] == "openai_turn_admission_denied" {
					events = append(events, event)
				}
			}
			require.Len(t, events, 1)
			require.Equal(t, reason, events[0]["reason"])
			require.Equal(t, float64(selected.ID), events[0]["account_id"])
			require.Len(t, events[0]["request_id_hash"], 32)
			for _, private := range []string{"private-prompt", "private-request-id", "synthetic-bearer"} {
				require.NotContains(t, output.String(), private)
			}
		})
	}
}

func TestCompactAdmissionChangePreservesNormalImageModelPermissions(t *testing.T) {
	selected := turnAdmissionAccount()
	selected.Credentials["model_mapping"] = map[string]any{"draw-alias": "gpt-image-2"}
	selected.Extra = map[string]any{"use_responses_api": true}
	group := &Group{ID: 9, Status: StatusActive, AllowImageGeneration: true,
		ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"draw-alias"}}}
	selected.GroupIDs = []int64{group.ID}
	selected.Groups = []*Group{group}
	selected.AccountGroups = []AccountGroup{{GroupID: group.ID, AllowedModels: []string{"draw-alias", openAIImagesResponsesMainModelValue()}}}
	svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: selected}, false)
	upstream := &httpUpstreamRecorder{resp: passthroughAdmissionResponse()}
	svc.httpUpstream = upstream
	body := []byte(`{"model":"draw-alias","input":"draw","stream":false}`)
	c := adaptiveProtocolTestContext("/v1/responses", body)
	c.Set("api_key", &APIKey{GroupID: &group.ID, Group: group})
	_, err := svc.Forward(context.Background(), c, selected, body)
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, openAIImagesResponsesMainModelValue(), gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestCompactForwardAdmissionUsesFinalModel(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, global := range []bool{false, true} {
			for _, block := range []string{"request_model", "outbound_model", "during_admission"} {
				name := block
				if passthrough {
					name += "/passthrough"
				} else {
					name += "/native"
				}
				if global {
					name += "/global"
				} else {
					name += "/account"
				}
				t.Run(name, func(t *testing.T) {
					selected := turnAdmissionAccount()
					selected.Extra = map[string]any{"openai_passthrough": passthrough}
					if !global {
						selected.Credentials["compact_model_mapping"] = map[string]any{"gpt-5.4": "gpt-5.5"}
					}
					repo := &turnAdmissionRepo{account: selected}
					svc := newTurnAdmissionGateway(repo, false)
					if global {
						svc.cfg.Gateway.OpenAICompactModel = "gpt-5.5"
					}
					blockModel := "gpt-5.5"
					if block == "request_model" {
						blockModel = "gpt-5.4"
					}
					installBlock := func() {
						now := time.Now()
						svc.recordOpenAIAccountModelTransientFailure(selected, blockModel, now)
						svc.recordOpenAIAccountModelTransientFailure(selected, blockModel, now)
					}
					if block == "during_admission" {
						repo.afterRead = func(n int, _ *Account) {
							if n == 2 {
								installBlock()
							}
						}
					} else {
						installBlock()
					}
					upstream := &httpUpstreamRecorder{resp: passthroughAdmissionResponse()}
					svc.httpUpstream = upstream
					body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false}`)
					ctx, pricingAt := WithGatewayTokenRequestPricing(context.Background())
					c := adaptiveProtocolTestContext("/v1/responses/compact", body)
					c.Request = c.Request.WithContext(ctx)
					result, err := svc.Forward(ctx, c, selected, body)
					if block != "request_model" {
						require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
						require.Nil(t, result)
						require.Empty(t, upstream.requests)
						return
					}
					require.NoError(t, err)
					require.Len(t, upstream.requests, 1)
					require.Equal(t, "gpt-5.5", gjson.GetBytes(upstream.lastBody, "model").String())
					require.Equal(t, "gpt-5.4", result.Model)
					require.Equal(t, "gpt-5.5", result.UpstreamModel)
					require.Equal(t, 3, result.Usage.InputTokens)
					require.Equal(t, 2, result.Usage.OutputTokens)
					require.Equal(t, pricingAt, GatewayTokenRequestPricingAtFromContext(c.Request.Context()))
				})
			}
		}
	}
}

func TestCompactForwardAdmissionPreservesAliasAndOutboundPermissions(t *testing.T) {
	for _, allowed := range [][]string{{"gpt-5.4"}, {"gpt-5.5"}, {"gpt-5.4", "gpt-5.5"}} {
		selected := turnAdmissionAccount()
		selected.GroupIDs = []int64{9}
		selected.Groups = []*Group{{ID: 9, Status: StatusActive, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}}}
		selected.AccountGroups = []AccountGroup{{GroupID: 9, AllowedModels: allowed}}
		selected.Credentials["compact_model_mapping"] = map[string]any{"gpt-5.4": "gpt-5.5"}
		svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: selected}, false)
		upstream := &httpUpstreamRecorder{resp: passthroughAdmissionResponse()}
		svc.httpUpstream = upstream
		body := []byte(`{"model":"gpt-5.4","input":"hello","stream":false}`)
		c := adaptiveProtocolTestContext("/v1/responses/compact", body)
		ctx := context.WithValue(context.Background(), ctxkey.Model, "public-model")
		c.Request = c.Request.WithContext(ctx)
		groupID := int64(9)
		c.Set("api_key", &APIKey{GroupID: &groupID})
		_, err := svc.Forward(ctx, c, selected, body)
		if len(allowed) == 2 {
			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
		} else {
			require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
			require.Empty(t, upstream.requests)
		}
	}
}
