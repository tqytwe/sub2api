package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	InputPerToken   float64
	OutputPerToken  float64
	CacheReadToken  float64
	CacheWriteToken float64
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

	prices, fetchErr := fetchExternalPricing(r.svc.PricingService())
	if fetchErr != nil {
		warnings = append(warnings, fetchErr.Error())
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
		return
	}

	knownKeys, _ := r.svc.CatalogKeys(ctx)
	channelSvc := r.svc.ChannelService()
	if channelSvc == nil {
		warnings = append(warnings, "channel service unavailable")
	} else {
		entries, err := channelSvc.ListAllModelPricingEntries(ctx)
		if err != nil {
			warnings = append(warnings, err.Error())
		} else {
			for _, entry := range entries {
				for _, modelName := range entry.Models {
					ext, ok := matchExternalPrice(prices, modelName, entry.Platform)
					if !ok {
						continue
					}
					changed := false
					if ext.InputPerToken > 0 {
						v := ext.InputPerToken
						entry.InputPrice = &v
						changed = true
					}
					if ext.OutputPerToken > 0 {
						v := ext.OutputPerToken
						entry.OutputPrice = &v
						changed = true
					}
					if ext.CacheReadToken > 0 {
						v := ext.CacheReadToken
						entry.CacheReadPrice = &v
						changed = true
					}
					if ext.CacheWriteToken > 0 {
						v := ext.CacheWriteToken
						entry.CacheWritePrice = &v
						changed = true
					}
					if changed {
						if err := channelSvc.UpdateModelPricingPrices(ctx, &entry); err == nil {
							result.Updated++
						}
					}
					key := catalogSyncKey(modelName, entry.Platform)
					delete(knownKeys, key)
				}
			}
		}
	}

	for _, ext := range prices {
		key := catalogSyncKey(ext.ModelName, ext.Platform)
		if _, exists := knownKeys[key]; exists {
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
		return "litellm"
	}
	return src
}

func fetchExternalPricing(pricingSvc *PricingService) ([]ExternalModelPrice, error) {
	switch syncSourceName() {
	case "aihubmix":
		return fetchAiHubMixPricing()
	default:
		return fetchLiteLLMPricing(pricingSvc)
	}
}

func fetchLiteLLMPricing(pricingSvc *PricingService) ([]ExternalModelPrice, error) {
	if pricingSvc == nil {
		return nil, fmt.Errorf("pricing service unavailable")
	}
	raw := pricingSvc.ListAllModelPricing()
	out := make([]ExternalModelPrice, 0, len(raw))
	for name, p := range raw {
		if p == nil {
			continue
		}
		platform := p.LiteLLMProvider
		if platform == "" {
			platform = inferPlatformFromModel(name)
		}
		out = append(out, ExternalModelPrice{
			ModelName:       name,
			Platform:        platform,
			InputPerToken:   p.InputCostPerToken,
			OutputPerToken:  p.OutputCostPerToken,
			CacheReadToken:  p.CacheReadInputTokenCost,
			CacheWriteToken: p.CacheCreationInputTokenCost,
			Source:          "litellm",
			Raw: map[string]any{
				"input_price":  p.InputCostPerToken,
				"output_price": p.OutputCostPerToken,
				"mode":         p.Mode,
			},
		})
	}
	return out, nil
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
		resp, reqErr := http.Get(url) //nolint:gosec // admin-configured URL
		if reqErr != nil {
			return nil, reqErr
		}
		defer func() { _ = resp.Body.Close() }()
		body, err = io.ReadAll(resp.Body)
	}
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			Pricing struct {
				InputPerMillion  float64 `json:"input_per_million"`
				OutputPerMillion float64 `json:"output_per_million"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := make([]ExternalModelPrice, 0, len(payload.Data))
	for _, m := range payload.Data {
		if m.ID == "" {
			continue
		}
		platform := strings.ToLower(strings.TrimSpace(m.OwnedBy))
		if platform == "" {
			platform = inferPlatformFromModel(m.ID)
		}
		inTok := m.Pricing.InputPerMillion / 1_000_000
		outTok := m.Pricing.OutputPerMillion / 1_000_000
		out = append(out, ExternalModelPrice{
			ModelName:      m.ID,
			Platform:       platform,
			InputPerToken:  inTok,
			OutputPerToken: outTok,
			Source:         "aihubmix",
			Raw: map[string]any{
				"input_price":  inTok,
				"output_price": outTok,
			},
		})
	}
	return out, nil
}

func matchExternalPrice(prices []ExternalModelPrice, modelName, platform string) (ExternalModelPrice, bool) {
	modelLower := strings.ToLower(modelName)
	platformLower := strings.ToLower(platform)
	for _, p := range prices {
		if strings.ToLower(p.ModelName) == modelLower {
			if platformLower == "" || strings.ToLower(p.Platform) == platformLower {
				return p, true
			}
		}
	}
	for _, p := range prices {
		if strings.ToLower(p.ModelName) == modelLower {
			return p, true
		}
	}
	return ExternalModelPrice{}, false
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
