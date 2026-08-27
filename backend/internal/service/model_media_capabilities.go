package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// These operation names are part of the server-owned media contract. Canvas
	// and the mobile client deliberately do not guess aliases such as
	// "generation", so accepting one here would make an administrator save a
	// declaration that no client can execute.
	CatalogImageOperationCreate   = "create"
	CatalogImageOperationEdit     = "edit"
	CatalogVideoOperationGenerate = "generate"
)

// CatalogMediaCapabilities is the server-owned media declaration stored with
// one catalog row. The persisted JSON intentionally remains raw on the catalog
// entity so provider extensions survive an administrator edit. This typed view
// is only used for validation and runtime capability resolution.
type CatalogMediaCapabilities struct {
	Version    string                         `json:"version"`
	Adapter    string                         `json:"adapter"`
	Modalities []string                       `json:"modalities"`
	Image      *CatalogImageMediaCapabilities `json:"image,omitempty"`
	Video      *CatalogVideoMediaCapabilities `json:"video,omitempty"`
}

type CatalogImageMediaCapabilities struct {
	Operations             []string `json:"operations"`
	SizingKind             string   `json:"sizing_kind,omitempty"`
	SupportedSizes         []string `json:"supported_sizes,omitempty"`
	MinDimension           int      `json:"min_dimension,omitempty"`
	MaxDimension           int      `json:"max_dimension,omitempty"`
	DimensionStep          int      `json:"dimension_step,omitempty"`
	MaxAspectRatio         float64  `json:"max_aspect_ratio,omitempty"`
	SupportedAspectRatios  []string `json:"supported_aspect_ratios,omitempty"`
	SupportedOutputFormats []string `json:"supported_output_formats,omitempty"`
	MaxReferenceImages     int      `json:"max_reference_images,omitempty"`
}

type CatalogVideoMediaCapabilities struct {
	Operations            []string `json:"operations"`
	SupportedResolutions  []string `json:"supported_resolutions,omitempty"`
	SupportedAspectRatios []string `json:"supported_aspect_ratios,omitempty"`
	DurationsSeconds      []int    `json:"durations_seconds,omitempty"`
	// GenerateAudio and Watermark are intentionally opt-in.  A false or
	// omitted value must not be treated as an upstream default because the
	// mobile and Canvas clients use this declaration to decide which controls
	// are safe to expose.
	GenerateAudio bool `json:"generate_audio,omitempty"`
	Watermark     bool `json:"watermark,omitempty"`
	// The precise reference types are preferred for new declarations.  The
	// aggregate field is retained for compatibility with early catalog rows;
	// runtime adapters still fail closed until they implement a lossless,
	// owner-scoped reference transport.
	MaxReferenceAssets int `json:"max_reference_assets,omitempty"`
	MaxReferenceImages int `json:"max_reference_images,omitempty"`
	MaxReferenceVideos int `json:"max_reference_videos,omitempty"`
	MaxReferenceAudios int `json:"max_reference_audios,omitempty"`
}

