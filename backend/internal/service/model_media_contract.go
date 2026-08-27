package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

var (
	// ErrModelMediaCapabilityNotDeclared is intentionally distinct from a
	// routing miss. A client can retry after an administrator declares the
	// model, but must never guess a capability from its name.
	ErrModelMediaCapabilityNotDeclared = errors.New("model media capability is not declared")
	ErrModelMediaAdapterUnsupported    = errors.New("declared media adapter is not executable")
)

// GatewayModelContract is the public, provider-neutral subset of a catalog
// entry. It is safe to return from /v1/models and managed workspace bootstrap:
// it contains no account, mapping, pricing, or credential material.
type GatewayModelContract struct {
	ID                string                  `json:"id"`
	Platform          string                  `json:"platform,omitempty"`
	Modalities        []string                `json:"modalities,omitempty"`
	ImageCapabilities *ModelImageCapabilities `json:"image_capabilities,omitempty"`
	// VideoCapabilities is projected into the same transport shape used by the
	// dedicated mobile-video bootstrap. The persisted catalog has intentionally
	// different field names (supported_aspect_ratios/durations_seconds); never
	// leak that storage shape to one managed client but not another.
	VideoCapabilities *MobileVideoCapabilities `json:"video_capabilities,omitempty"`
	Adapter           string                   `json:"adapter,omitempty"`
	CapabilityVersion string                   `json:"capability_version,omitempty"`
}

// ModelImageCapabilities is the public, provider-neutral image capability
// shape. Catalog storage intentionally uses its own field names so schema
// migrations remain isolated from clients; every managed client receives this
// projection instead. Keep this shape aligned with the mobile and Canvas
// contracts rather than exposing supported_aspect_ratios or
// supported_output_formats from the catalog JSON.
type ModelImageCapabilities struct {
	Operations         []string `json:"operations"`
	SizingKind         string   `json:"sizing_kind,omitempty"`
	SupportedSizes     []string `json:"supported_sizes,omitempty"`
	SupportedRatios    []string `json:"supported_ratios,omitempty"`
	SupportedFormats   []string `json:"supported_formats,omitempty"`
	MinDimension       int      `json:"min_dimension,omitempty"`
	MaxDimension       int      `json:"max_dimension,omitempty"`
	DimensionStep      int      `json:"dimension_step,omitempty"`
	MaxAspectRatio     float64  `json:"max_aspect_ratio,omitempty"`
	MaxReferenceImages int      `json:"max_reference_images,omitempty"`
}

func (c ModelImageCapabilities) Clone() ModelImageCapabilities {
	c.Operations = cloneCatalogStrings(c.Operations)
	c.SupportedSizes = cloneCatalogStrings(c.SupportedSizes)
	c.SupportedRatios = cloneCatalogStrings(c.SupportedRatios)
	c.SupportedFormats = cloneCatalogStrings(c.SupportedFormats)
	return c
}

func (c GatewayModelContract) Supports(modality, operation string) bool {
	modality = strings.ToLower(strings.TrimSpace(modality))
	operation = strings.ToLower(strings.TrimSpace(operation))
	if modality == "" || operation == "" {
		return false
	}
	declared := false
	for _, candidate := range c.Modalities {
		if strings.EqualFold(strings.TrimSpace(candidate), modality) {
			declared = true
			break
		}
	}
	if !declared {
		return false
	}
	var operations []string
	switch modality {
	case "image":
		if c.ImageCapabilities != nil {
			operations = c.ImageCapabilities.Operations
		}
	case "video":
		if c.VideoCapabilities != nil {
			operations = c.VideoCapabilities.Operations
		}
	default:
		return false
	}
	for _, candidate := range operations {
		if strings.EqualFold(strings.TrimSpace(candidate), operation) {
			return true
		}
	}
	return false
}

// ResolveGatewayModelContracts enriches an already schedulable model list with
// an administrator-owned catalog declaration. Mapping and scheduler state
// remain the visibility source; a catalog row can never add a model by itself.
func (s *ModelCatalogService) ResolveGatewayModelContracts(
	ctx context.Context,
	group Group,
	mappedModels []string,
) ([]GatewayModelContract, error) {
	entries, err := s.listVisibleCatalogContracts(ctx)
	if err != nil {
		// A transient catalog read must not turn an explicitly mapped chat model
		// into a default model. It remains visible without media extensions.
		entries = nil
	}

	contracts := make([]GatewayModelContract, 0, len(mappedModels)+len(entries))
	seen := make(map[string]struct{}, len(mappedModels)+len(entries))
	appendModel := func(modelID string, entry *SiteModelCatalogEntry, mustBeCatalogExecutable bool) {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" || strings.HasSuffix(modelID, "*") {
			return
		}
		key := strings.ToLower(modelID)
		if _, exists := seen[key]; exists {
			return
		}
		contract := GatewayModelContract{ID: modelID, Platform: group.Platform}
		if entry != nil {
			resolved, resolveErr := s.catalogEntryGatewayContract(ctx, *entry, group)
			if resolveErr != nil {
				if mustBeCatalogExecutable {
					return
				}
			} else {
				contract = resolved
			}
		}
		seen[key] = struct{}{}
		contracts = append(contracts, contract)
	}

	for _, modelID := range mappedModels {
		appendModel(modelID, findCatalogContractEntry(entries, group, modelID), false)
	}
	return contracts, nil
}

