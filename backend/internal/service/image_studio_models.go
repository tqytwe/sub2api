package service

import (
	"context"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

var (
	ErrImageStudioImageNotAllowed = infraerrors.BadRequest("IMAGE_STUDIO_IMAGE_NOT_ALLOWED", "image generation is not enabled for this group")
	ErrImageStudioNoImageModels   = infraerrors.BadRequest("IMAGE_STUDIO_NO_IMAGE_MODELS", "no image generation models are available for this API key group")
	ErrImageStudioModelNotAllowed = infraerrors.BadRequest("IMAGE_STUDIO_MODEL_NOT_ALLOWED", "selected image model is not available for this API key group")
)

var imageStudioModelPreference = []string{
	"gpt-image-2",
	"gpt-image-1.5",
	"gpt-image-1",
	"gemini-3.1-flash-image",
	"gemini-3.1-flash-image-preview",
	"gemini-2.5-flash-image",
	"grok-imagine-image-quality",
	"grok-imagine-image",
	"grok-imagine",
	"grok-imagine-edit",
}

type ImageStudioModelOption struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	ImageStudioModelCapabilities
}

type ImageStudioModelResolver interface {
	GetAvailableModels(ctx context.Context, groupID *int64, platform string) []string
}

type ImageStudioInputCostEstimator interface {
	EstimateImageStudioInputCost(
		ctx context.Context,
		model string,
		apiKey *APIKey,
		imageInputTokens int,
	) (float64, error)
}

type ImageStudioRateMultiplierResolver interface {
	ResolveUserGroupRateMultiplier(
		ctx context.Context,
		userID, groupID int64,
		groupDefaultMultiplier float64,
	) float64
}

func (s *ImageStudioService) ListModels(ctx context.Context, userID, apiKeyID int64) ([]ImageStudioModelOption, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrImageStudioDisabled
	}
	apiKey, err := s.resolveAPIKey(ctx, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	models, err := s.listImageModelsForAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	out := make([]ImageStudioModelOption, 0, len(models))
	for _, model := range models {
		caps := s.ResolveModelCapabilities(apiKey, model)
		out = append(out, ImageStudioModelOption{
			ID:                           model,
			DisplayName:                  imageStudioModelDisplayName(model),
			ImageStudioModelCapabilities: caps,
		})
	}
	return out, nil
}

func (s *ImageStudioService) resolveImageModel(ctx context.Context, apiKey *APIKey, requestedModel string) (string, error) {
	models, err := s.listImageModelsForAPIKey(ctx, apiKey)
	if err != nil {
		return "", err
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel != "" {
		for _, model := range models {
			if model == requestedModel {
				return model, nil
			}
		}
		return "", ErrImageStudioModelNotAllowed
	}
	if len(models) == 0 {
		return "", ErrImageStudioNoImageModels
	}
	return models[0], nil
}

func (s *ImageStudioService) listImageModelsForAPIKey(ctx context.Context, apiKey *APIKey) ([]string, error) {
	if apiKey == nil {
		return nil, ErrImageStudioAPIKey
	}
	if apiKey.Group != nil && !GroupAllowsImageGeneration(apiKey.Group) {
		return nil, ErrImageStudioImageNotAllowed
	}

	platform := PlatformOpenAI
	if apiKey.Group != nil && strings.TrimSpace(apiKey.Group.Platform) != "" {
		platform = apiKey.Group.Platform
	}

	candidates := defaultImageModelIDsForPlatform(platform)
	if s.gateway != nil && apiKey.GroupID != nil {
		if mapped := s.gateway.GetAvailableModels(ctx, apiKey.GroupID, platform); len(mapped) > 0 {
			candidates = mapped
		}
	}

	models := filterImageGenerationModels(candidates)
	if apiKey.Group != nil && apiKey.Group.CustomModelsListEnabled() {
		models = filterImageModelsByCustomList(models, apiKey.Group.ModelsListConfig.Models)
	}
	models = sortImageStudioModels(models)
	if len(models) == 0 {
		return nil, ErrImageStudioNoImageModels
	}
	return models, nil
}

func filterImageGenerationModels(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(models))
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := ResolveImageStudioModelCapability(model); !ok {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	return out
}

func filterImageModelsByCustomList(models, patterns []string) []string {
	if len(patterns) == 0 {
		return models
	}
	out := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		for _, pattern := range patterns {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			if pattern == model || (strings.HasSuffix(pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*"))) {
				if _, ok := seen[model]; ok {
					break
				}
				seen[model] = struct{}{}
				out = append(out, model)
				break
			}
		}
	}
	return out
}

func sortImageStudioModels(models []string) []string {
	if len(models) <= 1 {
		return models
	}
	rank := make(map[string]int, len(imageStudioModelPreference))
	for i, model := range imageStudioModelPreference {
		rank[model] = i
	}
	sort.SliceStable(models, func(i, j int) bool {
		left, leftOK := rank[models[i]]
		right, rightOK := rank[models[j]]
		switch {
		case leftOK && rightOK:
			return left < right
		case leftOK:
			return true
		case rightOK:
			return false
		default:
			return models[i] < models[j]
		}
	})
	return models
}

func defaultImageModelIDsForPlatform(platform string) []string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformGemini:
		return defaultGeminiImageModelIDs()
	case PlatformGrok:
		return defaultGrokImageModelIDs()
	default:
		return defaultOpenAIImageModelIDs()
	}
}

func defaultOpenAIImageModelIDs() []string {
	out := make([]string, 0, len(openai.DefaultModels))
	for _, model := range openai.DefaultModels {
		if isOpenAIImageGenerationModel(model.ID) {
			out = append(out, model.ID)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), imageStudioModelPreference...)
	}
	return sortImageStudioModels(out)
}

func defaultGeminiImageModelIDs() []string {
	out := make([]string, 0, len(geminicli.DefaultModels))
	for _, model := range geminicli.DefaultModels {
		if isImageGenerationModel(model.ID) {
			out = append(out, model.ID)
		}
	}
	return sortImageStudioModels(out)
}

func defaultGrokImageModelIDs() []string {
	defaults := xai.DefaultModels()
	out := make([]string, 0, len(defaults))
	for _, model := range defaults {
		if isGrokImageGenerationModel(model.ID) {
			out = append(out, model.ID)
		}
	}
	return sortImageStudioModels(out)
}

func imageStudioModelDisplayName(model string) string {
	model = strings.TrimSpace(model)
	for _, item := range openai.DefaultModels {
		if item.ID == model && strings.TrimSpace(item.DisplayName) != "" {
			return item.DisplayName
		}
	}
	for _, item := range geminicli.DefaultModels {
		if item.ID == model && strings.TrimSpace(item.DisplayName) != "" {
			return item.DisplayName
		}
	}
	for _, item := range xai.DefaultModels() {
		if item.ID == model && strings.TrimSpace(item.DisplayName) != "" {
			return item.DisplayName
		}
	}
	return model
}
