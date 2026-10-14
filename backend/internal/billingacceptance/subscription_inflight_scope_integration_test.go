//go:build integration && billing_acceptance

package billingacceptance

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runInflightNativeScope(t *testing.T, db *sql.DB, client *dbent.Client) {
	for _, packaged := range []bool{false, true} {
		for _, native := range []bool{true, false} {
			for _, revoke := range []bool{false, true} {
				t.Run(fmt.Sprintf("package_%t_native_%t_revoke_%t", packaged, native, revoke), func(t *testing.T) {
					gate := newAdmissionResponseGate(t)
					f := newAdmissionFixture(t, db, client, packaged, gate)
					_, err := db.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{openai_responses_supported}',to_jsonb($2::boolean)) WHERE id=$1`, f.account.ID, native)
					require.NoError(t, err)
					paths := make(chan string, 1)
					f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						f.sends.Add(1)
						paths <- r.URL.Path
						w.Header().Set("Content-Type", "application/json")
						if native {
							_, _ = io.WriteString(w, `{"id":"synthetic-native","object":"response","model":"gpt-5.1","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"synthetic"}]}],"usage":{"input_tokens":100,"output_tokens":20}}`)
						} else {
							_, _ = io.WriteString(w, `{"id":"synthetic-cc","object":"chat.completion","model":"gpt-5.1","choices":[{"index":0,"message":{"role":"assistant","content":"synthetic"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120}}`)
						}
					})
					before := f.readCounters(t)
					response := startAdmissionRequest(f)
					awaitResponseGate(t, gate)
					actualPath := <-paths
					if native {
						require.Equal(t, "/v1/responses", actualPath)
					} else {
						require.Equal(t, "/v1/chat/completions", actualPath)
					}
					var receipts int
					require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_receipts WHERE api_key_id=$1`, f.key.ID).Scan(&receipts))
					if native {
						assert.Equal(t, 1, receipts)
					} else {
						assert.Zero(t, receipts, "Responses-to-Chat conversion must not obtain native admission")
					}
					if revoke {
						require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
					}
					gate.open()
					r := receiveAdmissionResponse(t, response)
					require.NoError(t, r.err)
					require.Equal(t, http.StatusOK, r.status, r.body)
					call := f.awaitSettlements(t, 1)[0]
					if !native && revoke {
						assert.Error(t, call.err, "conversion retains legacy revoked-subscription refusal")
						assertCounters(t, before, f.readCounters(t))
					} else {
						require.NoError(t, call.err)
						assertCounters(t, inflightPlainTextDelta(t, f), f.readCounters(t).minus(before))
					}
					t.Logf("actual_path=%s native=%t revoke=%t receipts=%d sends=%d apply_error=%v receipt_id_present=%t delta=%+v", actualPath, native, revoke, receipts, f.sends.Load(), call.err, call.command.SubscriptionReceiptID != "", f.readCounters(t).minus(before))
				})
			}
		}
	}
}

func runInflightExpiryWait(t *testing.T, db *sql.DB, client *dbent.Client) {
	cases := []struct {
		name          string
		packaged      bool
		lock, expires string
	}{
		{"ordinary_subscription_lock", false, "subscription", "subscription"},
		{"package_subscription_lock", true, "subscription", "subscription"},
		{"package_entitlement_lock_sub_expires", true, "entitlement", "subscription"},
		{"package_entitlement_lock_ent_expires", true, "entitlement", "entitlement"},
		{"ordinary_request_lock", false, "request", "subscription"},
		{"package_request_lock_sub_expires", true, "request", "subscription"},
		{"package_request_lock_ent_expires", true, "request", "entitlement"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			beforeSend := newAdmissionResponseGate(t)
			f := newAdmissionFixture(t, db, client, tc.packaged, nil, beforeSend)
			before := f.readCounters(t)
			expiresTable, expiresID := "user_subscriptions", f.sub.ID
			if tc.expires == "entitlement" {
				expiresTable, expiresID = "subscription_package_entitlements", f.sub.PackageEntitlement.ID
			}
			_, err := db.Exec(`UPDATE `+expiresTable+` SET expires_at=clock_timestamp()+INTERVAL '1500 milliseconds' WHERE id=$1`, expiresID)
			require.NoError(t, err)
			response := startAdmissionRequest(f)
			awaitResponseGate(t, beforeSend)
			tx, err := db.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			var pid int
			require.NoError(t, tx.QueryRow(`SELECT pg_backend_pid()`).Scan(&pid))
			var lockedID int64
			switch tc.lock {
			case "subscription":
				require.NoError(t, tx.QueryRow(`SELECT id FROM user_subscriptions WHERE id=$1 FOR UPDATE`, f.sub.ID).Scan(&lockedID))
			case "entitlement":
				require.NoError(t, tx.QueryRow(`SELECT id FROM subscription_package_entitlements WHERE id=$1 FOR UPDATE`, f.sub.PackageEntitlement.ID).Scan(&lockedID))
			case "request":
				var requestID string
				require.NoError(t, tx.QueryRow(`SELECT id FROM gateway_requests WHERE api_key_id=$1 FOR UPDATE`, f.key.ID).Scan(&requestID))
			}
			beforeSend.open()
			waiter := awaitPGBlockedBy(t, db, pid)
			var query string
			require.NoError(t, db.QueryRow(`SELECT query FROM pg_stat_activity WHERE pid=$1`, waiter).Scan(&query))
			if tc.lock == "request" {
				require.Contains(t, query, "UPDATE gateway_requests")
			} else {
				require.Contains(t, query, "SELECT expires_at")
			}
			require.Zero(t, f.sends.Load(), "the actual upstream is still behind the held PG row lock")
			require.Eventually(t, func() bool {
				var expired bool
				return db.QueryRow(`SELECT expires_at<=clock_timestamp() FROM `+expiresTable+` WHERE id=$1`, expiresID).Scan(&expired) == nil && expired
			}, 2500*time.Millisecond, 10*time.Millisecond)
			require.NoError(t, tx.Commit())
			r := receiveAdmissionResponse(t, response)
			require.NoError(t, r.err)
			if f.sends.Load() > 0 {
				f.awaitSettlements(t, 1)
			}
			var receipts int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_receipts WHERE api_key_id=$1`, f.key.ID).Scan(&receipts))
			t.Logf("lock=%s expires=%s blocker_pid=%d waiter_pid=%d status=%d sends=%d receipts=%d delta=%+v", tc.lock, tc.expires, pid, waiter, r.status, f.sends.Load(), receipts, f.readCounters(t).minus(before))
			assert.GreaterOrEqual(t, r.status, 400, r.body)
			assert.Zero(t, f.sends.Load(), "expiry reached while waiting must prevent the actual send")
			assert.Zero(t, receipts, "all admission locks must be followed by a fresh expiry check")
			assertCounters(t, before, f.readCounters(t))
		})
	}
}