// ResolveModelMediaContract validates the exact declared media modality and
// adapter for an already-authorized group/model. It is used by managed Canvas
// and mobile paths before an image or video request is dispatched.
func (s *ModelCatalogService) ResolveModelMediaContract(
	ctx context.Context,
	group Group,
	modelID string,
) (*GatewayModelContract, error) {
	entries, err := s.listVisibleCatalogContracts(ctx)
	if err != nil {
		return nil, err
	}
	entry := findCatalogContractEntry(entries, group, modelID)
	if entry == nil {
		return nil, ErrModelMediaCapabilityNotDeclared
	}
	contract, err := s.catalogEntryGatewayContract(ctx, *entry, group)
	if err != nil {
		return nil, err
	}
	if len(contract.Modalities) == 0 {
		return nil, ErrModelMediaCapabilityNotDeclared
	}
	return &contract, nil
}

// ResolveImageStudioCatalogContracts projects explicit catalog declarations
// into the Image Studio shape after the gateway has established that each model
// has a schedulable account and an allowed mapping. It deliberately does not
// add catalog-only rows to the caller's candidate list.
//
// A legacy NULL declaration is not marked as reviewed so Image Studio can use
// the small audited compatibility list during migration. Any explicit JSON
// declaration, including an invalid or non-image declaration, is marked as
// reviewed and therefore suppresses fallback name inference.
func (s *ModelCatalogService) ResolveImageStudioCatalogContracts(
	ctx context.Context,
	group Group,
	mappedModels []string,
) (map[string]ImageStudioCatalogContractResolution, error) {
	entries, err := s.listVisibleCatalogContracts(ctx)
	if err != nil {
		return nil, err
	}

	resolutions := make(map[string]ImageStudioCatalogContractResolution, len(mappedModels))
	seen := make(map[string]struct{}, len(mappedModels))
	for _, candidate := range mappedModels {
		modelID := strings.TrimSpace(candidate)
		if modelID == "" || strings.HasSuffix(modelID, "*") {
			continue
		}
		key := strings.ToLower(modelID)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}

		entry := findCatalogContractEntry(entries, group, modelID)
		if !catalogMediaCapabilitiesExplicit(entry) {
			continue
		}

		resolution := ImageStudioCatalogContractResolution{Declared: true}
		contract, contractErr := s.catalogEntryGatewayContract(ctx, *entry, group)
		if contractErr == nil && contract.Supports("image", CatalogImageOperationCreate) && contract.ImageCapabilities != nil {
			if capabilities, supported := imageStudioCapabilitiesFromCatalogContract(contract); supported {
				resolution.Capabilities = &capabilities
			}
		}
		resolutions[key] = resolution
	}
	return resolutions, nil
}

func (s *ModelCatalogService) listVisibleCatalogContracts(ctx context.Context) ([]SiteModelCatalogEntry, error) {
	if s == nil || s.repo == nil {
		return nil, ErrModelMediaCapabilityNotDeclared
	}
	visible := true
	return s.repo.ListCatalog(ctx, CatalogListFilter{VisibleAuth: &visible})
}

func findCatalogContractEntry(entries []SiteModelCatalogEntry, group Group, modelID string) *SiteModelCatalogEntry {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil
	}
	var generic *SiteModelCatalogEntry
	for index := range entries {
		entry := &entries[index]
		if !strings.EqualFold(strings.TrimSpace(entry.ModelName), modelID) ||
			!catalogContractEntryMatchesGroup(*entry, group) {
			continue
		}
		if normalizeCatalogContractPlatform(entry.Platform) == normalizeCatalogContractPlatform(group.Platform) {
			return entry
		}
		if strings.TrimSpace(entry.Platform) == "" {
			generic = entry
		}
	}
	return generic
}

func catalogContractEntryMatchesGroup(entry SiteModelCatalogEntry, group Group) bool {
	if !catalogAllowsGroup(entry, group.ID, group.Platform) {
		return false
	}
	entryPlatform := normalizeCatalogContractPlatform(entry.Platform)
	groupPlatform := normalizeCatalogContractPlatform(group.Platform)
	if entryPlatform == "" || groupPlatform == "" || groupPlatform == PlatformComposite {
		return true
	}
	return entryPlatform == groupPlatform
}

func normalizeCatalogContractPlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "", "—":
		return ""
	case "openai":
		return PlatformOpenAI
	case "anthropic", "claude":
		return PlatformAnthropic
	case "gemini", "google":
		return PlatformGemini
	case "grok", "xai":
		return PlatformGrok
	default:
		return strings.ToLower(strings.TrimSpace(platform))
	}
}

