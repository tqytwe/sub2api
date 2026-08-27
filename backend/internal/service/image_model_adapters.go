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
	agnesImage20FlashModelID = "agnes-image-2.0-flash"
	agnesImage21FlashModelID = "agnes-image-2.1-flash"

	// SenseNova is registered only for the two exact public model IDs. Do not
	// use a prefix match here: a future similarly named mapped model needs its
	// own verified contract before it can receive provider-specific fields.
	imageModelAdapterSenseNovaID = "sensenova"
	senseNovaU15LiteModelID      = "sensenova-u1.5-lite"
	senseNovaU1FastModelID       = "sensenova-u1-fast"
)

type imageModelAdapter interface {
	ID() string
	Matches(model string) bool
	RequiresOutputFanout() bool
	ResolveCapability(model string) (ImageStudioModelCapabilities, bool)
	BuildImageStudioPayload(operation, model, prompt, size string, count int, req ImageStudioGenerateRequest, referenceIDs []string) (string, []byte, bool, error)
	RewriteOpenAIImagesBody(body []byte, contentType string, parsed *OpenAIImagesRequest, upstreamModel string) ([]byte, string, bool, error)
}

var registeredImageModelAdapters = []imageModelAdapter{
	agnesImageModelAdapter{},
	senseNovaImageModelAdapter{},
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

func adaptedImageModelRequiresOutputFanout(model string) bool {
	adapter, ok := findImageModelAdapter(model)
	return ok && adapter.RequiresOutputFanout()
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
	switch strings.ToLower(strings.TrimSpace(model)) {
	case agnesImage20FlashModelID, agnesImage21FlashModelID:
		return true
	default:
		return false
	}
}

func (agnesImageModelAdapter) RequiresOutputFanout() bool {
	return true
}

func (agnesImageModelAdapter) ResolveCapability(model string) (ImageStudioModelCapabilities, bool) {
	if !(agnesImageModelAdapter{}).Matches(model) {
		return ImageStudioModelCapabilities{}, false
	}
	model = strings.ToLower(strings.TrimSpace(model))
	if model == agnesImage20FlashModelID {
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
	model = strings.ToLower(strings.TrimSpace(model))
	payload := map[string]any{
		"model":  model,
		"prompt": prompt,
		"n":      count,
		"extra_body": map[string]any{
			"response_format": "b64_json",
		},
	}
	if model == agnesImage20FlashModelID {
		payload["size"] = normalizeAgnesImage20Size(size)
	} else {
		agnesSize, ratio := agnesImageStudioSizeAndRatio(size, req.Aspect, req.Tier)
		payload["size"] = agnesSize
		payload["ratio"] = ratio
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
	out, err := sjson.SetBytes(body, "model", strings.ToLower(strings.TrimSpace(upstreamModel)))
	if err != nil {
		return nil, "", true, fmt.Errorf("rewrite agnes image model: %w", err)
	}
	if gjson.GetBytes(out, "n").Exists() {
		out, err = sjson.DeleteBytes(out, "n")
		if err != nil {
			return nil, "", true, fmt.Errorf("remove unsupported agnes image count: %w", err)
		}
	}
	if strings.EqualFold(strings.TrimSpace(upstreamModel), agnesImage20FlashModelID) {
		out, err = sjson.SetBytes(out, "size", normalizeAgnesImage20Size(parsed.Size))
		if err != nil {
			return nil, "", true, fmt.Errorf("rewrite agnes image size: %w", err)
		}
		if gjson.GetBytes(out, "ratio").Exists() {
			out, err = sjson.DeleteBytes(out, "ratio")
			if err != nil {
				return nil, "", true, fmt.Errorf("remove agnes 2.0 ratio: %w", err)
			}
		}
	} else {
		ratio := strings.TrimSpace(gjson.GetBytes(body, "ratio").String())
		if !agnesImageRatioAllowed(ratio) {
			ratio = ""
		}
		agnesSize, inferredRatio := agnesImageStudioSizeAndRatio(parsed.Size, ratio, "")
		if ratio == "" {
			ratio = inferredRatio
		}
		out, err = sjson.SetBytes(out, "size", agnesSize)
		if err != nil {
			return nil, "", true, fmt.Errorf("rewrite agnes image size: %w", err)
		}
		out, err = sjson.SetBytes(out, "ratio", ratio)
		if err != nil {
			return nil, "", true, fmt.Errorf("rewrite agnes image ratio: %w", err)
		}
	}
	out, err = moveOpenAIImagesResponseFormatToExtraBody(out)
	if err != nil {
		return nil, "", true, err
	}
	out, err = removeAgnesImageOutputFormat(out)
	if err != nil {
		return nil, "", true, err
	}
	return out, contentType, true, nil
}

// senseNovaImageModelAdapter keeps the verified upstream watermark control
// server-owned. Both models default to a clean output and callers cannot turn
// it back on through an OpenAI-compatible request.
type senseNovaImageModelAdapter struct{}

func (senseNovaImageModelAdapter) ID() string { return imageModelAdapterSenseNovaID }

func (senseNovaImageModelAdapter) Matches(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case senseNovaU15LiteModelID, senseNovaU1FastModelID:
		return true
	default:
		return false
	}
}

func (senseNovaImageModelAdapter) RequiresOutputFanout() bool { return false }

func (a senseNovaImageModelAdapter) ResolveCapability(model string) (ImageStudioModelCapabilities, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if !a.Matches(model) {
		return ImageStudioModelCapabilities{}, false
	}
	if model == senseNovaU15LiteModelID {
		return ImageStudioModelCapabilities{
			Platform: PlatformOpenAI, ProviderID: imageModelAdapterSenseNovaID,
			ProfileID: imageModelAdapterSenseNovaID + ":" + model + ":v1", Revision: imageStudioCapabilityRevision,
			Operations: []string{"create"}, SizingKind: "fixed", SupportedSizes: allImageStudioCatalogSizes(),
			SupportedOutputFormats: []string{"png", "jpeg", "webp"}, DefaultSize: defaultImageStudioSize, DefaultOutputFormat: "png",
			MinOutputCount: 1, MaxOutputCount: 1,
		}, true
	}
	return ImageStudioModelCapabilities{
		Platform: PlatformOpenAI, ProviderID: imageModelAdapterSenseNovaID,
		ProfileID: imageModelAdapterSenseNovaID + ":" + model + ":v1", Revision: imageStudioCapabilityRevision,
		Operations: []string{"create"}, SizingKind: "fixed",
		SupportedSizes: []string{"1664x2496", "2496x1664", "1760x2368", "2368x1760", "1824x2272", "2272x1824", "2048x2048", "2752x1536", "1536x2752", "3072x1376", "1344x3136"},
		DefaultSize:    "2752x1536",
	}, true
}

func (a senseNovaImageModelAdapter) BuildImageStudioPayload(
	operation, model, prompt, size string,
	count int,
	req ImageStudioGenerateRequest,
	referenceIDs []string,
) (string, []byte, bool, error) {
	model = strings.ToLower(strings.TrimSpace(model))
	if !a.Matches(model) {
		return "", nil, false, nil
	}
	if strings.TrimSpace(operation) != "create" || len(referenceIDs) > 0 {
		return "", nil, true, ErrImageStudioOperationNotSupported
	}
	if count <= 0 {
		count = 1
	}
	payload := map[string]any{"model": model, "prompt": prompt, "n": count, "size": size, "watermark": false}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, true, err
	}
	return openAIImagesGenerationsEndpoint, body, true, nil
}

func (a senseNovaImageModelAdapter) RewriteOpenAIImagesBody(
	body []byte,
	contentType string,
	parsed *OpenAIImagesRequest,
	upstreamModel string,
) ([]byte, string, bool, error) {
	upstreamModel = strings.ToLower(strings.TrimSpace(upstreamModel))
	if !a.Matches(upstreamModel) {
		return nil, "", false, nil
	}
	if parsed == nil || parsed.Endpoint != openAIImagesGenerationsEndpoint || parsed.Multipart {
		return nil, "", true, ErrImageStudioOperationNotSupported
	}
	out, err := sjson.SetBytes(body, "model", upstreamModel)
	if err != nil {
		return nil, "", true, fmt.Errorf("rewrite SenseNova image model: %w", err)
	}
	// This must remain a JSON boolean, not a string and not a prompt hint.
	out, err = sjson.SetBytes(out, "watermark", false)
	if err != nil {
		return nil, "", true, fmt.Errorf("set SenseNova watermark: %w", err)
	}
	return out, contentType, true, nil
}

func normalizeAgnesImage20Size(size string) string {
	size = strings.TrimSpace(size)
	if size == "" || normalizeAgnesImageTier(size) != "" {
		return defaultImageStudioSize
	}
	return size
}

func moveOpenAIImagesResponseFormatToExtraBody(out []byte) ([]byte, error) {
	var err error
	if extraFormat := strings.TrimSpace(gjson.GetBytes(out, "extra_body.response_format").String()); extraFormat == "" {
		if format := strings.TrimSpace(gjson.GetBytes(out, "response_format").String()); format != "" {
			out, err = sjson.SetBytes(out, "extra_body.response_format", format)
			if err != nil {
				return nil, fmt.Errorf("rewrite agnes image response_format: %w", err)
			}
		}
	}
	if gjson.GetBytes(out, "response_format").Exists() {
		out, err = sjson.DeleteBytes(out, "response_format")
		if err != nil {
			return nil, fmt.Errorf("remove agnes top-level response_format: %w", err)
		}
	}
	return out, nil
}

func removeAgnesImageOutputFormat(out []byte) ([]byte, error) {
	var err error
	if gjson.GetBytes(out, "output_format").Exists() {
		out, err = sjson.DeleteBytes(out, "output_format")
		if err != nil {
			return nil, fmt.Errorf("remove agnes top-level output_format: %w", err)
		}
	}
	if gjson.GetBytes(out, "extra_body.output_format").Exists() {
		out, err = sjson.DeleteBytes(out, "extra_body.output_format")
		if err != nil {
			return nil, fmt.Errorf("remove agnes extra_body output_format: %w", err)
		}
	}
	return out, nil
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
