package service

import (
	"context"
	"errors"
	"sort"
	"strings"
)

// MobileVideoCapabilitiesVersion identifies the server-owned contract returned
// to mobile clients. It changes only when the response shape changes.
const MobileVideoCapabilitiesVersion = "2026-08-26.1"

const (
	MobileVideoSuppressionNotMapped             = "not_mapped"
	MobileVideoSuppressionCapabilityNotDeclared = "capability_not_declared"
	MobileVideoSuppressionPriceMissing          = "price_missing"
	MobileVideoSuppressionAdapterUnsupported    = "adapter_unsupported"
	MobileVideoSuppressionNoSchedulableAccount  = "no_schedulable_account"
	MobileVideoSuppressionGroupPermissionDenied = "group_permission_denied"
	// Mobile video currently reserves wallet balance before a task is
	// activated. Subscription quota reservation has different settlement
	// semantics and must be implemented as its own contract rather than
	// falling through to a cash hold.
	MobileVideoSuppressionSubscriptionReservationUnsupported = "subscription_reservation_unsupported"
)

// MobileVideoSuppressedModel is diagnostic metadata for an authorized group.
// It never changes group membership, mappings, prices, or the public task
// payload; clients can use it to show a truthful unavailable state.
type MobileVideoSuppressedModel struct {
	Model string `json:"model"`
	Code  string `json:"code"`
}

// MobileVideoResolvedModel is the fully authorized, executable model
// declaration. It is constructed only after the catalog declaration, mapping,
// schedulable account and pricing checks all pass.
type MobileVideoResolvedModel struct {
	// ID and Name match the common workspace model contract. Model remains for
	// already-released mobile clients that consumed the earlier video bootstrap.
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Model      string   `json:"model"`
	Modalities []string `json:"modalities"`
	// Platform is the concrete upstream selected for this model. It stays
	// separate from GroupPlatform because a composite group can route to more
	// than one provider.
	Platform          string                  `json:"platform"`
	CapabilityVersion string                  `json:"capability_version"`
	GroupID           int64                   `json:"group_id"`
	GroupName         string                  `json:"group_name"`
	GroupPlatform     string                  `json:"group_platform"`
	Adapter           string                  `json:"adapter"`
	Capabilities      MobileVideoCapabilities `json:"video_capabilities"`
	UnitPrices        map[string]float64      `json:"unit_prices,omitempty"`
}

func (m MobileVideoResolvedModel) PriceForResolution(resolution string) *float64 {
	for key, value := range m.UnitPrices {
		if strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(resolution)) {
			price := value
			return &price
		}
	}
	return nil
}

type MobileVideoAvailableGroup struct {
	ID                   int64                        `json:"id"`
	Name                 string                       `json:"name"`
	Platform             string                       `json:"platform"`
	VideoAvailable       bool                         `json:"video_available"`
	VideoUnavailableCode string                       `json:"video_unavailable_code,omitempty"`
	Models               []MobileVideoResolvedModel   `json:"models"`
	Suppressed           []MobileVideoSuppressedModel `json:"suppressed,omitempty"`
}

type MobileVideoBootstrap struct {
	ProtocolVersion     int                         `json:"protocol_version"`
	CapabilitiesVersion string                      `json:"capabilities_version"`
	Groups              []MobileVideoAvailableGroup `json:"groups"`
}

// MobileVideoAvailabilityResolver is the single source of truth consumed by
// mobile video routes. Implementations must fail closed rather than infer a
// media capability from a model name, group name, or default model setting.
type MobileVideoAvailabilityResolver interface {
	Bootstrap(context.Context, int64) (*MobileVideoBootstrap, error)
	Resolve(context.Context, int64, int64, string) (*MobileVideoResolvedModel, error)
}

type mobileVideoSelectableGroupProvider interface {
	GetNextChatSelectableGroups(context.Context, int64) ([]Group, error)
}

type mobileVideoCatalogProvider interface {
	ListCatalog(context.Context, CatalogListFilter) ([]SiteModelCatalogEntry, error)
}