func (s *ModelCatalogService) catalogEntryGatewayContract(
	ctx context.Context,
	entry SiteModelCatalogEntry,
	group Group,
) (GatewayModelContract, error) {
	contract := GatewayModelContract{
		ID:       strings.TrimSpace(entry.ModelName),
		Platform: normalizeCatalogContractPlatform(entry.Platform),
	}
	declaration, err := ParseCatalogMediaCapabilities(entry.MediaCapabilities)
	if err != nil {
		return GatewayModelContract{}, ErrModelMediaCapabilityNotDeclared
	}
	if declaration == nil {
		return contract, nil
	}
	if !s.catalogMediaDeclarationHasExecutableAdapter(ctx, contract.ID, group, entry.MediaCapabilities, declaration) {
		return GatewayModelContract{}, ErrModelMediaAdapterUnsupported
	}
	contract.Adapter = strings.TrimSpace(declaration.Adapter)
	contract.CapabilityVersion = strings.TrimSpace(declaration.Version)
	contract.Modalities = normalizedCatalogModalities(declaration.Modalities)
	if declaration.Image != nil {
		contract.ImageCapabilities = modelImageCapabilitiesFromCatalog(declaration.Image)
	}
	if declaration.Video != nil && catalogDeclaresVideoModality(declaration) {
		video, _, valid := mobileVideoCapabilitiesFromCatalog(entry.MediaCapabilities)
		if !valid {
			return GatewayModelContract{}, ErrModelMediaAdapterUnsupported
		}
		contract.VideoCapabilities = cloneGatewayVideoCapabilities(&video)
	}
	return contract, nil
}

func catalogMediaCapabilitiesExplicit(entry *SiteModelCatalogEntry) bool {
	if entry == nil {
		return false
	}
	declaration, err := ParseCatalogMediaCapabilities(entry.MediaCapabilities)
	return declaration != nil || err != nil
}

// imageStudioCapabilitiesFromCatalogContract retains the registered adapter's
// internal worker profile while applying catalog-owned restrictions to the
// Image Studio controls. The worker profile is intentionally not replaced by
// the catalog version: queued jobs pin the exact executable adapter profile.
func imageStudioCapabilitiesFromCatalogContract(contract GatewayModelContract) (ImageStudioModelCapabilities, bool) {
	if !contract.Supports("image", CatalogImageOperationCreate) || contract.ImageCapabilities == nil {
		return ImageStudioModelCapabilities{}, false
	}
	if strings.EqualFold(strings.TrimSpace(contract.Adapter), catalogOpenAIImagesAdapterID) {
		return resolveCatalogOpenAIImagesCapability(contract)
	}
	capability, supported := ResolveImageStudioProviderCapability(contract.Platform, contract.ID)
	if !supported {
		return ImageStudioModelCapabilities{}, false
	}

	declared := contract.ImageCapabilities
	capability.Operations = cloneCatalogStrings(declared.Operations)
	if value := strings.TrimSpace(declared.SizingKind); value != "" {
		capability.SizingKind = value
	}
	if len(declared.SupportedSizes) > 0 {
		capability.SupportedSizes = cloneCatalogStrings(declared.SupportedSizes)
	}
	if len(declared.SupportedRatios) > 0 {
		capability.SupportedAspectRatios = cloneCatalogStrings(declared.SupportedRatios)
	}
	if len(declared.SupportedFormats) > 0 {
		capability.SupportedOutputFormats = cloneCatalogStrings(declared.SupportedFormats)
	}
	if declared.MinDimension > 0 {
		capability.MinDimension = declared.MinDimension
	}
	if declared.MaxDimension > 0 {
		capability.MaxDimension = declared.MaxDimension
	}
	if declared.DimensionStep > 0 {
		capability.DimensionStep = declared.DimensionStep
	}
	if declared.MaxAspectRatio > 0 {
		capability.MaxAspectRatio = declared.MaxAspectRatio
	}
	if declared.MaxReferenceImages > 0 {
		capability.MaxReferenceImages = declared.MaxReferenceImages
	}
	if adapter := strings.TrimSpace(contract.Adapter); adapter != "" {
		capability.ProviderID = adapter
	}
	return capability, true
}

const (
	// catalogOpenAIImagesAdapterID identifies the deliberately narrow generic
	// adapter. It is only for providers that already accept the standard
	// OpenAI-compatible /v1/images/* request shape; provider-specific adapters
	// such as SenseNova remain exact-model registrations.
	catalogOpenAIImagesAdapterID      = "openai_images"
	catalogOpenAIImagesProfilePrefix  = "openai_images:"
	catalogOpenAIImagesRevisionPrefix = "catalog-openai-images:"
)

var catalogOpenAIImagesDefaultSizes = []string{
	"1024x1024",
	"1536x1024",
	"1024x1536",
}

