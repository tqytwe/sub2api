package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchAIHubMixPricing_ParsesCurrentResponseAndUnits(t *testing.T) {
	t.Setenv("AIHUBMIX_MODELS_URL", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	payload := `{
  "success": true,
  "message": "",
  "data": [{
    "model_id": "claude-sonnet-5",
    "model_name": "Claude Sonnet 5",
    "pricing": {"input": 2, "output": 10, "cache_read": 0.2, "cache_write": 2.5},
    "types": "llm"
  }]
}`
	require.NoError(t, os.WriteFile(path, []byte(payload), 0o600))
	t.Setenv("AIHUBMIX_MODELS_FILE", path)

	prices, err := fetchAiHubMixPricing()
	require.NoError(t, err)
	require.Len(t, prices, 1)
	price := prices[0]
	require.Equal(t, "claude-sonnet-5", price.ModelName)
	require.Equal(t, PlatformAnthropic, price.Platform)
	require.Equal(t, "aihubmix", price.Source)
	require.InDelta(t, 2.0/1_000_000, *price.InputPerToken, 1e-15)
	require.InDelta(t, 10.0/1_000_000, *price.OutputPerToken, 1e-15)
	require.InDelta(t, 0.2/1_000_000, *price.CacheReadToken, 1e-15)
	require.InDelta(t, 2.5/1_000_000, *price.CacheWriteToken, 1e-15)
	require.Equal(t, "Claude Sonnet 5", price.Raw["display_name"])
}

func TestSyncSourceName_DefaultsToAIHubMix(t *testing.T) {
	t.Setenv("MODEL_SYNC_SOURCE", "")
	require.Equal(t, "aihubmix", syncSourceName())
}
