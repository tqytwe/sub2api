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
	"agnes-image-2.1-flash",
	"agnes-image-2.0-flash",
	"sensenova-u1.5-lite",
	"sensenova-u1-fast",
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
	// RequiresConfiguredPrice is internal policy metadata. It is intentionally
	// omitted from client responses because prices remain group-owned billing
	// configuration, not a model capability.
	RequiresConfiguredPrice bool `json:"-"`
	ImageStudioModelCapabilities
}

type ImageStudioModelResolver interface {
	GetAvailableModels(ctx context.Context, groupID *int64, platform string) []string
}

// ImageStudioCatalogContractResolution records whether an administrator has
// explicitly reviewed a mapped model's media declaration. A reviewed row that
// cannot produce an image has a nil capability and must not fall back to
// name-based inference.
type ImageStudioCatalogContractResolution struct {
	Declared     bool
	Capabilities *ImageStudioModelCapabilities
}

// ImageStudioCatalogContractResolver keeps Image Studio's user-facing model
// choices aligned with the server-owned catalog contract. The gateway remains
// responsible for schedulable-account and model-mapping discovery.
type ImageStudioCatalogContractResolver interface {
	ResolveImageStudioCatalogContracts(
		ctx context.Context,
		group Group,
		mappedModels []string,
	) (map[string]ImageStudioCatalogContractResolution, error)
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
	return s.listImageModelOptionsForAPIKey(ctx, apiKey)
}

func (s *ImageStudioService) resolveImageModel(ctx context.Context, apiKey *APIKey, requestedModel string) (string, error) {
	option, err := s.resolveImageModelOption(ctx, apiKey, requestedModel)
	if err != nil {
		return "", err
	}
	return option.ID, nil
}

func (s *ImageStudioService) resolveImageModelOption(
	ctx context.Context,
	apiKey *APIKey,
	requestedModel string,
) (ImageStudioModelOption, error) {
	options, err := s.listImageModelOptionsForAPIKey(ctx, apiKey)
	if err != nil {
		return ImageStudioModelOption{}, err
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel != "" {
		for _, option := range options {
			if option.ID == requestedModel {
				return option, nil
			}
		}
		return ImageStudioModelOption{}, ErrImageStudioModelNotAllowed
	}
	if len(options) == 0 {
		return ImageStudioModelOption{}, ErrImageStudioNoImageModels
	}
	return options[0], nil
}

func (s *ImageStudioService) listImageModelsForAPIKey(ctx context.Context, apiKey *APIKey) ([]string, error) {
	options, err := s.listImageModelOptionsForAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(options))
	for _, option := range options {
		models = append(models, option.ID)
	}
	return models, nil
}

func (s *ImageStudioService) listImageModelOptionsForAPIKey(ctx context.Context, apiKey *APIKey) ([]ImageStudioModelOption, error) {
	if apiKey == nil {
		return nil, ErrImageStudioAPIKey
	}
	if apiKey.Group != nil && !GroupAllowsImageGeneration(apiKey.Group) {
		return nil, ErrImageStudioImageNotAllowed
	}

	platform, err := imageStudioListingPlatformForAPIKey(apiKey)
	if err != nil {
		return nil, err
	}

	candidates := defaultImageModelIDsForPlatform(platform)
	if s.gateway != nil && apiKey.GroupID != nil {
		gatewayPlatform := platform
		if platform == PlatformComposite {
			// Composite groups are backed by concrete accounts. Requesting the
			// synthetic composite platform would filter those accounts out before
			// the catalog can apply the explicit openai_images contract.
			gatewayPlatform = ""
		}
		mapped := s.gateway.GetAvailableModels(ctx, apiKey.GroupID, gatewayPlatform)
		if len(mapped) == 0 {
			return nil, ErrImageStudioNoImageModels
		}
		candidates = mapped
	}

	if apiKey.Group != nil && apiKey.Group.ModelAllowlistEnabled() {
		candidates = apiKey.Group.ModelAllowlist.FilterForListing(candidates)
	}

	if s.catalog != nil && apiKey.Group != nil {
		resolutions, err := s.catalog.ResolveImageStudioCatalogContracts(ctx, *apiKey.Group, candidates)
		if err != nil {
			// A catalog read failure must not revive image capabilities from model
			// names. Returning the error keeps the managed surface fail-closed.
			return nil, err
		}
		return imageStudioCatalogModelOptions(platform, candidates, resolutions)
	}

	models := filterImageGenerationModelsForPlatform(platform, candidates)
	if len(models) == 0 {
		return nil, ErrImageStudioNoImageModels
	}
	models = sortImageStudioModels(models)
	options := make([]ImageStudioModelOption, 0, len(models))
	for _, model := range models {
		capabilities := s.ResolveModelCapabilities(apiKey, model)
		options = append(options, imageStudioModelOption(model, capabilities))
	}
	return options, nil
}

