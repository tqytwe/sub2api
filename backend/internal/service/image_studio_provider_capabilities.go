package service

import (
	"fmt"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrImageStudioOperationNotSupported = infraerrors.BadRequest(
		"IMAGE_STUDIO_OPERATION_NOT_SUPPORTED",
		"image operation is not supported for the selected model",
	)
	ErrImageStudioBackgroundNotSupported = infraerrors.BadRequest(
		"IMAGE_STUDIO_BACKGROUND_NOT_SUPPORTED",
		"image background is not supported for the selected model",
	)
	ErrImageStudioOutputFormatNotSupported = infraerrors.BadRequest(
		"IMAGE_STUDIO_OUTPUT_FORMAT_NOT_SUPPORTED",
		"image output format is not supported for the selected model",
	)
	ErrImageStudioOutputCompressionNotSupported = infraerrors.BadRequest(
		"IMAGE_STUDIO_OUTPUT_COMPRESSION_NOT_SUPPORTED",
		"image output compression is not supported for the selected model and format",
	)
	ErrImageStudioInputFidelityNotSupported = infraerrors.BadRequest(
		"IMAGE_STUDIO_INPUT_FIDELITY_NOT_SUPPORTED",
		"image input fidelity is not supported for the selected model and operation",
	)
	ErrImageStudioStyleNotSupported = infraerrors.BadRequest(
		"IMAGE_STUDIO_STYLE_NOT_SUPPORTED",
		"image style is not supported for the selected model",
	)
)

type ImageStudioOutputCompressionCapability struct {
	Min     int      `json:"min"`
	Max     int      `json:"max"`
	Formats []string `json:"formats"`
}

type ImageStudioModelCapabilities struct {
	Platform                 string                                  `json:"platform,omitempty"`
	ProfileID                string                                  `json:"capability_profile_id,omitempty"`
	Revision                 string                                  `json:"capability_revision,omitempty"`
	Operations               []string                                `json:"operations,omitempty"`
	SizingKind               string                                  `json:"sizing_kind,omitempty"`
	SupportedSizes           []string                                `json:"supported_sizes,omitempty"`
	SupportedAspectRatios    []string                                `json:"supported_aspect_ratios,omitempty"`
	SupportedResolutions     []string                                `json:"supported_resolutions,omitempty"`
	SupportedQualities       []string                                `json:"supported_qualities,omitempty"`
	SupportedBackgrounds     []string                                `json:"supported_backgrounds,omitempty"`
	SupportedOutputFormats   []string                                `json:"supported_output_formats,omitempty"`
	SupportedInputFidelities []string                                `json:"supported_input_fidelities,omitempty"`
	InputFidelityMode        string                                  `json:"input_fidelity_mode,omitempty"`
	SupportsTransparency     bool                                    `json:"supports_transparency"`
	OutputCompression        *ImageStudioOutputCompressionCapability `json:"output_compression,omitempty"`
	MaxReferenceImages       int                                     `json:"max_reference_images,omitempty"`
	DefaultSize              string                                  `json:"default_size,omitempty"`
	DefaultAspectRatio       string                                  `json:"default_aspect_ratio,omitempty"`
	DefaultResolution        string                                  `json:"default_resolution,omitempty"`
	DefaultQuality           string                                  `json:"default_quality,omitempty"`
	DefaultBackground        string                                  `json:"default_background,omitempty"`
	DefaultOutputFormat      string                                  `json:"default_output_format,omitempty"`
	DefaultInputFidelity     string                                  `json:"default_input_fidelity,omitempty"`
}

const imageStudioCapabilityRevision = "2026-07-16.1"

const imageStudioOpenAICompatibleProfile = "openai_compatible"

func ResolveImageStudioModelCapability(model string) (ImageStudioModelCapabilities, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return ImageStudioModelCapabilities{}, false
	}
	if capability, ok := resolveOpenAIImageStudioCapability(model); ok {
		return capability, true
	}
	if capability, ok := resolveGeminiImageStudioCapability(model); ok {
		return capability, true
	}
	if capability, ok := resolveGrokImageStudioCapability(model); ok {
		return capability, true
	}
	if !isGenericOpenAICompatibleImageModel(model) {
		return ImageStudioModelCapabilities{}, false
	}
	return resolveGenericOpenAICompatibleImageStudioCapability(model), true
}

func ResolveImageStudioProviderCapability(platform, model string) (ImageStudioModelCapabilities, bool) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	model = strings.ToLower(strings.TrimSpace(model))
	switch platform {
	case PlatformOpenAI:
		return resolveOpenAIImageStudioCapability(model)
	case PlatformGemini:
		return resolveGeminiImageStudioCapability(model)
	case PlatformGrok:
		return resolveGrokImageStudioCapability(model)
	default:
		return ImageStudioModelCapabilities{}, false
	}
}

