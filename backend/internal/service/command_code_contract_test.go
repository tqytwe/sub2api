package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandCodeAccountContract(t *testing.T) {
	account := &Account{Platform: PlatformCommandCode, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	require.Equal(t, PlatformCommandCode, NormalizeOpenAICompatiblePlatform(account.Platform))
	require.True(t, account.IsOpenAICompatible())
	require.Equal(t, APIProtocolAdaptive, account.GetAPIProtocol())
	require.Equal(t, "https://api.commandcode.ai/provider/v1", account.GetOpenAIBaseURL())
	require.Equal(t, "https://api.commandcode.ai/provider", account.GetAnthropicProtocolBaseURL())
	require.Equal(t, "test", account.GetOpenAIProtocolAPIKey())
	require.Equal(t, APIProtocolAnthropic, resolveCommandCodeProtocol(account, APIProtocolResponses, "anthropic/claude-sonnet-4", nil))
	require.Equal(t, APIProtocolChatCompletions, resolveCommandCodeProtocol(account, APIProtocolChatCompletions, "openai/gpt-5", nil))
	require.Equal(t, APIProtocolResponses, resolveCommandCodeProtocol(account, APIProtocolResponses, "deepseek/deepseek-v4-flash", []string{APIProtocolChatCompletions, APIProtocolResponses}))
}

func commandCodeTestAccount(id int64) *Account {
	return &Account{ID: id, Platform: PlatformCommandCode, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "user_test_key"}}
}

func TestCommandCodeOfficialCatalogCacheIsolatesCredentials(t *testing.T) {
	const catalogURL = DefaultCommandCodeBaseURL + "/models"
	require.NotEqual(t, modelProtocolCatalogKey(commandCodeTestAccount(1), catalogURL), modelProtocolCatalogKey(commandCodeTestAccount(2), catalogURL))
	got, err := parseModelProtocolCatalog([]byte(`{"data":[{"id":"vendor/model","supported_endpoints":["/responses","/chat/completions"]}]}`))
	require.NoError(t, err)
	require.Equal(t, []string{APIProtocolResponses, APIProtocolChatCompletions}, got["vendor/model"])
}