// GatewayModelAvailabilityResolver is implemented by GatewayService. It is the
// minimum scheduler view required to decide whether a declared video model can
// be executed: the model must be mapped on the adapter's concrete platform and
// that platform must currently have a schedulable account.
//
// Keep this separate from callers that only need a model list. Those callers
// can preserve legacy chat/image behavior while managed video fails closed.
type GatewayModelAvailabilityResolver interface {
	GetAvailableModels(context.Context, *int64, string) []string
	GetSchedulablePlatforms(context.Context, *int64) map[string]struct{}
}

// CatalogMobileVideoAvailabilityResolver intersects the administrator-owned
// catalog declaration with the user's authorized groups and current scheduler
// inventory. It never uses a model-name regular expression or models_list as a
// media capability fallback.
type CatalogMobileVideoAvailabilityResolver struct {
	groups  mobileVideoSelectableGroupProvider
	catalog mobileVideoCatalogProvider
	models  GatewayModelAvailabilityResolver
}

func NewCatalogMobileVideoAvailabilityResolver(
	groups mobileVideoSelectableGroupProvider,
	catalog mobileVideoCatalogProvider,
	models GatewayModelAvailabilityResolver,
) *CatalogMobileVideoAvailabilityResolver {
	return &CatalogMobileVideoAvailabilityResolver{groups: groups, catalog: catalog, models: models}
}

func (r *CatalogMobileVideoAvailabilityResolver) Bootstrap(ctx context.Context, userID int64) (*MobileVideoBootstrap, error) {
	if r == nil || r.groups == nil || r.catalog == nil || r.models == nil || userID <= 0 {
		return nil, ErrMobileVideoCapabilityUnavailable
	}
	groups, err := r.groups.GetNextChatSelectableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Media declarations are a user-facing capability contract. A mapped
	// account alone is not permission to disclose or execute a catalog row that
	// administrators hid from authenticated users.
	visibleAuth := true
	entries, err := r.catalog.ListCatalog(ctx, CatalogListFilter{VisibleAuth: &visibleAuth})
	if err != nil {
		return nil, err
	}
	// A hidden catalog declaration is an explicit instruction not to disclose a
	// model to authenticated clients. Fetch it only inside this server-side
	// resolver so an old account mapping cannot reintroduce its name through
	// the diagnostic list.
	hiddenAuth := false
	hiddenEntries, err := r.catalog.ListCatalog(ctx, CatalogListFilter{VisibleAuth: &hiddenAuth})
	if err != nil {
		return nil, err
	}

	result := &MobileVideoBootstrap{
		ProtocolVersion:     1,
		CapabilitiesVersion: MobileVideoCapabilitiesVersion,
		Groups:              make([]MobileVideoAvailableGroup, 0, len(groups)),
	}
	for _, group := range groups {
		result.Groups = append(result.Groups, r.resolveGroup(ctx, group, entries, hiddenEntries))
	}
	return result, nil
}

// ResolveGroup resolves the executable video declaration for an already
// authorized group without creating or inspecting a managed session. Gateway
// and workspace callers use it to project the exact same selected contract as
// the mobile bootstrap. Keeping this selection here prevents each surface from
// choosing a different provider when a composite group maps one public model
// ID on multiple upstreams.
func (r *CatalogMobileVideoAvailabilityResolver) ResolveGroup(
	ctx context.Context,
	group Group,
) (MobileVideoAvailableGroup, error) {
	if r == nil || r.catalog == nil || r.models == nil || group.ID <= 0 {
		return MobileVideoAvailableGroup{}, ErrMobileVideoCapabilityUnavailable
	}
	visibleAuth := true
	entries, err := r.catalog.ListCatalog(ctx, CatalogListFilter{VisibleAuth: &visibleAuth})
	if err != nil {
		return MobileVideoAvailableGroup{}, err
	}
	hiddenAuth := false
	hiddenEntries, err := r.catalog.ListCatalog(ctx, CatalogListFilter{VisibleAuth: &hiddenAuth})
	if err != nil {
		return MobileVideoAvailableGroup{}, err
	}
	return r.resolveGroup(ctx, group, entries, hiddenEntries), nil
}

