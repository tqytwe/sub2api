package service

import (
	"encoding/json"
	"fmt"
	"mime"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	imageModelAdapterAgnesID = "agnes"
	agnesImageFlashModelID   = "agnes-image-2.1-flash"
)

type imageModelAdapter interface {
	ID() string
	Matches(model string) bool
	ResolveCapability(model string) (ImageStudioModelCapabilities, bool)
	BuildImageStudioPayload(operation, model, prompt, size string, count int, req ImageStudioGenerateRequest, referenceIDs []string) (string, []byte, bool, error)
	RewriteOpenAIImagesBody(body []byte, contentType string, parsed *OpenAIImagesRequest, upstreamModel string) ([]byte, string, bool, error)
}

var registeredImageModelAdapters = []imageModelAdapter{
	agnesImageModelAdapter{},
}

func findImageModelAdapter(model string) (imageModelAdapter, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return nil, false
	}
	for _, adapter := range registeredImageModelAdapters {
		if adapter.Matches(model) {
			return adapter, true
		}
	}
	return nil, false
}

func resolveAdaptedImageStudioCapability(model string) (ImageStudioModelCapabilities, bool) {
	adapter, ok := findImageModelAdapter(model)
	if !ok {
		return ImageStudioModelCapabilities{}, false
	}
	return adapter.ResolveCapability(model)
}

func isRegisteredOpenAICompatibleImageModel(model string) bool {
	adapter, ok := findImageModelAdapter(model)
	return ok && strings.TrimSpace(adapter.ID()) != ""
}

func buildAdaptedImageStudioProviderPayload(
	operation, model, prompt, size string,
	count int,
	req ImageStudioGenerateRequest,
	referenceIDs []string,
) (string, []byte, bool, error) {
	adapter, ok := findImageModelAdapter(model)
	if !ok {
		return "", nil, false, nil
	}
	return adapter.BuildImageStudioPayload(operation, model, prompt, size, count, req, referenceIDs)
}

func rewriteAdaptedOpenAIImagesBody(
	body []byte,
	contentType string,
	parsed *OpenAIImagesRequest,
	upstreamModel string,
) ([]byte, string, bool, error) {
	adapter, ok := findImageModelAdapter(upstreamModel)
	if !ok {
		return nil, "", false, nil
	}
	return adapter.RewriteOpenAIImagesBody(body, contentType, parsed, upstreamModel)
}

type agnesImageModelAdapter struct{}

func (agnesImageModelAdapter) ID() string {
	return imageModelAdapterAgnesID
}

func (agnesImageModelAdapter) Matches(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), agnesImageFlashModelID)
}

func (agnesImageModelAdapter) ResolveCapability(model string) (ImageStudioModelCapabilities, bool) {
	if !(agnesImageModelAdapter{}).Matches(model) {
		return ImageStudioModelCapabilities{}, false
	}
	model = strings.ToLower(strings.TrimSpace(model))
	return ImageStudioModelCapabilities{
		Platform:              PlatformOpenAI,
		ProviderID:            imageModelAdapterAgnesID,
		ProfileID:             imageModelAdapterAgnesID + ":" + model + ":v1",
		Revision:              imageStudioCapabilityRevision,
		Operations:            []string{"create"},
		SizingKind:            "aspect_resolution",
		SupportedSizes:        agnesImageStudioSizes(),
		SupportedAspectRatios: []string{"1:1", "2:3", "3:2", "9:16", "16:9"},
		SupportedResolutions:  []string{"1k", "2k", "3k", "4k"},
		MaxReferenceImages:    0,
		DefaultSize:           defaultImageStudioSize,
		DefaultAspectRatio:    "1:1",
		DefaultResolution:     "1k",
	}, true
}