// resolveCatalogOpenAIImagesCapability turns an explicit catalog declaration
// into an executable OpenAI-compatible Image Studio profile without using the
// model name as a capability signal. This is intentionally stricter than the
// legacy generic resolver: only the standard endpoint, size and multipart
// edit semantics are available, and provider-specific controls are rejected.
func resolveCatalogOpenAIImagesCapability(contract GatewayModelContract) (ImageStudioModelCapabilities, bool) {
	modelID := strings.TrimSpace(contract.ID)
	version := strings.TrimSpace(contract.CapabilityVersion)
	if modelID == "" || version == "" || contract.ImageCapabilities == nil ||
		!catalogOpenAIImagesAdapterSupportsPlatform(contract.Platform) ||
		!contract.Supports("image", CatalogImageOperationCreate) ||
		!catalogOpenAIImagesProjectedCapabilityValid(contract.ImageCapabilities) {
		return ImageStudioModelCapabilities{}, false
	}

	declared := contract.ImageCapabilities
	capability := ImageStudioModelCapabilities{
		Platform:              PlatformOpenAI,
		ProviderID:            catalogOpenAIImagesAdapterID,
		ProfileID:             catalogOpenAIImagesProfileID(modelID, version),
		Revision:              catalogOpenAIImagesRevision(version),
		Operations:            cloneCatalogStrings(declared.Operations),
		SizingKind:            strings.TrimSpace(declared.SizingKind),
		SupportedSizes:        cloneCatalogStrings(declared.SupportedSizes),
		SupportedAspectRatios: cloneCatalogStrings(declared.SupportedRatios),
		MinDimension:          declared.MinDimension,
		MaxDimension:          declared.MaxDimension,
		DimensionStep:         declared.DimensionStep,
		MaxAspectRatio:        declared.MaxAspectRatio,
		MaxReferenceImages:    declared.MaxReferenceImages,
		// A generic OpenAI-compatible declaration has no provider-specific
		// quality contract. Do not silently accept a quality that will not be
		// placed on the upstream request.
		RejectUndeclaredQuality: true,
	}
	if capability.SizingKind == "" {
		capability.SizingKind = "fixed"
	}
	if capability.SizingKind == "fixed" && len(capability.SupportedSizes) == 0 {
		// The base OpenAI Images contract has three stable dimensions. A catalog
		// declaration may omit size limits to use that conservative default, but
		// it never inherits a name-derived provider profile.
		capability.SupportedSizes = append([]string(nil), catalogOpenAIImagesDefaultSizes...)
	}
	if capability.SizingKind == "custom_dimensions" {
		if imageStudioCustomDimensionsAllowed(capability, defaultImageStudioSize) {
			capability.DefaultSize = defaultImageStudioSize
		} else {
			minimum := ((capability.MinDimension + capability.DimensionStep - 1) / capability.DimensionStep) * capability.DimensionStep
			if minimum > 0 && minimum <= capability.MaxDimension {
				capability.DefaultSize = fmt.Sprintf("%dx%d", minimum, minimum)
			}
		}
	} else if len(capability.SupportedSizes) > 0 {
		capability.DefaultSize = capability.SupportedSizes[0]
	}
	if capability.DefaultSize == "" {
		return ImageStudioModelCapabilities{}, false
	}
	return capability, true
}

func catalogOpenAIImagesProfileID(modelID, version string) string {
	return catalogOpenAIImagesProfilePrefix + strings.ToLower(strings.TrimSpace(modelID)) + ":" + strings.TrimSpace(version)
}

func catalogOpenAIImagesRevision(version string) string {
	return catalogOpenAIImagesRevisionPrefix + strings.TrimSpace(version)
}

// resolveCatalogOpenAIImagesWorkerCapability verifies a queued, encrypted
// generic declaration without rereading the mutable catalog. The envelope is
// pinned to its exact model and administrator-declared capability version;
// changing either causes the worker to fail closed.
func resolveCatalogOpenAIImagesWorkerCapability(
	platform, modelID, profileID, revision string,
) (ImageStudioModelCapabilities, bool) {
	if !strings.EqualFold(strings.TrimSpace(platform), PlatformOpenAI) ||
		!strings.HasPrefix(strings.TrimSpace(revision), catalogOpenAIImagesRevisionPrefix) {
		return ImageStudioModelCapabilities{}, false
	}
	version := strings.TrimPrefix(strings.TrimSpace(revision), catalogOpenAIImagesRevisionPrefix)
	if version == "" {
		return ImageStudioModelCapabilities{}, false
	}
	return ImageStudioModelCapabilities{
		Platform:                PlatformOpenAI,
		ProviderID:              catalogOpenAIImagesAdapterID,
		ProfileID:               catalogOpenAIImagesProfileID(modelID, version),
		Revision:                strings.TrimSpace(revision),
		Operations:              []string{CatalogImageOperationCreate, CatalogImageOperationEdit},
		SizingKind:              "fixed",
		RejectUndeclaredQuality: true,
	}, true
}

func catalogOpenAIImagesAdapterSupportsPlatform(platform string) bool {
	switch normalizeCatalogContractPlatform(platform) {
	case PlatformOpenAI, PlatformComposite:
		return true
	default:
		return false
	}
}

