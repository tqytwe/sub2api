package service

import (
	"encoding/base64"
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

type senseNovaImageModelAdapter struct{}

func (senseNovaImageModelAdapter) ID() string {
	return imageModelAdapterSenseNovaID
}

func (senseNovaImageModelAdapter) Matches(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case senseNovaU15LiteModelID, senseNovaU1FastModelID:
		return true
	default:
		return false
	}
}

func (senseNovaImageModelAdapter) RequiresOutputFanout() bool {
	return false
}

func (a senseNovaImageModelAdapter) ResolveCapability(model string) (ImageStudioModelCapabilities, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if !a.Matches(model) {
		return ImageStudioModelCapabilities{}, false
	}
	if model == senseNovaU15LiteModelID {
		return ImageStudioModelCapabilities{
			Platform:               PlatformOpenAI,
			ProviderID:             imageModelAdapterSenseNovaID,
			ProfileID:              imageModelAdapterSenseNovaID + ":" + model + ":v1",
			Revision:               imageStudioCapabilityRevision,
			Operations:             []string{"create", "edit"},
			SizingKind:             "custom_dimensions",
			MinDimension:           512,
			MaxDimension:           4096,
			DimensionStep:          32,
			MaxAspectRatio:         3,
			SupportedOutputFormats: []string{"png", "jpeg", "webp"},
			MaxReferenceImages:     maxImageStudioReferences,
			DefaultSize:            defaultImageStudioSize,
			DefaultOutputFormat:    "png",
		}, true
	}
	return ImageStudioModelCapabilities{
		Platform:   PlatformOpenAI,
		ProviderID: imageModelAdapterSenseNovaID,
		ProfileID:  imageModelAdapterSenseNovaID + ":" + model + ":v1",
		Revision:   imageStudioCapabilityRevision,
		Operations: []string{"create"},
		SizingKind: "fixed",
		SupportedSizes: []string{
			"1664x2496", "2496x1664", "1760x2368", "2368x1760", "1824x2272", "2272x1824",
			"2048x2048", "2752x1536", "1536x2752", "3072x1376", "1344x3136",
		},
		DefaultSize: defaultSenseNovaU1FastSize,
	}, true
}

const defaultSenseNovaU1FastSize = "2752x1536"

func isSenseNovaImageModel(model string) bool {
	return (senseNovaImageModelAdapter{}).Matches(model)
}

func isSenseNovaU15LiteModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), senseNovaU15LiteModelID)
}