func (r *CatalogMobileVideoAvailabilityResolver) Resolve(ctx context.Context, userID, groupID int64, model string) (*MobileVideoResolvedModel, error) {
	model = strings.TrimSpace(model)
	if model == "" || groupID <= 0 {
		return nil, ErrMobileVideoModelUnavailable
	}
	bootstrap, err := r.Bootstrap(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, group := range bootstrap.Groups {
		if group.ID != groupID {
			continue
		}
		if code := strings.TrimSpace(group.VideoUnavailableCode); code != "" {
			return nil, mobileVideoSuppressionError(code)
		}
		for index := range group.Models {
			if strings.EqualFold(group.Models[index].Model, model) {
				resolved := group.Models[index]
				return &resolved, nil
			}
		}
		for _, suppressed := range group.Suppressed {
			if strings.EqualFold(suppressed.Model, model) {
				return nil, mobileVideoSuppressionError(suppressed.Code)
			}
		}
		return nil, ErrMobileVideoModelUnavailable
	}
	return nil, ErrMobileVideoCapabilityUnavailable
}

func (r *CatalogMobileVideoAvailabilityResolver) resolveGroup(
	ctx context.Context,
	group Group,
	entries []SiteModelCatalogEntry,
	hiddenEntries []SiteModelCatalogEntry,
) MobileVideoAvailableGroup {
	result := MobileVideoAvailableGroup{
		ID: group.ID, Name: group.Name, Platform: group.Platform,
		Models: []MobileVideoResolvedModel{}, Suppressed: []MobileVideoSuppressedModel{},
	}
	if group.IsSubscriptionType() {
		result.VideoUnavailableCode = MobileVideoSuppressionSubscriptionReservationUnsupported
		return result
	}

	schedulablePlatforms := r.models.GetSchedulablePlatforms(ctx, &group.ID)
	platformsToInspect := mobileVideoCandidatePlatforms(group, entries, schedulablePlatforms)
	mappedByPlatform := make(map[string]map[string]string, len(platformsToInspect))
	candidates := make(map[string]string)
	for _, platform := range platformsToInspect {
		mapped := r.models.GetAvailableModels(ctx, &group.ID, platform)
		byName := make(map[string]string, len(mapped))
		for _, model := range mapped {
			model = strings.TrimSpace(model)
			if model == "" || !mobileVideoModelAllowedByGroup(group, model) {
				continue
			}
			key := strings.ToLower(model)
			byName[key] = model
			candidates[key] = model
		}
		mappedByPlatform[platform] = byName
	}
	for _, entry := range entries {
		if !catalogVideoEntryAppliesToGroup(entry, group) {
			continue
		}
		model := strings.TrimSpace(entry.ModelName)
		if model != "" && mobileVideoModelAllowedByGroup(group, model) {
			key := strings.ToLower(model)
			if _, exists := candidates[key]; !exists {
				candidates[key] = model
			}
		}
	}

	keys := make([]string, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		model := candidates[key]
		entry := findMobileVideoCatalogEntry(entries, group, model, mappedByPlatform, schedulablePlatforms)
		if entry == nil {
			if findMobileVideoCatalogEntry(hiddenEntries, group, model, mappedByPlatform, schedulablePlatforms) != nil {
				// Do not reveal a catalog row an administrator deliberately hid.
				continue
			}
			if mobileVideoMappedOnAnyPlatform(mappedByPlatform, key) {
				result.Suppressed = append(result.Suppressed, MobileVideoSuppressedModel{Model: model, Code: MobileVideoSuppressionCapabilityNotDeclared})
			} else if !mobileVideoGroupHasRequiredAccount(group, schedulablePlatforms, "") {
				result.Suppressed = append(result.Suppressed, MobileVideoSuppressedModel{Model: model, Code: MobileVideoSuppressionNoSchedulableAccount})
			} else {
				result.Suppressed = append(result.Suppressed, MobileVideoSuppressedModel{Model: model, Code: MobileVideoSuppressionNotMapped})
			}
			continue
		}
		capabilities, adapter, valid := mobileVideoCapabilitiesFromCatalog(entry.MediaCapabilities)
		if !valid {
			result.Suppressed = append(result.Suppressed, MobileVideoSuppressedModel{Model: model, Code: MobileVideoSuppressionCapabilityNotDeclared})
			continue
		}
		platform, supportedAdapter := mobileVideoAdapterPlatform(adapter)
		if !supportedAdapter ||
			!mobileVideoCatalogEntryMatchesAdapterPlatform(*entry, platform) ||
			!mobileVideoAdapterMatchesGroup(platform, group.Platform) ||
			!mobileVideoAdapterSupportsCapabilities(adapter, capabilities) {
			result.Suppressed = append(result.Suppressed, MobileVideoSuppressedModel{Model: model, Code: MobileVideoSuppressionAdapterUnsupported})
			continue
		}
		if _, hasAccount := schedulablePlatforms[platform]; !hasAccount {
			result.Suppressed = append(result.Suppressed, MobileVideoSuppressedModel{Model: model, Code: MobileVideoSuppressionNoSchedulableAccount})
			continue
		}
		if _, mapped := mappedByPlatform[platform][key]; !mapped {
			result.Suppressed = append(result.Suppressed, MobileVideoSuppressedModel{Model: model, Code: MobileVideoSuppressionNotMapped})
			continue
		}
		prices := mobileVideoUnitPrices(group, model, capabilities)
		if len(prices) == 0 {
			result.Suppressed = append(result.Suppressed, MobileVideoSuppressedModel{Model: model, Code: MobileVideoSuppressionPriceMissing})
			continue
		}
		// A client may only select a tier that is both declared and billable for
		// this exact group/model. Returning an unpriced declaration creates a
		// misleading UI and an avoidable job-time rejection.
		capabilities.SupportedResolutions = mobileVideoPricedResolutions(capabilities.SupportedResolutions, prices)
		result.Models = append(result.Models, MobileVideoResolvedModel{
			ID: model, Name: mobileVideoCatalogDisplayName(entry, model), Model: model,
			Modalities: []string{"video"}, Platform: platform, CapabilityVersion: MobileVideoCapabilitiesVersion,
			GroupID: group.ID, GroupName: group.Name, GroupPlatform: group.Platform,
			Adapter: adapter, Capabilities: capabilities, UnitPrices: prices,
		})
	}
	result.VideoAvailable = len(result.Models) > 0
	if !result.VideoAvailable && len(result.Suppressed) > 0 {
		result.VideoUnavailableCode = result.Suppressed[0].Code
	}
	return result
}

func mobileVideoCatalogDisplayName(entry *SiteModelCatalogEntry, fallback string) string {
	if entry != nil && entry.DisplayName != nil {
		if value := strings.TrimSpace(*entry.DisplayName); value != "" {
			return value
		}
	}
	return strings.TrimSpace(fallback)
}

// mobileVideoCandidatePlatforms identifies only concrete provider platforms
// that can safely be asked for a group's model mappings. Composite is not a
// provider: its concrete target always comes from an explicitly declared
// adapter, never from a model-name heuristic.
func mobileVideoCandidatePlatforms(group Group, entries []SiteModelCatalogEntry, schedulable map[string]struct{}) []string {
	platforms := make(map[string]struct{})
	groupPlatform := strings.TrimSpace(group.Platform)
	if groupPlatform != "" && groupPlatform != PlatformComposite {
		platforms[groupPlatform] = struct{}{}
	} else if groupPlatform == PlatformComposite {
		for platform := range schedulable {
			platform = strings.TrimSpace(platform)
			if platform != "" && platform != PlatformComposite {
				platforms[platform] = struct{}{}
			}
		}
		for _, entry := range entries {
			if !catalogVideoEntryAppliesToGroup(entry, group) {
				continue
			}
			_, adapter, valid := mobileVideoCapabilitiesFromCatalog(entry.MediaCapabilities)
			if platform, supported := mobileVideoAdapterPlatform(adapter); valid && supported {
				platforms[platform] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(platforms))
	for platform := range platforms {
		result = append(result, platform)
	}
	sort.Strings(result)
	return result
}

func mobileVideoAdapterPlatform(adapter string) (string, bool) {
	switch strings.TrimSpace(adapter) {
	case MobileVideoAdapterGrok:
		return PlatformGrok, true
	case MobileVideoAdapterAgnes:
		return PlatformOpenAI, true
	default:
		return "", false
	}
}

func mobileVideoAdapterMatchesGroup(adapterPlatform, groupPlatform string) bool {
	groupPlatform = normalizeCatalogContractPlatform(groupPlatform)
	return groupPlatform == PlatformComposite || groupPlatform == adapterPlatform
}

// mobileVideoModelAllowedByGroup mirrors the gateway visibility rule. A mobile
// video bootstrap is an authorization response, so the shared group allowlist
// must be honored before a catalog row becomes a candidate or diagnostic entry.
func mobileVideoModelAllowedByGroup(group Group, model string) bool {
	if !group.ModelAllowlistEnabled() {
		return true
	}
	return group.ModelAllowlist.Allows(strings.TrimSpace(model))
}

func mobileVideoMappedOnAnyPlatform(mapped map[string]map[string]string, key string) bool {
	for _, byName := range mapped {
		if _, found := byName[key]; found {
			return true
		}
	}
	return false
}

func mobileVideoGroupHasRequiredAccount(group Group, schedulable map[string]struct{}, adapterPlatform string) bool {
	platform := strings.TrimSpace(adapterPlatform)
	if platform == "" {
		platform = strings.TrimSpace(group.Platform)
	}
	if platform == "" || platform == PlatformComposite {
		return false
	}
	_, found := schedulable[platform]
	return found
}

func catalogVideoEntryAppliesToGroup(entry SiteModelCatalogEntry, group Group) bool {
	if strings.TrimSpace(entry.ModelName) == "" {
		return false
	}
	return catalogVideoEntryBoundToGroup(entry, group) && catalogVideoEntryPlatformMatchesGroup(entry, group)
}

// catalogVideoEntryBoundToGroup handles explicit catalog membership only. An
// explicit group binding does not make a declaration executable on a different
// concrete provider platform: model names can legitimately be reused by two
// upstreams in site_model_catalog.
func catalogVideoEntryBoundToGroup(entry SiteModelCatalogEntry, group Group) bool {
	return catalogAllowsGroup(entry, group.ID, group.Platform)
}

func catalogVideoEntryPlatformMatchesGroup(entry SiteModelCatalogEntry, group Group) bool {
	groupPlatform := normalizeCatalogContractPlatform(group.Platform)
	if groupPlatform == PlatformComposite {
		return true
	}
	entryPlatform := normalizeCatalogContractPlatform(entry.Platform)
	return entryPlatform != "" && entryPlatform == groupPlatform
}

// mobileVideoCatalogSelection makes catalog lookup independent of repository
// return order. Composite groups may have the same model declared for several
// concrete upstreams, so the declaration that is both mapped and schedulable
// on its adapter platform wins. Invalid declarations still participate as a
// fallback so callers can return a precise suppression reason instead of
// silently treating a declared model as absent.
type mobileVideoCatalogSelection struct {
	entry                 *SiteModelCatalogEntry
	platform              string
	adapter               string
	hasMapping            bool
	hasSchedulableAccount bool
	adapterMatchesCatalog bool
}

func findMobileVideoCatalogEntry(
	entries []SiteModelCatalogEntry,
	group Group,
	model string,
	mappedByPlatform map[string]map[string]string,
	schedulablePlatforms map[string]struct{},
) *SiteModelCatalogEntry {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	key := strings.ToLower(model)
	selections := make([]mobileVideoCatalogSelection, 0, 1)
	for index := range entries {
		entry := &entries[index]
		if !strings.EqualFold(strings.TrimSpace(entry.ModelName), model) || !catalogVideoEntryAppliesToGroup(*entry, group) {
			continue
		}

		platform := normalizeCatalogContractPlatform(entry.Platform)
		_, adapter, valid := mobileVideoCapabilitiesFromCatalog(entry.MediaCapabilities)
		adapterPlatform, adapterSupported := mobileVideoAdapterPlatform(adapter)
		adapterMatchesCatalog := valid && adapterSupported && mobileVideoCatalogEntryMatchesAdapterPlatform(*entry, adapterPlatform)
		if valid && adapterSupported {
			platform = adapterPlatform
		}
		_, hasMapping := mappedByPlatform[platform][key]
		_, hasSchedulableAccount := schedulablePlatforms[platform]
		selections = append(selections, mobileVideoCatalogSelection{
			entry: entry, platform: platform, adapter: adapter,
			hasMapping: hasMapping, hasSchedulableAccount: hasSchedulableAccount,
			adapterMatchesCatalog: adapterMatchesCatalog,
		})
	}
	if len(selections) == 0 {
		return nil
	}
	sort.Slice(selections, func(left, right int) bool {
		leftRank, rightRank := mobileVideoCatalogSelectionRank(selections[left]), mobileVideoCatalogSelectionRank(selections[right])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if selections[left].platform != selections[right].platform {
			return selections[left].platform < selections[right].platform
		}
		if selections[left].adapter != selections[right].adapter {
			return selections[left].adapter < selections[right].adapter
		}
		leftCatalogPlatform := normalizeCatalogContractPlatform(selections[left].entry.Platform)
		rightCatalogPlatform := normalizeCatalogContractPlatform(selections[right].entry.Platform)
		if leftCatalogPlatform != rightCatalogPlatform {
			return leftCatalogPlatform < rightCatalogPlatform
		}
		if selections[left].entry.ID != selections[right].entry.ID {
			return selections[left].entry.ID < selections[right].entry.ID
		}
		return string(selections[left].entry.MediaCapabilities) < string(selections[right].entry.MediaCapabilities)
	})
	return selections[0].entry
}

func mobileVideoCatalogSelectionRank(selection mobileVideoCatalogSelection) int {
	if selection.hasMapping && selection.hasSchedulableAccount && selection.adapterMatchesCatalog {
		return 0
	}
	if selection.hasMapping && selection.hasSchedulableAccount {
		return 1
	}
	if selection.hasSchedulableAccount {
		return 2
	}
	if selection.hasMapping {
		return 3
	}
	return 4
}

func mobileVideoCatalogEntryMatchesAdapterPlatform(entry SiteModelCatalogEntry, adapterPlatform string) bool {
	entryPlatform := normalizeCatalogContractPlatform(entry.Platform)
	adapterPlatform = normalizeCatalogContractPlatform(adapterPlatform)
	return entryPlatform != "" && adapterPlatform != "" && entryPlatform == adapterPlatform
}

func mobileVideoCapabilitiesFromCatalog(raw []byte) (MobileVideoCapabilities, string, bool) {
	declaration, err := ParseCatalogMediaCapabilities(raw)
	if err != nil || declaration == nil || declaration.Video == nil || !catalogDeclaresVideoModality(declaration) {
		return MobileVideoCapabilities{}, "", false
	}
	capabilities := MobileVideoCapabilities{
		Operations:           cloneNonEmptyMobileVideoStrings(declaration.Video.Operations),
		SupportedResolutions: cloneNonEmptyMobileVideoStrings(declaration.Video.SupportedResolutions),
		SupportedRatios:      cloneNonEmptyMobileVideoStrings(declaration.Video.SupportedAspectRatios),
		SupportedDurations:   clonePositiveMobileVideoDurations(declaration.Video.DurationsSeconds),
		MaxReferenceAssets:   positiveMobileVideoLimit(declaration.Video.MaxReferenceAssets),
		MaxReferenceImages:   positiveMobileVideoLimit(declaration.Video.MaxReferenceImages),
		MaxReferenceVideos:   positiveMobileVideoLimit(declaration.Video.MaxReferenceVideos),
		MaxReferenceAudios:   positiveMobileVideoLimit(declaration.Video.MaxReferenceAudios),
		GenerateAudio:        declaration.Video.GenerateAudio,
		Watermark:            declaration.Video.Watermark,
	}
	if !capabilities.SupportsOperation("generate") || len(capabilities.SupportedResolutions) == 0 || len(capabilities.SupportedRatios) == 0 || len(capabilities.SupportedDurations) == 0 {
		return MobileVideoCapabilities{}, "", false
	}
	return capabilities, strings.TrimSpace(declaration.Adapter), true
}

func positiveMobileVideoLimit(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}

// The two currently registered mobile adapters accept text-to-video payloads
// only.  Do not advertise a catalog reference limit before the worker has a
// verified ownership/type resolver and a lossless upstream encoding for it.
// This keeps a future admin declaration from turning into an executable but
// unsupported request path.
func mobileVideoAdapterSupportsCapabilities(adapter string, capabilities MobileVideoCapabilities) bool {
	switch strings.TrimSpace(adapter) {
	case MobileVideoAdapterGrok:
		return capabilities.MaxReferenceAssets == 0 &&
			capabilities.MaxReferenceImages == 0 &&
			capabilities.MaxReferenceVideos == 0 &&
			capabilities.MaxReferenceAudios == 0
	case MobileVideoAdapterAgnes:
		return AgnesVideoMobileCapabilitiesSupported(capabilities)
	default:
		return false
	}
}

func catalogDeclaresVideoModality(declaration *CatalogMediaCapabilities) bool {
	for _, modality := range declaration.Modalities {
		if strings.EqualFold(strings.TrimSpace(modality), "video") {
			return true
		}
	}
	return false
}

func cloneNonEmptyMobileVideoStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func clonePositiveMobileVideoDurations(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 || value > 3_600 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func mobileVideoUnitPrices(group Group, model string, capabilities MobileVideoCapabilities) map[string]float64 {
	prices := make(map[string]float64, len(capabilities.SupportedResolutions))
	for _, resolution := range capabilities.SupportedResolutions {
		if price := group.GetVideoPriceForModel(model, resolution); price != nil {
			prices[resolution] = *price
		}
	}
	return prices
}

func mobileVideoPricedResolutions(declared []string, prices map[string]float64) []string {
	resolved := make([]string, 0, len(declared))
	for _, resolution := range declared {
		if _, priced := prices[resolution]; priced {
			resolved = append(resolved, resolution)
		}
	}
	return resolved
}

func mobileVideoSuppressionError(code string) error {
	switch code {
	case MobileVideoSuppressionNoSchedulableAccount:
		return &MobileVideoAvailabilityError{Code: code, cause: ErrMobileVideoCapabilityUnavailable}
	case MobileVideoSuppressionNotMapped:
		return &MobileVideoAvailabilityError{Code: code, cause: ErrMobileVideoModelUnavailable}
	case MobileVideoSuppressionCapabilityNotDeclared,
		MobileVideoSuppressionAdapterUnsupported,
		MobileVideoSuppressionPriceMissing,
		MobileVideoSuppressionGroupPermissionDenied,
		MobileVideoSuppressionSubscriptionReservationUnsupported:
		return &MobileVideoAvailabilityError{Code: code, cause: ErrMobileVideoCapabilityUnavailable}
	default:
		return ErrMobileVideoCapabilityUnavailable
	}
}

// MobileVideoAvailabilityError carries a stable diagnostic code while keeping
// callers able to use errors.Is with the existing public error contract.
type MobileVideoAvailabilityError struct {
	Code  string
	cause error
}

func (e *MobileVideoAvailabilityError) Error() string {
	if e == nil || e.cause == nil {
		return "mobile video capability unavailable"
	}
	return e.cause.Error()
}

func (e *MobileVideoAvailabilityError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func MobileVideoAvailabilityCode(err error) string {
	var availabilityErr *MobileVideoAvailabilityError
	if errors.As(err, &availabilityErr) {
		return availabilityErr.Code
	}
	return ""
}