func catalogOpenAIImagesProjectedCapabilityValid(image *ModelImageCapabilities) bool {
	if image == nil || len(image.SupportedFormats) > 0 || image.MaxReferenceImages < 0 ||
		!hasOnlyCatalogOperations(image.Operations, CatalogImageOperationCreate, CatalogImageOperationEdit) {
		return false
	}
	if imageStudioStringAllowed(image.Operations, CatalogImageOperationEdit) && image.MaxReferenceImages <= 0 {
		return false
	}
	if image.MaxReferenceImages > maxImageStudioReferences {
		return false
	}

	sizingKind := strings.TrimSpace(image.SizingKind)
	switch sizingKind {
	case "", "fixed":
		for _, size := range image.SupportedSizes {
			if _, _, ok := parseImageStudioDimensions(size); !ok {
				return false
			}
		}
		return true
	case "custom_dimensions":
		capability := ImageStudioModelCapabilities{
			SizingKind:     sizingKind,
			MinDimension:   image.MinDimension,
			MaxDimension:   image.MaxDimension,
			DimensionStep:  image.DimensionStep,
			MaxAspectRatio: image.MaxAspectRatio,
		}
		minimum := ((capability.MinDimension + capability.DimensionStep - 1) / capability.DimensionStep) * capability.DimensionStep
		return minimum > 0 && imageStudioCustomDimensionsAllowed(capability, fmt.Sprintf("%dx%d", minimum, minimum))
	default:
		return false
	}
}

func modelImageCapabilitiesFromCatalog(capability *CatalogImageMediaCapabilities) *ModelImageCapabilities {
	if capability == nil {
		return nil
	}
	image := ModelImageCapabilities{
		Operations:         cloneCatalogStrings(capability.Operations),
		SizingKind:         strings.TrimSpace(capability.SizingKind),
		SupportedSizes:     cloneCatalogStrings(capability.SupportedSizes),
		SupportedRatios:    cloneCatalogStrings(capability.SupportedAspectRatios),
		SupportedFormats:   cloneCatalogStrings(capability.SupportedOutputFormats),
		MinDimension:       capability.MinDimension,
		MaxDimension:       capability.MaxDimension,
		DimensionStep:      capability.DimensionStep,
		MaxAspectRatio:     capability.MaxAspectRatio,
		MaxReferenceImages: capability.MaxReferenceImages,
	}
	return &image
}

func cloneGatewayVideoCapabilities(capabilities *MobileVideoCapabilities) *MobileVideoCapabilities {
	if capabilities == nil {
		return nil
	}
	clone := capabilities.Clone()
	return &clone
}

// GatewayModelContractFromMobileVideo builds the complete, selected video
// projection used by every managed surface. A composite group can map one
// public model ID to several providers, but its adapter, capability version,
// platform and video limits must always come from the same selected row.
//
// The returned contract intentionally contains only the selected video mode.
// A single top-level adapter/version cannot safely represent an image contract
// selected from a different provider for the same public ID. Callers may keep
// an existing image mode only when it already has the same adapter and version.
func GatewayModelContractFromMobileVideo(model MobileVideoResolvedModel) (GatewayModelContract, bool) {
	modelID := strings.TrimSpace(model.Model)
	if modelID == "" {
		modelID = strings.TrimSpace(model.ID)
	}
	adapter := strings.TrimSpace(model.Adapter)
	adapterPlatform, supported := mobileVideoAdapterPlatform(adapter)
	if modelID == "" || !supported || !model.Capabilities.SupportsOperation("generate") {
		return GatewayModelContract{}, false
	}
	platform := normalizeCatalogContractPlatform(model.Platform)
	if platform == "" {
		platform = adapterPlatform
	}
	if platform != adapterPlatform {
		return GatewayModelContract{}, false
	}
	version := strings.TrimSpace(model.CapabilityVersion)
	if version == "" {
		version = MobileVideoCapabilitiesVersion
	}
	capabilities := cloneGatewayVideoCapabilities(&model.Capabilities)
	if capabilities == nil {
		return GatewayModelContract{}, false
	}
	return GatewayModelContract{
		ID:                modelID,
		Platform:          platform,
		Modalities:        []string{"video"},
		VideoCapabilities: capabilities,
		Adapter:           adapter,
		CapabilityVersion: version,
	}, true
}

// ResolveGatewayExecutableVideoContracts is the shared video selection path
// for /v1/models and managed workspace responses. Authorization is performed
// by the caller; this method only evaluates catalog, mapping, scheduler,
// adapter and billing prerequisites for that exact group.
func (s *ModelCatalogService) ResolveGatewayExecutableVideoContracts(
	ctx context.Context,
	group Group,
	models GatewayModelAvailabilityResolver,
) (map[string]GatewayModelContract, error) {
	if s == nil || models == nil {
		return nil, ErrMobileVideoCapabilityUnavailable
	}
	resolver := NewCatalogMobileVideoAvailabilityResolver(nil, s, models)
	resolvedGroup, err := resolver.ResolveGroup(ctx, group)
	if err != nil {
		return nil, err
	}
	contracts := make(map[string]GatewayModelContract, len(resolvedGroup.Models))
	for _, model := range resolvedGroup.Models {
		contract, ok := GatewayModelContractFromMobileVideo(model)
		if !ok {
			continue
		}
		contracts[strings.ToLower(strings.TrimSpace(contract.ID))] = contract
	}
	return contracts, nil
}

