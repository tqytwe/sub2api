//go:build integration

package repository

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/accountgroup"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Use the isolated PostgreSQL harness: the authoritative read starts its own
// read-only transaction and must observe committed updates, never a test mock.
func TestOpenAICompactAdmissionPostgresLatestModelLimits(t *testing.T) {
	ctx := context.Background()
	client := integrationEntClient
	group := mustCreateGroup(t, client, &service.Group{Name: "compact-admission-test", Platform: service.PlatformOpenAI, Status: service.StatusActive})
	a := mustCreateAccount(t, client, &service.Account{Name: "compact-admission-test", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Credentials: map[string]any{"model_mapping": map[string]any{"public-model": "gpt-5.4"}, "compact_model_mapping": map[string]any{"public-model": "gpt-5.5"}}})
	membership, err := client.AccountGroup.Create().SetAccountID(a.ID).SetGroupID(group.ID).SetAllowedModels([]string{"gpt-5.5"}).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = client.AccountGroup.Delete().Where(accountgroup.AccountIDEQ(a.ID), accountgroup.GroupIDEQ(group.ID)).Exec(ctx)
		_ = client.Account.DeleteOneID(a.ID).Exec(ctx)
		_ = client.Group.DeleteOneID(group.ID).Exec(ctx)
	})
	repo := NewAccountRepository(client, integrationDB, nil)
	selected, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	svc := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses/compact", nil)
	c.Set("api_key", &service.APIKey{GroupID: &group.ID})
	for _, resetAt := range []time.Time{time.Now().Add(time.Minute), time.Now().Add(-time.Second)} {
		err := client.Account.UpdateOneID(a.ID).SetExtra(map[string]any{"model_rate_limits": map[string]any{"gpt-5.5": map[string]any{"rate_limit_reset_at": resetAt.Format(time.RFC3339)}}}).Exec(ctx)
		require.NoError(t, err)
		admitted, err := svc.AdmitOpenAITurn(ctx, c, selected, "gpt-5.5")
		if resetAt.After(time.Now()) {
			var denied *service.OpenAITurnAdmissionError
			require.ErrorAs(t, err, &denied)
			require.Equal(t, "model_rate_limited", denied.Reason)
			require.Nil(t, admitted)
		} else {
			require.NoError(t, err)
			require.Equal(t, a.ID, admitted.ID)
		}
	}
	require.NoError(t, client.AccountGroup.UpdateOne(membership).SetAllowedModels([]string{"other-model"}).Exec(ctx))
	_, err = svc.AdmitOpenAITurn(ctx, c, selected, "gpt-5.5")
	var denied *service.OpenAITurnAdmissionError
	require.ErrorAs(t, err, &denied)
	require.Equal(t, "model_not_allowed_in_group", denied.Reason)
}

func TestOpenAIResponsesToolsAdmissionPostgresLatestProtocol(t *testing.T) {
	ctx, err := service.WithOpenAIResponsesToolRequirements(context.Background(), []byte(`{"tools":[{"type":"function","name":"lookup"}]}`))
	require.NoError(t, err)
	ctx = service.WithOpenAIForwardModel(ctx, "public-alias", false)
	client := integrationEntClient
	a := mustCreateAccount(t, client, &service.Account{Name: "responses-tools-admission-test", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Credentials: map[string]any{"model_mapping": map[string]any{"public-alias": "gpt-6.1-sol"}},
		Extra:       map[string]any{"openai_responses_mode": "force_responses"}})
	t.Cleanup(func() { _ = client.Account.DeleteOneID(a.ID).Exec(ctx) })
	repo := NewAccountRepository(client, integrationDB, nil)
	selected, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	svc := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	_, err = svc.AdmitOpenAITurn(ctx, nil, selected, "gpt-6.1-sol")
	require.NoError(t, err)
	require.NoError(t, client.Account.UpdateOneID(a.ID).SetExtra(map[string]any{"openai_responses_mode": "auto", "openai_responses_supported": false}).Exec(ctx))
	_, err = svc.AdmitOpenAITurn(ctx, nil, selected, "gpt-6.1-sol")
	var denied *service.OpenAITurnAdmissionError
	require.ErrorAs(t, err, &denied)
	require.Equal(t, "account_binding_changed", denied.Reason, "the stale route cannot pass authoritative admission")
	selected, err = repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	_, err = svc.AdmitOpenAITurn(ctx, nil, selected, "gpt-6.1-sol")
	require.ErrorAs(t, err, &denied)
	require.Equal(t, "responses_tools_protocol_mismatch", denied.Reason)
}
