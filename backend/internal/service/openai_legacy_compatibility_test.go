package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAILegacyCompatibilityMasksRecoveryAndAllowsWSRetry(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	svc.cfg.Gateway.DisableEncryptedAgentOwnerBinding = true
	manager, c, _ := issue715WSManager(t, svc, context.Background())
	defer manager.abort()
	body := []byte(issue715OpaqueBody)
	strict := svc.WithOpenAIAgentMessageRecovery(context.Background(), c, body, "fixture-session", "gpt-6-astra")
	require.NotNil(t, openAIAgentRecoveryFromContext(strict))
	require.True(t, OpenAIAgentRecoveryNoReplay(strict))

	svc.cfg.Gateway.OpenAILegacyCompatibilityMode = true
	legacy := svc.WithOpenAIWSAgentRecovery(strict, c, body, "fixture-session", "gpt-6-astra")
	require.Nil(t, openAIAgentRecoveryFromContext(legacy), "must mask inherited strict recovery state")
	require.False(t, openAIAgentRecoveryBound(legacy))
	require.True(t, OpenAIWSAgentRecoveryCanRetry(legacy))
	require.Nil(t, newOpenAIWSAgentRecovery(svc, legacy, c))
	require.False(t, svc.preserveOpenAIAgentHistory(body))
	gotCtx, gotBody, changed := svc.TryOmitOpenAI409History(legacy, c, body, "fixture-session", "gpt-6-astra", nil)
	require.False(t, changed)
	require.Same(t, legacy, gotCtx)
	require.Equal(t, body, gotBody)

	svc.cfg.Gateway.OpenAILegacyCompatibilityMode = false
	strictAgain := svc.WithOpenAIAgentMessageRecovery(legacy, c, body, "fixture-session", "gpt-6-astra")
	require.NotNil(t, openAIAgentRecoveryFromContext(strictAgain))
	require.True(t, OpenAIAgentRecoveryNoReplay(strictAgain))
}