func (s *ModelCatalogService) catalogMediaDeclarationHasExecutableAdapter(
	ctx context.Context,
	modelID string,
	group Group,
	raw []byte,
	declaration *CatalogMediaCapabilities,
) bool {
	if declaration == nil {
		return false
	}
	for _, modality := range normalizedCatalogModalities(declaration.Modalities) {
		switch modality {
		case "image":
			if !GroupAllowsImageGeneration(&group) ||
				declaration.Image == nil ||
				!s.catalogImageBillingConfigured(ctx, group, modelID) ||
				!catalogImageAdapterSupportsModel(declaration.Adapter, group.Platform, modelID) ||
				!catalogImageAdapterSupportsCapabilities(declaration.Adapter, modelID, declaration.Image) {
				return false
			}
		case "video":
			// Use the same parser and execution prerequisites as the mobile video
			// worker. A declaration with a noncanonical operation or incomplete
			// size/ratio/duration tuple must not make Canvas advertise a video
			// group that the worker will immediately reject.
			capabilities, adapter, supported := mobileVideoCapabilitiesFromCatalog(raw)
			adapterPlatform, platformMatches := mobileVideoAdapterPlatform(adapter)
			if declaration.Video == nil || !supported || !platformMatches ||
				!mobileVideoAdapterMatchesGroup(adapterPlatform, group.Platform) ||
				!mobileVideoAdapterSupportsCapabilities(adapter, capabilities) {
				return false
			}
			// Keep the managed workspace and /v1/models contract aligned with
			// mobile video bootstrap: the catalog declaration's group binding,
			// adapter and explicit unit price define the executable video contract.
			// The image-generation flag remains image-only and cannot suppress a
			// separately declared video model.
			if len(mobileVideoUnitPrices(group, modelID, capabilities)) == 0 {
				return false
			}
		default:
			// Chat routing is already proven by the mapped/schedulable model list.
			// There is no managed audio execution adapter in this release, so an
			// audio declaration cannot be published as executable yet. Unknown
			// modalities fail closed until a dedicated adapter is registered.
			if modality != "chat" {
				return false
			}
		}
	}
	return true
}

// catalogImageBillingConfigured is the pricing half of the media contract.
// A catalog declaration may narrow an adapter's capabilities, but it must not
// turn the historical Image Studio $0.04 estimate fallback into a public
// promise that a model is billable. Explicit group image prices take priority;
// otherwise reuse the same model-pricing resolver used by image settlement.
func (s *ModelCatalogService) catalogImageBillingConfigured(ctx context.Context, group Group, modelID string) bool {
	for _, price := range []*float64{group.ImagePrice1K, group.ImagePrice2K, group.ImagePrice4K} {
		if catalogMediaPriceValid(price) {
			return true
		}
	}

	resolver := NewModelPricingResolver(s.channelService, s.billingService)
	resolved := resolver.Resolve(ctx, PricingInput{
		Model:   modelID,
		GroupID: &group.ID,
		Group:   &group,
	})
	return catalogResolvedImagePricingValid(resolved)
}

func catalogResolvedImagePricingValid(resolved *ResolvedPricing) bool {
	if resolved == nil {
		return false
	}
	switch resolved.Mode {
	case BillingModeImage, BillingModePerRequest:
		if resolved.channelPricing != nil && catalogMediaPriceValid(resolved.channelPricing.PerRequestPrice) {
			return true
		}
		for _, tier := range resolved.RequestTiers {
			if catalogMediaPriceValid(tier.PerRequestPrice) {
				return true
			}
		}
		return false
	case BillingModeToken:
		if resolved.channelPricing != nil && catalogMediaPriceValid(resolved.channelPricing.ImageOutputPrice) {
			return true
		}
		return resolved.BasePricing != nil &&
			resolved.BasePricing.ImageOutputPriceExplicit &&
			catalogMediaPriceFinite(resolved.BasePricing.ImageOutputPricePerToken)
	default:
		return false
	}
}

func catalogMediaPriceValid(price *float64) bool {
	return price != nil && catalogMediaPriceFinite(*price)
}

func catalogMediaPriceFinite(price float64) bool {
	return !math.IsNaN(price) && !math.IsInf(price, 0) && price >= 0
}

func catalogImageAdapterSupportsModel(adapterID, groupPlatform, modelID string) bool {
	adapterID = strings.ToLower(strings.TrimSpace(adapterID))
	if adapter, found := findImageModelAdapter(modelID); found {
		return strings.EqualFold(adapter.ID(), adapterID)
	}
	if adapterID == catalogOpenAIImagesAdapterID {
		return catalogOpenAIImagesAdapterSupportsPlatform(groupPlatform)
	}

	var platform string
	switch adapterID {
	case "gemini_images":
		platform = PlatformGemini
	case "grok_images":
		platform = PlatformGrok
	default:
		return false
	}
	if normalizedGroup := normalizeCatalogContractPlatform(groupPlatform); normalizedGroup != "" &&
		normalizedGroup != PlatformComposite && normalizedGroup != platform {
		return false
	}
	capability, supported := ResolveImageStudioProviderCapability(platform, modelID)
	return supported && normalizeCatalogContractPlatform(capability.Platform) == platform
}