func (a agnesImageModelAdapter) BuildImageStudioPayload(
	operation, model, prompt, size string,
	count int,
	req ImageStudioGenerateRequest,
	referenceIDs []string,
) (string, []byte, bool, error) {
	if !a.Matches(model) {
		return "", nil, false, nil
	}
	if operation != "create" || len(referenceIDs) > 0 {
		return "", nil, true, ErrImageStudioOperationNotSupported
	}
	agnesSize, ratio := agnesImageStudioSizeAndRatio(size, req.Aspect, req.Tier)
	payload := map[string]any{
		"model":  strings.ToLower(strings.TrimSpace(model)),
		"prompt": prompt,
		"n":      count,
		"size":   agnesSize,
		"ratio":  ratio,
		"extra_body": map[string]any{
			"response_format": "b64_json",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, true, err
	}
	return openAIImagesGenerationsEndpoint, body, true, nil
}

func (a agnesImageModelAdapter) RewriteOpenAIImagesBody(
	body []byte,
	contentType string,
	parsed *OpenAIImagesRequest,
	upstreamModel string,
) ([]byte, string, bool, error) {
	if !a.Matches(upstreamModel) {
		return nil, "", false, nil
	}
	if parsed == nil {
		return nil, "", true, fmt.Errorf("parsed images request is required")
	}
	if parsed.Endpoint != openAIImagesGenerationsEndpoint || parsed.Multipart {
		return nil, "", true, ErrImageStudioOperationNotSupported
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && strings.EqualFold(mediaType, "multipart/form-data") {
		return nil, "", true, ErrImageStudioOperationNotSupported
	}
	ratio := strings.TrimSpace(gjson.GetBytes(body, "ratio").String())
	if !agnesImageRatioAllowed(ratio) {
		ratio = ""
	}
	agnesSize, inferredRatio := agnesImageStudioSizeAndRatio(parsed.Size, ratio, "")
	if ratio == "" {
		ratio = inferredRatio
	}
	out, err := sjson.SetBytes(body, "model", strings.ToLower(strings.TrimSpace(upstreamModel)))
	if err != nil {
		return nil, "", true, fmt.Errorf("rewrite agnes image model: %w", err)
	}
	out, err = sjson.SetBytes(out, "size", agnesSize)
	if err != nil {
		return nil, "", true, fmt.Errorf("rewrite agnes image size: %w", err)
	}
	out, err = sjson.SetBytes(out, "ratio", ratio)
	if err != nil {
		return nil, "", true, fmt.Errorf("rewrite agnes image ratio: %w", err)
	}
	if extraFormat := strings.TrimSpace(gjson.GetBytes(out, "extra_body.response_format").String()); extraFormat == "" {
		if format := strings.TrimSpace(gjson.GetBytes(out, "response_format").String()); format != "" {
			out, err = sjson.SetBytes(out, "extra_body.response_format", format)
			if err != nil {
				return nil, "", true, fmt.Errorf("rewrite agnes image response_format: %w", err)
			}
		}
	}
	if gjson.GetBytes(out, "response_format").Exists() {
		out, err = sjson.DeleteBytes(out, "response_format")
		if err != nil {
			return nil, "", true, fmt.Errorf("remove agnes top-level response_format: %w", err)
		}
	}
	return out, contentType, true, nil
}

func agnesImageStudioSizeAndRatio(size, aspect, tier string) (string, string) {
	normalizedTier := normalizeAgnesImageTier(tier)
	if normalizedTier == "" {
		normalizedTier = normalizeAgnesImageTier(size)
	}
	ratio := strings.TrimSpace(aspect)
	if !agnesImageRatioAllowed(ratio) {
		ratio = ""
	}
	if normalizedTier == "" || ratio == "" {
		inferredRatio, inferredTier := InferImageStudioAspectTier(size)
		if ratio == "" {
			ratio = inferredRatio
		}
		if normalizedTier == "" {
			normalizedTier = normalizeAgnesImageTier(inferredTier)
		}
	}
	if normalizedTier == "" {
		normalizedTier = ImageBillingSize1K
	}
	if ratio == "" {
		ratio = "1:1"
	}
	return normalizedTier, ratio
}

func normalizeAgnesImageTier(tier string) string {
	switch strings.ToUpper(strings.TrimSpace(tier)) {
	case ImageBillingSize1K:
		return ImageBillingSize1K
	case ImageBillingSize2K:
		return ImageBillingSize2K
	case ImageStudioTier3K:
		return ImageStudioTier3K
	case ImageBillingSize4K:
		return ImageBillingSize4K
	default:
		return ""
	}
}

func agnesImageRatioAllowed(ratio string) bool {
	switch strings.TrimSpace(ratio) {
	case "1:1", "2:3", "3:2", "9:16", "16:9":
		return true
	default:
		return false
	}
}
