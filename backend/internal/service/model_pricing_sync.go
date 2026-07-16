package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ModelPricingSyncRunner executes async pricing sync jobs.
type ModelPricingSyncRunner struct {
	svc *ModelCatalogService
}

func NewModelPricingSyncRunner(svc *ModelCatalogService) *ModelPricingSyncRunner {
	return &ModelPricingSyncRunner{svc: svc}
}

// ExternalModelPrice is normalized pricing from an online source (per token USD).
type ExternalModelPrice struct {
	ModelName       string
	Platform        string
	InputPerToken   *float64
	OutputPerToken  *float64
	CacheReadToken  *float64
	CacheWriteToken *float64
	Source          string
	Raw             map[string]any
}

// Run executes sync for a job ID.
func (r *ModelPricingSyncRunner) Run(ctx context.Context, jobID string) {
	job, err := r.svc.GetSyncJob(ctx, jobID)
	if err != nil || job == nil {
		return
	}
	result := ModelSyncResult{Source: syncSourceName()}
	warnings := make([]string, 0)

	prices, fetchErr := fetchExternalPricing()
	if fetchErr != nil {
		warnings = append(warnings, fetchErr.Error())
		result.Warnings = warnings
		now := time.Now()
		job.Status = "failed"
		job.Error = fetchErr.Error()
		job.CompletedAt = &now
		job.Result = map[string]any{
			"updated":    result.Updated,
			"discovered": result.Discovered,
			"retired":    result.Retired,
			"warnings":   result.Warnings,
			"source":     result.Source,
		}
		_ = r.svc.SyncComplete(ctx, job)
		return
	}

	knownKeys, _ := r.svc.CatalogKeys(ctx)
	syncedAt := time.Now()
	for _, ext := range prices {
		key := catalogSyncKey(ext.ModelName, ext.Platform)
		if _, exists := knownKeys[key]; exists {
			updated, err := r.svc.Repo().UpdateCatalogOfficialPrices(
				ctx, ext.ModelName, ext.Platform, ext.Source,
				ext.InputPerToken, ext.OutputPerToken, ext.CacheReadToken, ext.CacheWriteToken, syncedAt,
			)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %v", ext.ModelName, err))
				continue
			}
			result.Updated += updated
			continue
		}
		payload := ext.Raw
		if payload == nil {
			payload = map[string]any{
				"input_price":  ext.InputPerToken,
				"output_price": ext.OutputPerToken,
			}
		}
		if err := r.svc.UpsertDiscovery(ctx, &ModelDiscovery{
			ModelName: ext.ModelName,
			Platform:  ext.Platform,
			Source:    ext.Source,
			Payload:   payload,
			Status:    "new",
		}); err == nil {
			result.Discovered++
		}
	}

	result.Warnings = warnings
	now := time.Now()
	job.Status = "succeeded"
	job.CompletedAt = &now
	job.Result = map[string]any{
		"updated":    result.Updated,
		"discovered": result.Discovered,
		"retired":    result.Retired,
		"warnings":   result.Warnings,
		"source":     result.Source,
	}
	_ = r.svc.SyncComplete(ctx, job)
}

func syncSourceName() string {
	src := strings.ToLower(strings.TrimSpace(os.Getenv("MODEL_SYNC_SOURCE")))
	if src == "" {
		return "aihubmix"
	}
	return src
}

func fetchExternalPricing() ([]ExternalModelPrice, error) {
	switch syncSourceName() {
	case "aihubmix":
		return fetchAiHubMixPricing()
	default:
		return nil, fmt.Errorf("unsupported model pricing source %q; expected aihubmix", syncSourceName())
	}
}

func fetchAiHubMixPricing() ([]ExternalModelPrice, error) {
	filePath := strings.TrimSpace(os.Getenv("AIHUBMIX_MODELS_FILE"))
	url := strings.TrimSpace(os.Getenv("AIHUBMIX_MODELS_URL"))
	if url == "" {
		url = "https://aihubmix.com/api/v1/models"
	}

	var body []byte
	var err error
	if filePath != "" {
		body, err = os.ReadFile(filePath)
	} else {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if reqErr != nil {
			return nil, fmt.Errorf("create AIHubMix request: %w", reqErr)
		}
		if token := strings.TrimSpace(os.Getenv("AIHUBMIX_API_KEY")); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		client := &http.Client{Timeout: 20 * time.Second}
		resp, reqErr := client.Do(req) //nolint:gosec // admin-configured URL
		if reqErr != nil {
			return nil, fmt.Errorf("request AIHubMix models: %w", reqErr)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			preview, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return nil, fmt.Errorf("AIHubMix models returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(preview)))
		}
		body, err = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	}
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data []struct {
			ModelID   string `json:"model_id"`
			ModelName string `json:"model_name"`
			Pricing   struct {
				Input      *float64 `json:"input"`
				Output     *float64 `json:"output"`
				CacheRead  *float64 `json:"cache_read"`
				CacheWrite *float64 `json:"cache_write"`
			} `json:"pricing"`
			Types string `json:"types"`
		} `json:"data"`
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := make([]ExternalModelPrice, 0, len(payload.Data))
	for _, m := range payload.Data {
		if strings.TrimSpace(m.ModelID) == "" {
			continue
		}
		platform := inferPlatformFromModel(m.ModelID)
		inTok := perMillionToToken(m.Pricing.Input)
		outTok := perMillionToToken(m.Pricing.Output)
		cacheReadTok := perMillionToToken(m.Pricing.CacheRead)
		cacheWriteTok := perMillionToToken(m.Pricing.CacheWrite)
		out = append(out, ExternalModelPrice{
			ModelName:       m.ModelID,
			Platform:        platform,
			InputPerToken:   inTok,
			OutputPerToken:  outTok,
			CacheReadToken:  cacheReadTok,
			CacheWriteToken: cacheWriteTok,
			Source:          "aihubmix",
			Raw: map[string]any{
				"input_price":       ptrValue(inTok),
				"output_price":      ptrValue(outTok),
				"cache_read_price":  ptrValue(cacheReadTok),
				"cache_write_price": ptrValue(cacheWriteTok),
				"display_name":      m.ModelName,
				"types":             m.Types,
			},
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("AIHubMix returned no usable models (success=%s, message=%s)", strconv.FormatBool(payload.Success), payload.Message)
	}
	return out, nil
}

func perMillionToToken(v *float64) *float64 {
	if v == nil {
		return nil
	}
	value := *v / 1_000_000
	return &value
}

func ptrValue(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func catalogSyncKey(modelName, platform string) string {
	return strings.ToLower(strings.TrimSpace(modelName)) + "::" + strings.ToLower(strings.TrimSpace(platform))
}

func inferPlatformFromModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "claude"):
		return PlatformAnthropic
	case strings.HasPrefix(m, "gemini"):
		return PlatformGemini
	case strings.HasPrefix(m, "grok"):
		return PlatformGrok
	default:
		return PlatformOpenAI
	}
}