func imageStudioCatalogModelOptions(
	platform string,
	candidates []string,
	resolutions map[string]ImageStudioCatalogContractResolution,
) ([]ImageStudioModelOption, error) {
	options := make([]ImageStudioModelOption, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		model := strings.TrimSpace(candidate)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}

		resolution, reviewed := resolutions[key]
		if reviewed && resolution.Declared {
			if resolution.Capabilities == nil {
				continue
			}
			if !imageStudioCapabilityDispatchableForGroupPlatform(platform, *resolution.Capabilities) {
				continue
			}
			option := imageStudioModelOption(model, *resolution.Capabilities)
			option.RequiresConfiguredPrice = true
			options = append(options, option)
			continue
		}

		// Catalog migration compatibility is intentionally a short exact allow
		// list. New names require an explicit catalog declaration instead of a
		// substring match such as "*image*".
		capabilities, compatible := ResolveAuditedLegacyWorkspaceImageCapability(platform, model)
		if !compatible {
			continue
		}
		if !imageStudioCapabilityDispatchableForGroupPlatform(platform, capabilities) {
			continue
		}
		options = append(options, imageStudioModelOption(model, capabilities))
	}
	if len(options) == 0 {
		return nil, ErrImageStudioNoImageModels
	}
	sort.SliceStable(options, func(i, j int) bool {
		return imageStudioModelSortLess(options[i].ID, options[j].ID)
	})
	return options, nil
}

// imageStudioCapabilityDispatchableForGroupPlatform keeps selection and job
// dispatch aligned. A composite group may dispatch only catalog-reviewed
// OpenAI-compatible image capabilities. SenseNova's two exact U1 profiles use
// that same transport and are recognized by the composite resolver as OpenAI;
// other provider adapters remain hidden until they have a matching dispatch
// contract.
func imageStudioCapabilityDispatchableForGroupPlatform(
	groupPlatform string,
	capability ImageStudioModelCapabilities,
) bool {
	groupPlatform = strings.ToLower(strings.TrimSpace(groupPlatform))
	capabilityPlatform := strings.ToLower(strings.TrimSpace(capability.Platform))
	switch groupPlatform {
	case PlatformOpenAI:
		// Existing OpenAI-compatible account pools may expose Gemini image
		// models through /v1/images, so do not discard an already-resolved
		// capability merely because its native platform differs.
		return capabilityPlatform != ""
	case PlatformGemini, PlatformGrok:
		return capabilityPlatform == groupPlatform
	case PlatformComposite:
		if capabilityPlatform != PlatformOpenAI {
			return false
		}
		switch strings.ToLower(strings.TrimSpace(capability.ProviderID)) {
		case catalogOpenAIImagesAdapterID, imageModelAdapterSenseNovaID:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func imageStudioModelOption(model string, capabilities ImageStudioModelCapabilities) ImageStudioModelOption {
	return ImageStudioModelOption{
		ID:                           model,
		DisplayName:                  imageStudioModelDisplayName(model),
		ImageStudioModelCapabilities: capabilities,
	}
}

func imageStudioListingPlatformForAPIKey(apiKey *APIKey) (string, error) {
	if apiKey == nil || apiKey.Group == nil {
		return "", ErrImageStudioProviderNotSupported
	}
	platform := strings.ToLower(strings.TrimSpace(apiKey.Group.Platform))
	if platform == "" {
		return "", ErrImageStudioProviderNotSupported
	}
	switch platform {
	case PlatformOpenAI, PlatformGemini, PlatformGrok, PlatformComposite:
		return platform, nil
	default:
		return "", ErrImageStudioProviderNotSupported
	}
}

func filterImageGenerationModelsForPlatform(platform string, models []string) []string {
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
		if !imageStudioModelAllowedForPlatform(platform, model) {
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

func imageStudioModelAllowedForPlatform(platform, model string) bool {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		_, ok := ResolveImageStudioModelCapability(model)
		return ok
	}
	_, ok := ResolveImageStudioProviderCapability(platform, model)
	return ok
}

func filterImageModelsByCustomList(models, patterns []string) []string {
	if len(patterns) == 0 {
		return nil
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
	sort.SliceStable(models, func(i, j int) bool {
		return imageStudioModelSortLess(models[i], models[j])
	})
	return models
}

func imageStudioModelSortLess(left, right string) bool {
	rank := make(map[string]int, len(imageStudioModelPreference))
	for i, model := range imageStudioModelPreference {
		rank[model] = i
	}
	leftRank, leftKnown := rank[left]
	rightRank, rightKnown := rank[right]
	switch {
	case leftKnown && rightKnown:
		return leftRank < rightRank
	case leftKnown:
		return true
	case rightKnown:
		return false
	default:
		return left < right
	}
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
	switch model {
	case senseNovaU15LiteModelID:
		return "SenseNova U1.5 Lite"
	case senseNovaU1FastModelID:
		return "SenseNova U1 Fast"
	}
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
