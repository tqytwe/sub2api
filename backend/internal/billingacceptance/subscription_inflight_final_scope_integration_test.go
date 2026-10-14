//go:build integration && billing_acceptance

package billingacceptance

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func runInflightFinalMediaScope(t *testing.T, db *sql.DB, client *dbent.Client) {
	for _, packaged := range []bool{false, true} {
		for _, mode := range []string{"image_model", "codex_bridge"} {
			for _, explicit := range []bool{true, false} {
				t.Run(fmt.Sprintf("package_%t_%s_explicit_%t", packaged, mode, explicit), func(t *testing.T) {
					f := newAdmissionFixture(t, db, client, packaged)
					_, err := db.Exec(`UPDATE groups SET allow_image_generation=true,image_price_1k=0.2,image_price_2k=0.2,image_price_4k=0.2 WHERE id=$1`, *f.key.GroupID)
					require.NoError(t, err)
					model := "gpt-5.1"
					if mode == "image_model" && !explicit {
						model = "gpt-image-1"
					}
					tools := ""
					if explicit {
						tools = `,"tools":[{"type":"image_generation","model":"gpt-image-1"}]`
					}
					f.requestBody = fmt.Sprintf(`{"model":%q,"input":"synthetic image fixture"%s}`, model, tools)
					if mode == "codex_bridge" {
						_, err = db.Exec(`UPDATE accounts SET extra=extra || '{"codex_image_generation_bridge":true}'::jsonb WHERE id=$1`, f.account.ID)
						require.NoError(t, err)
						f.requestHeaders = map[string]string{"User-Agent": "codex_cli_rs/0.98.0"}
					}
					outbound := make(chan string, 1)
					f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						f.sends.Add(1)
						b, readErr := io.ReadAll(r.Body)
						assert.NoError(t, readErr)
						outbound <- string(b)
						assert.Equal(t, "/v1/responses", r.URL.Path)
						w.Header().Set("Content-Type", "application/json")
						_, _ = io.WriteString(w, `{"id":"synthetic-image-response","model":"gpt-5.1","status":"completed","output":[{"id":"synthetic-image","type":"image_generation_call","status":"completed","result":"c3ludGhldGlj","size":"1024x1024"}],"usage":{"input_tokens":100,"output_tokens":20}}`)
					})
					before := f.readCounters(t)
					status, body, err := f.request()
					require.NoError(t, err)
					require.Equal(t, http.StatusOK, status, body)
					f.workers.Stop()
					sent := <-outbound
					require.Equal(t, "image_generation", gjson.Get(sent, "tools.0.type").String(), "exercise real final-body media conversion")
					var receipts, outboxes int
					require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_receipts WHERE api_key_id=$1`, f.key.ID).Scan(&receipts))
					require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&outboxes))
					calls := f.billing.snapshot()
					t.Logf("mode=%s explicit=%t final_model=%s receipts=%d outboxes=%d apply=%d delta=%+v", mode, explicit, gjson.Get(sent, "model").String(), receipts, outboxes, len(calls), f.readCounters(t).minus(before))
					assert.Zero(t, receipts, "final media body must not retain prepared text admission")
					assert.Zero(t, outboxes, "original image pricing must not become unsupported text pending")
					if assert.Len(t, calls, 1, "original image settlement must still execute") {
						assert.NoError(t, calls[0].err)
						assert.Empty(t, calls[0].command.SubscriptionReceiptID)
						assert.InDelta(t, 0.1, calls[0].command.SubscriptionCost, 1e-8)
					}
					want := counterSnapshot{Daily: 0.1, Weekly: 0.1, Monthly: 0.1}
					if packaged {
						want.Requests, want.Amount, want.Tokens = 1, 0.1, 120
					}
					assertCounters(t, want, f.readCounters(t).minus(before))
					assert.EqualValues(t, 1, f.sends.Load())
				})
			}
		}
	}
}

func runInflightStandaloneAudio(t *testing.T, db *sql.DB, client *dbent.Client) {
	for _, passthrough := range []bool{false, true} {
		for _, event := range []struct{ name, frame string }{
			{"audio_delta", "data: " + `{"type":"response.audio.delta","response_id":"synthetic-audio","delta":"c3ludGhldGlj","sequence_number":1}` + "\n\n"},
			{"audio_done", "data: " + `{"type":"response.audio.done","response_id":"synthetic-audio","sequence_number":1}` + "\n\n"},
			{"part_event_fallback", "event: response.content_part.added\ndata: " + `{"part":{"type":"output_audio"}}` + "\n\n"},
		} {
			for _, mode := range []string{"terminal", "eof", "buffered"} {
				for _, zero := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s_%s_zero_%t_passthrough_%t", event.name, mode, zero, passthrough), func(t *testing.T) {
						f := newAdmissionFixture(t, db, client, true)
						f.stream = mode != "buffered"
						if passthrough {
							_, err := db.Exec(`UPDATE accounts SET extra=extra || '{"openai_passthrough":true}'::jsonb WHERE id=$1`, f.account.ID)
							require.NoError(t, err)
						}
						input, output := 100, 20
						if zero {
							input, output = 0, 0
						}
						f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							f.sends.Add(1)
							assert.Equal(t, "/v1/responses", r.URL.Path)
							w.Header().Set("Content-Type", "text/event-stream")
							usage := fmt.Sprintf(`{"input_tokens":%d,"output_tokens":%d}`, input, output)
							_, _ = io.WriteString(w, "data: "+`{"type":"response.in_progress","response":{"id":"synthetic-audio","model":"gpt-5.1","usage":`+usage+"}}\n\n"+event.frame)
							if mode != "eof" {
								_, _ = io.WriteString(w, "data: "+`{"type":"response.completed","response":{"id":"synthetic-audio","model":"gpt-5.1","status":"completed","output":[],"usage":`+usage+"}}\n\n")
							}
							_ = http.NewResponseController(w).Flush()
						})
						before := f.readCounters(t)
						status, body, err := f.request()
						require.NoError(t, err)
						require.Equal(t, http.StatusOK, status, body)
						f.workers.Stop()
						var noCommand, completed bool
						var reason, types string
						var observedInput int
						err = db.QueryRow(`SELECT command_payload IS NULL,completed_at IS NOT NULL,pending_reason,COALESCE(usage_payload::jsonb#>>'{unpriced_output_usage,output_types}',''),(usage_payload::jsonb->>'input_tokens')::int FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&noCommand, &completed, &reason, &types, &observedInput)
						t.Logf("event=%s mode=%s zero=%t passthrough=%t observation_error=%v no_command=%t completed=%t reason=%s types=%s apply=%d", event.name, mode, zero, passthrough, err, noCommand, completed, reason, types, len(f.billing.snapshot()))
						require.NoError(t, err, "observed standalone audio must be durably retained even without aggregate tokens")
						assert.True(t, noCommand)
						assert.False(t, completed)
						assert.Equal(t, "unsupported_usage", reason)
						assert.Contains(t, types, "audio")
						assert.Equal(t, input, observedInput)
						assert.Zero(t, recoverInflight(t, f))
						assert.Empty(t, f.billing.snapshot())
						assertCounters(t, before, f.readCounters(t))
						assert.EqualValues(t, 1, f.sends.Load())
					})
				}
			}
		}
	}
}