func resolveOpenAIImageStudioCapability(model string) (ImageStudioModelCapabilities, bool) {
	if !IsGPTImageGenerationModel(model) {
		return ImageStudioModelCapabilities{}, false
	}
	capabilityModel := model
	if strings.HasPrefix(model, "gpt-image-2-") {
		capabilityModel = "gpt-image-2"
	}
	capability := ImageStudioModelCapabilities{
		Platform:               PlatformOpenAI,
		ProfileID:              PlatformOpenAI + ":" + model + ":v1",
		Revision:               imageStudioCapabilityRevision,
		Operations:             []string{"create", "edit"},
		SupportedQualities:     []string{"auto", "low", "medium", "high"},
		SupportedOutputFormats: []string{"png", "jpeg", "webp"},
		OutputCompression: &ImageStudioOutputCompressionCapability{
			Min:     0,
			Max:     100,
			Formats: []string{"jpeg", "webp"},
		},
		MaxReferenceImages:  4,
		DefaultSize:         defaultImageStudioSize,
		DefaultQuality:      "auto",
		DefaultBackground:   "auto",
		DefaultOutputFormat: "png",
	}
	switch capabilityModel {
	case "gpt-image-2":
		capability.SizingKind = "custom"
		capability.SupportedSizes = validGPTImage2StudioSizes()
		capability.SupportedBackgrounds = []string{"auto", "opaque"}
		capability.SupportedInputFidelities = []string{"high"}
		capability.InputFidelityMode = "fixed"
		capability.DefaultInputFidelity = "high"
		capability.SupportsTransparency = false
	default:
		capability.SizingKind = "fixed"
		capability.SupportedSizes = []string{"1024x1024", "1536x1024", "1024x1536"}
		capability.SupportedBackgrounds = []string{"auto", "opaque", "transparent"}
		capability.SupportedInputFidelities = []string{"low", "high"}
		capability.InputFidelityMode = "selectable"
		capability.DefaultInputFidelity = "low"
		capability.SupportsTransparency = true
	}
	return capability, true
}

func resolveGeminiImageStudioCapability(model string) (ImageStudioModelCapabilities, bool) {
	if !isGeminiImageStudioModel(model) {
		return ImageStudioModelCapabilities{}, false
	}
	return ImageStudioModelCapabilities{
		Platform:               PlatformGemini,
		ProfileID:              PlatformGemini + ":" + model + ":v1",
		Revision:               imageStudioCapabilityRevision,
		Operations:             []string{"create", "edit"},
		SizingKind:             "aspect_resolution",
		SupportedSizes:         imageStudioSizesThroughTier(ImageBillingSize2K),
		SupportedAspectRatios:  []string{"1:1", "2:3", "3:2", "9:16", "16:9"},
		SupportedResolutions:   []string{"1k", "2k"},
		SupportedOutputFormats: []string{"png"},
		MaxReferenceImages:     4,
		DefaultSize:            defaultImageStudioSize,
		DefaultAspectRatio:     "1:1",
		DefaultResolution:      "1k",
		DefaultOutputFormat:    "png",
	}, true
}

func isGeminiImageStudioModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	normalized := strings.TrimPrefix(model, "models/")
	return isImageGenerationModel(model) ||
		strings.HasPrefix(normalized, "imagen-") ||
		strings.HasPrefix(normalized, "imagen_")
}

func resolveGrokImageStudioCapability(model string) (ImageStudioModelCapabilities, bool) {
	if !isGrokImageGenerationModel(model) {
		return ImageStudioModelCapabilities{}, false
	}
	return ImageStudioModelCapabilities{
		Platform:               PlatformGrok,
		ProfileID:              PlatformGrok + ":" + model + ":v1",
		Revision:               imageStudioCapabilityRevision,
		Operations:             []string{"create", "edit"},
		SizingKind:             "aspect_resolution",
		SupportedSizes:         imageStudioSizesThroughTier(ImageBillingSize2K),
		SupportedAspectRatios:  []string{"1:1", "2:3", "3:2", "9:16", "16:9"},
		SupportedResolutions:   []string{"1k", "2k"},
		SupportedQualities:     []string{"standard"},
		SupportedOutputFormats: []string{"jpeg"},
		MaxReferenceImages:     3,
		DefaultSize:            defaultImageStudioSize,
		DefaultAspectRatio:     "1:1",
		DefaultResolution:      "1k",
		DefaultQuality:         "standard",
		DefaultOutputFormat:    "jpeg",
	}, true
}

func isGenericOpenAICompatibleImageModel(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	normalized = strings.TrimPrefix(normalized, "models/")
	if normalized == "" ||
		strings.Contains(normalized, "embedding") ||
		strings.Contains(normalized, "moderation") ||
		strings.Contains(normalized, "rerank") {
		return false
	}
	if strings.Contains(normalized, "image") {
		return true
	}
	for _, marker := range []string{
		"dall-e",
		"stable-diffusion",
		"sdxl",
		"flux",
		"recraft",
		"midjourney",
	} {
		if strings.HasPrefix(normalized, marker) || strings.Contains(normalized, "-"+marker) {
			return true
		}
	}
	return false
}