func inflightPlainTextDelta(t *testing.T, f *admissionFixture) counterSnapshot {
	t.Helper()
	billing := service.NewBillingService(&config.Config{}, nil)
	cost, err := billing.CalculateCostUnified(service.CostInput{Ctx: context.Background(), Model: "gpt-5.1", Tokens: service.UsageTokens{InputTokens: 100, OutputTokens: 20}, RateMultiplier: 0.5, Resolver: service.NewModelPricingResolver(nil, billing)})
	require.NoError(t, err)
	charge := service.QuantizeUsageBillingAmount(cost.ActualCost)
	want := counterSnapshot{Daily: charge, Weekly: charge, Monthly: charge}
	if f.sub.PackageEntitlement != nil {
		want.Requests = 1
		want.Amount = cost.ActualCost
		want.Tokens = 120
	}
	return want
}

func runInflightLegacyScope(t *testing.T, db *sql.DB, client *dbent.Client) {
	cases := []struct {
		name, legacy, canonical, marker string
		image                           bool
	}{
		{"image", `{"model":"gpt-5.1","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,ZmFrZQ=="}}]}]}`, `{"model":"gpt-5.1","input":[{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,ZmFrZQ=="}]}]}`, `"input_image"`, true},
		{"file", `{"model":"gpt-5.1","messages":[{"role":"user","content":[{"type":"file","file":{"file_id":"synthetic-file"}}]}]}`, `{"model":"gpt-5.1","input":[{"role":"user","content":[{"type":"input_file","file_id":"synthetic-file"}]}]}`, `"input_file"`, false},
		{"tool", `{"model":"gpt-5.1","messages":[{"role":"tool","tool_call_id":"call_synthetic","content":"fixture"}]}`, `{"model":"gpt-5.1","input":[{"type":"item_reference","id":"call_synthetic"},{"type":"function_call_output","call_id":"call_synthetic","output":"fixture"}]}`, `"function_call_output"`, false},
		{"functions", `{"model":"gpt-5.1","messages":[{"role":"user","content":"hi"}],"functions":[{"name":"synthetic_lookup","parameters":{"type":"object","properties":{}}}]}`, `{"model":"gpt-5.1","input":"hi","tools":[{"type":"function","name":"synthetic_lookup","parameters":{"type":"object","properties":{}}}]}`, `"synthetic_lookup"`, false},
	}
	for _, packaged := range []bool{false, true} {
		for _, tc := range cases {
			var baseline counterSnapshot
			for _, legacy := range []bool{false, true} {
				t.Run(fmt.Sprintf("package_%t_%s_legacy_%t", packaged, tc.name, legacy), func(t *testing.T) {
					f := newAdmissionFixture(t, db, client, packaged)
					f.requestBody = tc.canonical
					if legacy {
						f.requestBody = tc.legacy
					}
					outbound := make(chan string, 1)
					f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						f.sends.Add(1)
						b, _ := io.ReadAll(r.Body)
						outbound <- string(b)
						w.Header().Set("Content-Type", "application/json")
						details := ""
						if tc.image {
							details = `,"input_tokens_details":{"image_tokens":4}`
						}
						_, _ = io.WriteString(w, `{"id":"synthetic-legacy","model":"gpt-5.1","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":20`+details+`}}`)
					})
					before := f.readCounters(t)
					status, body, err := f.request()
					require.NoError(t, err)
					require.Equal(t, 200, status, body)
					f.workers.Stop()
					require.Contains(t, <-outbound, tc.marker, "the real production conversion must preserve the excluded media/tool")
					var receipts, outboxes, unsupported int
					require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_receipts WHERE api_key_id=$1`, f.key.ID).Scan(&receipts))
					require.NoError(t, db.QueryRow(`SELECT COUNT(*),COUNT(*) FILTER(WHERE command_payload IS NULL AND pending_reason='unsupported_usage') FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&outboxes, &unsupported))
					calls := f.billing.snapshot()
					delta := f.readCounters(t).minus(before)
					t.Logf("legacy=%t kind=%s receipts=%d outboxes=%d unsupported=%d apply_calls=%d delta=%+v", legacy, tc.name, receipts, outboxes, unsupported, len(calls), delta)
					assert.Zero(t, receipts, "legacy aliases must not bypass the media/tool scope exclusion")
					assert.Zero(t, outboxes, "unsupported input keeps its original billing path")
					if assert.Len(t, calls, 1, "normal active media/tool consumption must retain the prior Apply path") {
						assert.NoError(t, calls[0].err)
						require.NotNil(t, calls[0].result)
						assert.True(t, calls[0].result.Applied)
					}
					if legacy {
						assertCounters(t, baseline, delta)
					} else {
						baseline = delta
						assert.Greater(t, delta.Daily, 0.0)
					}
				})
			}
		}
	}
}

