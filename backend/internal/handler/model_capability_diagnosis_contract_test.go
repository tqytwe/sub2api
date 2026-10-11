//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
	"time"
)

type capabilityDiagnosisRepo struct {
	service.AccountRepository
	accounts []service.Account
	err      error
}

func (r capabilityDiagnosisRepo) ListModelAvailabilityCandidates(_ context.Context, _ *int64, _ []string, _ bool) ([]service.Account, error) {
	return r.accounts, r.err
}

// Real service and existing handler classification over isolated HTTP, after failed selection.
func TestModelCapabilityDiagnosisHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, reason                        string
		expired, peer, readError, malformed bool
		status                              int
	}{
		{name: "explicit missing", reason: "upstream_404_model_not_found", status: 404},
		{name: "401 missing", reason: "upstream_401_model_not_found", status: 404},
		{name: "plan unsupported", reason: "upstream_400_codex_plan_gated_model", status: 404},
		{name: "ordinary capacity", reason: "upstream_429", status: 503},
		{name: "unknown", reason: "future_reason", status: 503},
		{name: "missing reason", status: 503},
		{name: "expired", reason: "upstream_404_model_not_found", expired: true, status: 503},
		{name: "peer supports", reason: "upstream_404_model_not_found", peer: true, status: 503},
		{name: "read failure", reason: "upstream_404_model_not_found", readError: true, status: 503},
		{name: "malformed expiry", reason: "upstream_404_model_not_found", malformed: true, status: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			until := time.Now().Add(time.Hour)
			if tc.expired {
				until = time.Now().Add(-time.Hour)
			}
			stamp := until.UTC().Format(time.RFC3339)
			if tc.malformed {
				stamp = "invalid"
			}
			a := service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"public-model": "provider-model"}}, Extra: map[string]any{"model_rate_limits": map[string]any{"provider-model": map[string]any{"rate_limit_reset_at": stamp, "reason": tc.reason}}}}
			repo := capabilityDiagnosisRepo{accounts: []service.Account{a}}
			if tc.peer {
				repo.accounts = append(repo.accounts, service.Account{ID: 2, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true})
			}
			if tc.readError {
				repo.err = errors.New("synthetic lookup failure")
			}
			svc := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			group := int64(7)
			key := &service.APIKey{GroupID: &group}
			router := gin.New()
			router.POST("/diagnose", func(c *gin.Context) {
				cls := classifyNoAccountErrorFromGin(c, svc, key, "public-model", "display-model", service.PlatformOpenAI)
				c.Header("X-Test-Business-Limited", fmt.Sprint(service.HasOpsClientBusinessLimited(c)))
				if c.Query("capacity") == "1" {
					cls = classifySelectionFailureError(fmt.Errorf("model_rate_limited=1"), cls)
				}
				c.JSON(cls.Status, gin.H{"error": gin.H{"type": cls.ErrType, "message": cls.Message}})
			})
			server := httptest.NewServer(router)
			defer server.Close()
			client := server.Client()
			client.Timeout = 3 * time.Second
			res, err := client.Post(server.URL+"/diagnose", "application/json", nil)
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, tc.status, res.StatusCode)
			require.Equal(t, fmt.Sprint(tc.status == 404), res.Header.Get("X-Test-Business-Limited"))
			capacity, err := client.Post(server.URL+"/diagnose?capacity=1", "application/json", nil)
			require.NoError(t, err)
			defer capacity.Body.Close()
			wantCapacity := 429
			if tc.status == 404 {
				wantCapacity = 404
			}
			require.Equal(t, wantCapacity, capacity.StatusCode)
			var p struct {
				Error struct{ Type, Message string }
			}
			require.NoError(t, json.NewDecoder(res.Body).Decode(&p))
			if tc.status == 404 {
				require.Equal(t, "model_not_found", p.Error.Type)
				require.Contains(t, p.Error.Message, "display-model")
				require.NotContains(t, p.Error.Message, "provider-model")
			} else {
				require.Equal(t, "api_error", p.Error.Type)
			}
		})
	}
}

func TestModelCapabilityDiagnosisForwardContextHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, forward, compactTarget, blocked string
		compact                               bool
		status                                int
	}{
		{"channel public rejected", "provider-model", "", "public-model", false, 503},
		{"channel provider rejected", "provider-model", "", "provider-model", false, 404},
		{"legacy public rejected", "public-model", "compact-model", "public-model", true, 503},
		{"legacy compact rejected", "public-model", "compact-model", "compact-model", true, 404},
		{"native v2 ignores legacy mapping", "public-model", "compact-model", "compact-model", false, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			until := time.Now().Add(time.Hour)
			a := service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, OverloadUntil: &until, Credentials: map[string]any{}, Extra: map[string]any{"openai_compact_mode": "force_on", "model_rate_limits": map[string]any{tc.blocked: map[string]any{"reason": "upstream_404_model_not_found", "rate_limit_reset_at": until.Format(time.RFC3339)}}}}
			if tc.compactTarget != "" {
				a.Credentials["compact_model_mapping"] = map[string]any{tc.forward: tc.compactTarget}
			}
			svc := service.NewOpenAIGatewayService(capabilityDiagnosisRepo{accounts: []service.Account{a}}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			group := int64(7)
			key := &service.APIKey{GroupID: &group}
			r := gin.New()
			r.POST("/diagnose", func(c *gin.Context) {
				c.Request = c.Request.WithContext(service.WithOpenAIForwardModel(c.Request.Context(), tc.forward, tc.compact))
				cls := classifyNoAccountErrorFromGin(c, svc, key, "public-model", "public-model", service.PlatformOpenAI)
				c.Header("X-Test-Business-Limited", fmt.Sprint(service.HasOpsClientBusinessLimited(c)))
				c.JSON(cls.Status, gin.H{"error": gin.H{"type": cls.ErrType}})
			})
			server := httptest.NewServer(r)
			defer server.Close()
			client := server.Client()
			client.Timeout = 3 * time.Second
			res, err := client.Post(server.URL+"/diagnose", "application/json", nil)
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, tc.status, res.StatusCode)
			require.Equal(t, fmt.Sprint(tc.status == 404), res.Header.Get("X-Test-Business-Limited"))
		})
	}
}