func catalogImageAdapterSupportsCapabilities(
	adapterID, modelID string,
	image *CatalogImageMediaCapabilities,
) bool {
	return validateCatalogImageAdapterCapabilities(adapterID, modelID, image) == nil
}

// validateCatalogImageAdapterCapabilities verifies an administrator-declared
// catalog shape against an exact adapter's intrinsic profile. A catalog row is
// allowed to narrow a provider capability, but never to turn a known adapter
// into a different one. This is used both while saving and while projecting
// old rows so a historical manual write cannot make clients advertise an
// operation the worker will reject later.
func validateCatalogImageAdapterCapabilities(
	adapterID, modelID string,
	image *CatalogImageMediaCapabilities,
) error {
	if image == nil {
		return fmt.Errorf("image capabilities are required")
	}
	if strings.EqualFold(strings.TrimSpace(adapterID), catalogOpenAIImagesAdapterID) {
		return validateCatalogOpenAIImagesCapabilities(image)
	}

	adapter, registered := findImageModelAdapter(modelID)
	if !registered {
		// Non-exact adapters retain their existing provider-specific validation.
		// Their executable profiles do not have a one-model intrinsic registry to
		// compare against here.
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(adapter.ID()), strings.TrimSpace(adapterID)) {
		return fmt.Errorf("adapter %q does not match registered adapter %q", adapterID, adapter.ID())
	}
	intrinsic, supported := adapter.ResolveCapability(modelID)
	if !supported {
		return fmt.Errorf("registered adapter %q has no capability profile for model %q", adapter.ID(), modelID)
	}
	return validateCatalogImageCapabilitiesNarrowing(intrinsic, image)
}

func validateCatalogImageCapabilitiesNarrowing(
	intrinsic ImageStudioModelCapabilities,
	declared *CatalogImageMediaCapabilities,
) error {
	if declared == nil {
		return fmt.Errorf("image capabilities are required")
	}
	if !catalogCapabilityStringsAreSubset(declared.Operations, intrinsic.Operations) {
		return fmt.Errorf("declared image operations exceed the adapter profile")
	}
	if sizingKind := strings.ToLower(strings.TrimSpace(declared.SizingKind)); sizingKind != "" &&
		sizingKind != strings.ToLower(strings.TrimSpace(intrinsic.SizingKind)) {
		return fmt.Errorf("declared image sizing kind %q does not match adapter profile %q", declared.SizingKind, intrinsic.SizingKind)
	}
	if !catalogCapabilityStringsAreSubset(declared.SupportedSizes, intrinsic.SupportedSizes) {
		return fmt.Errorf("declared image sizes exceed the adapter profile")
	}
	if !catalogCapabilityStringsAreSubset(declared.SupportedAspectRatios, intrinsic.SupportedAspectRatios) {
		return fmt.Errorf("declared image aspect ratios exceed the adapter profile")
	}
	if !catalogCapabilityStringsAreSubset(declared.SupportedOutputFormats, intrinsic.SupportedOutputFormats) {
		return fmt.Errorf("declared image output formats exceed the adapter profile")
	}
	if declared.MaxReferenceImages > intrinsic.MaxReferenceImages {
		return fmt.Errorf("declared image reference limit exceeds the adapter profile")
	}

	if strings.EqualFold(strings.TrimSpace(intrinsic.SizingKind), "custom_dimensions") {
		return validateCatalogCustomDimensionNarrowing(intrinsic, declared)
	}
	if declared.MinDimension != 0 || declared.MaxDimension != 0 ||
		declared.DimensionStep != 0 || declared.MaxAspectRatio != 0 {
		return fmt.Errorf("declared custom dimension limits are incompatible with adapter sizing kind %q", intrinsic.SizingKind)
	}
	return nil
}