func runInflightUnknownOutput(t *testing.T, db *sql.DB, client *dbent.Client) {
	for _, passthrough := range []bool{false, true} {
		for _, kind := range []string{"audio", "web_search", "file_search", "code_interpreter"} {
			for _, mode := range []string{"json", "terminal_sse", "progress_eof", "progress_terminal", "buffered_sse", "zero_json", "zero_eof", "no_usage_json", "zero_buffered_error", "invalid_sse", "part_eof", "part_terminal"} {
				if strings.HasPrefix(mode, "part_") && kind != "audio" {
					continue
				}
				if kind == "audio" && (mode == "no_usage_json" || mode == "invalid_sse") {
					continue
				}
				t.Run(fmt.Sprintf("%s_%s_passthrough_%t", kind, mode, passthrough), func(t *testing.T) {
					f := newAdmissionFixture(t, db, client, true)
					f.stream = mode != "json" && mode != "zero_json" && mode != "buffered_sse" && mode != "no_usage_json" && mode != "zero_buffered_error"
					if passthrough {
						_, err := db.Exec(`UPDATE accounts SET extra=extra || '{"openai_passthrough":true}'::jsonb WHERE id=$1`, f.account.ID)
						require.NoError(t, err)
					}
					input, outputTokens := 100, 20
					if strings.HasPrefix(mode, "zero_") || mode == "no_usage_json" || mode == "invalid_sse" {
						input, outputTokens = 0, 0
					}
					f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						f.sends.Add(1)
						assert.Equal(t, "/v1/responses", r.URL.Path)
						output := `[]`
						details := `,"input_tokens_details":{"audio_tokens":25},"output_tokens_details":{"audio_tokens":5}`
						if kind != "audio" {
							output = fmt.Sprintf(`[{"type":%q,"id":"tool_synthetic","status":"completed"}]`, kind+"_call")
							details = ""
						}
						if strings.HasPrefix(mode, "part_") {
							details = ""
						}
						usage := fmt.Sprintf(`{"input_tokens":%d,"output_tokens":%d%s}`, input, outputTokens, details)
						response := `{"id":"synthetic-output","model":"gpt-5.1","status":"completed","output":` + output + `,"usage":` + usage + `}`
						if mode == "no_usage_json" {
							response = `{"id":"synthetic-output","model":"gpt-5.1","status":"completed","output":` + output + `}`
						}
						if mode == "json" || mode == "zero_json" || mode == "no_usage_json" {
							w.Header().Set("Content-Type", "application/json")
							_, _ = io.WriteString(w, response)
							return
						}
						w.Header().Set("Content-Type", "text/event-stream")
						if mode == "zero_buffered_error" {
							w.Header().Set("Content-Length", "100000")
						}
						if mode == "invalid_sse" {
							_, _ = io.WriteString(w, "data: "+`{"type":"response.output_text.delta","delta":"fixture"}`+"\n\n"+"data: "+fmt.Sprintf(`{"type":"response.output_item.done","item":{"type":%q,"id":"tool_synthetic"},"broken":`, kind+"_call")+"\n\n")
							return
						}
						_, _ = io.WriteString(w, "data: "+`{"type":"response.output_text.delta","delta":"fixture"}`+"\n\n")
						if mode != "terminal_sse" {
							event := `{"type":"response.in_progress","response":{"id":"synthetic-output","usage":` + usage + `}}`
							if kind != "audio" {
								event = fmt.Sprintf(`{"type":"response.output_item.done","item":{"type":%q,"id":"tool_synthetic","status":"completed"},"usage":%s}`, kind+"_call", usage)
							}
							if strings.HasPrefix(mode, "part_") {
								_, _ = io.WriteString(w, "data: "+`{"type":"response.output_item.added","item":{"type":"message","content":[]}}`+"\n\n"+"data: "+`{"type":"response.content_part.added","part":{"type":"output_audio"}}`+"\n\n")
							}
							_, _ = io.WriteString(w, "data: "+event+"\n\n")
							if mode == "progress_eof" || mode == "zero_eof" || mode == "zero_buffered_error" || mode == "part_eof" {
								_ = http.NewResponseController(w).Flush()
								return
							}
							// Terminal aggregate counts are authoritative, but omitted modality/tool
							// evidence must not erase an earlier observation from this same send.
							response = `{"id":"synthetic-output","model":"gpt-5.1","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":20}}`
						}
						_, _ = io.WriteString(w, "data: "+`{"type":"response.completed","response":`+response+"}\n\n")
						_ = http.NewResponseController(w).Flush()
					})
					before := f.readCounters(t)
					status, body, err := f.request()
					require.NoError(t, err)
					wantStatus := 200
					if mode == "zero_buffered_error" {
						wantStatus = 502
					}
					require.Equal(t, wantStatus, status, body)
					f.workers.Stop()
					if mode == "invalid_sse" {
						var observations int
						require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&observations))
						assert.Zero(t, observations, "invalid JSON cannot prove observed output")
						assert.Empty(t, f.billing.snapshot())
						assertCounters(t, before, f.readCounters(t))
						assert.EqualValues(t, 1, f.sends.Load())
						return
					}
					var noCommand, completed bool
					var reason string
					var observedInput, observedInputAudio, observedOutputAudio, observedSearch int
					require.NoError(t, db.QueryRow(`SELECT command_payload IS NULL,completed_at IS NOT NULL,pending_reason,(usage_payload::jsonb->>'input_tokens')::int,COALESCE((usage_payload::jsonb#>>'{unpriced_output_usage,input_audio_tokens}')::int,0),COALESCE((usage_payload::jsonb#>>'{unpriced_output_usage,output_audio_tokens}')::int,0),COALESCE((usage_payload::jsonb#>>'{unpriced_output_usage,search_calls}')::int,0) FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&noCommand, &completed, &reason, &observedInput, &observedInputAudio, &observedOutputAudio, &observedSearch))
					t.Logf("mode=%s kind=%s passthrough=%t no_command=%t completed=%t reason=%q input=%d audio=%d/%d search=%d apply_calls=%d", mode, kind, passthrough, noCommand, completed, reason, observedInput, observedInputAudio, observedOutputAudio, observedSearch, len(f.billing.snapshot()))
					assert.True(t, noCommand, "unknown audio/tool output must never become a text billing command")
					assert.False(t, completed)
					assert.Equal(t, "unsupported_usage", reason)
					assert.Equal(t, input, observedInput)
					if strings.HasPrefix(mode, "part_") {
						var types string
						require.NoError(t, db.QueryRow(`SELECT COALESCE(usage_payload::jsonb#>>'{unpriced_output_usage,output_types}','') FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&types))
						assert.Contains(t, types, "output_audio")
						assert.Zero(t, observedInputAudio)
						assert.Zero(t, observedOutputAudio)
					} else if kind == "audio" {
						assert.Equal(t, 25, observedInputAudio)
						assert.Equal(t, 5, observedOutputAudio)
					} else if kind == "web_search" {
						assert.Equal(t, 1, observedSearch)
					} else {
						var observedTypes string
						require.NoError(t, db.QueryRow(`SELECT COALESCE(usage_payload::jsonb#>>'{unpriced_output_usage,output_types}','') FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&observedTypes))
						assert.Contains(t, observedTypes, kind+"_call")
					}
					require.Zero(t, recoverInflight(t, f))
					assert.Empty(t, f.billing.snapshot())
					assertCounters(t, before, f.readCounters(t))
					assert.EqualValues(t, 1, f.sends.Load())
				})
			}
		}
	}
}

func runInflightModelCase(t *testing.T, db *sql.DB, client *dbent.Client) {
	for _, compact := range []bool{false, true} {
		for _, upper := range []bool{false, true} {
			t.Run(fmt.Sprintf("compact_%t_upper_%t", compact, upper), func(t *testing.T) {
				f := newAdmissionFixture(t, db, client, true)
				model := "gpt-5.5"
				if upper {
					model = "GPT-5.5"
				}
				f.requestBody = fmt.Sprintf(`{"model":%q,"input":"fixture"}`, model)
				source := service.BillingModelSourceResponse
				if compact {
					f.requestPath = "/v1/responses/compact"
					source = service.BillingModelSourceRequested
					_, err := db.Exec(`UPDATE accounts SET credentials=credentials || jsonb_build_object('compact_model_mapping',jsonb_build_object($2::text,'gpt-5.5')),extra=extra || '{"openai_compact_mode":"force_on"}'::jsonb WHERE id=$1`, f.account.ID, model)
					require.NoError(t, err)
				}
				require.NoError(t, repository.NewChannelRepository(db).Create(context.Background(), &service.Channel{Name: fmt.Sprintf("model-case-%d", f.key.ID), Status: service.StatusActive, GroupIDs: []int64{*f.key.GroupID}, BillingModelSource: source}))
				outbound := make(chan string, 1)
				f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					f.sends.Add(1)
					b, _ := io.ReadAll(r.Body)
					outbound <- string(b)
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"id":"synthetic-case","model":"gpt-5.5","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":20}}`)
				})
				before := f.readCounters(t)
				status, body, err := f.request()
				require.NoError(t, err)
				require.Equal(t, 200, status, body)
				f.workers.Stop()
				sent := <-outbound
				calls := f.billing.snapshot()
				var noCommand, completed bool
				var reason string
				require.NoError(t, db.QueryRow(`SELECT command_payload IS NULL,completed_at IS NOT NULL,pending_reason FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&noCommand, &completed, &reason))
				var stats sql.NullFloat64
				statsErr := db.QueryRow(`SELECT account_stats_cost FROM usage_logs WHERE api_key_id=$1`, f.key.ID).Scan(&stats)
				t.Logf("requested=%s compact=%t outbound=%s no_command=%t complete=%t reason=%q apply_calls=%d stats=%+v stats_error=%v", model, compact, sent, noCommand, completed, reason, len(calls), stats, statsErr)
				assert.False(t, noCommand)
				assert.True(t, completed)
				assert.Empty(t, reason)
				if assert.Len(t, calls, 1) {
					assert.NoError(t, calls[0].err)
					billing := service.NewBillingService(&config.Config{}, nil)
					cost, err := billing.CalculateCostUnified(service.CostInput{Ctx: context.Background(), Model: "gpt-5.5", Tokens: service.UsageTokens{InputTokens: 100, OutputTokens: 20}, RateMultiplier: 0.5, Resolver: service.NewModelPricingResolver(nil, billing)})
					require.NoError(t, err)
					charge := service.QuantizeUsageBillingAmount(cost.ActualCost)
					assertCounters(t, counterSnapshot{Daily: charge, Weekly: charge, Monthly: charge, Requests: 1, Amount: cost.ActualCost, Tokens: 120}, f.readCounters(t).minus(before))
					var displayModel string
					require.NoError(t, db.QueryRow(`SELECT model FROM usage_logs WHERE api_key_id=$1`, f.key.ID).Scan(&displayModel))
					assert.Equal(t, model, displayModel, "pricing keys are canonical; displayed request model keeps its original spelling")
					assert.NoError(t, statsErr)
					assert.True(t, stats.Valid)
					assert.InDelta(t, cost.TotalCost, stats.Float64, 1e-10)
				}
			})
		}
	}
}

// Exercise the real sorted migration runner and real HTTP routing/settlement
// together after the independently merged routing migration 279.
func runInflightRoutingMigrationCoexistence(t *testing.T, db *sql.DB, client *dbent.Client) {
	var previous time.Time
	for _, filename := range []string{"278_gateway_request_task_page_index_notx.sql", "279_gateway_routing_evidence.sql", "280_subscription_inflight_admission.sql"} {
		raw, err := migrations.FS.ReadFile(filename)
		require.NoError(t, err)
		want := sha256.Sum256([]byte(strings.TrimSpace(string(raw))))
		var applied time.Time
		var checksum string
		require.NoError(t, db.QueryRow(`SELECT applied_at,checksum FROM schema_migrations WHERE filename=$1`, filename).Scan(&applied, &checksum))
		require.Equal(t, hex.EncodeToString(want[:]), checksum)
		require.False(t, applied.Before(previous), "real runner must apply 278, 279, then 280")
		previous = applied
		t.Logf("migration=%s checksum=%s applied_at=%s", filename, checksum, applied.UTC().Format(time.RFC3339Nano))
	}
	require.NoError(t, repository.ApplyMigrations(context.Background(), db), "complete runner restart is idempotent")
	f := newAdmissionFixture(t, db, client, true)
	before := f.readCounters(t)
	status, body, err := f.request()
	require.NoError(t, err)
	require.Equal(t, 200, status, body)
	f.workers.Stop()
	calls := f.billing.snapshot()
	require.Len(t, calls, 1)
	require.NoError(t, calls[0].err)
	assertCounters(t, inflightCharge(t, f, calls[0].command), f.readCounters(t).minus(before))
	assertInflightDetail(t, f, calls[0].command, true)
	var completed bool
	require.NoError(t, db.QueryRow(`SELECT completed_at IS NOT NULL FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&completed))
	assert.True(t, completed)
	assertAdmissionUsageLink(t, f, true)
	var version, state string
	var count, stored int
	require.Eventually(t, func() bool {
		err := db.QueryRow(`SELECT r.routing_coverage_version,r.routing_summary->>'status',d.event_count,jsonb_array_length(d.events) FROM gateway_requests r JOIN gateway_request_routing d ON d.request_id=r.id WHERE r.api_key_id=$1`, f.key.ID).Scan(&version, &state, &count, &stored)
		return err == nil && count > 0 && count == stored
	}, 3*time.Second, 10*time.Millisecond, "settlement must retain the routing writer's actual evidence")
	assert.Equal(t, requestledger.RoutingCoverageVersion, version)
	assert.NotEqual(t, "not_recorded", state)
	assert.EqualValues(t, 1, f.sends.Load())
}
