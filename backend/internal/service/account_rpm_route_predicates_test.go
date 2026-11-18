package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountModelScopedProtocolRoutePredicates(t *testing.T) {
	account := &Account{
		ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"model_mapping": map[string]any{"public-model": "mapped-model"}},
		Extra: map[string]any{
			"openai_excel_bps":            true,
			"openai_excel_bps_models":     []any{"mapped-model"},
			"openai_prism_browser":        true,
			"openai_prism_browser_models": []any{"mapped-model"},
		},
	}
	require.True(t, account.IsExcelBPSEnabledForModel("public-model"))
	require.False(t, account.IsExcelBPSEnabledForModel("other-model"))
	require.True(t, account.IsPrismBrowserEnabledForModel("public-model"))
	require.False(t, account.IsPrismBrowserEnabledForModel("other-model"))
	require.False(t, (&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: account.Extra}).IsExcelBPSEnabledForModel("public-model"))
}

func TestAccountCopilotSidecarPredicateIsTypeScoped(t *testing.T) {
	extra := map[string]any{"openai_copilot_sdk": true}
	require.True(t, (&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: extra}).IsCopilotSDKEnabled())
	require.False(t, (&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: extra}).IsCopilotSDKEnabled())
	require.False(t, (&Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, Extra: extra}).IsCopilotSDKEnabled())
}