func isSenseNovaU1FastModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), senseNovaU1FastModelID)
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
	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation == "" {
		operation = "create"
	}
	if operation != "create" && (operation != "edit" || model != senseNovaU15LiteModelID) {
		return "", nil, true, ErrImageStudioOperationNotSupported
	}
	if operation == "create" && len(referenceIDs) > 0 {
		return "", nil, true, ErrImageStudioOperationNotSupported
	}
	if count <= 0 {
		count = 1
	}
	endpoint := openAIImagesGenerationsEndpoint
	if operation == "edit" {
		endpoint = openAIImagesEditsEndpoint
	}
	payload := map[string]any{
		"model":     model,
		"prompt":    prompt,
		"n":         count,
		"size":      size,
		"watermark": true,
	}
	if model == senseNovaU15LiteModelID {
		outputFormat := normalizeImageStudioOutputFormat(req.OutputFormat)
		if outputFormat == "" {
			outputFormat = "png"
		}
		payload["output_format"] = outputFormat
		payload["response_format"] = "b64_json"
		payload["prompt_extend"] = true
		if operation == "edit" {
			payload["image_studio_job_reference_ids"] = referenceIDs
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, true, err
	}
	return endpoint, body, true, nil
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
	if parsed == nil {
		return nil, "", true, fmt.Errorf("parsed images request is required")
	}
	if err := validateSenseNovaOpenAIImagesRequest(body, parsed, upstreamModel); err != nil {
		return nil, "", true, err
	}
	if isSenseNovaU1FastModel(upstreamModel) {
		out, err := sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, "", true, fmt.Errorf("rewrite SenseNova U1 Fast image model: %w", err)
		}
		if size := strings.ToLower(strings.TrimSpace(parsed.Size)); size != "" {
			out, err = sjson.SetBytes(out, "size", size)
			if err != nil {
				return nil, "", true, fmt.Errorf("rewrite SenseNova U1 Fast image size: %w", err)
			}
		}
		out, err = removeSenseNovaU1FastUnsupportedFields(out)
		if err != nil {
			return nil, "", true, err
		}
		return out, contentType, true, nil
	}

	if parsed.Multipart {
		if parsed.Endpoint != openAIImagesEditsEndpoint {
			return nil, "", true, ErrImageStudioOperationNotSupported
		}
		out, err := buildSenseNovaU15MultipartEditJSON(parsed, upstreamModel)
		if err != nil {
			return nil, "", true, err
		}
		return out, "application/json", true, nil
	}
	out, err := sjson.SetBytes(body, "model", upstreamModel)
	if err != nil {
		return nil, "", true, fmt.Errorf("rewrite SenseNova U1.5 Lite image model: %w", err)
	}
	if size := strings.ToLower(strings.TrimSpace(parsed.Size)); size != "" {
		out, err = sjson.SetBytes(out, "size", size)
		if err != nil {
			return nil, "", true, fmt.Errorf("rewrite SenseNova U1.5 Lite image size: %w", err)
		}
	}
	if format := normalizeImageStudioOutputFormat(parsed.OutputFormat); format != "" {
		out, err = sjson.SetBytes(out, "output_format", format)
		if err != nil {
			return nil, "", true, fmt.Errorf("rewrite SenseNova U1.5 Lite output format: %w", err)
		}
	}
	if format := strings.ToLower(strings.TrimSpace(parsed.ResponseFormat)); format != "" {
		out, err = sjson.SetBytes(out, "response_format", format)
		if err != nil {
			return nil, "", true, fmt.Errorf("rewrite SenseNova U1.5 Lite response format: %w", err)
		}
	}
	return out, contentType, true, nil
}

func validateSenseNovaOpenAIImagesRequest(body []byte, parsed *OpenAIImagesRequest, model string) error {
	model = strings.ToLower(strings.TrimSpace(model))
	if !isSenseNovaImageModel(model) {
		return nil
	}
	if parsed == nil {
		return fmt.Errorf("parsed images request is required")
	}
	if isSenseNovaU1FastModel(model) {
		if parsed.Endpoint != openAIImagesGenerationsEndpoint || parsed.Multipart || senseNovaHasReferenceInput(body) {
			return ErrImageStudioOperationNotSupported
		}
		return validateSenseNovaU1FastRequest(parsed)
	}
	if parsed.Endpoint != openAIImagesGenerationsEndpoint && parsed.Endpoint != openAIImagesEditsEndpoint {
		return ErrImageStudioOperationNotSupported
	}
	if parsed.Endpoint == openAIImagesGenerationsEndpoint && (parsed.Multipart || senseNovaHasReferenceInput(body)) {
		return ErrImageStudioOperationNotSupported
	}
	if parsed.HasMask || senseNovaHasReferenceInput(body) && parsed.Endpoint != openAIImagesEditsEndpoint {
		return ErrImageStudioOperationNotSupported
	}
	return validateSenseNovaU15LiteRequest(parsed)
}

// SenseNova U1 Fast only accepts text-to-image generation. Inspect nested
// provider extension objects too: silently dropping a supplied reference image
// would make a caller believe an edit was performed.
func senseNovaHasReferenceInput(body []byte) bool {
	referenceKeys := map[string]struct{}{
		"image": {}, "images": {}, "image_url": {}, "image_urls": {},
		"input_image": {}, "input_images": {},
		"reference_image": {}, "reference_images": {}, "reference_image_urls": {},
		"mask": {},
	}
	var containsReference func(gjson.Result) bool
	containsReference = func(value gjson.Result) bool {
		switch {
		case value.IsObject():
			found := false
			value.ForEach(func(key, nested gjson.Result) bool {
				if _, ok := referenceKeys[strings.ToLower(strings.TrimSpace(key.String()))]; ok {
					found = true
					return false
				}
				if containsReference(nested) {
					found = true
					return false
				}
				return true
			})
			return found
		case value.IsArray():
			for _, nested := range value.Array() {
				if containsReference(nested) {
					return true
				}
			}
		}
		return false
	}
	return containsReference(gjson.ParseBytes(body))
}