// NormalizeCatalogMediaCapabilities validates the stable portion of a media
// declaration while returning a copy of the original JSON. Unknown modalities
// and provider-owned extension fields are deliberately retained verbatim.
// A nil or JSON null value means an unreviewed legacy catalog row.
func NormalizeCatalogMediaCapabilities(raw json.RawMessage) (json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("media capabilities must be valid JSON")
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, fmt.Errorf("media capabilities must be a JSON object")
	}

	var declaration CatalogMediaCapabilities
	if err := json.Unmarshal(raw, &declaration); err != nil {
		return nil, fmt.Errorf("decode media capabilities: %w", err)
	}
	if strings.TrimSpace(declaration.Version) == "" {
		return nil, fmt.Errorf("media capabilities version is required")
	}
	if strings.TrimSpace(declaration.Adapter) == "" {
		return nil, fmt.Errorf("media capabilities adapter is required")
	}
	if len(declaration.Modalities) == 0 {
		return nil, fmt.Errorf("media capabilities modalities are required")
	}

	modalities := make(map[string]struct{}, len(declaration.Modalities))
	for _, modality := range declaration.Modalities {
		modality = strings.ToLower(strings.TrimSpace(modality))
		if modality == "" {
			return nil, fmt.Errorf("media capabilities modality is invalid")
		}
		modalities[modality] = struct{}{}
	}
	if _, declared := modalities["image"]; declared {
		if declaration.Image == nil || !hasOnlyCatalogOperations(declaration.Image.Operations, CatalogImageOperationCreate, CatalogImageOperationEdit) {
			return nil, fmt.Errorf("image media capabilities require operations")
		}
		if err := validateCatalogImageLimits(declaration.Image); err != nil {
			return nil, err
		}
	} else if declaration.Image != nil && nonEmptyCapabilityOperations(declaration.Image.Operations) {
		return nil, fmt.Errorf("image media capabilities require the image modality")
	}
	if _, declared := modalities["video"]; declared {
		if declaration.Video == nil || !hasOnlyCatalogOperations(declaration.Video.Operations, CatalogVideoOperationGenerate) {
			return nil, fmt.Errorf("video media capabilities require operations")
		}
		if err := validateCatalogVideoLimits(declaration.Video); err != nil {
			return nil, err
		}
	} else if declaration.Video != nil && nonEmptyCapabilityOperations(declaration.Video.Operations) {
		return nil, fmt.Errorf("video media capabilities require the video modality")
	}

	return append(json.RawMessage(nil), raw...), nil
}

func nonEmptyCapabilityOperations(operations []string) bool {
	for _, operation := range operations {
		if strings.TrimSpace(operation) != "" {
			return true
		}
	}
	return false
}

func hasOnlyCatalogOperations(operations []string, allowed ...string) bool {
	if !nonEmptyCapabilityOperations(operations) {
		return false
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, operation := range allowed {
		allowedSet[operation] = struct{}{}
	}
	seen := make(map[string]struct{}, len(operations))
	for _, operation := range operations {
		operation = strings.ToLower(strings.TrimSpace(operation))
		if operation == "" {
			return false
		}
		if _, supported := allowedSet[operation]; !supported {
			return false
		}
		if _, duplicate := seen[operation]; duplicate {
			return false
		}
		seen[operation] = struct{}{}
	}
	return true
}

func validateCatalogImageLimits(image *CatalogImageMediaCapabilities) error {
	if image == nil {
		return fmt.Errorf("image media capabilities are required")
	}
	if image.MinDimension < 0 || image.MaxDimension < 0 || image.DimensionStep < 0 ||
		image.MaxAspectRatio < 0 || image.MaxReferenceImages < 0 {
		return fmt.Errorf("image media capability limits cannot be negative")
	}
	if image.MinDimension > 0 && image.MaxDimension > 0 && image.MinDimension > image.MaxDimension {
		return fmt.Errorf("image media capability min dimension cannot exceed max dimension")
	}
	return nil
}

func validateCatalogVideoLimits(video *CatalogVideoMediaCapabilities) error {
	if video == nil {
		return fmt.Errorf("video media capabilities are required")
	}
	if video.MaxReferenceAssets < 0 ||
		video.MaxReferenceImages < 0 ||
		video.MaxReferenceVideos < 0 ||
		video.MaxReferenceAudios < 0 {
		return fmt.Errorf("video media capability reference limits cannot be negative")
	}
	seenDurations := make(map[int]struct{}, len(video.DurationsSeconds))
	for _, duration := range video.DurationsSeconds {
		if duration <= 0 {
			return fmt.Errorf("video media capability durations must be positive")
		}
		if _, duplicate := seenDurations[duration]; duplicate {
			return fmt.Errorf("video media capability durations cannot repeat")
		}
		seenDurations[duration] = struct{}{}
	}
	return nil
}

// ParseCatalogMediaCapabilities returns nil for legacy rows and validates the
// declaration before exposing its typed, runtime-safe fields.
func ParseCatalogMediaCapabilities(raw json.RawMessage) (*CatalogMediaCapabilities, error) {
	normalized, err := NormalizeCatalogMediaCapabilities(raw)
	if err != nil || normalized == nil {
		return nil, err
	}
	var declaration CatalogMediaCapabilities
	if err := json.Unmarshal(normalized, &declaration); err != nil {
		return nil, fmt.Errorf("decode media capabilities: %w", err)
	}
	return &declaration, nil
}