func resolveGenericOpenAICompatibleImageStudioCapability(model string) ImageStudioModelCapabilities {
	return ImageStudioModelCapabilities{
		Platform:               PlatformOpenAI,
		ProfileID:              imageStudioOpenAICompatibleProfile + ":" + model + ":v1",
		Revision:               imageStudioCapabilityRevision,
		Operations:             []string{"create", "edit"},
		SizingKind:             "custom",
		SupportedSizes:         allImageStudioCatalogSizes(),
		SupportedOutputFormats: []string{"png", "jpeg", "webp"},
		MaxReferenceImages:     4,
		DefaultSize:            defaultImageStudioSize,
		DefaultOutputFormat:    "png",
	}
}

func validGPTImage2StudioSizes() []string {
	all := allImageStudioCatalogSizes()
	out := make([]string, 0, len(all))
	for _, size := range all {
		width, height, ok := parseImageStudioDimensions(size)
		if !ok || width < 320 || height < 320 || width > 3840 || height > 3840 {
			continue
		}
		if width%16 != 0 || height%16 != 0 {
			continue
		}
		long, short := width, height
		if short > long {
			long, short = short, long
		}
		if short == 0 || float64(long)/float64(short) > 3 {
			continue
		}
		out = append(out, size)
	}
	return out
}

func imageStudioSizesThroughTier(maxTier string) []string {
	maxRank := map[string]int{
		ImageBillingSize1K: 1,
		ImageBillingSize2K: 2,
		ImageBillingSize4K: 3,
	}[maxTier]
	out := make([]string, 0, len(imageStudioAspectCatalog)*maxRank)
	for _, aspect := range imageStudioAspectCatalog {
		tiers := imageStudioSizeMatrix[aspect.ID]
		for _, tier := range imageStudioTierCatalog {
			if map[string]int{
				ImageBillingSize1K: 1,
				ImageBillingSize2K: 2,
				ImageBillingSize4K: 3,
			}[tier.ID] > maxRank {
				continue
			}
			if size := strings.TrimSpace(tiers[tier.ID]); size != "" {
				out = append(out, size)
			}
		}
	}
	return out
}

func parseImageStudioDimensions(size string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return width, height, width > 0 && height > 0
}

func ValidateImageStudioProviderOptions(
	capability ImageStudioModelCapabilities,
	operation string,
	req ImageStudioGenerateRequest,
) error {
	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation == "" {
		operation = "create"
	}
	if !imageStudioStringAllowed(capability.Operations, operation) {
		return ErrImageStudioOperationNotSupported
	}
	background := strings.ToLower(strings.TrimSpace(req.Background))
	if background != "" {
		if !imageStudioStringAllowed(capability.SupportedBackgrounds, background) {
			return ErrImageStudioBackgroundNotSupported
		}
		if background == "transparent" && !capability.SupportsTransparency {
			return ErrImageStudioBackgroundNotSupported
		}
	}
	outputFormat := normalizeImageStudioOutputFormat(req.OutputFormat)
	if outputFormat != "" && !imageStudioStringAllowed(capability.SupportedOutputFormats, outputFormat) {
		return ErrImageStudioOutputFormatNotSupported
	}
	effectiveOutputFormat := outputFormat
	if effectiveOutputFormat == "" {
		effectiveOutputFormat = normalizeImageStudioOutputFormat(capability.DefaultOutputFormat)
	}
	if background == "transparent" && effectiveOutputFormat == "jpeg" {
		return ErrImageStudioOutputFormatNotSupported
	}
	if req.OutputCompression != nil {
		compression := capability.OutputCompression
		if compression == nil ||
			!imageStudioStringAllowed(compression.Formats, effectiveOutputFormat) ||
			*req.OutputCompression < compression.Min ||
			*req.OutputCompression > compression.Max {
			return ErrImageStudioOutputCompressionNotSupported
		}
	}
	if fidelity := strings.ToLower(strings.TrimSpace(req.InputFidelity)); fidelity != "" {
		if operation != "edit" || !imageStudioStringAllowed(capability.SupportedInputFidelities, fidelity) {
			return ErrImageStudioInputFidelityNotSupported
		}
	}
	if strings.TrimSpace(req.Style) != "" {
		return ErrImageStudioStyleNotSupported
	}
	if capability.MaxReferenceImages > 0 && len(req.ReferenceIDs) > capability.MaxReferenceImages {
		return ErrImageStudioReferenceLimit
	}
	return nil
}

func imageStudioStringAllowed(items []string, value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item)) == value {
			return true
		}
	}
	return false
}

func normalizeImageStudioOutputFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "":
		return ""
	case "jpg":
		return "jpeg"
	case "png", "jpeg", "webp":
		return strings.ToLower(strings.TrimSpace(format))
	default:
		return fmt.Sprint(strings.ToLower(strings.TrimSpace(format)))
	}
}
