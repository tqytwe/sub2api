package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderPreservesCredentialRuntimeDependencies(t *testing.T) {
	t.Run("OAuth refresh history", func(t *testing.T) {
		repo := &accountTokenGuardV2TestRepo{}
		oauth := ProvideOpenAIOAuthService(nil, nil, nil, repo)
		t.Cleanup(oauth.Stop)
		require.Same(t, repo, oauth.refreshFailureRecorder)
	})
	t.Run("Excel gateway runtime", func(t *testing.T) {
		gateway := &OpenAIGatewayService{}
		excel := ProvideOpenAIExcelOAuthService(nil, nil, nil, nil, nil, gateway)
		require.Same(t, excel, gateway.excelOAuthService)
	})
}
