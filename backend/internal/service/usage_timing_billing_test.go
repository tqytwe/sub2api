package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requesttiming"
	"github.com/stretchr/testify/require"
)

type timingBillingUsageRepo struct {
	*recoveryUsageLogRepo
	calls     int
	requestID string
	apiKeyID  int64
}

func (r *timingBillingUsageRepo) RecordRequestTiming(_ context.Context, requestID string, apiKeyID int64) {
	r.calls++
	r.requestID, r.apiKeyID = requestID, apiKeyID
}

func TestUsageTimingRegisteredOnceAfterBilling(t *testing.T) {
	repo := &timingBillingUsageRepo{recoveryUsageLogRepo: &recoveryUsageLogRepo{}}
	billing := &recoveryBillingRepo{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(repo, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	_, input := newRecoveryUsageFixture(repo.recoveryUsageLogRepo, billing)
	require.NoError(t, svc.RecordUsage(context.Background(), input))
	require.Equal(t, 1, repo.calls, "diagnostic registration must not run twice through the legacy billing path")
	require.Equal(t, repo.row.RequestID, repo.requestID)
	require.Equal(t, input.APIKey.ID, repo.apiKeyID)
	require.Equal(t, 1, billing.debits)
}

func TestUsageTimingPluginRouteDoesNotInventPhysicalHTTP(t *testing.T) {
	manager := &PluginManager{}
	manager.route.Store(&pluginRoute{pluginID: 1, rolloutPercent: 100, unavailable: "synthetic unavailable"})
	svc := &OpenAIGatewayService{pluginManager: manager}
	collector := requesttiming.New(time.Now(), 0)
	request := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	request = request.WithContext(requesttiming.With(request.Context(), collector))
	response, err := svc.doOpenAIUpstream(request, "", &Account{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth})
	require.Error(t, err)
	require.Nil(t, response)
	var snapshot requesttiming.Snapshot
	collector.WhenFinished(func(s requesttiming.Snapshot) { snapshot = s })
	collector.Finish(http.StatusBadGateway, false)
	require.Len(t, snapshot.Attempts, 1)
	require.Equal(t, "egress", snapshot.Attempts[0].Kind)
	require.Equal(t, "plugin", snapshot.Attempts[0].Route.Upstream)
	require.NotContains(t, snapshot.Attempts[0].Events, "connection_ready")
}
