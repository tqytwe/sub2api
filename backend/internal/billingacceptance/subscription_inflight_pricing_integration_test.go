//go:build integration && billing_acceptance

package billingacceptance

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func runInflightPricing(t *testing.T, db *sql.DB, client *dbent.Client) {
	t.Run("compact_upstream_account_stats_frozen_separately", func(t *testing.T) {
		gate := newAdmissionResponseGate(t)
		f := newAdmissionFixture(t, db, client, true, gate)
		f.requestPath = "/v1/responses/compact"
		_, err := db.Exec(`UPDATE accounts SET credentials=credentials || '{"compact_model_mapping":{"gpt-5.1":"gpt-5.5"}}'::jsonb,extra=extra || '{"openai_compact_mode":"force_on"}'::jsonb WHERE id=$1`, f.account.ID)
		require.NoError(t, err)
		channel := &service.Channel{Name: "compact-stats-" + uuid.NewString(), Status: service.StatusActive, GroupIDs: []int64{*f.key.GroupID}, BillingModelSource: service.BillingModelSourceRequested}
		require.NoError(t, repository.NewChannelRepository(db).Create(context.Background(), channel))
		outbound := make(chan string, 1)
		f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.sends.Add(1)
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				http.Error(w, "fixture read failed", http.StatusBadRequest)
				return
			}
			outbound <- gjson.GetBytes(body, "model").String()
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"compact-stats","model":"gpt-5.5","status":"completed","output":[{"type":"compaction","encrypted_content":"fixture"}],"usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":11,"cache_creation_tokens":7}}}`)
		})
		response := startAdmissionRequest(f)
		awaitResponseGate(t, gate)
		require.Equal(t, "gpt-5.5", <-outbound)
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		gate.open()
		r := receiveAdmissionResponse(t, response)
		require.NoError(t, r.err)
		require.Equal(t, http.StatusOK, r.status, r.body)
		call := f.awaitSettlements(t, 1)[0]
		require.NoError(t, call.err)
		require.NotEmpty(t, call.command.SubscriptionReceiptID)
		billing := service.NewBillingService(&config.Config{}, nil)
		tokens := service.UsageTokens{InputTokens: 82, OutputTokens: 20, CacheCreationTokens: 7, CacheReadTokens: 11}
		want, err := billing.CalculateCostUnified(service.CostInput{Ctx: context.Background(), Model: "gpt-5.5", Tokens: tokens, RateMultiplier: 1, Resolver: service.NewModelPricingResolver(nil, billing)})
		require.NoError(t, err)
		customer, err := billing.CalculateCostUnified(service.CostInput{Ctx: context.Background(), Model: "gpt-5.1", Tokens: tokens, RateMultiplier: 0.5, Resolver: service.NewModelPricingResolver(nil, billing)})
		require.NoError(t, err)
		require.NotEqual(t, customer.TotalCost, want.TotalCost, "fixture must distinguish customer and upstream statistics prices")
		require.InDelta(t, customer.ActualCost, call.command.BilledCost, 1e-10)
		var stats sql.NullFloat64
		require.NoError(t, db.QueryRow(`SELECT account_stats_cost FROM usage_logs WHERE api_key_id=$1`, f.key.ID).Scan(&stats))
		require.True(t, stats.Valid, "actual compact upstream model must retain its own statistics price")
		require.InDelta(t, want.TotalCost, stats.Float64, 1e-10)
		assertAdmissionUsageLink(t, f, true)
	})
	for _, source := range []string{service.BillingModelSourceRequested, service.BillingModelSourceChannelMapped, "per_request"} {
		t.Run("frozen_channel_"+source, func(t *testing.T) {
			gate := newAdmissionResponseGate(t)
			f := newAdmissionFixture(t, db, client, true, gate)
			f.requestBody = `{"model":"private-price-alias","input":"fixture","stream":false}`
			input, output, creation, read, request := 2e-6, 8e-6, 3e-6, 1e-6, 3.0
			pricingModel := "gpt-5.1"
			billingSource := source
			mode := service.BillingModeToken
			if source == service.BillingModelSourceRequested {
				pricingModel = "private-price-alias"
			}
			if source == "per_request" {
				billingSource = service.BillingModelSourceChannelMapped
				mode = service.BillingModePerRequest
			}
			channel := &service.Channel{Name: "inflight-" + uuid.NewString(), Status: service.StatusActive, BillingModelSource: billingSource, GroupIDs: []int64{*f.key.GroupID}, ModelMapping: map[string]map[string]string{service.PlatformOpenAI: {"private-price-alias": "gpt-5.1"}}, ModelPricing: []service.ChannelModelPricing{{Platform: service.PlatformOpenAI, Models: []string{pricingModel}, BillingMode: mode, InputPrice: &input, OutputPrice: &output, CacheWritePrice: &creation, CacheReadPrice: &read, PerRequestPrice: &request}}}
			repo := repository.NewChannelRepository(db)
			require.NoError(t, repo.Create(context.Background(), channel))
			_, err := db.Exec(`UPDATE groups SET billing_surcharge_override_enabled=true,billing_surcharge_enabled=true,billing_surcharge_mode='additive_multiplier',billing_surcharge_value=0.3 WHERE id=$1`, *f.key.GroupID)
			require.NoError(t, err)
			// Metadata is not a pricing candidate for requested/mapped billing.
			f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				f.sends.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"synthetic-alias","model":"unpriced-response-date-alias","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":11,"cache_creation_tokens":7}}}`)
			})
			before := f.readCounters(t)
			response := startAdmissionRequest(f)
			awaitResponseGate(t, gate)
			input, output, creation, read, request = 1, 2, 3, 4, 30
			require.NoError(t, repo.Update(context.Background(), channel))
			f.channels.InvalidateCache()
			_, err = db.Exec(`UPDATE groups SET rate_multiplier=7,billing_surcharge_value=4 WHERE id=$1`, *f.key.GroupID)
			require.NoError(t, err)
			require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
			gate.open()
			r := receiveAdmissionResponse(t, response)
			require.NoError(t, r.err)
			require.Equal(t, 200, r.status, r.body)
			call := f.awaitSettlements(t, 1)[0]
			require.NoError(t, call.err)
			total := 82*2e-6 + 20*8e-6 + 7*3e-6 + 11*1e-6
			if source == "per_request" {
				total = 3
			}
			billed := total * 0.8 // 0.5 customer multiplier + 0.3 frozen surcharge.
			require.InDelta(t, billed, call.command.BilledCost, 1e-10)
			require.InDelta(t, service.QuantizeUsageBillingAmount(billed), call.command.SubscriptionCost, 1e-10)
			delta := counterSnapshot{Daily: call.command.SubscriptionCost, Weekly: call.command.SubscriptionCost, Monthly: call.command.SubscriptionCost, Requests: 1, Amount: billed, Tokens: 120}
			assertCounters(t, delta, f.readCounters(t).minus(before))
			assertAdmissionUsageLink(t, f, true)
			assertInflightDetail(t, f, call.command, true)
		})
	}
	for _, feature := range []string{"long_context_intervals", "free_fast"} {
		t.Run("frozen_"+feature, func(t *testing.T) {
			gate := newAdmissionResponseGate(t)
			f := newAdmissionFixture(t, db, client, true, gate)
			low, high, output, highOutput, creation, highCreation, read, highRead, fast := 2e-6, 20e-6, 8e-6, 80e-6, 3e-6, 30e-6, 1e-6, 10e-6, 2.5
			pricing := service.ChannelModelPricing{Platform: service.PlatformOpenAI, Models: []string{"gpt-5.1"}, BillingMode: service.BillingModeToken, InputPrice: &low, OutputPrice: &output, CacheWritePrice: &creation, CacheReadPrice: &read, FastMultiplier: &fast}
			want := (82*low + 20*output + 7*creation + 11*read) * 0.5
			if feature == "long_context_intervals" {
				limit := 50
				pricing.Intervals = []service.PricingInterval{{MinTokens: 0, MaxTokens: &limit, InputPrice: &low, OutputPrice: &output, CacheWritePrice: &creation, CacheReadPrice: &read}, {MinTokens: 50, InputPrice: &high, OutputPrice: &highOutput, CacheWritePrice: &highCreation, CacheReadPrice: &highRead}}
				_, err := db.Exec(`UPDATE groups SET long_context_pricing_enabled=true WHERE id=$1`, *f.key.GroupID)
				require.NoError(t, err)
				want = (82*high + 20*highOutput + 7*highCreation + 11*highRead) * 0.5
			} else {
				_, err := db.Exec(`UPDATE groups SET free_openai_fast=true WHERE id=$1`, *f.key.GroupID)
				require.NoError(t, err)
				f.requestBody = `{"model":"gpt-5.1","input":"fixture","service_tier":"priority"}`
				f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					f.sends.Add(1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"id":"fast-fixture","model":"gpt-5.1","service_tier":"priority","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":11,"cache_creation_tokens":7}}}`)
				})
			}
			repo := repository.NewChannelRepository(db)
			channel := &service.Channel{Name: "frozen-feature-" + uuid.NewString(), Status: service.StatusActive, GroupIDs: []int64{*f.key.GroupID}, BillingModelSource: service.BillingModelSourceChannelMapped, ModelPricing: []service.ChannelModelPricing{pricing}}
			require.NoError(t, repo.Create(context.Background(), channel))
			before := f.readCounters(t)
			response := startAdmissionRequest(f)
			awaitResponseGate(t, gate)
			_, err := db.Exec(`UPDATE groups SET free_openai_fast=false,long_context_pricing_enabled=false WHERE id=$1`, *f.key.GroupID)
			require.NoError(t, err)
			channel.ModelPricing = nil
			require.NoError(t, repo.Update(context.Background(), channel))
			f.channels.InvalidateCache()
			require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
			gate.open()
			r := receiveAdmissionResponse(t, response)
			require.NoError(t, r.err)
			require.Equal(t, 200, r.status, r.body)
			call := f.awaitSettlements(t, 1)[0]
			require.NoError(t, call.err)
			require.InDelta(t, want, call.command.BilledCost, 1e-10)
			assertCounters(t, counterSnapshot{Daily: call.command.SubscriptionCost, Weekly: call.command.SubscriptionCost, Monthly: call.command.SubscriptionCost, Requests: 1, Amount: want, Tokens: 120}, f.readCounters(t).minus(before))
			assertAdmissionUsageLink(t, f, true)
		})
	}

	t.Run("unknown_response_pricing_preserves_real_usage_pending", func(t *testing.T) {
		f := newAdmissionFixture(t, db, client, true)
		before := f.readCounters(t)
		repo := repository.NewChannelRepository(db)
		require.NoError(t, repo.Create(context.Background(), &service.Channel{Name: "unknown-response-" + uuid.NewString(), Status: service.StatusActive, GroupIDs: []int64{*f.key.GroupID}, BillingModelSource: service.BillingModelSourceResponse}))
		f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			f.sends.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"unknown-response-price","model":"unpriced-response-model","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":11,"cache_creation_tokens":7}}}`)
		})
		status, body, err := f.request()
		require.NoError(t, err)
		require.Equal(t, 200, status, body)
		require.Empty(t, f.billing.snapshot())
		assertCounters(t, before, f.readCounters(t))
		assertCounterProofCounts(t, f.counterFixture, 0, 0)
		var noCommand bool
		var reason string
		var input int
		require.NoError(t, db.QueryRow(`SELECT command_payload IS NULL,pending_reason,(usage_payload::jsonb->>'input_tokens')::int FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&noCommand, &reason, &input))
		require.True(t, noCommand)
		require.Equal(t, "unsupported_usage", reason)
		require.Equal(t, 100, input)
		require.Zero(t, recoverInflight(t, f))
		require.EqualValues(t, 1, f.sends.Load())
	})

	for _, kind := range []string{"input_file", "input_audio", "input_image"} {
		t.Run("media_scope_exclusion_"+kind, func(t *testing.T) {
			f := newAdmissionFixture(t, db, client, true)
			f.requestBody = fmt.Sprintf(`{"model":"gpt-5.1","input":[{"role":"user","content":[{"type":"%s","file_id":"synthetic-fixture","data":"fixture","image_url":"data:image/png;base64,fixture"}]}]}`, kind)
			_, _, err := f.request()
			require.NoError(t, err)
			var count int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_receipts WHERE api_key_id=$1`, f.key.ID).Scan(&count))
			require.Zero(t, count)
		})
	}
}