func validateCatalogCustomDimensionNarrowing(
	intrinsic ImageStudioModelCapabilities,
	declared *CatalogImageMediaCapabilities,
) error {
	if len(declared.SupportedSizes) > 0 || len(declared.SupportedAspectRatios) > 0 {
		return fmt.Errorf("declared fixed sizes or aspect ratios are incompatible with custom dimensions")
	}
	if intrinsic.MinDimension <= 0 || intrinsic.MaxDimension < intrinsic.MinDimension || intrinsic.DimensionStep <= 0 {
		return fmt.Errorf("adapter custom dimension profile is invalid")
	}

	minimum := intrinsic.MinDimension
	if declared.MinDimension > 0 {
		if declared.MinDimension < intrinsic.MinDimension || declared.MinDimension > intrinsic.MaxDimension {
			return fmt.Errorf("declared minimum dimension exceeds the adapter profile")
		}
		minimum = declared.MinDimension
	}
	maximum := intrinsic.MaxDimension
	if declared.MaxDimension > 0 {
		if declared.MaxDimension < intrinsic.MinDimension || declared.MaxDimension > intrinsic.MaxDimension {
			return fmt.Errorf("declared maximum dimension exceeds the adapter profile")
		}
		maximum = declared.MaxDimension
	}
	if minimum > maximum {
		return fmt.Errorf("declared image dimensions are empty")
	}

	step := intrinsic.DimensionStep
	if declared.DimensionStep > 0 {
		if declared.DimensionStep < intrinsic.DimensionStep || declared.DimensionStep%intrinsic.DimensionStep != 0 {
			return fmt.Errorf("declared dimension step expands the adapter profile")
		}
		step = declared.DimensionStep
	}
	if firstAllowed := ((minimum + step - 1) / step) * step; firstAllowed > maximum {
		return fmt.Errorf("declared image dimensions contain no supported size")
	}

	if declared.MaxAspectRatio > 0 &&
		(intrinsic.MaxAspectRatio <= 0 || declared.MaxAspectRatio > intrinsic.MaxAspectRatio) {
		return fmt.Errorf("declared maximum aspect ratio exceeds the adapter profile")
	}
	return nil
}

func catalogCapabilityStringsAreSubset(declared, intrinsic []string) bool {
	if len(declared) == 0 {
		return true
	}
	allowed := make(map[string]struct{}, len(intrinsic))
	for _, value := range intrinsic {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			allowed[value] = struct{}{}
		}
	}
	for _, value := range declared {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return false
		}
		if _, supported := allowed[value]; !supported {
			return false
		}
	}
	return true
}

func validateCatalogOpenAIImagesCapabilities(image *CatalogImageMediaCapabilities) error {
	if image == nil {
		return fmt.Errorf("openai_images requires image capabilities")
	}
	projected := &ModelImageCapabilities{
		Operations:         cloneCatalogStrings(image.Operations),
		SizingKind:         strings.TrimSpace(image.SizingKind),
		SupportedSizes:     cloneCatalogStrings(image.SupportedSizes),
		SupportedRatios:    cloneCatalogStrings(image.SupportedAspectRatios),
		SupportedFormats:   cloneCatalogStrings(image.SupportedOutputFormats),
		MinDimension:       image.MinDimension,
		MaxDimension:       image.MaxDimension,
		DimensionStep:      image.DimensionStep,
		MaxAspectRatio:     image.MaxAspectRatio,
		MaxReferenceImages: image.MaxReferenceImages,
	}
	if !catalogOpenAIImagesProjectedCapabilityValid(projected) {
		return fmt.Errorf("openai_images capabilities require standard sizes, supported create/edit semantics, and no provider-specific output format")
	}
	return nil
}

// ValidateCatalogMediaCapabilitiesAdapter verifies only adapter executability
// at save time. Group authorization and pricing are runtime properties: a
// catalog row may target several groups, while image billing deliberately has
// an established default-price fallback when a group has no explicit override.
func ValidateCatalogMediaCapabilitiesAdapter(entry *SiteModelCatalogEntry) error {
	if entry == nil {
		return fmt.Errorf("catalog entry is required")
	}
	declaration, err := ParseCatalogMediaCapabilities(entry.MediaCapabilities)
	if err != nil {
		return err
	}
	if declaration == nil {
		return nil
	}

	modelID := strings.TrimSpace(entry.ModelName)
	platform := normalizeCatalogContractPlatform(entry.Platform)
	for _, modality := range normalizedCatalogModalities(declaration.Modalities) {
		switch modality {
		case "image":
			if declaration.Image == nil ||
				!catalogImageAdapterSupportsModel(declaration.Adapter, platform, modelID) {
				return fmt.Errorf("media capabilities adapter %q cannot execute image model %q", declaration.Adapter, modelID)
			}
			if err := validateCatalogImageAdapterCapabilities(declaration.Adapter, modelID, declaration.Image); err != nil {
				return fmt.Errorf("media capabilities adapter %q cannot execute image model %q: %w", declaration.Adapter, modelID, err)
			}
		case "video":
			capabilities, adapter, supported := mobileVideoCapabilitiesFromCatalog(entry.MediaCapabilities)
			adapterPlatform, platformMatches := mobileVideoAdapterPlatform(adapter)
			if declaration.Video == nil || !supported || !platformMatches ||
				!mobileVideoAdapterMatchesGroup(adapterPlatform, platform) ||
				!mobileVideoAdapterSupportsCapabilities(adapter, capabilities) {
				return fmt.Errorf("media capabilities adapter %q cannot execute video model %q", declaration.Adapter, modelID)
			}
		case "chat":
			// Chat execution is proved by the account mapping and scheduler; the
			// media registry owns only additional media adapter checks.
		default:
			return fmt.Errorf("media capabilities modality %q has no executable adapter", modality)
		}
	}
	return nil
}

func normalizedCatalogModalities(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func cloneCatalogStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}