func validateSenseNovaU15LiteRequest(parsed *OpenAIImagesRequest) error {
	if parsed == nil {
		return fmt.Errorf("parsed images request is required")
	}
	if parsed.Stream {
		return fmt.Errorf("SenseNova U1.5 Lite does not support streaming image responses")
	}
	if parsed.N != 1 {
		return fmt.Errorf("SenseNova U1.5 Lite only supports n=1")
	}
	if err := validateSenseNovaU15LiteSize(parsed.Size); err != nil {
		return err
	}
	if format := normalizeImageStudioOutputFormat(parsed.OutputFormat); format != "" &&
		!imageStudioStringAllowed([]string{"png", "jpeg", "webp"}, format) {
		return ErrImageStudioOutputFormatNotSupported
	}
	if format := strings.ToLower(strings.TrimSpace(parsed.ResponseFormat)); format != "" &&
		format != "b64_json" && format != "url" {
		return ErrImageResponseFormatInvalid
	}
	return nil
}

func validateSenseNovaU15LiteSize(size string) error {
	size = strings.ToLower(strings.TrimSpace(size))
	if size == "" || size == "auto" {
		return nil
	}
	capability, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, senseNovaU15LiteModelID)
	if !ok || !imageStudioCustomDimensionsAllowed(capability, size) {
		return ErrImageStudioSizeNotSupported
	}
	return nil
}

func validateSenseNovaU1FastRequest(parsed *OpenAIImagesRequest) error {
	if parsed == nil {
		return fmt.Errorf("parsed images request is required")
	}
	if parsed.Stream {
		return fmt.Errorf("SenseNova U1 Fast does not support streaming image responses")
	}
	size := strings.ToLower(strings.TrimSpace(parsed.Size))
	if size == "" {
		return nil
	}
	capability, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, senseNovaU1FastModelID)
	if !ok || !imageStudioStringAllowed(capability.SupportedSizes, size) {
		return ErrImageStudioSizeNotSupported
	}
	return nil
}

func removeSenseNovaU1FastUnsupportedFields(out []byte) ([]byte, error) {
	var err error
	for _, path := range []string{
		"response_format",
		"output_format",
		"prompt_extend",
		"quality",
		"background",
		"moderation",
		"input_fidelity",
		"style",
		"output_compression",
		"partial_images",
		"extra_body",
	} {
		if !gjson.GetBytes(out, path).Exists() {
			continue
		}
		out, err = sjson.DeleteBytes(out, path)
		if err != nil {
			return nil, fmt.Errorf("remove unsupported SenseNova U1 Fast field %s: %w", path, err)
		}
	}
	return out, nil
}

func buildSenseNovaU15MultipartEditJSON(parsed *OpenAIImagesRequest, model string) ([]byte, error) {
	if parsed == nil || len(parsed.Uploads) == 0 {
		return nil, ErrImageStudioReferenceNotFound
	}
	images := make([]map[string]string, 0, len(parsed.Uploads))
	for _, upload := range parsed.Uploads {
		contentType, err := validateImageStudioReference(upload.ContentType, upload.Data)
		if err != nil {
			return nil, err
		}
		images = append(images, map[string]string{
			"image_url": "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(upload.Data),
		})
	}
	payload := map[string]any{
		"model":  model,
		"prompt": parsed.Prompt,
		"n":      1,
		"images": images,
	}
	if size := strings.ToLower(strings.TrimSpace(parsed.Size)); size != "" {
		payload["size"] = size
	}
	if format := strings.ToLower(strings.TrimSpace(parsed.ResponseFormat)); format != "" {
		payload["response_format"] = format
	}
	if format := normalizeImageStudioOutputFormat(parsed.OutputFormat); format != "" {
		payload["output_format"] = format
	}
	if parsed.Watermark != nil {
		payload["watermark"] = *parsed.Watermark
	}
	if parsed.PromptExtend != nil {
		payload["prompt_extend"] = *parsed.PromptExtend
	}
	return json.Marshal(payload)
}
