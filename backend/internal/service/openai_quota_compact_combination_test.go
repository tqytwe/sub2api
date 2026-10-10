package service

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A read-only quota refresh may update scheduler observations, but must neither
// invalidate a selected route nor clear an independent compact-model cooldown.
func TestCompactAdmissionAfterReadOnlyQuotaRefresh(t *testing.T) {
	for _, snapshot := range []struct{ name, body string }{
		{"complete", `{"rate_limit":{"primary_window":{"used_percent":37,"limit_window_seconds":18000},"secondary_window":{"used_percent":42,"limit_window_seconds":604800}}}`},
		{"missing", `{"rate_limit":{"primary_window":{},"secondary_window":null}}`},
	} {
		for _, persisted := range []bool{false, true} {
			for _, expired := range []bool{false, true} {
				name := snapshot.name + "/" + map[bool]string{false: "transient", true: "persisted"}[persisted] + "/" + map[bool]string{false: "active", true: "expired"}[expired]
				t.Run(name, func(t *testing.T) {
					usage, selected, reads, forbidden := newReadOnlyUsageTestService(t, http.StatusOK, snapshot.body)
					selected.Schedulable = true
					selected.Concurrency = 1
					selected.Credentials["model_mapping"] = map[string]any{"public-model": "gpt-5.4"}
					selected.Credentials["compact_model_mapping"] = map[string]any{"public-model": "gpt-5.5"}
					selected.Extra = map[string]any{"openai_compact_mode": "force_on", "codex_5h_used_percent": 12.0}
					reset := time.Now().Add(time.Hour)
					if expired {
						reset = time.Now().Add(-time.Hour)
					}
					if persisted {
						selected.Extra[modelRateLimitsKey] = map[string]any{"gpt-5.5": map[string]any{"rate_limit_reset_at": reset.Format(time.RFC3339)}}
					}
					svc := &OpenAIGatewayService{}
					if !persisted {
						failureTime := time.Now()
						if expired {
							failureTime = failureTime.Add(-time.Hour)
						}
						svc.recordOpenAIAccountModelTransientFailure(selected, "gpt-5.5", failureTime)
						svc.recordOpenAIAccountModelTransientFailure(selected, "gpt-5.5", failureTime)
					}
					originalExtra := maps.Clone(selected.Extra)
					_, err := usage.GetUsage(context.Background(), selected.ID, true)
					require.NoError(t, err)
					require.EqualValues(t, 1, reads.Load())
					require.Zero(t, forbidden.Load())
					require.Equal(t, originalExtra, selected.Extra, "quota reads must not mutate the selected snapshot")
					quotaRepo, ok := usage.accountRepo.(*readOnlyUsageRepo)
					require.True(t, ok)
					updates := quotaRepo.extraUpdates[selected.ID]
					require.NotContains(t, updates, modelRateLimitsKey)
					latest := *selected
					latest.Extra = maps.Clone(selected.Extra)
					// Match both the JSONB encoding and key merge at the database boundary.
					// Quota updates include a typed credits snapshot before persistence.
					encoded, err := json.Marshal(updates)
					require.NoError(t, err)
					var persistedUpdates map[string]any
					require.NoError(t, json.Unmarshal(encoded, &persistedUpdates))
					maps.Copy(latest.Extra, persistedUpdates)
					require.Equal(t, openAITurnRouteFingerprint(selected), openAITurnRouteFingerprint(&latest))
					if snapshot.name == "complete" {
						require.Equal(t, 37.0, latest.Extra["codex_5h_used_percent"])
					} else {
						require.Nil(t, latest.Extra["codex_5h_used_percent"])
					}
					svc.accountRepo = compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{latest}}}
					ctx := WithOpenAIForwardModel(context.Background(), "public-model", true)
					candidate, _, _ := svc.selectBestAccount(ctx, nil, PlatformOpenAI, []Account{latest}, "public-model", nil, true, "", false)
					require.Equal(t, !expired, candidate == nil, "selection must retain the compact cooldown after refreshing quota")
					admitted, err := svc.AdmitOpenAITurn(ctx, nil, selected, "gpt-5.5")
					if expired {
						require.NoError(t, err)
						require.NotNil(t, admitted)
					} else {
						var denial *OpenAITurnAdmissionError
						require.ErrorAs(t, err, &denial)
						require.Equal(t, map[bool]string{false: "model_runtime_blocked", true: "model_rate_limited"}[persisted], denial.Reason)
						require.Nil(t, admitted)
					}
				})
			}
		}
	}
}
